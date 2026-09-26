package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
)

func seeded(t *testing.T) *Store {
	t.Helper()
	s := New()
	readings := []catalog.Reading{
		{MeterCode: "M-1", Timestamp: at(1), ConsumptionKWh: 10},
		{MeterCode: "M-1", Timestamp: at(2), ConsumptionKWh: 20},
		{MeterCode: "M-2", Timestamp: at(1), ConsumptionKWh: 5},
	}
	events := []catalog.OperationalEvent{{MeterCode: "M-1", Timestamp: at(1), Type: catalog.EventTypeOperationalChange}}
	if err := s.Seed(
		[]catalog.Meter{{Code: "M-1", Name: "One"}, {Code: "M-2", Name: "Two"}},
		readings, events); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return s
}

func at(hour int) time.Time {
	return time.Date(2026, 9, 1, hour, 0, 0, 0, time.UTC)
}

func anomaly(kind analysis.AnomalyType, severity analysis.Severity, confidence float64) analysis.Anomaly {
	return analysis.Anomaly{
		MeterCode:   "M-1",
		WindowStart: at(1),
		WindowEnd:   at(2),
		Type:        kind,
		Severity:    severity,
		Confidence:  confidence,
		Reason:      "because",
	}
}

func TestStoreReturnsNotFoundRatherThanZeroValues(t *testing.T) {
	s := seeded(t)
	ctx := context.Background()

	if _, err := s.Meter(ctx, "M-NOPE"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Meter err = %v, want ErrNotFound", err)
	}
	if _, err := s.LatestRun(ctx); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("LatestRun err = %v, want ErrNotFound", err)
	}
	if _, err := s.Anomaly(ctx, 404); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Anomaly err = %v, want ErrNotFound", err)
	}
}

func TestStoreOrdersAnomaliesTheWayAnOperatorReadsThem(t *testing.T) {
	s := seeded(t)
	ctx := context.Background()
	runID, err := s.StartRun(ctx, at(1), at(2))
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	saved := []analysis.Anomaly{
		{MeterCode: "M-2", Type: analysis.AnomalyFalsePositive, Severity: analysis.SeverityLow, Confidence: 0.99},
		{MeterCode: "M-1", Type: analysis.AnomalyExplainable, Severity: analysis.SeverityMedium, Confidence: 0.5},
		{MeterCode: "M-1", Type: analysis.AnomalyDataQuality, Severity: analysis.SeverityHigh, Confidence: 0.7},
		{MeterCode: "M-2", Type: analysis.AnomalyReal, Severity: analysis.SeverityHigh, Confidence: 0.8},
	}
	if err := s.SaveAnomalies(ctx, runID, saved); err != nil {
		t.Fatalf("SaveAnomalies: %v", err)
	}
	if err := s.FinishRun(ctx, runID, store.RunCompleted, len(saved), ""); err != nil {
		t.Fatal(err)
	}

	anomalies, err := s.Anomalies(ctx)
	if err != nil {
		t.Fatalf("Anomalies: %v", err)
	}

	// A false positive with a higher confidence than a real anomaly must not
	// outrank it: severity and type come first (ADR-0010).
	var got []string
	for _, a := range anomalies {
		got = append(got, string(a.Type))
	}
	want := []string{"REAL_ANOMALY", "DATA_QUALITY", "EXPLAINABLE", "FALSE_POSITIVE"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestNarrationReplacesProseAndRecordsWhoWroteIt(t *testing.T) {
	s := seeded(t)
	ctx := context.Background()
	runID, _ := s.StartRun(ctx, at(1), at(2))
	if err := s.SaveAnomalies(ctx, runID, []analysis.Anomaly{anomaly(analysis.AnomalyReal, analysis.SeverityHigh, 0.9)}); err != nil {
		t.Fatalf("SaveAnomalies: %v", err)
	}
	s.FinishRun(ctx, runID, store.RunCompleted, 1, "")
	stored, _ := s.Anomalies(ctx)

	if err := s.Narrate(ctx, stored[0].ID, store.SourceLLM, "narrated", "act"); err != nil {
		t.Fatalf("Narrate: %v", err)
	}

	got, _ := s.Anomaly(ctx, stored[0].ID)
	if got.Reason != "narrated" || got.RecommendedAction != "act" {
		t.Errorf("prose = %q / %q, want the narrator's", got.Reason, got.RecommendedAction)
	}
	if got.ExplanationSource != store.SourceLLM || got.ExplanationStatus != store.ExplanationReady {
		t.Errorf("source/status = %q/%q, want llm/READY", got.ExplanationSource, got.ExplanationStatus)
	}
}

func TestFailedNarrationLeavesTheRulesProseInPlace(t *testing.T) {
	s := seeded(t)
	ctx := context.Background()
	runID, _ := s.StartRun(ctx, at(1), at(2))
	if err := s.SaveAnomalies(ctx, runID, []analysis.Anomaly{anomaly(analysis.AnomalyReal, analysis.SeverityHigh, 0.9)}); err != nil {
		t.Fatalf("SaveAnomalies: %v", err)
	}
	s.FinishRun(ctx, runID, store.RunCompleted, 1, "")
	stored, _ := s.Anomalies(ctx)

	if err := s.MarkNarrationFailed(ctx, stored[0].ID); err != nil {
		t.Fatalf("MarkNarrationFailed: %v", err)
	}

	got, _ := s.Anomaly(ctx, stored[0].ID)
	if got.Reason != "because" {
		t.Errorf("Reason = %q, want the rules prose to survive a failed narration", got.Reason)
	}
	if got.ExplanationSource != store.SourceRules {
		t.Errorf("ExplanationSource = %q, want %q", got.ExplanationSource, store.SourceRules)
	}
	if got.ExplanationStatus != store.ExplanationFailed {
		t.Errorf("ExplanationStatus = %q, want %q", got.ExplanationStatus, store.ExplanationFailed)
	}
}

func TestAnomaliesReturnsOnlyTheLatestRun(t *testing.T) {
	s := seeded(t)
	ctx := context.Background()

	first, _ := s.StartRun(ctx, at(1), at(2))
	s.SaveAnomalies(ctx, first, []analysis.Anomaly{anomaly(analysis.AnomalyReal, analysis.SeverityHigh, 0.9)})
	s.FinishRun(ctx, first, store.RunCompleted, 1, "")
	second, _ := s.StartRun(ctx, at(1), at(2))
	s.SaveAnomalies(ctx, second, []analysis.Anomaly{anomaly(analysis.AnomalyDataQuality, analysis.SeverityHigh, 0.9)})
	s.FinishRun(ctx, second, store.RunCompleted, 1, "")

	anomalies, err := s.Anomalies(ctx)
	if err != nil {
		t.Fatalf("Anomalies: %v", err)
	}
	if len(anomalies) != 1 || anomalies[0].Type != analysis.AnomalyDataQuality {
		t.Fatalf("got %+v, want only the second run's anomaly", anomalies)
	}
}

func TestConsumptionTotalsAndWindow(t *testing.T) {
	s := seeded(t)
	ctx := context.Background()

	totals, err := s.ConsumptionTotals(ctx)
	if err != nil {
		t.Fatalf("ConsumptionTotals: %v", err)
	}
	if totals["M-1"] != 30 || totals["M-2"] != 5 {
		t.Errorf("totals = %v, want M-1 30 and M-2 5", totals)
	}

	from, to, err := s.ReadingWindow(ctx)
	if err != nil {
		t.Fatalf("ReadingWindow: %v", err)
	}
	if !from.Equal(at(1)) || !to.Equal(at(2)) {
		t.Errorf("window = %v..%v, want %v..%v", from, to, at(1), at(2))
	}
}

func TestFailingStorePropagatesItsError(t *testing.T) {
	s := seeded(t)
	boom := errors.New("database is down")
	s.Fails = boom

	if _, err := s.Meters(context.Background()); !errors.Is(err, boom) {
		t.Errorf("err = %v, want the store's failure", err)
	}
}

func TestSeedingTwiceDoesNotDuplicateTheCatalogue(t *testing.T) {
	s := seeded(t)
	if err := s.Seed(
		[]catalog.Meter{{Code: "M-1", Name: "One renamed"}, {Code: "M-2", Name: "Two"}},
		s.currentReadings(), s.currentEvents()); err != nil {
		t.Fatalf("reseed: %v", err)
	}

	list, _ := s.Meters(context.Background())
	if len(list) != 2 {
		t.Fatalf("got %d meters, want 2", len(list))
	}
	if list[0].Name != "One renamed" {
		t.Errorf("name = %q, want the reseeded name", list[0].Name)
	}
}
