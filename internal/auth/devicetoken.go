package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/gabe-santos/yogurt/internal/clock"
	"github.com/gabe-santos/yogurt/internal/store"
)

// DeviceTokens issues and validates long-lived bearer credentials for
// non-browser clients, per ADR-0005. A token is random and opaque; only its
// hash reaches the database. Unlike a session it carries no expiry — only
// Revoke ends it.
type DeviceTokens struct {
	store *store.Store
	clock clock.Clock
}

// NewDeviceTokens returns a DeviceTokens backed by s.
func NewDeviceTokens(s *store.Store, c clock.Clock) *DeviceTokens {
	return &DeviceTokens{store: s, clock: c}
}

// Issue creates a named device token and returns it alongside its raw value,
// which is recoverable only from this call's return.
func (d *DeviceTokens) Issue(ctx context.Context, name string) (store.DeviceToken, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return store.DeviceToken{}, "", fmt.Errorf("generate device token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	created, err := d.store.CreateDeviceToken(ctx, name, hashToken(token), d.clock.Now())
	if err != nil {
		return store.DeviceToken{}, "", err
	}
	return created, token, nil
}

// Authenticate reports whether token names a live device token, recording
// this call as its last use.
func (d *DeviceTokens) Authenticate(ctx context.Context, token string) (bool, error) {
	if token == "" {
		return false, nil
	}
	return d.store.AuthenticateDeviceToken(ctx, hashToken(token), d.clock.Now())
}

// List returns every device token, oldest first.
func (d *DeviceTokens) List(ctx context.Context) ([]store.DeviceToken, error) {
	return d.store.DeviceTokens(ctx)
}

// Revoke ends a device token immediately. It returns store.ErrNoDeviceToken
// when there is no such token.
func (d *DeviceTokens) Revoke(ctx context.Context, id int64) error {
	return d.store.DeleteDeviceToken(ctx, id)
}
