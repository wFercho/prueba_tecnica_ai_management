package narrate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

// reply builds a chat completion response carrying content.
func reply(t *testing.T, content string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"choices": []any{map[string]any{
			"message": map[string]string{"content": content},
		}},
	})
	if err != nil {
		t.Fatalf("encode reply: %v", err)
	}
	return string(payload)
}

func server(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	return s
}

func narratorFor(baseURL string) OpenAI {
	return OpenAI{APIKey: "test-key", BaseURL: baseURL, Model: "test-model"}
}

func TestOpenAINarratesAConfirmingModel(t *testing.T) {
	s := server(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want the key as a bearer token", got)
		}
		w.Write([]byte(reply(t, `{"type":"REAL_ANOMALY","severity":"HIGH","confirms":true,
			"reason":"El consumo subió a las 14:00 del 12 de septiembre y permaneció el doble de su línea base durante 58 horas; también aumentó la corriente.",
			"action":"Preguntar a operaciones si el cambio de carga estaba previsto."}`)))
	})

	narrative, err := narratorFor(s.URL).Narrate(context.Background(), evidence())
	if err != nil {
		t.Fatalf("Narrate: %v", err)
	}

	if narrative.Source != catalog.SourceLLM {
		t.Errorf("Source = %q, want %q", narrative.Source, catalog.SourceLLM)
	}
	if !strings.Contains(narrative.Reason, "58 horas") {
		t.Errorf("Reason = %q, want the model's own words", narrative.Reason)
	}
	if !strings.HasSuffix(narrative.Action, ".") {
		t.Errorf("Action = %q, want a full stop", narrative.Action)
	}
}

func TestOpenAIRejectsAModelThatReclassifiesTheFinding(t *testing.T) {
	// The detector is authoritative. A model that turns a real anomaly into a
	// false positive is not narrating; it is re-deciding, and its answer is
	// refused so the deterministic explanation stands.
	s := server(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(reply(t, `{"type":"FALSE_POSITIVE","severity":"HIGH","confirms":true,
			"reason":"This looks like a scheduled outage to me, honestly, probably some maintenance work.",
			"action":"Do nothing about it at all."}`)))
	})

	_, err := narratorFor(s.URL).Narrate(context.Background(), evidence())
	if err == nil {
		t.Fatal("got no error, want the reclassification refused")
	}
	if !strings.Contains(err.Error(), "classification is the detector's") {
		t.Errorf("err = %v, want it to say the classification is not the model's to change", err)
	}
}

func TestOpenAIRejectsAModelThatCannotConfirm(t *testing.T) {
	s := server(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(reply(t, `{"type":"REAL_ANOMALY","severity":"HIGH","confirms":false,
			"reason":"I am not sure this is real, the numbers could be explained many different ways honestly.",
			"action":"Maybe look at it sometime."}`)))
	})

	if _, err := narratorFor(s.URL).Narrate(context.Background(), evidence()); err == nil {
		t.Fatal("got no error, want an unconfirmable narration refused")
	}
}

func TestOpenAIRejectsUnusableProse(t *testing.T) {
	cases := map[string]string{
		"empty":    `{"type":"REAL_ANOMALY","severity":"HIGH","confirms":true,"reason":"","action":"Do something."}`,
		"terse":    `{"type":"REAL_ANOMALY","severity":"HIGH","confirms":true,"reason":"It went up.","action":"Look."}`,
		"not json": `I think the load probably went up for some reason, hard to say honestly.`,
	}
	for name, content := range cases {
		s := server(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Write([]byte(reply(t, content)))
		})
		if _, err := narratorFor(s.URL).Narrate(context.Background(), evidence()); err == nil {
			t.Errorf("accepted %s output: %s", name, content)
		}
	}
}

func TestOpenAIStripsMarkupButKeepsTheExplanation(t *testing.T) {
	// Markup is removed and the explanation survives: the model is untrusted, but
	// its useful output is worth keeping.
	s := server(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(reply(t, `{"type":"REAL_ANOMALY","severity":"HIGH","confirms":true,
			"reason":"<b>El consumo</b> subió a las 14:00 del 12 de septiembre y siguió por encima de la línea base durante 58 horas.",
			"action":"Consultar con operaciones la causa del cambio."}`)))
	})

	narrative, err := narratorFor(s.URL).Narrate(context.Background(), evidence())
	if err != nil {
		t.Fatalf("Narrate: %v", err)
	}
	if strings.ContainsAny(narrative.Reason, "<>") {
		t.Errorf("Reason = %q, want no markup", narrative.Reason)
	}
	if !strings.Contains(narrative.Reason, "58 horas") {
		t.Errorf("Reason = %q, want the explanation kept", narrative.Reason)
	}
}

func TestOpenAISendsTheEpisodeAndNotTheHistory(t *testing.T) {
	var sent map[string]any
	s := server(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Write([]byte(reply(t, `{"type":"REAL_ANOMALY","severity":"HIGH","confirms":true,
			"reason":"El consumo aumentó a las 14:00 del 12 de septiembre y permaneció por encima de la línea base durante 58 horas.",
			"action":"Preguntar al equipo de operaciones por el cambio."}`)))
	})

	if _, err := narratorFor(s.URL).Narrate(context.Background(), evidence()); err != nil {
		t.Fatalf("Narrate: %v", err)
	}

	messages, _ := sent["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want a system prompt and the evidence", len(messages))
	}
	user, _ := messages[1].(map[string]any)
	// The user turn is already a JSON string, so it is compared as one rather
	// than re-encoded.
	userTurn, _ := user["content"].(string)
	for _, want := range []string{`"type":"REAL_ANOMALY"`, `"severity":"HIGH"`, `"hourly_readings"`, `"reported_event"`, `"confidence_terms"`, `"quality_findings"`, `"voltage_v"`, `"current_a"`, `"power_factor"`, `"deviation"`} {
		if !strings.Contains(userTurn, want) {
			t.Errorf("the evidence sent omits %s: %s", want, userTurn)
		}
	}
	// The meter's history is the thing ADR-0007 refuses to send; if the payload
	// ever grows a full series, this is the assertion that catches it.
	if got := strings.Count(userTurn, `"hour":`); got != 1 {
		t.Errorf("the payload carries %d readings, want only the episode's own", got)
	}
}

func TestOpenAIRejectsEnglishProse(t *testing.T) {
	for _, content := range []string{
		`{"type":"REAL_ANOMALY","severity":"HIGH","confirms":true,
			"reason":"The load rose at 14:00 and stayed at twice its normal level for 58 hours.",
			"action":"Ask operations about the load."}`,
		`{"type":"REAL_ANOMALY","severity":"HIGH","confirms":true,
			"reason":"El consumo subió a las 14:00 y se mantuvo por encima de su línea base durante 58 horas.",
			"action":"Ask operations about the load."}`,
	} {
		s := server(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Write([]byte(reply(t, content)))
		})
		if _, err := narratorFor(s.URL).Narrate(t.Context(), evidence()); err == nil {
			t.Fatalf("English narration replaced the Spanish rules explanation: %s", content)
		}
	}
}

func TestOpenAIReportsAModelOutageAsAnError(t *testing.T) {
	s := server(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	if _, err := narratorFor(s.URL).Narrate(context.Background(), evidence()); err == nil {
		t.Fatal("got no error, want a 429 to surface as an error so the rules prose survives")
	}

	var unreachable OpenAI
	unreachable.APIKey = "k"
	unreachable.BaseURL = "http://127.0.0.1:1"
	if _, err := unreachable.Narrate(context.Background(), evidence()); err == nil {
		t.Fatal("got no error, want an unreachable model to surface as an error")
	}
}

func TestOpenAIWithoutAKeyDoesNotCallOut(t *testing.T) {
	if _, err := (OpenAI{}).Narrate(context.Background(), evidence()); err == nil {
		t.Fatal("got no error, want a narrator with no key to fail fast rather than call out")
	}
}

func TestDecodeVerdictToleratesAFencedReply(t *testing.T) {
	verdict, err := decodeVerdict("```json\n{\"type\":\"REAL_ANOMALY\",\"severity\":\"HIGH\",\"confirms\":true,\"reason\":\"x\",\"action\":\"y\"}\n```")
	if err != nil {
		t.Fatalf("decodeVerdict: %v", err)
	}
	if verdict.Type != analysis.AnomalyReal {
		t.Errorf("Type = %q, want %q", verdict.Type, analysis.AnomalyReal)
	}
}
