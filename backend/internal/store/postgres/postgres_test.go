package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
)

// The SQL has no other coverage. Every other layer is tested against the in-memory
// store, which is a real implementation of the contract — so a store that honours
// the contract in Go while its SQL quietly does not is invisible until a human runs
// the stack. That is not a hypothetical: Readings once passed a zero `to` straight
// into `timestamp <= $3`, matched no rows, and served every investigation view empty.
//
// These tests run only when TEST_DATABASE_URL points at a PostgreSQL that may be
// wiped, so `make test-db` can run them against a throwaway container.
func testDB(t *testing.T) *DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set, so the SQL is not exercised")
	}
	ctx := context.Background()
	db, err := Open(ctx, url)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(db.Close)
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	wipe(t, db)
	return db
}

// wipe empties the tables the store owns. A fresh container needs no wipe, but a
// reused one does, and leaving yesterday's rows would make these tests lie.
func wipe(t *testing.T, db *DB) {
	t.Helper()
	if _, err := db.Exec(context.Background(),
		`TRUNCATE readings, events, anomalies, analysis_runs, meters RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func TestTheSeededCatalogueAndSeriesComeBack(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	rows := []catalog.Reading{
		{
			MeterCode: "M-1", Timestamp: from,
			ConsumptionKWh: 10, VoltageV: 220, CurrentA: 45, PowerFactor: 0.95, IngestedStatus: "OK",
		},
		{
			MeterCode: "M-1", Timestamp: from.Add(time.Hour),
			ConsumptionKWh: 12, VoltageV: 221, CurrentA: 46, PowerFactor: 0.94, IngestedStatus: "OK",
		},
	}
	if err := db.UpsertMeters(ctx, []catalog.Meter{{Code: "M-1", Name: "Medidor M-1"}}); err != nil {
		t.Fatalf("UpsertMeters: %v", err)
	}
	if err := db.AppendReadings(ctx, rows); err != nil {
		t.Fatalf("AppendReadings: %v", err)
	}

	// A zero bound is unbounded on that side, and that is how the investigation view
	// asks for a meter's whole history.
	all, err := db.Readings(ctx, "M-1", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("Readings with zero bounds: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("with zero bounds the meter's whole history is %d readings, want 2", len(all))
	}
	if !all[0].Timestamp.Equal(from) || !all[1].Timestamp.Equal(from.Add(time.Hour)) {
		t.Errorf("readings came back out of order: %s, %s", all[0].Timestamp, all[1].Timestamp)
	}
	if all[0].PowerFactor != 0.95 {
		t.Errorf("the power factor round-tripped as %v, want 0.95", all[0].PowerFactor)
	}

	windowFrom, windowTo, err := db.ReadingWindow(ctx)
	if err != nil {
		t.Fatalf("ReadingWindow: %v", err)
	}
	if !windowFrom.Equal(from) || !windowTo.Equal(from.Add(time.Hour)) {
		t.Errorf("the window is %s..%s, want %s..%s", windowFrom, windowTo, from, from.Add(time.Hour))
	}

	totals, err := db.ConsumptionTotals(ctx)
	if err != nil {
		t.Fatalf("ConsumptionTotals: %v", err)
	}
	if totals["M-1"] != 22 {
		t.Errorf("the total for M-1 is %v, want 22", totals["M-1"])
	}
}

func TestAWindowedReadIsTheSameWindowTheDetectorSaw(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	var batch []catalog.Reading
	for hour := range 5 {
		batch = append(batch, catalog.Reading{
			MeterCode: "M-1", Timestamp: base.Add(time.Duration(hour) * time.Hour),
			ConsumptionKWh: float64(hour), IngestedStatus: "OK",
		})
	}
	if err := db.UpsertMeters(ctx, []catalog.Meter{{Code: "M-1"}}); err != nil {
		t.Fatalf("UpsertMeters: %v", err)
	}
	if err := db.AppendReadings(ctx, batch); err != nil {
		t.Fatalf("AppendReadings: %v", err)
	}

	inside, err := db.Readings(ctx, "M-1", base.Add(time.Hour), base.Add(3*time.Hour))
	if err != nil {
		t.Fatalf("Readings: %v", err)
	}
	if len(inside) != 3 {
		t.Fatalf("the window held %d readings, want 3 (both bounds are inclusive)", len(inside))
	}

	empty, err := db.Readings(ctx, "M-1", base.Add(-48*time.Hour), base.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("Readings for an empty window: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("a window before the series returned %d readings, want 0", len(empty))
	}
}

func TestTheImportIsIdempotent(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	at := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
	reading := catalog.Reading{MeterCode: "M-1", Timestamp: at, ConsumptionKWh: 30, IngestedStatus: "OK"}
	event := catalog.OperationalEvent{MeterCode: "M-1", Timestamp: at, Type: catalog.EventTypeMaintenance, Description: "Filtro"}

	if err := db.UpsertMeters(ctx, []catalog.Meter{{Code: "M-1", Name: "old name"}}); err != nil {
		t.Fatalf("UpsertMeters: %v", err)
	}
	for range 2 {
		if err := db.AppendReadings(ctx, []catalog.Reading{reading}); err != nil {
			t.Fatalf("AppendReadings: %v", err)
		}
		if err := db.AppendEvents(ctx, []catalog.OperationalEvent{event}); err != nil {
			t.Fatalf("AppendEvents: %v", err)
		}
	}

	readings, err := db.AllReadings(ctx)
	if err != nil {
		t.Fatalf("AllReadings: %v", err)
	}
	if len(readings) != 1 {
		t.Errorf("importing twice left %d readings, want 1", len(readings))
	}
	events, err := db.Events(ctx)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("importing twice left %d events, want 1", len(events))
	}

	// A re-import is also how a corrected reading arrives, so the second write wins.
	reading.ConsumptionKWh = 31
	if err := db.AppendReadings(ctx, []catalog.Reading{reading}); err != nil {
		t.Fatalf("AppendReadings again: %v", err)
	}
	readings, err = db.AllReadings(ctx)
	if err != nil {
		t.Fatalf("AllReadings: %v", err)
	}
	if len(readings) != 1 || readings[0].ConsumptionKWh != 31 {
		t.Errorf("after the correction there is %d reading(s), the first at %v kWh; want 1 at 31", len(readings), readings[0].ConsumptionKWh)
	}
}

func TestTwoEventsOfDifferentTypesInTheSameHourAreTwoEvents(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	at := time.Date(2026, 9, 3, 9, 0, 0, 0, time.UTC)
	events := []catalog.OperationalEvent{
		{MeterCode: "M-1", Timestamp: at, Type: catalog.EventTypeMaintenance, Description: "a"},
		{MeterCode: "M-1", Timestamp: at, Type: catalog.EventTypeDataQuality, Description: "b"},
		{MeterCode: "M-1", Timestamp: at, Type: catalog.EventTypeMaintenance, Description: "a again"},
	}
	// The event table has a foreign key to the catalogue, so an event for a meter
	// that was never imported is refused rather than silently stored.
	if err := db.UpsertMeters(ctx, []catalog.Meter{{Code: "M-1"}}); err != nil {
		t.Fatalf("UpsertMeters: %v", err)
	}
	if err := db.AppendEvents(ctx, events); err != nil {
		t.Fatalf("AppendEvents: %v", err)
	}
	stored, err := db.Events(ctx)
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("three reported events became %d rows, want 2: identity is (meter, timestamp, type)", len(stored))
	}
}

func TestARunAndItsAnomaliesSurviveTheRoundTrip(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	from := time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)
	to := from.Add(72 * time.Hour)

	if err := db.UpsertMeters(ctx, []catalog.Meter{{Code: "M-1"}, {Code: "M-2"}}); err != nil {
		t.Fatalf("UpsertMeters: %v", err)
	}
	runID, err := db.StartRun(ctx, from, to)
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	run, err := db.Run(ctx, runID)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if run.State != store.RunRunning || run.FinishedAt != nil {
		t.Errorf("a fresh run is %s, finished at %v; want RUNNING, unfinished", run.State, run.FinishedAt)
	}

	findings := []analysis.Anomaly{
		{
			MeterCode: "M-1", WindowStart: from, WindowEnd: to, AffectedReadings: 40,
			Type: analysis.AnomalyReal, Severity: analysis.SeverityHigh,
			Confidence: 0.99, ConfidenceBand: analysis.BandHigh, DetectedBy: "spike",
			Reason: "rules prose", RecommendedAction: "look at it",
			Corroborating:     []string{"voltage", "current"},
			DeviationSeries:   []analysis.Point{{Timestamp: from, ActualKWh: 30, BaselineKWh: 10, Deviation: 200}},
			ExplanationSource: catalog.SourceRules, ExplanationStatus: catalog.ExplanationPending,
			Status: catalog.StatusOpen,
		},
		{
			MeterCode: "M-2", WindowStart: from, WindowEnd: to, AffectedReadings: 5,
			Type: analysis.AnomalyDataQuality, Severity: analysis.SeverityHigh,
			Confidence: 0.96, ConfidenceBand: analysis.BandHigh, DetectedBy: "quality",
			Reason: "flat voltage", RecommendedAction: "check the sensor",
			ExplanationSource: catalog.SourceRules, ExplanationStatus: catalog.ExplanationPending,
			Status: catalog.StatusOpen,
		},
	}
	if err := db.SaveAnomalies(ctx, runID, findings); err != nil {
		t.Fatalf("SaveAnomalies: %v", err)
	}
	if err := db.FinishRun(ctx, runID, store.RunCompleted, len(findings), ""); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}

	latest, err := db.LatestRun(ctx)
	if err != nil {
		t.Fatalf("LatestRun: %v", err)
	}
	if latest.ID != runID || latest.State != store.RunCompleted || latest.AnomalyCount != 2 {
		t.Fatalf("the latest run is %+v, want id %d COMPLETED with 2 anomalies", latest, runID)
	}

	// Most urgent first: the detector ranked M-1 above M-2 and the list must keep
	// that order, because the dashboard shows the first rows.
	stored, err := db.Anomalies(ctx)
	if err != nil {
		t.Fatalf("Anomalies: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("the run has %d anomalies, want 2", len(stored))
	}
	if stored[0].MeterCode != "M-1" || stored[1].MeterCode != "M-2" {
		t.Errorf("the anomalies came back as %s then %s, want M-1 then M-2", stored[0].MeterCode, stored[1].MeterCode)
	}
	if stored[0].ID == 0 || stored[0].RunID != runID {
		t.Errorf("the first anomaly has id %d and run %d, want a real id and run %d", stored[0].ID, stored[0].RunID, runID)
	}
	if len(stored[0].DeviationSeries) != 1 || stored[0].DeviationSeries[0].Deviation != 200 {
		t.Errorf("the deviation series did not survive: %+v", stored[0].DeviationSeries)
	}
	if len(stored[0].Corroborating) != 2 {
		t.Errorf("the corroborating measurements did not survive: %v", stored[0].Corroborating)
	}

	// Narration is an upgrade of the same row, and a failed one leaves the rules
	// prose in place while making itself visible.
	if err := db.Narrate(ctx, stored[0].ID, catalog.SourceLLM, "narrated reason", "narrated action"); err != nil {
		t.Fatalf("Narrate: %v", err)
	}
	upgraded, err := db.Anomaly(ctx, stored[0].ID)
	if err != nil {
		t.Fatalf("Anomaly: %v", err)
	}
	if upgraded.Reason != "narrated reason" || upgraded.ExplanationSource != catalog.SourceLLM || upgraded.ExplanationStatus != catalog.ExplanationReady {
		t.Errorf("after narration the anomaly reads %+v", upgraded)
	}
	if err := db.MarkNarrationFailed(ctx, stored[1].ID); err != nil {
		t.Fatalf("MarkNarrationFailed: %v", err)
	}
	failed, err := db.Anomaly(ctx, stored[1].ID)
	if err != nil {
		t.Fatalf("Anomaly: %v", err)
	}
	if failed.ExplanationStatus != catalog.ExplanationFailed || failed.Reason != "flat voltage" {
		t.Errorf("a failed narration should leave the rules prose and mark itself failed, got %+v", failed)
	}

	// The operator's decision is its own field and never the detector's to set.
	if err := db.SetStatus(ctx, stored[1].ID, catalog.StatusAcknowledged); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	acknowledged, err := db.Anomaly(ctx, stored[1].ID)
	if err != nil {
		t.Fatalf("Anomaly: %v", err)
	}
	if acknowledged.Status != catalog.StatusAcknowledged {
		t.Errorf("the status is %s, want ACKNOWLEDGED", acknowledged.Status)
	}
}

func TestAFailedRunKeepsItsReason(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	runID, err := db.StartRun(ctx, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if err := db.FinishRun(ctx, runID, store.RunFailed, 0, "the detector panicked"); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}
	run, err := db.Run(ctx, runID)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if run.State != store.RunFailed || run.Error != "the detector panicked" {
		t.Errorf("the failed run reads %+v", run)
	}
}

func TestMissingRowsAreNotFound(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if _, err := db.Meter(ctx, "M-404"); err == nil || !strings.Contains(err.Error(), store.ErrNotFound.Error()) {
		t.Errorf("asking for a meter that is not there returned %v", err)
	}
	if _, err := db.Anomaly(ctx, 9999); err == nil || !strings.Contains(err.Error(), store.ErrNotFound.Error()) {
		t.Errorf("asking for an anomaly that is not there returned %v", err)
	}
	if _, err := db.LatestRun(ctx); err == nil || !strings.Contains(err.Error(), store.ErrNotFound.Error()) {
		t.Errorf("asking for a run before any has run returned %v", err)
	}
}
