package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Setting reads one preference by key, returning def when it has never been
// set.
func (s *Store) Setting(ctx context.Context, key, def string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return def, nil
	case err != nil:
		return "", fmt.Errorf("read setting %q: %w", key, err)
	}
	return value, nil
}

// SetSetting stores one preference by key, replacing whatever it held.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = excluded.value`, key, value); err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}
