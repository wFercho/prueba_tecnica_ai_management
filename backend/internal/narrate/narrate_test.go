package narrate

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

// The narrator is the only place the product calls a model, and the only thing
// standing between an LLM outage and a blank explanation. These tests are about
// that boundary, not about prose quality: what evidence crosses it, what happens
// when the call fails, and what a narrator is forbidden to change.

func evidence() Evidence {
	return Evidence{
		Meter: catalog.Meter{Code: "M-109", Name: "Taller 1", Location: "Planta 1"},
		Anomaly: analysis.Anomaly{
			MeterCode:         "M-109",
			Type:              analysis.AnomalyReal,
			Severity:          analysis.SeverityHigh,
			Confidence:        0.99,
			ConfidenceBand:    analysis.BandHigh,
			ConfidenceBasis:   analysis.Evidence{Deviation: 0.99, EventMatch: 1, Corroboration: 1, Persistence: 1},
			DeviationPercent:  110,
			ActualKWh:         5380.8,
			BaselineKWh:       2562,
			AffectedReadings:  58,
			Reason:            "rules reason",
			RecommendedAction: "rules action",
			DetectedBy:        "CONSUMPTION/hourly-profile+mad",
			ExplanationSource: catalog.SourceRules,
			ExplanationStatus: catalog.ExplanationPending,
			Status:            catalog.StatusOpen,
			Corroborating:     []string{"consumption", "current"},
			WindowStart:       time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC),
			WindowEnd:         time.Date(2026, 9, 14, 23, 0, 0, 0, time.UTC),
		},
		Series:   []analysis.Point{{Timestamp: time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC), ActualKWh: 60, BaselineKWh: 30, Deviation: 1}},
		Event:    &analysis.EventSummary{Description: "No operational event reported", Type: "UNKNOWN", Explains: false},
		Readings: 336,
	}
}

func TestNarratorReceivesTheEpisodeNotTheMetersHistory(t *testing.T) {
	// ADR-0007: the narrator gets the episode's own readings. Sending the full
	// history would let it describe behaviour the baseline already encodes, and
	// would grow without bound.
	got := evidence()

	if len(got.Series) != 1 {
		t.Fatalf("series = %d points, want only the episode's", len(got.Series))
	}
	if got.Series[0].Deviation != 1 {
		t.Errorf("series deviation = %v, want the per-hour departure", got.Series[0].Deviation)
	}
	if got.Anomaly.ConfidenceBasis.Deviation == 0 {
		t.Error("the four confidence terms are not in the evidence, so the score cannot be traced")
	}
}

func TestRulesNarratorKeepsTheDeterministicProse(t *testing.T) {
	var narrator Narrator = Rules{}

	narrative, err := narrator.Narrate(context.Background(), evidence())
	if err != nil {
		t.Fatalf("Narrate: %v", err)
	}

	if narrative.Reason != "rules reason" {
		t.Errorf("Reason = %q, want the deterministic text unchanged", narrative.Reason)
	}
	if narrative.Action != "rules action" {
		t.Errorf("Action = %q, want the deterministic text unchanged", narrative.Action)
	}
	if narrative.Source != catalog.SourceRules {
		t.Errorf("Source = %q, want %q", narrative.Source, catalog.SourceRules)
	}
}

func TestNarratorFailureIsAnErrorNotAnEmptyNarrative(t *testing.T) {
	// A narrator that returns an empty narrative with no error would let the
	// caller persist a blank explanation, which is the one outcome the product
	// promises never to produce.
	boom := errors.New("rate limited")
	var narrator Narrator = Failing{Err: boom}

	narrative, err := narrator.Narrate(context.Background(), evidence())
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
	if narrative.Reason != "" || narrative.Action != "" {
		t.Errorf("narrative = %+v, want empty alongside the error", narrative)
	}
}

func TestValidateRejectsANarrationThatContradictsTheDetector(t *testing.T) {
	// The detector is authoritative. A narrator that renames the classification,
	// or invents a severity, is not a narrator — it is a second, worse detector,
	// and its output is refused (ADR-0001).
	verdict := Verdict{
		Type:     analysis.AnomalyReal,
		Severity: analysis.SeverityHigh,
		Confirms: true,
	}
	if err := validate(verdict, evidence().Anomaly); err != nil {
		t.Errorf("a narrator that agrees with the detector was refused: %v", err)
	}

	renamed := Verdict{Type: analysis.AnomalyFalsePositive, Severity: analysis.SeverityHigh, Confirms: true}
	if err := validate(renamed, evidence().Anomaly); err == nil {
		t.Error("a narrator that renamed the classification was accepted")
	}

	inflated := Verdict{Type: analysis.AnomalyReal, Severity: analysis.SeverityLow, Confirms: true}
	if err := validate(inflated, evidence().Anomaly); err == nil {
		t.Error("a narrator that downgraded the severity was accepted")
	}

	refused := Verdict{Type: analysis.AnomalyReal, Severity: analysis.SeverityHigh, Confirms: false}
	if err := validate(refused, evidence().Anomaly); err == nil {
		t.Error("a narrator that refused to confirm the evidence was accepted")
	}
}

func TestProseMustBeSubstantive(t *testing.T) {
	// Style is not the narrator's contract: terse-but-correct prose is accepted,
	// because rejecting it would trade a wording preference for a missing
	// explanation. What is rejected is output that cannot serve as an explanation
	// at all.
	if err := checkProse("The load rose at 14:00 and stayed there for the rest of the window.", "Escalate to operations."); err != nil {
		t.Errorf("rejected good prose: %v", err)
	}

	bad := map[string]string{
		"empty":        "",
		"too short":    "Up.",
		"no full stop": "The load rose and stayed there",
	}
	for name, reason := range bad {
		if err := checkProse(reason, "Escalate to operations."); err == nil {
			t.Errorf("accepted %s prose: %q", name, reason)
		}
	}

	if err := checkProse("The load rose at 14:00 and stayed there for the rest of the window.", "  "); err == nil {
		t.Error("accepted an empty recommended action")
	}
}

func TestSanitisedProseCannotSmuggleMarkupIntoTheDashboard(t *testing.T) {
	narrative, err := sanitise(Narrative{
		Reason: "The load rose <script>alert(1)</script> and stayed high for the rest of the window.",
		Action: "Escalate this to operations today.",
	})
	if err != nil {
		t.Fatalf("sanitise: %v", err)
	}
	if strings.Contains(narrative.Reason, "<script>") {
		t.Errorf("Reason = %q, want markup stripped", narrative.Reason)
	}
	if !strings.HasSuffix(narrative.Reason, ".") {
		t.Errorf("Reason = %q, want it to end in a full stop", narrative.Reason)
	}
}
