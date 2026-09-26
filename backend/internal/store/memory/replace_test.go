package memory

import (
	"errors"
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
)

func TestReplaceDatasetKeepsIdentitiesButRemovesPreviousResults(t *testing.T) {
	ctx := t.Context()
	s := New()
	if err := s.ProvisionUser(ctx, "demo@example.com", []byte("hashed")); err != nil {
		t.Fatal(err)
	}
	user, _ := s.UserByEmail(ctx, "demo@example.com")
	if err := s.CreateSession(ctx, user.ID, []byte("digest"), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceDataset(ctx, []catalog.Meter{{Code: "M-1"}}, []catalog.Reading{{MeterCode: "M-1"}}, nil); err != nil {
		t.Fatal(err)
	}
	id, _ := s.StartRun(ctx, time.Now(), time.Now())
	if err := s.FinishRun(ctx, id, store.RunCompleted, 0, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceDataset(ctx, []catalog.Meter{{Code: "M-2"}}, []catalog.Reading{{MeterCode: "M-2"}}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LatestRun(ctx); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("stale run after seed: %v", err)
	}
	if _, err := s.Meter(ctx, "M-1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("stale meter after seed: %v", err)
	}
	if _, err := s.SessionUser(ctx, []byte("digest")); err != nil {
		t.Fatalf("seed revoked user session: %v", err)
	}
}
