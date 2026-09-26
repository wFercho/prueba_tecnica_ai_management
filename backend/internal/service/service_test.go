package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/narrate"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store/memory"
)

// These tests are about the promises the run flow makes, not about the detection
// itself, which the analysis package already proves against the delivered data:
// every anomaly is persisted with prose before the caller hears about the run,
// narration never blocks the caller, and a narrator that fails degrades the
// wording and nothing else (ADR-0006).

// scriptedNarrator answers with a fixed narrative, and can be made to fail or to
// block so the degraded paths are exercised rather than assumed.
type scriptedNarrator struct {
	mu       sync.Mutex
	calls    int
	err      error
	gate     chan struct{}
	narrated map[int64]bool
}

func (s *scriptedNarrator) Narrate(ctx context.Context, _ narrate.Evidence) (narrate.Narrative, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()

	if s.gate != nil {
		select {
		case <-s.gate:
		case <-ctx.Done():
			return narrate.Narrative{}, ctx.Err()
		}
	}
	if s.err != nil {
		return narrate.Narrative{}, s.err
	}
	return narrate.Narrative{
		Reason: "El consumo aumentó a las 14:00 y se mantuvo por encima de su línea base durante el episodio.",
		Action: "Consultar con operaciones si el cambio de carga estaba previsto.",
		Source: catalog.SourceLLM,
	}, nil
}

func (s *scriptedNarrator) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func deliveredStore(t *testing.T) *memory.Store {
	t.Helper()
	s := memory.New()
	readings := make([]catalog.Reading, 0, 12*14*24)
	// A dataset small enough for a unit test and shaped like the delivered one:
	// one meter that doubles its load and one that goes quiet.
	for day := range 14 {
		for hour := range 24 {
			base := catalog.Reading{
				MeterCode:      "M-1",
				Timestamp:      time.Date(2026, 9, 1, hour, 0, 0, 0, time.UTC).AddDate(0, 0, day),
				ConsumptionKWh: 30,
				VoltageV:       220,
				CurrentA:       100,
				PowerFactor:    0.95,
				IngestedStatus: "OK",
			}
			if day >= 10 {
				base.ConsumptionKWh = 60
				base.CurrentA = 200
			}
			readings = append(readings, base)
		}
	}
	if err := s.Seed([]catalog.Meter{{Code: "M-1", Name: "Meter one"}}, readings, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return s
}

func newService(t *testing.T, narrator narrate.Narrator) (*Service, *memory.Store) {
	t.Helper()
	fake := deliveredStore(t)
	return New(fake, narrator, analysis.DefaultDetectorConfig()), fake
}

func TestAnalyzePersistsEveryAnomalyWithProseBeforeReturning(t *testing.T) {
	// ADR-0006: the deterministic explanation is written first, always. If this
	// ever fails, an anomaly can reach the dashboard with nothing to show.
	gate := make(chan struct{})
	service, fake := newService(t, &scriptedNarrator{gate: gate})
	t.Cleanup(func() { close(gate); service.WaitForNarration() })

	run, err := service.Analyze(context.Background())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if run.State != store.RunCompleted {
		t.Fatalf("run state = %q, want %q: a run is complete when detection is, not when narration is",
			run.State, store.RunCompleted)
	}

	stored, err := fake.Anomalies(context.Background())
	if err != nil {
		t.Fatalf("Anomalies: %v", err)
	}
	if len(stored) == 0 {
		t.Fatal("the run found nothing, so this test proves nothing")
	}
	for _, anomaly := range stored {
		if anomaly.Reason == "" || anomaly.RecommendedAction == "" {
			t.Errorf("anomaly %d persisted with no prose", anomaly.ID)
		}
		if anomaly.ExplanationSource != catalog.SourceRules {
			t.Errorf("anomaly %d source = %q, want rules at this point", anomaly.ID, anomaly.ExplanationSource)
		}
	}
}

func TestRunCountsWhatItFound(t *testing.T) {
	service, fake := newService(t, &scriptedNarrator{})

	run, err := service.Analyze(context.Background())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	stored, _ := fake.Anomalies(context.Background())

	if run.AnomalyCount != len(stored) {
		t.Errorf("run reports %d anomalies, the store holds %d", run.AnomalyCount, len(stored))
	}
	if run.FinishedAt == nil {
		t.Error("a completed run has no finish time")
	}
	if run.WindowStart.IsZero() || run.WindowEnd.IsZero() {
		t.Error("the run does not record the window it analysed")
	}
}

func TestRulesOnlyRunHasReadySpanishProseBeforeTheResponse(t *testing.T) {
	svc, data := newService(t, nil)
	run, err := svc.Analyze(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	progress, err := svc.Run(t.Context(), run.ID)
	if err != nil || progress.Narrating != 0 || progress.Explained != run.AnomalyCount {
		t.Fatalf("rules progress = %+v, %v", progress, err)
	}
	findings, err := data.Anomalies(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range findings {
		if finding.ExplanationSource != catalog.SourceRules || finding.ExplanationStatus != catalog.ExplanationReady ||
			!strings.Contains(finding.Reason, "línea base") || finding.RecommendedAction == "" {
			t.Errorf("rules prose not ready and Spanish at response time: %+v", finding)
		}
	}
}

func TestAnalyzeDoesNotWaitForTheNarrator(t *testing.T) {
	// A narrator that never answers must not hold the run. The run is already
	// complete and durable; the words can arrive whenever they arrive.
	gate := make(chan struct{})
	t.Cleanup(func() { close(gate) })
	service, _ := newService(t, &scriptedNarrator{gate: gate})

	done := make(chan error, 1)
	go func() {
		_, err := service.Analyze(context.Background())
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Analyze: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Analyze waited for the narrator")
	}
}

func TestNarrationUpgradesEachAnomalyOnceItAnswers(t *testing.T) {
	gate := make(chan struct{})
	narrator := &scriptedNarrator{gate: gate}
	service, fake := newService(t, narrator)

	if _, err := service.Analyze(context.Background()); err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	close(gate)
	service.WaitForNarration()

	stored, _ := fake.Anomalies(context.Background())
	if len(stored) == 0 {
		t.Fatal("nothing to narrate")
	}
	for _, anomaly := range stored {
		if anomaly.ExplanationSource != catalog.SourceLLM {
			t.Errorf("anomaly %d source = %q, want llm", anomaly.ID, anomaly.ExplanationSource)
		}
		if anomaly.ExplanationStatus != catalog.ExplanationReady {
			t.Errorf("anomaly %d status = %q, want READY", anomaly.ID, anomaly.ExplanationStatus)
		}
		if !strings.Contains(anomaly.Reason, "se mantuvo por encima de su línea base") {
			t.Errorf("anomaly %d kept the template: %q", anomaly.ID, anomaly.Reason)
		}
	}
	if narrator.callCount() != len(stored) {
		t.Errorf("the narrator was called %d times for %d anomalies", narrator.callCount(), len(stored))
	}
}

func TestAFailedNarrationLeavesTheRulesProseAndSaysSo(t *testing.T) {
	// The degraded path is the one ADR-0006 exists for, so it is tested rather
	// than assumed: a narrator that always fails must leave every row explained
	// and visibly marked as not yet narrated.
	service, fake := newService(t, narrate.Failing{Err: errors.New("model unreachable")})

	if _, err := service.Analyze(context.Background()); err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	service.WaitForNarration()

	stored, _ := fake.Anomalies(context.Background())
	if len(stored) == 0 {
		t.Fatal("nothing to narrate")
	}
	for _, anomaly := range stored {
		if anomaly.Reason == "" {
			t.Errorf("anomaly %d lost its explanation to a failed narration", anomaly.ID)
		}
		if anomaly.ExplanationSource != catalog.SourceRules {
			t.Errorf("anomaly %d source = %q, want rules to survive", anomaly.ID, anomaly.ExplanationSource)
		}
		if anomaly.ExplanationStatus != catalog.ExplanationFailed {
			t.Errorf("anomaly %d status = %q, want FAILED so the failure is visible",
				anomaly.ID, anomaly.ExplanationStatus)
		}
	}
}

// unwritableStore fails the anomaly write and nothing else, so the run reaches the
// point of saving and the test exercises the path that has to record the failure.
type unwritableStore struct {
	store.Store
	err error
}

func (u unwritableStore) SaveAnomalies(context.Context, int64, []analysis.Anomaly) error {
	return u.err
}

func TestAFailedRunIsRecordedAsFailedRatherThanLeftRunning(t *testing.T) {
	fake := deliveredStore(t)
	service := New(unwritableStore{Store: fake, err: errors.New("disk full")},
		&scriptedNarrator{}, analysis.DefaultDetectorConfig())

	if _, err := service.Analyze(context.Background()); err == nil {
		t.Fatal("got no error, want the store's failure to surface")
	}
	run, err := fake.LatestRun(context.Background())
	if err != nil {
		t.Fatalf("LatestRun: %v", err)
	}
	if run.State != store.RunFailed {
		t.Errorf("run state = %q, want FAILED rather than left RUNNING forever", run.State)
	}
	if run.Error == "" {
		t.Error("a failed run records no reason")
	}
	if run.AnomalyCount != 0 {
		t.Errorf("a run that saved nothing reports %d anomalies", run.AnomalyCount)
	}
}

func TestAnalyzingWithNoReadingsIsReportedNotSilentlyEmpty(t *testing.T) {
	empty := memory.New()
	service := New(empty, &scriptedNarrator{}, analysis.DefaultDetectorConfig())

	_, err := service.Analyze(context.Background())
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound: a dashboard must not show zero anomalies when there is no data", err)
	}
}

func TestMeterHealthIsDerivedFromOpenAnomalies(t *testing.T) {
	// Health is computed on read and never stored, so the seed, the detector and
	// the UI cannot each hold a different opinion about the same meter (ADR-0009).
	service, _ := newService(t, &scriptedNarrator{})
	if _, err := service.Analyze(context.Background()); err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	view, err := service.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}

	var row *MeterView
	for i := range view.Meters {
		if view.Meters[i].MeterID == "M-1" {
			row = &view.Meters[i]
		}
	}
	if row == nil {
		t.Fatal("the meter is missing from the dashboard")
	}

	// The expectation is derived from the findings, so this asserts the derivation
	// holds rather than re-asserting what the detector decided.
	anomalies, _ := service.Anomalies(context.Background())
	open := 0
	worst := analysis.Severity("")
	for _, anomaly := range anomalies {
		if anomaly.MeterCode != "M-1" || anomaly.Status != catalog.StatusOpen {
			continue
		}
		open++
		if severityRank(anomaly.Severity) < severityRank(worst) {
			worst = anomaly.Severity
		}
	}
	if open == 0 {
		t.Fatal("the meter has no open anomaly, so there is no health to derive")
	}
	if row.OpenAnomalies != open {
		t.Errorf("open anomalies = %d, want %d", row.OpenAnomalies, open)
	}
	if row.Health != health(worst) {
		t.Errorf("Health = %q, want %q derived from an open %s finding", row.Health, health(worst), worst)
	}
	if row.WorstSeverity != worst {
		t.Errorf("WorstSeverity = %q, want %q", row.WorstSeverity, worst)
	}
}

func TestDashboardReportsTheLastRunAndWhatItFound(t *testing.T) {
	service, _ := newService(t, &scriptedNarrator{})
	run, err := service.Analyze(context.Background())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	service.WaitForNarration()

	view, err := service.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}

	if view.LastRun == nil {
		t.Fatal("the dashboard reports no last run")
	}
	if view.LastRun.ID != run.ID {
		t.Errorf("last run = %d, want %d", view.LastRun.ID, run.ID)
	}
	if view.LastRun.State != store.RunCompleted {
		t.Errorf("last run state = %q, want COMPLETED", view.LastRun.State)
	}
	// The run completed, so every explanation has arrived: nothing pending, and
	// every finding explained.
	if view.LastRun.Narrating != 0 {
		t.Errorf("narrating = %d after every explanation arrived, want 0", view.LastRun.Narrating)
	}
	if view.LastRun.Explained != view.LastRun.AnomalyCount {
		t.Errorf("explained %d of %d anomalies", view.LastRun.Explained, view.LastRun.AnomalyCount)
	}
	if len(view.Meters) != 1 {
		t.Fatalf("got %d meters, want 1", len(view.Meters))
	}
	if view.Meters[0].TotalKWh <= 0 {
		t.Errorf("total consumption = %v, want the dashboard's headline number", view.Meters[0].TotalKWh)
	}
	if view.Meters[0].OpenAnomalies == 0 {
		t.Error("the meter shows no open anomalies although one was just found")
	}
}

func TestDashboardBeforeAnyRunIsEmptyRatherThanBroken(t *testing.T) {
	// An evaluator who starts the stack and opens the dashboard before pressing
	// Analyse should see a coherent page, not an error.
	service, _ := newService(t, &scriptedNarrator{})

	view, err := service.Dashboard(context.Background())
	if err != nil {
		t.Fatalf("Dashboard before any run: %v", err)
	}
	if view.LastRun != nil {
		t.Errorf("LastRun = %+v, want none", view.LastRun)
	}
	if len(view.Meters) != 1 {
		t.Errorf("meters = %d, want the catalogue to show before any run", len(view.Meters))
	}
}

func TestMeterDetailCarriesTheReadingsAndTheirBaseline(t *testing.T) {
	service, _ := newService(t, &scriptedNarrator{})

	detail, err := service.MeterDetail(context.Background(), "M-1")
	if err != nil {
		t.Fatalf("MeterDetail: %v", err)
	}
	if len(detail.Points) != 14*24 {
		t.Errorf("points = %d, want the meter's 336 readings", len(detail.Points))
	}
	if len(detail.Baseline) == 0 {
		t.Error("the detail view has no baseline, so the chart cannot show one")
	}
	known := 0
	for _, point := range detail.Points {
		if point.BaselineKWh > 0 {
			known++
		}
	}
	if known == 0 {
		t.Error("no reading has a baseline")
	}
}

func TestMeterDetailForAnUnknownMeterIsNotFound(t *testing.T) {
	service, _ := newService(t, &scriptedNarrator{})

	if _, err := service.MeterDetail(context.Background(), "M-NOPE"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
