package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
)

type session struct {
	userID    int64
	expiresAt time.Time
}

func (s *Store) ProvisionUser(_ context.Context, email string, passwordHash []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail(); err != nil {
		return err
	}
	if s.users == nil {
		s.users = map[string]store.User{}
	}
	if _, exists := s.users[email]; !exists {
		s.nextID++
		s.users[email] = store.User{ID: s.nextID, Email: email, PasswordHash: append([]byte(nil), passwordHash...)}
	}
	return nil
}

func (s *Store) UserByEmail(_ context.Context, email string) (store.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.fail(); err != nil {
		return store.User{}, err
	}
	user, ok := s.users[email]
	if !ok {
		return store.User{}, store.ErrNotFound
	}
	user.PasswordHash = append([]byte(nil), user.PasswordHash...)
	return user, nil
}

func (s *Store) CreateSession(_ context.Context, userID int64, tokenHash []byte, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail(); err != nil {
		return err
	}
	var found bool
	for _, user := range s.users {
		found = found || user.ID == userID
	}
	if !found {
		return fmt.Errorf("user %d: %w", userID, store.ErrNotFound)
	}
	if s.sessions == nil {
		s.sessions = map[string]session{}
	}
	s.sessions[string(tokenHash)] = session{userID: userID, expiresAt: expiresAt}
	return nil
}

func (s *Store) SessionUser(_ context.Context, tokenHash []byte) (store.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.fail(); err != nil {
		return store.User{}, err
	}
	session, found := s.sessions[string(tokenHash)]
	if !found || !session.expiresAt.After(time.Now()) {
		return store.User{}, store.ErrNotFound
	}
	for _, user := range s.users {
		if user.ID == session.userID {
			user.PasswordHash = append([]byte(nil), user.PasswordHash...)
			return user, nil
		}
	}
	return store.User{}, store.ErrNotFound
}

func (s *Store) RevokeSession(_ context.Context, tokenHash []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail(); err != nil {
		return err
	}
	delete(s.sessions, string(tokenHash))
	return nil
}
