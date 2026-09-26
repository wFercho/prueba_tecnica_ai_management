package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
)

func TestReplaceDatasetRollsBackFailureAndPreservesIdentities(t *testing.T) {
	db := testDB(t)
	ctx := t.Context()
	at := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if err := db.ProvisionUser(ctx, "demo@example.com", []byte("hash")); err != nil {
		t.Fatal(err)
	}
	user, _ := db.UserByEmail(ctx, "demo@example.com")
	if err := db.CreateSession(ctx, user.ID, []byte("digest"), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceDataset(ctx, []catalog.Meter{{Code: "M-1", Name: "uno"}},
		[]catalog.Reading{{MeterCode: "M-1", Timestamp: at, ConsumptionKWh: 1, IngestedStatus: "OK"}}, nil); err != nil {
		t.Fatal(err)
	}
	id, err := db.StartRun(ctx, at, at)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.FinishRun(ctx, id, store.RunCompleted, 0, ""); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceDataset(ctx, []catalog.Meter{{Code: "M-2", Name: "dos"}},
		[]catalog.Reading{{MeterCode: "missing", Timestamp: at, IngestedStatus: "OK"}}, nil); err == nil {
		t.Fatal("invalid import should fail")
	}
	if _, err := db.Meter(ctx, "M-1"); err != nil {
		t.Fatalf("failed seed removed old meter: %v", err)
	}
	if _, err := db.LatestRun(ctx); err != nil {
		t.Fatalf("failed seed removed old run: %v", err)
	}
	if err := db.ReplaceDataset(ctx, []catalog.Meter{{Code: "M-2", Name: "dos"}},
		[]catalog.Reading{{MeterCode: "M-2", Timestamp: at, ConsumptionKWh: 2, IngestedStatus: "OK"}}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.LatestRun(ctx); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("old run survived reset: %v", err)
	}
	if _, err := db.Meter(ctx, "M-1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("stale meter survived reset: %v", err)
	}
	if _, err := db.SessionUser(ctx, []byte("digest")); err != nil {
		t.Fatalf("seed revoked demo session: %v", err)
	}
}
