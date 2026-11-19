package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNoGist reports an Entry with no Gist stored.
var ErrNoGist = errors.New("no such Gist")

// Gist is a reader-requested condensation of an Entry's Article, or of its
// own body when the Article cannot be reduced (ADR-0018 et seq). One per
// Entry: regenerating replaces the row rather than keeping history.
type Gist struct {
	EntryID   int64
	Text      string
	Source    string // "article" or "feed"
	Model     string
	CreatedAt time.Time
}

// Gist reads the stored Gist for an Entry. It returns ErrNoGist when the
// Entry has none.
func (s *Store) Gist(ctx context.Context, entryID int64) (Gist, error) {
	var g Gist
	var createdAt int64
	err := s.db.QueryRowContext(ctx,
		`SELECT entry_id, text, source, model, created_at FROM gists WHERE entry_id = ?`, entryID,
	).Scan(&g.EntryID, &g.Text, &g.Source, &g.Model, &createdAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Gist{}, ErrNoGist
	case err != nil:
		return Gist{}, fmt.Errorf("read gist for entry %d: %w", entryID, err)
	}
	g.CreatedAt = time.Unix(createdAt, 0).UTC()
	return g, nil
}

// SaveGist stores a Gist, replacing whatever this Entry held: regenerating
// replaces rather than keeping history.
func (s *Store) SaveGist(ctx context.Context, g Gist, now time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO gists (entry_id, text, source, model, created_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (entry_id) DO UPDATE SET
		 	text = excluded.text, source = excluded.source, model = excluded.model, created_at = excluded.created_at`,
		g.EntryID, g.Text, g.Source, g.Model, now.Unix())
	if err != nil {
		return fmt.Errorf("save gist for entry %d: %w", g.EntryID, err)
	}
	return nil
}
