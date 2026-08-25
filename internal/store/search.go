package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/gabe-santos/rss-reader/internal/sanitize"
)

// searchLimit caps how many results of each kind a search returns: enough
// for a reader to scan, not a paginated list.
const searchLimit = 20

// EntrySearchResult is one Entry a search matched, plus a short excerpt of
// the content the match was found in.
type EntrySearchResult struct {
	Entry   Entry
	Snippet string
}

// execer is satisfied by both *sql.DB and *sql.Tx, so indexEntry can run
// standalone (backfill) or inside a caller's transaction (every write).
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// indexEntry replaces one Entry's row in the search index with the plain
// text of its current title and content. entries_fts stores its own copy
// rather than reading entries directly (an external-content FTS5 table)
// because reducing HTML to plain text — the point of the index, so a search
// matches words rather than markup — is not something a SQL trigger can do;
// this is the one place in the Store that keeps it in step, called from
// every path that changes an Entry's title or content, or removes it.
func indexEntry(ctx context.Context, exec execer, id int64, title, content string) error {
	if _, err := exec.ExecContext(ctx, `DELETE FROM entries_fts WHERE rowid = ?`, id); err != nil {
		return fmt.Errorf("index entry %d: %w", id, err)
	}
	if _, err := exec.ExecContext(ctx,
		`INSERT INTO entries_fts (rowid, title, content) VALUES (?, ?, ?)`,
		id, sanitize.PlainText(title), sanitize.PlainText(content)); err != nil {
		return fmt.Errorf("index entry %d: %w", id, err)
	}
	return nil
}

// backfillSearchIndex indexes every Entry the search index does not carry
// yet: nothing, once every Entry has been through indexEntry at least once,
// and every Entry a migration or an older run of this application stored
// before the index existed, the first time this runs against that database.
func backfillSearchIndex(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx,
		`SELECT e.id, e.title, e.content FROM entries e
		 LEFT JOIN entries_fts x ON x.rowid = e.id
		 WHERE x.rowid IS NULL`)
	if err != nil {
		return fmt.Errorf("backfill search index: %w", err)
	}

	type unindexed struct {
		id             int64
		title, content string
	}
	var pending []unindexed
	for rows.Next() {
		var entry unindexed
		if err := rows.Scan(&entry.id, &entry.title, &entry.content); err != nil {
			rows.Close()
			return fmt.Errorf("backfill search index: %w", err)
		}
		pending = append(pending, entry)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("backfill search index: %w", err)
	}
	rows.Close()

	for _, entry := range pending {
		if err := indexEntry(ctx, db, entry.id, entry.title, entry.content); err != nil {
			return fmt.Errorf("backfill search index: %w", err)
		}
	}
	return nil
}

// ftsQuery renders reader input as an FTS5 MATCH expression: every word
// becomes its own quoted, prefix-matched phrase, so punctuation and FTS5's
// own query syntax in the reader's text (a hyphen, a colon, the word "OR")
// cannot break the query or search on something they did not type. Phrases
// are ANDed, so adding a word narrows the result. Blank input has no terms.
func ftsQuery(raw string) string {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return ""
	}
	phrases := make([]string, 0, len(fields))
	for _, field := range fields {
		phrases = append(phrases, `"`+strings.ReplaceAll(field, `"`, `""`)+`"*`)
	}
	return strings.Join(phrases, " AND ")
}

// SearchEntries finds Entries whose title or Feed-supplied content matches
// query, best match first, each with a short plain-text excerpt of where it
// matched. Only what the Feed itself supplied is indexed, not an extracted
// Article. Blank query matches nothing.
func (s *Store) SearchEntries(ctx context.Context, query string) ([]EntrySearchResult, error) {
	match := ftsQuery(query)
	if match == "" {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+entryColumns+`, snippet(entries_fts, 1, '', '', '…', 12)
		 FROM entries_fts
		 JOIN entries e ON e.id = entries_fts.rowid
		 JOIN feeds f ON f.id = e.feed_id
		 WHERE entries_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`, match, searchLimit)
	if err != nil {
		return nil, fmt.Errorf("search entries: %w", err)
	}
	defer rows.Close()

	var results []EntrySearchResult
	for rows.Next() {
		var entry Entry
		var publishedAt, read, starred, archived, updatedAt int64
		var snippet string
		if err := rows.Scan(&entry.ID, &entry.FeedID, &entry.FeedTitle, &entry.GUID, &entry.Title,
			&entry.URL, &entry.Content, &publishedAt, &read, &starred, &archived, &updatedAt, &entry.ChangeSeq, &snippet); err != nil {
			return nil, fmt.Errorf("search entries: %w", err)
		}
		entry.PublishedAt = time.Unix(publishedAt, 0).UTC()
		entry.UpdatedAt = time.Unix(updatedAt, 0).UTC()
		entry.Read = read != 0
		entry.Starred = starred != 0
		entry.Archived = archived != 0
		results = append(results, EntrySearchResult{Entry: entry, Snippet: snippet})
	}
	return results, rows.Err()
}

// SearchFeeds finds Feeds whose name contains query, by name, so that search
// can also navigate to a Feed. Blank query matches nothing.
func (s *Store) SearchFeeds(ctx context.Context, query string) ([]Feed, error) {
	term := strings.TrimSpace(query)
	if term == "" {
		return nil, nil
	}
	pattern := "%" + escapeLike(term) + "%"

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+feedColumns+`
		 FROM feeds
		 WHERE title LIKE ? ESCAPE '\'
		 ORDER BY title COLLATE NOCASE, id
		 LIMIT ?`, pattern, searchLimit)
	if err != nil {
		return nil, fmt.Errorf("search feeds: %w", err)
	}
	defer rows.Close()

	var feeds []Feed
	for rows.Next() {
		feed, err := scanFeed(rows)
		if err != nil {
			return nil, fmt.Errorf("search feeds: %w", err)
		}
		feeds = append(feeds, feed)
	}
	return feeds, rows.Err()
}

// escapeLike escapes a LIKE pattern's own wildcard characters so a reader's
// literal % or _ is matched literally rather than as a wildcard.
func escapeLike(term string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(term)
}
