// Package store owns the SQLite database, its embedded migrations, and every SQL
// statement in the application.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, per ADR-0006
)

// FileName is the name of the database file inside the data directory.
const FileName = "reader.db"

// Store is the application's database.
type Store struct {
	db *sql.DB
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows, so a single scan
// function can read one row or iterate many.
type rowScanner interface {
	Scan(dest ...any) error
}

// boolToInt renders a bool the way every boolean column in this schema
// stores one: 0 or 1.
func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// Open creates the data directory and database file if they do not exist,
// applies every pending migration, and returns a ready store.
func Open(ctx context.Context, dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, fmt.Errorf("create data directory %s: %w", dataDir, err)
	}

	path := filepath.Join(dataDir, FileName)
	dsn := "file:" + url.PathEscape(path) + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}

	// One writer at a time: SQLite serialises writes anyway, and a single
	// connection removes SQLITE_BUSY from the picture.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to database %s: %w", path, err)
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	if err := backfillSearchIndex(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

// CreateSession records a new session by the hash of its token.
func (s *Store) CreateSession(ctx context.Context, tokenHash string, createdAt, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, created_at, expires_at) VALUES (?, ?, ?)`,
		tokenHash, createdAt.Unix(), expiresAt.Unix())
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// SessionExpiry reports when the session with this token hash expires. It
// returns false when no such session exists.
func (s *Store) SessionExpiry(ctx context.Context, tokenHash string) (time.Time, bool, error) {
	var expiresAt int64
	err := s.db.QueryRowContext(ctx,
		`SELECT expires_at FROM sessions WHERE token_hash = ?`, tokenHash).Scan(&expiresAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return time.Time{}, false, nil
	case err != nil:
		return time.Time{}, false, fmt.Errorf("read session: %w", err)
	}
	return time.Unix(expiresAt, 0).UTC(), true, nil
}

// DeleteSession removes one session, if it is there.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes every session that has expired by now.
func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.Unix()); err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}
