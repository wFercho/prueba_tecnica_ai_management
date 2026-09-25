// Package narrate is the boundary between the deterministic detector and a
// language model.
//
// The detector decides what happened. This package decides only how it is said.
// That separation is what makes the product safe to run with a model attached: a
// narrator can be wrong, slow or absent, and the worst it can do is produce worse
// words for a finding the detector has already committed to.
package narrate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

// Evidence is what crosses the boundary.
//
// It is the episode, not the meter: the readings inside the window, the baseline
// they were judged against, the reported event, and the four confidence terms.
// The meter's full history is deliberately absent — the baseline already encodes
// normal behaviour, so it would add tokens and prompt for nothing (ADR-0007).
type Evidence struct {
	Meter   catalog.Meter
	Anomaly analysis.Anomaly
	// Series is the episode's own hourly readings against the baseline.
	Series []analysis.Point
	// Event is the operational event the anomaly was read against, or nil.
	Event *analysis.EventSummary
	// Readings is how many readings the meter has in total, so a narrator can
	// say "16 of 336 readings" rather than implying a sixth of all data is bad.
	Readings int
}

// Narrative is what the narrator produced: prose, and who wrote it.
type Narrative struct {
	Reason string
	Action string
	Source catalog.ExplanationSource
}

// Narrator writes prose for a finding. One method, so the detector's output can
// be explained by a model, by the template, or by a test double without any of
// them knowing about the others.
type Narrator interface {
	Narrate(ctx context.Context, evidence Evidence) (Narrative, error)
}

// Verdict is a narrator's view of the finding itself. A narrator that returns
// this is allowed to confirm the detector and nothing else: the classification,
// the severity and the confidence are the detector's, and a narrator that
// disagrees is refused rather than obeyed (ADR-0001).
type Verdict struct {
	Type     analysis.AnomalyType `json:"type"`
	Severity analysis.Severity    `json:"severity"`
	// Confirms says the evidence in the episode supports the finding. A narrator
	// that cannot confirm is one the caller should hear about, not persist.
	Confirms bool   `json:"confirms"`
	Reason   string `json:"reason"`
	Action   string `json:"action"`
}

// Rules is the narrator that always works: it returns the deterministic template
// the detector already wrote. It is the fallback when no model is configured, and
// the behaviour every other narrator must match when the model is unavailable.
type Rules struct{}

// Narrate returns the deterministic prose unchanged, credited to the rules.
func (Rules) Narrate(_ context.Context, evidence Evidence) (Narrative, error) {
	return Narrative{
		Reason: evidence.Anomaly.Reason,
		Action: evidence.Anomaly.RecommendedAction,
		Source: catalog.SourceRules,
	}, nil
}

// Failing is a narrator that always fails. It exists so the degraded path can be
// tested rather than assumed: an LLM outage is the failure mode ADR-0006 promises
// the product survives, and the only honest way to test that promise is to
// produce the outage.
type Failing struct{ Err error }

// Narrate fails with Err.
func (f Failing) Narrate(context.Context, Evidence) (Narrative, error) {
	return Narrative{}, f.Err
}

// validate refuses a verdict that contradicts the detector.
func validate(verdict Verdict, anomaly analysis.Anomaly) error {
	if !verdict.Confirms {
		return fmt.Errorf("the narrator could not confirm a %s anomaly at confidence %.2f: %s",
			anomaly.Type, anomaly.Confidence, verdict.Reason)
	}
	if verdict.Type != anomaly.Type {
		return fmt.Errorf("the narrator called a %s anomaly a %s: the classification is the detector's, not the model's",
			anomaly.Type, verdict.Type)
	}
	if verdict.Severity != anomaly.Severity {
		return fmt.Errorf("the narrator called a %s anomaly %s: severity is the detector's, not the model's",
			anomaly.Severity, verdict.Severity)
	}
	return nil
}

// checkProse rejects output that is empty, too short to be an explanation, or
// missing its full stop. A narrator that returns three words is not narrating, and
// persisting "Up." over a well-reasoned paragraph would be a regression in the
// product.
//
// The length floor applies to the reason only. A recommended action is an
// instruction — "No escalar." is a complete and correct one — and holding it to an
// explanation's length would push the narrator towards padding.
func checkProse(reason, action string) error {
	if strings.TrimSpace(reason) == "" {
		return errors.New("the narrator returned an empty reason")
	}
	if utf8.RuneCountInString(strings.TrimSpace(reason)) < minProseRunes {
		return fmt.Errorf("the narrator returned %d characters of reason, which is not an explanation",
			utf8.RuneCountInString(strings.TrimSpace(reason)))
	}
	if !strings.HasSuffix(strings.TrimSpace(reason), ".") {
		return fmt.Errorf("the narrator's reason does not end in a full stop: %q", reason)
	}
	if strings.TrimSpace(action) == "" {
		return errors.New("the narrator returned an empty recommended action")
	}
	if !strings.HasSuffix(strings.TrimSpace(action), ".") {
		return fmt.Errorf("the narrator's recommended action does not end in a full stop: %q", action)
	}
	return nil
}

// minProseRunes is the shortest acceptable explanation. Below this a narrator is
// summarising rather than explaining.
const minProseRunes = 40

// sanitise strips anything that would be markup when rendered, and normalises
// the punctuation. Model output is untrusted text: it is rendered next to
// operational data, and it must never be able to inject a tag.
func sanitise(narrative Narrative) (Narrative, error) {
	reason := neutraliseMarkup(narrative.Reason)
	action := neutraliseMarkup(narrative.Action)
	if !strings.HasSuffix(reason, ".") {
		reason += "."
	}
	if !strings.HasSuffix(action, ".") {
		action += "."
	}
	if err := checkProse(reason, action); err != nil {
		return Narrative{}, err
	}
	return Narrative{Reason: reason, Action: action, Source: narrative.Source}, nil
}

// neutraliseMarkup removes angle-bracketed tags and the characters that let a
// reader's browser treat text as markup.
func neutraliseMarkup(text string) string {
	var out strings.Builder
	insideTag := false
	for _, r := range strings.TrimSpace(text) {
		switch {
		case r == '<':
			insideTag = true
		case r == '>':
			insideTag = false
		case insideTag:
			// Dropped: the tag's contents are markup, not prose.
		default:
			out.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(out.String()), " ")
}
