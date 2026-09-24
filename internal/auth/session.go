package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gabe-santos/yogurt/internal/clock"
	"github.com/gabe-santos/yogurt/internal/store"
)

// Sessions issues and validates the browser's session tokens. A token is random
// and opaque; only its hash reaches the database.
type Sessions struct {
	store *store.Store
	clock clock.Clock
	ttl   time.Duration
}

// NewSessions returns sessions valid for ttl after they are issued.
func NewSessions(s *store.Store, c clock.Clock, ttl time.Duration) *Sessions {
	return &Sessions{store: s, clock: c, ttl: ttl}
}

// TTL is how long an issued session stays valid.
func (s *Sessions) TTL() time.Duration { return s.ttl }

// Issue starts a new session and returns its token.
func (s *Sessions) Issue(ctx context.Context) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	now := s.clock.Now()
	if err := s.store.DeleteExpiredSessions(ctx, now); err != nil {
		return "", err
	}
	if err := s.store.CreateSession(ctx, hashToken(token), now, now.Add(s.ttl)); err != nil {
		return "", err
	}
	return token, nil
}

// Valid reports whether a token names a session that has not expired. An
// expired session is removed as it is found.
func (s *Sessions) Valid(ctx context.Context, token string) (bool, error) {
	if token == "" {
		return false, nil
	}
	hash := hashToken(token)
	expiresAt, found, err := s.store.SessionExpiry(ctx, hash)
	if err != nil || !found {
		return false, err
	}
	if !expiresAt.After(s.clock.Now()) {
		return false, s.store.DeleteSession(ctx, hash)
	}
	return true, nil
}

// Revoke ends the session named by a token.
func (s *Sessions) Revoke(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteSession(ctx, hashToken(token))
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
