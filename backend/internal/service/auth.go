package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
)

const SessionLifetime = 24 * time.Hour

var ErrInvalidCredentials = errors.New("credenciales inválidas")

// ProvisionUser adds an account only if absent, preserving an existing password
// and every active session when the server or seed is run again.
func (s *Service) ProvisionUser(ctx context.Context, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return fmt.Errorf("provision user: email and password are required")
	}
	if _, err := s.store.UserByEmail(ctx, email); err == nil {
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.store.ProvisionUser(ctx, email, hash)
}

func (s *Service) Login(ctx context.Context, email, password string) (store.User, string, error) {
	user, err := s.store.UserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if errors.Is(err, store.ErrNotFound) {
		return store.User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return store.User{}, "", err
	}
	if bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(password)) != nil {
		return store.User{}, "", ErrInvalidCredentials
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return store.User{}, "", fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	digest := sha256.Sum256([]byte(token))
	if err := s.store.CreateSession(ctx, user.ID, digest[:], time.Now().UTC().Add(SessionLifetime)); err != nil {
		return store.User{}, "", err
	}
	user.PasswordHash = nil
	return user, token, nil
}

func (s *Service) SessionUser(ctx context.Context, token string) (store.User, error) {
	if token == "" {
		return store.User{}, store.ErrNotFound
	}
	digest := sha256.Sum256([]byte(token))
	user, err := s.store.SessionUser(ctx, digest[:])
	user.PasswordHash = nil
	return user, err
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	digest := sha256.Sum256([]byte(token))
	return s.store.RevokeSession(ctx, digest[:])
}
