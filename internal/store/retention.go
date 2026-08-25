package store

import (
	"context"
	"fmt"
	"time"
)

// SinceCursor is a position in the changed-since feed: the change sequence
// number of the last change a caller has seen, per ADR-0004. Zero is the
// start of history.
type SinceCursor int64

// Tombstone reports an Entry retention cleanup removed, so a delta reader
// catching up with a since cursor learns it is gone.
type Tombstone struct {
	EntryID   int64
	DeletedAt time.Time
	ChangeSeq int64
}

// EntriesSince reads every Entry changed after cursor, and every tombstone
// retention has recorded after cursor, oldest change first, up to limit
// combined across both. Every change and tombstone draws its ChangeSeq from
// the same counter, so the two streams merge by comparing it directly: no
// value is ever produced twice, and nothing needs a tie-break. It returns the
// position to resume from — the highest ChangeSeq returned, or cursor
// unchanged when there was nothing new, so a caller that is caught up gets an
// empty result rather than drifting.
func (s *Store) EntriesSince(ctx context.Context, cursor SinceCursor, limit int) ([]Entry, []Tombstone, SinceCursor, error) {
	changed, err := s.changedEntriesSince(ctx, cursor, limit)
	if err != nil {
		return nil, nil, 0, err
	}
	removed, err := s.tombstonesSince(ctx, cursor, limit)
	if err != nil {
		return nil, nil, 0, err
	}

	var entries []Entry
	var tombstones []Tombstone
	next := cursor
	ei, ti := 0, 0
	for len(entries)+len(tombstones) < limit && (ei < len(changed) || ti < len(removed)) {
		takeEntry := ei < len(changed) &&
			(ti >= len(removed) || changed[ei].ChangeSeq < removed[ti].ChangeSeq)
		if takeEntry {
			entries = append(entries, changed[ei])
			next = SinceCursor(changed[ei].ChangeSeq)
			ei++
			continue
		}
		tombstones = append(tombstones, removed[ti])
		next = SinceCursor(removed[ti].ChangeSeq)
		ti++
	}
	return entries, tombstones, next, nil
}

// changedEntriesSince reads Entries whose ChangeSeq places them after cursor,
// oldest first.
func (s *Store) changedEntriesSince(ctx context.Context, cursor SinceCursor, limit int) ([]Entry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+entryColumns+`
		 FROM entries e JOIN feeds f ON f.id = e.feed_id
		 WHERE e.change_seq > ?
		 ORDER BY e.change_seq ASC LIMIT ?`, int64(cursor), limit)
	if err != nil {
		return nil, fmt.Errorf("read changed entries: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		entry, err := scanEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("read changed entries: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// tombstonesSince reads tombstones whose ChangeSeq places them after cursor,
// oldest first.
func (s *Store) tombstonesSince(ctx context.Context, cursor SinceCursor, limit int) ([]Tombstone, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT entry_id, deleted_at, change_seq FROM entry_tombstones
		 WHERE change_seq > ?
		 ORDER BY change_seq ASC LIMIT ?`, int64(cursor), limit)
	if err != nil {
		return nil, fmt.Errorf("read tombstones: %w", err)
	}
	defer rows.Close()

	var tombstones []Tombstone
	for rows.Next() {
		var tombstone Tombstone
		var deletedAt int64
		if err := rows.Scan(&tombstone.EntryID, &deletedAt, &tombstone.ChangeSeq); err != nil {
			return nil, fmt.Errorf("read tombstones: %w", err)
		}
		tombstone.DeletedAt = time.Unix(deletedAt, 0).UTC()
		tombstones = append(tombstones, tombstone)
	}
	return tombstones, rows.Err()
}

// CleanupExpiredEntries removes every unstarred Entry this reader has held
// for longer than maxAge, recording a tombstone for each so a delta reader
// learns it is gone. Age is measured from CreatedAt — when this reader first
// stored the Entry — rather than PublishedAt, so a Feed whose current
// document still lists an old item cannot have that item deleted and then
// re-inserted by the next poll forever. Starred is the only exemption:
// Archived confers none, and an Archived Entry expires on the same terms as
// any other unstarred one. An extracted Article is removed once no Entry
// references its URL any longer, so it outlives its own Entry exactly when a
// Starred Entry — never removed by this — still points at the same URL. It
// returns how many Entries were removed.
func (s *Store) CleanupExpiredEntries(ctx context.Context, maxAge time.Duration, now time.Time) (int, error) {
	if maxAge <= 0 {
		return 0, nil
	}
	cutoff := now.Add(-maxAge).Unix()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired entries: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx,
		`SELECT id, url FROM entries WHERE starred = 0 AND created_at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired entries: %w", err)
	}
	var ids []int64
	urls := make(map[string]bool)
	for rows.Next() {
		var id int64
		var url string
		if err := rows.Scan(&id, &url); err != nil {
			rows.Close()
			return 0, fmt.Errorf("cleanup expired entries: %w", err)
		}
		ids = append(ids, id)
		urls[url] = true
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("cleanup expired entries: %w", err)
	}
	rows.Close()

	if len(ids) == 0 {
		return 0, tx.Commit()
	}

	start, err := nextChangeSeqRange(ctx, tx, len(ids))
	if err != nil {
		return 0, fmt.Errorf("cleanup expired entries: %w", err)
	}

	// sqlChunkSize keeps every statement well under SQLite's bound-parameter
	// limit even for a retention run that expires many thousands of rows at
	// once, such as the first run against a database seeded by OPML import.
	const sqlChunkSize = 500
	for offset := 0; offset < len(ids); offset += sqlChunkSize {
		end := min(offset+sqlChunkSize, len(ids))
		batch := ids[offset:end]

		values := make([]byte, 0, len(batch)*8)
		args := make([]any, 0, len(batch)*3)
		for i, id := range batch {
			if i > 0 {
				values = append(values, ',', ' ')
			}
			values = append(values, "(?, ?, ?)"...)
			args = append(args, id, now.Unix(), start+int64(offset+i))
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO entry_tombstones (entry_id, deleted_at, change_seq)
			 VALUES `+string(values)+`
			 ON CONFLICT (entry_id) DO UPDATE SET deleted_at = excluded.deleted_at, change_seq = excluded.change_seq`,
			args...); err != nil {
			return 0, fmt.Errorf("cleanup expired entries: %w", err)
		}

		placeholders, idArgs := placeholderList(batch)
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM entries_fts WHERE rowid IN (`+placeholders+`)`, idArgs...); err != nil {
			return 0, fmt.Errorf("cleanup expired entries: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM entries WHERE id IN (`+placeholders+`)`, idArgs...); err != nil {
			return 0, fmt.Errorf("cleanup expired entries: %w", err)
		}
	}

	urlList := make([]string, 0, len(urls))
	for url := range urls {
		urlList = append(urlList, url)
	}
	for offset := 0; offset < len(urlList); offset += sqlChunkSize {
		end := min(offset+sqlChunkSize, len(urlList))
		urlPlaceholders, urlArgs := placeholderList(urlList[offset:end])
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM articles WHERE url IN (`+urlPlaceholders+`)
			 AND url NOT IN (SELECT DISTINCT url FROM entries)`, urlArgs...); err != nil {
			return 0, fmt.Errorf("cleanup expired entries: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("cleanup expired entries: %w", err)
	}
	return len(ids), nil
}

// placeholderList renders a "?, ?, ?" placeholder list sized to values, and
// the values themselves as the args slice a query using it needs.
func placeholderList[T any](values []T) (string, []any) {
	placeholders := make([]byte, 0, len(values)*3)
	args := make([]any, len(values))
	for i, v := range values {
		if i > 0 {
			placeholders = append(placeholders, ',', ' ')
		}
		placeholders = append(placeholders, '?')
		args[i] = v
	}
	return string(placeholders), args
}
