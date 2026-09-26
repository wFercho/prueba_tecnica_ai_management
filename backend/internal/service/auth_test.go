package service

import (
	"errors"
	"testing"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store/memory"
	"golang.org/x/crypto/bcrypt"
)

func TestProvisionUserCanAddAccountsWithoutReplacingTheirPasswords(t *testing.T) {
	ctx := t.Context()
	data := memory.New()
	svc := New(data, nil, analysis.DefaultDetectorConfig())
	if err := svc.ProvisionUser(ctx, "admin@email.com", "initial"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ProvisionUser(ctx, "admin@email.com", "replacement"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ProvisionUser(ctx, "second@email.com", "second-password"); err != nil {
		t.Fatal(err)
	}
	first, err := data.UserByEmail(ctx, "admin@email.com")
	if err != nil || string(first.PasswordHash) == "initial" || bcrypt.CompareHashAndPassword(first.PasswordHash, []byte("initial")) != nil {
		t.Fatalf("original hash replaced or plaintext stored: %+v, %v", first, err)
	}
	if _, _, err := svc.Login(ctx, "admin@email.com", "replacement"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("replacement password worked: %v", err)
	}
	if _, _, err := svc.Login(ctx, "second@email.com", "second-password"); err != nil {
		t.Fatalf("second account cannot sign in: %v", err)
	}
	if _, err := svc.SessionUser(ctx, "not-a-token"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("invalid token accepted: %v", err)
	}
}
