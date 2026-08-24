package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrFeedExists reports a Feed URL the reader is already subscribed to.
var ErrFeedExists = errors.New("already subscribed to this Feed")

// ErrNoFeed reports a Feed that is not there.
var ErrNoFeed = errors.New("no such Feed")

// ErrNoEntry reports an Entry that is not there.
var ErrNoEntry = errors.New("no such Entry")

// Feed is one subscription.
type Feed struct {
	ID        int64
	URL       string
	Title     string
	SiteURL   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Entry is one item a Feed carried. FeedTitle is filled by reads, not writes:
// it belongs to the Feed, and the reading list needs it beside every Entry.
type Entry struct {
	ID          int64
	FeedID      int64
	FeedTitle   string
	GUID        string
	Title       string
	URL         string
	Content     string
	PublishedAt time.Time
	// Read is set once the reader has seen this Entry, by hand or by opening
	// it. It is untouched by SaveEntries.
	Read bool
}

// Cursor is a position in the newest-first reading list. The zero value is the
// top of the list.
type Cursor struct {
	PublishedAt time.Time
	ID          int64
}

// IsZero reports whether the cursor points at the top of the list.
func (c Cursor) IsZero() bool { return c.ID == 0 && c.PublishedAt.IsZero() }

// EntryQuery selects a page of the reading list.
type EntryQuery struct {
	// FeedID scopes the page to one Feed; zero means every Feed.
	FeedID int64
	// UnreadOnly scopes the page to Entries not yet Read.
	UnreadOnly bool
	// After is the position the last page ended at.
	After Cursor
	// Limit is the largest number of Entries to return.
	Limit int
}

// CreateFeed stores a new subscription and returns it with its assigned id. It
// returns ErrFeedExists when this Feed URL is already subscribed.
func (s *Store) CreateFeed(ctx context.Context, feed Feed, now time.Time) (Feed, error) {
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO feeds (url, title, site_url, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (url) DO NOTHING
		 RETURNING id`,
		feed.URL, feed.Title, feed.SiteURL, now.Unix(), now.Unix()).Scan(&feed.ID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Feed{}, ErrFeedExists
	case err != nil:
		return Feed{}, fmt.Errorf("create feed %s: %w", feed.URL, err)
	}
	feed.CreatedAt = now.UTC()
	feed.UpdatedAt = now.UTC()
	return feed, nil
}

// Feed reads one subscription. It returns ErrNoFeed when there is no such Feed.
func (s *Store) Feed(ctx context.Context, id int64) (Feed, error) {
	var feed Feed
	var createdAt, updatedAt int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, url, title, site_url, created_at, updated_at FROM feeds WHERE id = ?`, id).
		Scan(&feed.ID, &feed.URL, &feed.Title, &feed.SiteURL, &createdAt, &updatedAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Feed{}, ErrNoFeed
	case err != nil:
		return Feed{}, fmt.Errorf("read feed %d: %w", id, err)
	}
	feed.CreatedAt = time.Unix(createdAt, 0).UTC()
	feed.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return feed, nil
}

// Feeds reads the whole collection, by title.
func (s *Store) Feeds(ctx context.Context) ([]Feed, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, url, title, site_url, created_at, updated_at
		 FROM feeds ORDER BY title COLLATE NOCASE, id`)
	if err != nil {
		return nil, fmt.Errorf("read feeds: %w", err)
	}
	defer rows.Close()

	var feeds []Feed
	for rows.Next() {
		var feed Feed
		var createdAt, updatedAt int64
		if err := rows.Scan(&feed.ID, &feed.URL, &feed.Title, &feed.SiteURL, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("read feeds: %w", err)
		}
		feed.CreatedAt = time.Unix(createdAt, 0).UTC()
		feed.UpdatedAt = time.Unix(updatedAt, 0).UTC()
		feeds = append(feeds, feed)
	}
	return feeds, rows.Err()
}

// SaveEntries stores what a Feed carried, in one transaction. An Entry the
// publisher has shown before is updated in place when anything about it changed
// and left alone when nothing did, so that seeing the same item twice — in one
// document or across two fetches — stores it once.
func (s *Store) SaveEntries(ctx context.Context, feedID int64, entries []Entry, now time.Time) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("save entries: %w", err)
	}
	defer tx.Rollback()

	statement, err := tx.PrepareContext(ctx,
		`INSERT INTO entries (feed_id, guid, title, url, content, published_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (feed_id, guid) DO UPDATE SET
		     title = excluded.title,
		     url = excluded.url,
		     content = excluded.content,
		     published_at = excluded.published_at,
		     updated_at = excluded.updated_at
		 WHERE title <> excluded.title
		    OR url <> excluded.url
		    OR content <> excluded.content
		    OR published_at <> excluded.published_at`)
	if err != nil {
		return fmt.Errorf("save entries: %w", err)
	}
	defer statement.Close()

	for _, entry := range entries {
		if _, err := statement.ExecContext(ctx,
			feedID, entry.GUID, entry.Title, entry.URL, entry.Content,
			entry.PublishedAt.Unix(), now.Unix(), now.Unix()); err != nil {
			return fmt.Errorf("save entry %q of feed %d: %w", entry.GUID, feedID, err)
		}
	}
	return tx.Commit()
}

// Entries reads one page of the reading list, newest first.
func (s *Store) Entries(ctx context.Context, q EntryQuery) ([]Entry, error) {
	where := make([]string, 0, 3)
	args := make([]any, 0, 4)
	if q.FeedID != 0 {
		where = append(where, "e.feed_id = ?")
		args = append(args, q.FeedID)
	}
	if q.UnreadOnly {
		where = append(where, "e.read = 0")
	}
	if !q.After.IsZero() {
		// Keyset paging: strictly older than the last Entry of the page before,
		// with the id settling identical timestamps.
		where = append(where, "(e.published_at < ? OR (e.published_at = ? AND e.id < ?))")
		args = append(args, q.After.PublishedAt.Unix(), q.After.PublishedAt.Unix(), q.After.ID)
	}

	query := `SELECT e.id, e.feed_id, f.title, e.guid, e.title, e.url, e.content, e.published_at, e.read
		 FROM entries e JOIN feeds f ON f.id = e.feed_id`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY e.published_at DESC, e.id DESC LIMIT ?"
	args = append(args, q.Limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("read entries: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var entry Entry
		var publishedAt int64
		var read int64
		if err := rows.Scan(&entry.ID, &entry.FeedID, &entry.FeedTitle, &entry.GUID,
			&entry.Title, &entry.URL, &entry.Content, &publishedAt, &read); err != nil {
			return nil, fmt.Errorf("read entries: %w", err)
		}
		entry.PublishedAt = time.Unix(publishedAt, 0).UTC()
		entry.Read = read != 0
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// Entry reads one Entry by id. It returns ErrNoEntry when there is no such
// Entry.
func (s *Store) Entry(ctx context.Context, id int64) (Entry, error) {
	var entry Entry
	var publishedAt, read int64
	err := s.db.QueryRowContext(ctx,
		`SELECT e.id, e.feed_id, f.title, e.guid, e.title, e.url, e.content, e.published_at, e.read
		 FROM entries e JOIN feeds f ON f.id = e.feed_id
		 WHERE e.id = ?`, id).
		Scan(&entry.ID, &entry.FeedID, &entry.FeedTitle, &entry.GUID, &entry.Title,
			&entry.URL, &entry.Content, &publishedAt, &read)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Entry{}, ErrNoEntry
	case err != nil:
		return Entry{}, fmt.Errorf("read entry %d: %w", id, err)
	}
	entry.PublishedAt = time.Unix(publishedAt, 0).UTC()
	entry.Read = read != 0
	return entry, nil
}

// SetEntryRead sets an Entry's Read state by hand — an idempotent declaration,
// not a toggle — overriding whatever opening it did automatically, and returns
// the Entry as stored. It returns ErrNoEntry when there is no such Entry.
func (s *Store) SetEntryRead(ctx context.Context, id int64, read bool, now time.Time) (Entry, error) {
	readValue := 0
	if read {
		readValue = 1
	}
	result, err := s.db.ExecContext(ctx,
		`UPDATE entries SET read = ?, updated_at = ? WHERE id = ?`, readValue, now.Unix(), id)
	if err != nil {
		return Entry{}, fmt.Errorf("set entry read: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Entry{}, fmt.Errorf("set entry read: %w", err)
	}
	if affected == 0 {
		return Entry{}, ErrNoEntry
	}
	return s.Entry(ctx, id)
}
