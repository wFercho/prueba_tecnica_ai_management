package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
)

func (db *DB) ProvisionUser(ctx context.Context, email string, passwordHash []byte) error {
	_, err := db.pool.Exec(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) ON CONFLICT (email) DO NOTHING`, email, passwordHash)
	if err != nil {
		return fmt.Errorf("provision user: %w", err)
	}
	return nil
}

func (db *DB) UserByEmail(ctx context.Context, email string) (store.User, error) {
	var user store.User
	err := db.pool.QueryRow(ctx, `SELECT id, email, password_hash FROM users WHERE email = $1`, email).Scan(&user.ID, &user.Email, &user.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.User{}, store.ErrNotFound
	}
	if err != nil {
		return store.User{}, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (db *DB) CreateSession(ctx context.Context, userID int64, tokenHash []byte, expiresAt time.Time) error {
	_, err := db.pool.Exec(ctx, `INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`, userID, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (db *DB) SessionUser(ctx context.Context, tokenHash []byte) (store.User, error) {
	var user store.User
	err := db.pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.password_hash FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now()`, tokenHash).
		Scan(&user.ID, &user.Email, &user.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.User{}, store.ErrNotFound
	}
	if err != nil {
		return store.User{}, fmt.Errorf("lookup session: %w", err)
	}
	return user, nil
}

func (db *DB) RevokeSession(ctx context.Context, tokenHash []byte) error {
	_, err := db.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}
