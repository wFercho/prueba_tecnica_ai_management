package narrate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

// OpenAI narrates through a chat completions API.
//
// It asks for a verdict rather than prose alone, and that is the point: the
// model has to restate the detector's own classification before it is trusted
// with words. A model that cannot confirm what the detector found has produced
// something the product should not persist, however fluent it reads.
type OpenAI struct {
	// APIKey and BaseURL are the credentials and endpoint. BaseURL defaults to
	// the public API when empty, so a caller that only sets a key works.
	APIKey  string
	BaseURL string
	Model   string
	// HTTPClient is the client used for the call. A nil client gets one with a
	// timeout, because a narrator must never be able to hold a run open
	// indefinitely: the run is already complete by the time narration starts.
	HTTPClient *http.Client
	// MaxTokens bounds the response. An explanation of a 58-hour episode does
	// not need a page, and an unbounded completion is an unbounded bill.
	MaxTokens int
}

// DefaultBaseURL is the OpenAI chat completions endpoint.
const DefaultBaseURL = "https://api.openai.com/v1"

// defaultTimeout is how long a narration may take before the rules prose is
// declared the answer.
const defaultTimeout = 20 * time.Second

// prompt is the system instruction. It states the constraint the code enforces,
// because a model told nothing will eventually try to reclassify a finding.
const prompt = `You explain energy anomalies to an operations engineer.

You are given a finding that a deterministic detector has already made. Your job
is to explain it in plain language, not to judge it.

Rules you must follow:
- You may not change the type or the severity. They are given to you and are final.
- Use only the evidence provided. Every number you state must come from the
  evidence; do not estimate, extrapolate or add context you were not given.
- If the evidence does not identify a cause, say that nothing on record accounts
  for the change. Do not speculate about what probably caused it.
- Name the hour the change began and how long it lasted, taken from the series.
- Write in plain sentences. No bullet points, no headings, no markdown.
- The reason must be at least one full sentence and end with a full stop. The
  recommended action must be one instruction, naming who should do what, and end
  with a full stop.

Reply with JSON only, matching this shape:
{"type": "<type from the evidence>", "severity": "<severity from the evidence>",
 "confirms": true, "reason": "<explanation>", "action": "<recommended action>"}`

// Narrate calls the API and returns its prose, if the model both confirms the
// finding and produces something worth persisting.
func (o OpenAI) Narrate(ctx context.Context, evidence Evidence) (Narrative, error) {
	if o.APIKey == "" {
		return Narrative{}, fmt.Errorf("narrate: no API key configured")
	}
	body, err := o.request(evidence)
	if err != nil {
		return Narrative{}, err
	}

	httpClient := o.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL()+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Narrative{}, fmt.Errorf("narrate: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return Narrative{}, fmt.Errorf("narrate: call model: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return Narrative{}, fmt.Errorf("narrate: model returned %s", resp.Status)
	}
	return o.parse(resp.Body, evidence)
}

func (o OpenAI) baseURL() string {
	if o.BaseURL != "" {
		return strings.TrimRight(o.BaseURL, "/")
	}
	return DefaultBaseURL
}

func (o OpenAI) maxTokens() int {
	if o.MaxTokens > 0 {
		return o.MaxTokens
	}
	return 400
}

func (o OpenAI) model() string {
	if o.Model != "" {
		return o.Model
	}
	return "gpt-4o-mini"
}

// request builds the chat completion payload. The evidence is rendered as compact
// JSON rather than prose so the model reads the numbers as data, and so the shape
// of what it is given is visible in the payload.
func (o OpenAI) request(evidence Evidence) ([]byte, error) {
	anomaly := evidence.Anomaly
	correlation := "none reported"
	if evidence.Event != nil {
		// A zero timestamp is formatted as year one, which would tell the model a
		// cause was reported in 0001. Leave the time out rather than print a lie.
		correlation = evidence.Event.Description
		if !evidence.Event.Timestamp.IsZero() {
			correlation = fmt.Sprintf("%s at %s", correlation, evidence.Event.Timestamp.Format(time.RFC3339))
		}
		correlation = fmt.Sprintf("%s (%s), explains=%t", correlation, evidence.Event.Type, evidence.Event.Explains)
	}
	points := make([]map[string]any, 0, len(evidence.Series))
	for _, point := range evidence.Series {
		points = append(points, map[string]any{
			"hour":     point.Timestamp.Format(time.RFC3339),
			"actual":   point.ActualKWh,
			"baseline": point.BaselineKWh,
		})
	}

	userEvidence := map[string]any{
		"meter":              evidence.Meter.Name,
		"code":               string(evidence.Meter.Code),
		"type":               string(anomaly.Type),
		"severity":           string(anomaly.Severity),
		"window_start":       anomaly.WindowStart.Format(time.RFC3339),
		"window_end":         anomaly.WindowEnd.Format(time.RFC3339),
		"readings_in_window": anomaly.AffectedReadings,
		"readings_total":     evidence.Readings,
		"deviation_percent":  anomaly.DeviationPercent,
		"actual_kwh":         anomaly.ActualKWh,
		"baseline_kwh":       anomaly.BaselineKWh,
		"confidence":         anomaly.Confidence,
		"confidence_terms": map[string]float64{
			"deviation":     anomaly.ConfidenceBasis.Deviation,
			"event_match":   anomaly.ConfidenceBasis.EventMatch,
			"corroboration": anomaly.ConfidenceBasis.Corroboration,
			"persistence":   anomaly.ConfidenceBasis.Persistence,
		},
		"corroborating":   anomaly.Corroborating,
		"reported_event":  correlation,
		"hourly_readings": points,
		"detector_said":   anomaly.Reason,
	}

	userPayload, err := json.Marshal(userEvidence)
	if err != nil {
		return nil, fmt.Errorf("narrate: encode evidence: %w", err)
	}
	payload, err := json.Marshal(map[string]any{
		"model":           o.model(),
		"temperature":     0.2,
		"max_tokens":      o.maxTokens(),
		"response_format": map[string]any{"type": "json_object"},
		"messages": []map[string]string{
			{"role": "system", "content": prompt},
			{"role": "user", "content": string(userPayload)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("narrate: encode request: %w", err)
	}
	return payload, nil
}

// completion is the part of the API response this package reads.
type completion struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// parse reads the model's reply and checks it before returning it.
func (o OpenAI) parse(body io.Reader, evidence Evidence) (Narrative, error) {
	var reply completion
	if err := json.NewDecoder(body).Decode(&reply); err != nil {
		return Narrative{}, fmt.Errorf("narrate: decode response: %w", err)
	}
	if len(reply.Choices) == 0 {
		return Narrative{}, fmt.Errorf("narrate: the model returned no choices")
	}

	verdict, err := decodeVerdict(reply.Choices[0].Message.Content)
	if err != nil {
		return Narrative{}, err
	}
	if err := validate(verdict, evidence.Anomaly); err != nil {
		return Narrative{}, err
	}
	return sanitise(Narrative{
		Reason: verdict.Reason,
		Action: verdict.Action,
		Source: catalog.SourceLLM,
	})
}

// decodeVerdict reads the model's JSON answer, tolerating a fence around it,
// which models add even when told not to.
func decodeVerdict(content string) (Verdict, error) {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end < start {
		return Verdict{}, fmt.Errorf("narrate: the model's reply was not JSON: %q", truncate(content))
	}

	var verdict Verdict
	if err := json.Unmarshal([]byte(trimmed[start:end+1]), &verdict); err != nil {
		return Verdict{}, fmt.Errorf("narrate: decode verdict: %w", err)
	}
	return verdict, nil
}

func truncate(text string) string {
	if len(text) <= 120 {
		return text
	}
	return text[:120] + "…"
}
