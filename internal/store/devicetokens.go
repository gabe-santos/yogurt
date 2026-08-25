package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNoDeviceToken reports a device token that is not there.
var ErrNoDeviceToken = errors.New("no such device token")

// DeviceToken is a named, long-lived bearer credential for a non-browser
// client, per ADR-0005. Its raw value is never stored; only its hash is.
type DeviceToken struct {
	ID         int64
	Name       string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// CreateDeviceToken stores a new device token by the hash of its raw value
// and returns it with its assigned id.
func (s *Store) CreateDeviceToken(ctx context.Context, name, tokenHash string, now time.Time) (DeviceToken, error) {
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO device_tokens (name, token_hash, created_at) VALUES (?, ?, ?)`,
		name, tokenHash, now.Unix())
	if err != nil {
		return DeviceToken{}, fmt.Errorf("create device token: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return DeviceToken{}, fmt.Errorf("create device token: %w", err)
	}
	return DeviceToken{ID: id, Name: name, CreatedAt: now}, nil
}

// DeviceTokens reads the whole collection, oldest first.
func (s *Store) DeviceTokens(ctx context.Context) ([]DeviceToken, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, created_at, last_used_at FROM device_tokens ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("read device tokens: %w", err)
	}
	defer rows.Close()

	var tokens []DeviceToken
	for rows.Next() {
		token, err := scanDeviceToken(rows)
		if err != nil {
			return nil, fmt.Errorf("read device tokens: %w", err)
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

func scanDeviceToken(row rowScanner) (DeviceToken, error) {
	var (
		token      DeviceToken
		createdAt  int64
		lastUsedAt sql.NullInt64
	)
	if err := row.Scan(&token.ID, &token.Name, &createdAt, &lastUsedAt); err != nil {
		return DeviceToken{}, err
	}
	token.CreatedAt = time.Unix(createdAt, 0).UTC()
	if lastUsedAt.Valid {
		at := time.Unix(lastUsedAt.Int64, 0).UTC()
		token.LastUsedAt = &at
	}
	return token, nil
}

// AuthenticateDeviceToken records a use of the device token named by this
// hash and reports whether such a token exists. A revoked, and so deleted,
// token reports false.
func (s *Store) AuthenticateDeviceToken(ctx context.Context, tokenHash string, now time.Time) (bool, error) {
	result, err := s.db.ExecContext(ctx,
		`UPDATE device_tokens SET last_used_at = ? WHERE token_hash = ?`, now.Unix(), tokenHash)
	if err != nil {
		return false, fmt.Errorf("authenticate device token: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("authenticate device token: %w", err)
	}
	return affected > 0, nil
}

// DeleteDeviceToken revokes a device token, ending it immediately. It
// returns ErrNoDeviceToken when there is no such token.
func (s *Store) DeleteDeviceToken(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM device_tokens WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete device token: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete device token: %w", err)
	}
	if affected == 0 {
		return ErrNoDeviceToken
	}
	return nil
}
