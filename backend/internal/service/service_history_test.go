package service

import (
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store/memory"
)

func TestDashboardSeparatesLastFailedAttemptFromSuccessfulFindings(t *testing.T) {
	svc, data := newService(t, nil)
	ctx := t.Context()
	initial, err := svc.Dashboard(ctx)
	if err != nil || initial.LastRun != nil || initial.Meters[0].Health != HealthUnassessed {
		t.Fatalf("before analysis = %+v, %v", initial, err)
	}
	run, err := svc.Analyze(ctx)
	if err != nil {
		t.Fatal(err)
	}
	svc.WaitForNarration()
	failed, err := data.StartRun(ctx, time.Now(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := data.FinishRun(ctx, failed, store.RunFailed, 0, "simulated error"); err != nil {
		t.Fatal(err)
	}
	view, err := svc.Dashboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if view.LastRun == nil || view.LastRun.ID != failed || view.LastRun.State != store.RunFailed {
		t.Errorf("last attempt = %+v", view.LastRun)
	}
	if view.LastSuccessfulRun == nil || view.LastSuccessfulRun.ID != run.ID || view.HighPriority == 0 {
		t.Errorf("last successful = %+v, high priority = %d", view.LastSuccessfulRun, view.HighPriority)
	}
	visible, err := svc.Anomalies(ctx)
	if err != nil || len(visible) != run.AnomalyCount {
		t.Errorf("visible anomalies = %d, want %d: %v", len(visible), run.AnomalyCount, err)
	}
}

func TestShortHistoryStaysUnevaluatedWhileOtherMetersAreAnalyzed(t *testing.T) {
	ctx := t.Context()
	full := deliveredStore(t)
	meters, _ := full.Meters(ctx)
	readings, _ := full.AllReadings(ctx)
	meters = append(meters, catalog.Meter{Code: "SHORT", Name: "Pocas lecturas"})
	readings = append(readings, catalog.Reading{
		MeterCode: "SHORT", Timestamp: readings[0].Timestamp,
		ConsumptionKWh: 1000, VoltageV: 220, CurrentA: 10, PowerFactor: 0.95, IngestedStatus: "OK",
	})
	data := memory.New()
	if err := data.Seed(meters, readings, nil); err != nil {
		t.Fatal(err)
	}
	svc := New(data, nil, analysis.DefaultDetectorConfig())
	if _, err := svc.Analyze(ctx); err != nil {
		t.Fatal(err)
	}
	summary, err := svc.Dashboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range summary.Meters {
		if row.MeterID == "SHORT" && (row.Health != HealthInsufficient || row.VariationPercent != nil) {
			t.Errorf("short history evaluated: %+v", row)
		}
	}
	anomalies, _ := svc.Anomalies(ctx)
	var hasLong bool
	for _, anomaly := range anomalies {
		if anomaly.MeterCode == "SHORT" && anomaly.Type != analysis.AnomalyDataQuality {
			t.Errorf("invented consumption finding for SHORT: %+v", anomaly)
		}
		if anomaly.MeterCode == "M-1" {
			hasLong = true
		}
	}
	if !hasLong {
		t.Fatal("analyzing SHORT silenced another meter with sufficient history")
	}
}
