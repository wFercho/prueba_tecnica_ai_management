package memory

import (
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
)

func TestLatestCompletedRunSurvivesLaterFailure(t *testing.T) {
	ctx := t.Context()
	s := New()
	start := time.Now()
	first, _ := s.StartRun(ctx, start, start)
	if err := s.SaveAnomalies(ctx, first, []analysis.Anomaly{{MeterCode: "M-109", Severity: analysis.SeverityHigh}}); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishRun(ctx, first, store.RunCompleted, 1, ""); err != nil {
		t.Fatal(err)
	}
	second, _ := s.StartRun(ctx, start, start)
	if err := s.FinishRun(ctx, second, store.RunFailed, 0, "simulated error"); err != nil {
		t.Fatal(err)
	}
	latest, err := s.LatestRun(ctx)
	if err != nil || latest.ID != second {
		t.Fatalf("latest attempt = %+v, %v", latest, err)
	}
	completed, err := s.LatestCompletedRun(ctx)
	if err != nil || completed.ID != first {
		t.Fatalf("latest successful = %+v, %v", completed, err)
	}
	visible, err := s.Anomalies(ctx)
	if err != nil || len(visible) != 1 || visible[0].RunID != first {
		t.Fatalf("visible results = %+v, %v", visible, err)
	}
	if older, err := s.RunAnomalies(ctx, second); err != nil || len(older) != 0 {
		t.Fatalf("failed run results = %+v, %v", older, err)
	}
}
