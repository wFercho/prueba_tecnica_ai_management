package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
)

func TestIdentityPersistsHashAndRevocableExpiringSessions(t *testing.T) {
	db := testDB(t)
	ctx := t.Context()
	if err := db.ProvisionUser(ctx, "first@example.com", []byte("hash-one")); err != nil {
		t.Fatal(err)
	}
	if err := db.ProvisionUser(ctx, "first@example.com", []byte("replacement")); err != nil {
		t.Fatal(err)
	}
	if err := db.ProvisionUser(ctx, "second@example.com", []byte("hash-two")); err != nil {
		t.Fatal(err)
	}
	first, err := db.UserByEmail(ctx, "first@example.com")
	if err != nil || string(first.PasswordHash) != "hash-one" {
		t.Fatalf("first user was overwritten: %+v, %v", first, err)
	}
	second, err := db.UserByEmail(ctx, "second@example.com")
	if err != nil || second.ID == first.ID {
		t.Fatalf("second user = %+v, %v", second, err)
	}
	token := []byte("only-a-digest-goes-in-the-database")
	if err := db.CreateSession(ctx, first.ID, token, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SessionUser(ctx, token); err != nil {
		t.Fatalf("active session: %v", err)
	}
	if err := db.RevokeSession(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SessionUser(ctx, token); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("revoked session: %v", err)
	}
	if err := db.CreateSession(ctx, first.ID, []byte("expired"), time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SessionUser(ctx, []byte("expired")); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expired session: %v", err)
	}
}
