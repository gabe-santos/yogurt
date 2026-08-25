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
	GroupID   int64
	Suspended bool
	CreatedAt time.Time
	UpdatedAt time.Time

	// ETag and LastModified are the validators from the last response this
	// application read, sent back as conditional-request headers so an
	// unchanged Feed can be confirmed without resending its whole document.
	ETag         string
	LastModified string
	// NextCheckAt is when the schedule should next check this Feed, per
	// pullpolicy. The zero value means due now.
	NextCheckAt time.Time
	// LastCheckedAt and LastSuccessAt are the zero value until this Feed's
	// first check, so a Feed the schedule has not reached yet is
	// distinguishable from one that keeps failing.
	LastCheckedAt time.Time
	LastSuccessAt time.Time
	// LastError is what the most recent failed check reported, empty after a
	// successful one.
	LastError string
	// ConsecutiveFailures counts failed checks since the last success, and
	// drives backoff.
	ConsecutiveFailures int
	// IconStoredAt is when the Feed Icon was last stored, the zero value
	// when the Feed has none. It also serves as the icon endpoint's
	// cache-busting version.
	IconStoredAt time.Time
	// IconCheckedAt is when this Feed was last checked for an icon,
	// regardless of outcome, the zero value when it has never been checked.
	// It gates re-probing a site that has no usable icon.
	IconCheckedAt time.Time
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
	// UpdatedAt is when this Entry last changed, by SaveEntries or by the
	// reader's own state.
	UpdatedAt time.Time
	// ChangeSeq is this Entry's position in the changed-since feed's total
	// order, per ADR-0004: assigned fresh whenever SaveEntries or the
	// reader's own state actually changes it. Entry ids cannot serve this —
	// an id is assigned once at creation and never renumbered by a later
	// update.
	ChangeSeq int64
	// Read, Starred, and Archived belong to the reader and are untouched by
	// SaveEntries. Archived always implies Read.
	Read     bool
	Starred  bool
	Archived bool
}

// Cursor is a position in the newest-first reading list. The zero value is the
// top of the list.
type Cursor struct {
	PublishedAt time.Time
	ID          int64
}

// IsZero reports whether the cursor points at the top of the list.
func (c Cursor) IsZero() bool { return c.ID == 0 && c.PublishedAt.IsZero() }

// EntrySelection is the filter and scope shared by list and bulk state changes.
type EntrySelection struct {
	// FeedID scopes the selection to one Feed; zero means every Feed.
	FeedID int64
	// GroupID scopes the selection to one Group; zero means every Group. Ignored
	// when FeedID is set.
	GroupID int64
	// UnreadOnly and StarredOnly narrow the active reading list. ArchivedOnly
	// selects the Archive instead; every other selection excludes Archived Entries.
	UnreadOnly   bool
	StarredOnly  bool
	ArchivedOnly bool
}

type EntryQuery struct {
	EntrySelection
	// OldestFirst reverses the default newest-first publish-date order.
	OldestFirst bool
	// After is the position the last page ended at.
	After Cursor
	// Around is an Entry id to start the page at, inclusive, rather than at
	// the top of the list: how a search result opens within its ordinary
	// list instead of a standalone view. Zero means no anchor. It takes
	// precedence over After when both are set, and the Entry it names is
	// only returned if the query's own selection matches it.
	Around int64
	// Limit is the largest number of Entries to return.
	Limit int
}

// CreateFeed stores a new subscription and returns it with its assigned id. A
// zero GroupID is resolved to the default Group, so a Feed is never
// unreachable. It returns ErrFeedExists when this Feed URL is already
// subscribed.
func (s *Store) CreateFeed(ctx context.Context, feed Feed, now time.Time) (Feed, error) {
	if feed.GroupID == 0 {
		groupID, err := s.defaultGroupID(ctx)
		if err != nil {
			return Feed{}, fmt.Errorf("create feed %s: %w", feed.URL, err)
		}
		feed.GroupID = groupID
	}

	err := s.db.QueryRowContext(ctx,
		`INSERT INTO feeds (url, title, site_url, group_id, suspended, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (url) DO NOTHING
		 RETURNING id`,
		feed.URL, feed.Title, feed.SiteURL, feed.GroupID, boolToInt(feed.Suspended), now.Unix(), now.Unix()).Scan(&feed.ID)
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

const feedColumns = `id, url, title, site_url, group_id, suspended, created_at, updated_at,
	etag, last_modified, next_check_at, last_checked_at, last_success_at, last_error, consecutive_failures,
	icon_stored_at, icon_checked_at`

func scanFeed(row rowScanner) (Feed, error) {
	var feed Feed
	var suspended int64
	var createdAt, updatedAt, nextCheckAt, lastCheckedAt, lastSuccessAt int64
	var iconStoredAt, iconCheckedAt int64
	if err := row.Scan(&feed.ID, &feed.URL, &feed.Title, &feed.SiteURL, &feed.GroupID,
		&suspended, &createdAt, &updatedAt,
		&feed.ETag, &feed.LastModified, &nextCheckAt, &lastCheckedAt, &lastSuccessAt,
		&feed.LastError, &feed.ConsecutiveFailures,
		&iconStoredAt, &iconCheckedAt); err != nil {
		return Feed{}, err
	}
	feed.Suspended = suspended != 0
	feed.CreatedAt = time.Unix(createdAt, 0).UTC()
	feed.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	feed.NextCheckAt = unixOrZero(nextCheckAt)
	feed.LastCheckedAt = unixOrZero(lastCheckedAt)
	feed.LastSuccessAt = unixOrZero(lastSuccessAt)
	feed.IconStoredAt = unixOrZero(iconStoredAt)
	feed.IconCheckedAt = unixOrZero(iconCheckedAt)
	return feed, nil
}

// unixOrZero reads a stored "0 means unset" epoch column as Go's zero Time,
// rather than the misleading instant 1970-01-01.
func unixOrZero(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.Unix(v, 0).UTC()
}

// Feed reads one subscription. It returns ErrNoFeed when there is no such Feed.
func (s *Store) Feed(ctx context.Context, id int64) (Feed, error) {
	feed, err := scanFeed(s.db.QueryRowContext(ctx,
		`SELECT `+feedColumns+` FROM feeds WHERE id = ?`, id))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Feed{}, ErrNoFeed
	case err != nil:
		return Feed{}, fmt.Errorf("read feed %d: %w", id, err)
	}
	return feed, nil
}

// Feeds reads the whole collection, by title.
func (s *Store) Feeds(ctx context.Context) ([]Feed, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+feedColumns+` FROM feeds ORDER BY title COLLATE NOCASE, id`)
	if err != nil {
		return nil, fmt.Errorf("read feeds: %w", err)
	}
	defer rows.Close()

	var feeds []Feed
	for rows.Next() {
		feed, err := scanFeed(rows)
		if err != nil {
			return nil, fmt.Errorf("read feeds: %w", err)
		}
		feeds = append(feeds, feed)
	}
	return feeds, rows.Err()
}

// FeedPatch declares the fields of a Feed the reader wants to change; a nil
// field is left as stored, so title, Group, and suspension can be changed
// independently of one another in a single idempotent declaration.
type FeedPatch struct {
	Title     *string
	GroupID   *int64
	Suspended *bool
}

// UpdateFeed applies a FeedPatch to a Feed in one transaction, so a Group
// that turns out not to exist changes nothing rather than leaving the Feed
// half-updated. It returns ErrNoFeed when there is no such Feed, and
// ErrNoGroup when GroupID names a Group that is not there.
func (s *Store) UpdateFeed(ctx context.Context, id int64, patch FeedPatch, now time.Time) (Feed, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Feed{}, fmt.Errorf("update feed %d: %w", id, err)
	}
	defer tx.Rollback()

	var exists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM feeds WHERE id = ?`, id).Scan(&exists)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Feed{}, ErrNoFeed
	case err != nil:
		return Feed{}, fmt.Errorf("update feed %d: %w", id, err)
	}

	if patch.GroupID != nil {
		var groupExists int
		err = tx.QueryRowContext(ctx, `SELECT 1 FROM groups WHERE id = ?`, *patch.GroupID).Scan(&groupExists)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return Feed{}, ErrNoGroup
		case err != nil:
			return Feed{}, fmt.Errorf("update feed %d: %w", id, err)
		}
	}

	if patch.Title != nil {
		if _, err := tx.ExecContext(ctx,
			`UPDATE feeds SET title = ?, updated_at = ? WHERE id = ?`, *patch.Title, now.Unix(), id); err != nil {
			return Feed{}, fmt.Errorf("update feed %d: %w", id, err)
		}
	}
	if patch.GroupID != nil {
		if _, err := tx.ExecContext(ctx,
			`UPDATE feeds SET group_id = ?, updated_at = ? WHERE id = ?`, *patch.GroupID, now.Unix(), id); err != nil {
			return Feed{}, fmt.Errorf("update feed %d: %w", id, err)
		}
	}
	if patch.Suspended != nil {
		if _, err := tx.ExecContext(ctx,
			`UPDATE feeds SET suspended = ?, updated_at = ? WHERE id = ?`,
			boolToInt(*patch.Suspended), now.Unix(), id); err != nil {
			return Feed{}, fmt.Errorf("update feed %d: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Feed{}, fmt.Errorf("update feed %d: %w", id, err)
	}
	return s.Feed(ctx, id)
}

// DeleteFeed removes a Feed and every Entry it carried, in one transaction. It
// returns ErrNoFeed when there is no such Feed.
func (s *Store) DeleteFeed(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete feed %d: %w", id, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM entries_fts WHERE rowid IN (SELECT id FROM entries WHERE feed_id = ?)`, id); err != nil {
		return fmt.Errorf("delete feed %d: %w", id, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM entries WHERE feed_id = ?`, id); err != nil {
		return fmt.Errorf("delete feed %d: %w", id, err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM feeds WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete feed %d: %w", id, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete feed %d: %w", id, err)
	}
	if affected == 0 {
		return ErrNoFeed
	}
	return tx.Commit()
}

// FetchResult declares what the store should now believe about a Feed's
// remote state after one attempt to check it, successful or not. It is
// computed by the pull package, not the store: pullpolicy decides NextCheckAt,
// and the store only persists the decision.
type FetchResult struct {
	ETag         string
	LastModified string
	Success      bool
	// Error is the failure's message. Ignored when Success is true.
	Error string
	// ConsecutiveFailures is the new count after this attempt. Ignored when
	// Success is true, since a success resets it to zero.
	ConsecutiveFailures int
	NextCheckAt         time.Time
}

// RecordFetchResult updates a Feed's fetch state after an attempt to check it.
// A successful attempt clears the error and failure count and advances both
// last_checked_at and last_success_at; a failed attempt advances only
// last_checked_at and records why. It returns ErrNoFeed when there is no such
// Feed.
func (s *Store) RecordFetchResult(ctx context.Context, id int64, result FetchResult, now time.Time) error {
	lastError := ""
	failures := 0
	if !result.Success {
		lastError = result.Error
		failures = result.ConsecutiveFailures
	}

	query := `UPDATE feeds SET etag = ?, last_modified = ?, last_checked_at = ?,
		last_error = ?, consecutive_failures = ?, next_check_at = ?, updated_at = ?`
	args := []any{
		result.ETag, result.LastModified, now.Unix(),
		lastError, failures, result.NextCheckAt.Unix(), now.Unix(),
	}
	if result.Success {
		query += `, last_success_at = ?`
		args = append(args, now.Unix())
	}
	query += ` WHERE id = ?`
	args = append(args, id)

	outcome, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("record fetch result for feed %d: %w", id, err)
	}
	affected, err := outcome.RowsAffected()
	if err != nil {
		return fmt.Errorf("record fetch result for feed %d: %w", id, err)
	}
	if affected == 0 {
		return ErrNoFeed
	}
	return nil
}

// ErrNoIcon reports a Feed that exists but has never stored a usable Feed
// Icon.
var ErrNoIcon = errors.New("no such Feed Icon")

// FeedIcon reads a Feed's stored Feed Icon. It returns ErrNoFeed when there
// is no such Feed, and ErrNoIcon when the Feed has none stored.
func (s *Store) FeedIcon(ctx context.Context, id int64) (data []byte, mediaType string, err error) {
	var iconStoredAt int64
	err = s.db.QueryRowContext(ctx,
		`SELECT icon_data, icon_media_type, icon_stored_at FROM feeds WHERE id = ?`, id,
	).Scan(&data, &mediaType, &iconStoredAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, "", ErrNoFeed
	case err != nil:
		return nil, "", fmt.Errorf("read feed icon %d: %w", id, err)
	}
	if iconStoredAt == 0 || len(data) == 0 {
		return nil, "", ErrNoIcon
	}
	return data, mediaType, nil
}

// SetFeedIcon stores a Feed's Feed Icon, advancing both its stored-at and
// checked-at timestamps: stored-at so the serving endpoint's cache-busting
// version changes, checked-at so this Feed is not re-probed again within the
// re-probe window. It returns ErrNoFeed when there is no such Feed.
func (s *Store) SetFeedIcon(ctx context.Context, id int64, data []byte, mediaType string, now time.Time) error {
	outcome, err := s.db.ExecContext(ctx,
		`UPDATE feeds SET icon_data = ?, icon_media_type = ?, icon_stored_at = ?, icon_checked_at = ? WHERE id = ?`,
		data, mediaType, now.Unix(), now.Unix(), id)
	if err != nil {
		return fmt.Errorf("set feed icon %d: %w", id, err)
	}
	affected, err := outcome.RowsAffected()
	if err != nil {
		return fmt.Errorf("set feed icon %d: %w", id, err)
	}
	if affected == 0 {
		return ErrNoFeed
	}
	return nil
}

// MarkFeedIconChecked records that a Feed was checked for an icon and none
// was found, advancing icon_checked_at without touching any icon already
// stored — so a site that briefly stops advertising one does not cost the
// reader an icon it already has. It returns ErrNoFeed when there is no such
// Feed.
func (s *Store) MarkFeedIconChecked(ctx context.Context, id int64, now time.Time) error {
	outcome, err := s.db.ExecContext(ctx,
		`UPDATE feeds SET icon_checked_at = ? WHERE id = ?`, now.Unix(), id)
	if err != nil {
		return fmt.Errorf("mark feed icon checked %d: %w", id, err)
	}
	affected, err := outcome.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark feed icon checked %d: %w", id, err)
	}
	if affected == 0 {
		return ErrNoFeed
	}
	return nil
}

// DueFeeds reads every non-suspended Feed whose schedule says it should be
// checked by now, soonest-due first. This is what the background scheduler
// polls; a manual refresh reads every Feed instead, regardless of schedule or
// suspension.
func (s *Store) DueFeeds(ctx context.Context, now time.Time) ([]Feed, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+feedColumns+` FROM feeds WHERE suspended = 0 AND next_check_at <= ? ORDER BY next_check_at`,
		now.Unix())
	if err != nil {
		return nil, fmt.Errorf("read due feeds: %w", err)
	}
	defer rows.Close()

	var feeds []Feed
	for rows.Next() {
		feed, err := scanFeed(rows)
		if err != nil {
			return nil, fmt.Errorf("read due feeds: %w", err)
		}
		feeds = append(feeds, feed)
	}
	return feeds, rows.Err()
}

// FeedUnreadCounts reads the number of unread Entries per Feed, omitting a
// Feed with none, so callers know the rest are zero.
func (s *Store) FeedUnreadCounts(ctx context.Context) (map[int64]int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT feed_id, COUNT(*) FROM entries WHERE read = 0 GROUP BY feed_id`)
	if err != nil {
		return nil, fmt.Errorf("read unread counts: %w", err)
	}
	defer rows.Close()

	counts := make(map[int64]int)
	for rows.Next() {
		var feedID int64
		var count int
		if err := rows.Scan(&feedID, &count); err != nil {
			return nil, fmt.Errorf("read unread counts: %w", err)
		}
		counts[feedID] = count
	}
	return counts, rows.Err()
}

// SaveEntries stores what a Feed carried, in one transaction. An Entry the
// publisher has shown before is updated in place when anything about it changed
// and left alone when nothing did, so that seeing the same item twice — in one
// document or across two fetches — stores it once. The search index is
// updated for exactly the Entries that changed, alongside the row itself.
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
		`INSERT INTO entries (feed_id, guid, title, url, content, published_at, created_at, updated_at, change_seq)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (feed_id, guid) DO UPDATE SET
		     title = excluded.title,
		     url = excluded.url,
		     content = excluded.content,
		     published_at = excluded.published_at,
		     updated_at = excluded.updated_at,
		     change_seq = excluded.change_seq
		 WHERE title <> excluded.title
		    OR url <> excluded.url
		    OR content <> excluded.content
		    OR published_at <> excluded.published_at
		 RETURNING id`)
	if err != nil {
		return fmt.Errorf("save entries: %w", err)
	}
	defer statement.Close()

	for _, entry := range entries {
		seq, err := nextChangeSeq(ctx, tx)
		if err != nil {
			return fmt.Errorf("save entry %q of feed %d: %w", entry.GUID, feedID, err)
		}
		var id int64
		err = statement.QueryRowContext(ctx,
			feedID, entry.GUID, entry.Title, entry.URL, entry.Content,
			entry.PublishedAt.Unix(), now.Unix(), now.Unix(), seq).Scan(&id)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			// The WHERE clause found nothing changed: no row was touched, so
			// nothing was returned and the search index is already correct.
			continue
		case err != nil:
			return fmt.Errorf("save entry %q of feed %d: %w", entry.GUID, feedID, err)
		}
		if err := indexEntry(ctx, tx, id, entry.Title, entry.Content); err != nil {
			return fmt.Errorf("save entry %q of feed %d: %w", entry.GUID, feedID, err)
		}
	}
	return tx.Commit()
}

// entryWhere renders the one filter-and-scope definition used by both listing
// and bulk state declarations, so mark-all-read cannot select more than the UI.
func entryWhere(selection EntrySelection) ([]string, []any) {
	where := make([]string, 0, 5)
	args := make([]any, 0, 2)
	if selection.FeedID != 0 {
		where = append(where, "e.feed_id = ?")
		args = append(args, selection.FeedID)
	} else if selection.GroupID != 0 {
		where = append(where, "f.group_id = ?")
		args = append(args, selection.GroupID)
	}
	if selection.ArchivedOnly {
		where = append(where, "e.archived = 1")
	} else {
		where = append(where, "e.archived = 0")
		if selection.UnreadOnly {
			where = append(where, "e.read = 0")
		}
		if selection.StarredOnly {
			where = append(where, "e.starred = 1")
		}
	}
	return where, args
}

// entryColumns is the column list every Entry read selects, aliased for the
// entries-joined-to-feeds shape every one of those reads uses.
const entryColumns = `e.id, e.feed_id, f.title, e.guid, e.title, e.url, e.content,
	e.published_at, e.read, e.starred, e.archived, e.updated_at, e.change_seq`

func scanEntry(row rowScanner) (Entry, error) {
	var entry Entry
	var publishedAt, read, starred, archived, updatedAt int64
	if err := row.Scan(&entry.ID, &entry.FeedID, &entry.FeedTitle, &entry.GUID, &entry.Title,
		&entry.URL, &entry.Content, &publishedAt, &read, &starred, &archived, &updatedAt, &entry.ChangeSeq); err != nil {
		return Entry{}, err
	}
	entry.PublishedAt = time.Unix(publishedAt, 0).UTC()
	entry.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	entry.Read = read != 0
	entry.Starred = starred != 0
	entry.Archived = archived != 0
	return entry, nil
}

// Entries reads one page of the reading list in publish-date order, newest
// first unless OldestFirst is set. Archived Entries are excluded unless
// ArchivedOnly selects the Archive view. It returns ErrNoEntry when Around
// names an Entry that is not there.
func (s *Store) Entries(ctx context.Context, q EntryQuery) ([]Entry, error) {
	where, args := entryWhere(q.EntrySelection)
	switch {
	case q.Around != 0:
		// Inclusive of the anchor itself, so the page it opens starts at the
		// Entry a search result pointed at.
		anchor, err := s.Entry(ctx, q.Around)
		if err != nil {
			return nil, err
		}
		if q.OldestFirst {
			where = append(where, "(e.published_at > ? OR (e.published_at = ? AND e.id >= ?))")
		} else {
			where = append(where, "(e.published_at < ? OR (e.published_at = ? AND e.id <= ?))")
		}
		args = append(args, anchor.PublishedAt.Unix(), anchor.PublishedAt.Unix(), anchor.ID)
	case !q.After.IsZero():
		// Keyset paging: strictly beyond the last Entry of the page before in
		// the requested direction, with the id settling identical timestamps.
		if q.OldestFirst {
			where = append(where, "(e.published_at > ? OR (e.published_at = ? AND e.id > ?))")
		} else {
			where = append(where, "(e.published_at < ? OR (e.published_at = ? AND e.id < ?))")
		}
		args = append(args, q.After.PublishedAt.Unix(), q.After.PublishedAt.Unix(), q.After.ID)
	}
	order := "DESC"
	if q.OldestFirst {
		order = "ASC"
	}

	query := `SELECT ` + entryColumns + `
		 FROM entries e JOIN feeds f ON f.id = e.feed_id
		 WHERE ` + strings.Join(where, " AND ") +
		" ORDER BY e.published_at " + order + ", e.id " + order + " LIMIT ?"
	args = append(args, q.Limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("read entries: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		entry, err := scanEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("read entries: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// Entry reads one Entry by id. It returns ErrNoEntry when there is no such
// Entry.
func (s *Store) Entry(ctx context.Context, id int64) (Entry, error) {
	entry, err := scanEntry(s.db.QueryRowContext(ctx,
		`SELECT `+entryColumns+`
		 FROM entries e JOIN feeds f ON f.id = e.feed_id
		 WHERE e.id = ?`, id))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Entry{}, ErrNoEntry
	case err != nil:
		return Entry{}, fmt.Errorf("read entry %d: %w", id, err)
	}
	return entry, nil
}

// EntryState is the complete reader-owned state of an Entry. It is declared as
// values rather than toggles so replaying the same mutation is harmless.
type EntryState struct {
	Read     bool
	Starred  bool
	Archived bool
}

// SetEntryState declares an Entry's reader-owned state and returns it as
// stored. Archived always implies Read. A replay leaves updated_at and
// change_seq untouched.
func (s *Store) SetEntryState(ctx context.Context, id int64, state EntryState, now time.Time) (Entry, error) {
	if state.Archived {
		state.Read = true
	}
	read, starred, archived := boolToInt(state.Read), boolToInt(state.Starred), boolToInt(state.Archived)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Entry{}, fmt.Errorf("set entry state: %w", err)
	}
	defer tx.Rollback()

	seq, err := nextChangeSeq(ctx, tx)
	if err != nil {
		return Entry{}, fmt.Errorf("set entry state: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE entries SET read = ?, starred = ?, archived = ?, updated_at = ?, change_seq = ?
		 WHERE id = ? AND (read <> ? OR starred <> ? OR archived <> ?)`,
		read, starred, archived, now.Unix(), seq, id, read, starred, archived); err != nil {
		return Entry{}, fmt.Errorf("set entry state: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Entry{}, fmt.Errorf("set entry state: %w", err)
	}
	return s.Entry(ctx, id)
}

// MarkEntriesRead marks every Entry in selection Read. It uses the same
// selection as Entries and updates only unread rows, so a replay has no
// additional effect and Archived can never be made unread through this path.
// Each row draws its own change_seq — sharing one across the batch would let
// a paged delta read that stops mid-batch skip every row still to come.
func (s *Store) MarkEntriesRead(ctx context.Context, selection EntrySelection, now time.Time) error {
	where, selectionArgs := entryWhere(selection)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mark Entries read: %w", err)
	}
	defer tx.Rollback()

	idRows, err := tx.QueryContext(ctx,
		`SELECT e.id FROM entries e JOIN feeds f ON f.id = e.feed_id
		 WHERE `+strings.Join(where, " AND ")+" AND e.read = 0",
		selectionArgs...)
	if err != nil {
		return fmt.Errorf("mark Entries read: %w", err)
	}
	var ids []int64
	for idRows.Next() {
		var id int64
		if err := idRows.Scan(&id); err != nil {
			idRows.Close()
			return fmt.Errorf("mark Entries read: %w", err)
		}
		ids = append(ids, id)
	}
	if err := idRows.Err(); err != nil {
		return fmt.Errorf("mark Entries read: %w", err)
	}
	idRows.Close()

	if len(ids) == 0 {
		return tx.Commit()
	}

	start, err := nextChangeSeqRange(ctx, tx, len(ids))
	if err != nil {
		return fmt.Errorf("mark Entries read: %w", err)
	}

	// sqlChunkSize keeps every statement well under SQLite's bound-parameter
	// limit even when mark-all-read fires over a large backlog.
	const sqlChunkSize = 500
	for offset := 0; offset < len(ids); offset += sqlChunkSize {
		end := min(offset+sqlChunkSize, len(ids))
		batch := ids[offset:end]

		values := make([]byte, 0, len(batch)*24)
		args := make([]any, 0, len(batch)*2+1)
		args = append(args, now.Unix())
		for i, id := range batch {
			if i > 0 {
				values = append(values, " UNION ALL "...)
			}
			values = append(values, "SELECT ? AS id, ? AS seq"...)
			args = append(args, id, start+int64(offset+i))
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE entries SET read = 1, updated_at = ?, change_seq = v.seq
			 FROM (`+string(values)+`) AS v
			 WHERE entries.id = v.id`,
			args...); err != nil {
			return fmt.Errorf("mark Entries read: %w", err)
		}
	}
	return tx.Commit()
}
