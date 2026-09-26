package postgres

import (
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
)

func TestFailedRunKeepsMostRecentSuccessfulResults(t *testing.T) {
	db := testDB(t)
	ctx := t.Context()
	if err := db.UpsertMeters(ctx, []catalog.Meter{{Code: "M-1", Name: "Uno"}}); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	first, err := db.StartRun(ctx, at, at)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveAnomalies(ctx, first, []analysis.Anomaly{{MeterCode: "M-1", Type: analysis.AnomalyReal,
		Severity: analysis.SeverityHigh, WindowStart: at, WindowEnd: at, Reason: "regla", RecommendedAction: "investigar", DetectedBy: "test"}}); err != nil {
		t.Fatal(err)
	}
	if err := db.FinishRun(ctx, first, store.RunCompleted, 1, ""); err != nil {
		t.Fatal(err)
	}
	second, err := db.StartRun(ctx, at, at)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.FinishRun(ctx, second, store.RunFailed, 0, "falló"); err != nil {
		t.Fatal(err)
	}
	completed, err := db.LatestCompletedRun(ctx)
	if err != nil || completed.ID != first {
		t.Fatalf("completed = %+v, %v", completed, err)
	}
	visible, err := db.Anomalies(ctx)
	if err != nil || len(visible) != 1 || visible[0].RunID != first {
		t.Fatalf("visible = %+v, %v", visible, err)
	}
	failed, err := db.RunAnomalies(ctx, second)
	if err != nil || len(failed) != 0 {
		t.Fatalf("failed run findings = %+v, %v", failed, err)
	}
}
