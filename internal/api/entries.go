package api

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gabe-santos/rss-reader/internal/store"
)

// Page sizes for the reading list.
const (
	defaultPageSize = 50
	maxPageSize     = 200
)

// entryView is one Entry as the API presents it.
type entryView struct {
	ID          int64     `json:"id"`
	FeedID      int64     `json:"feed_id"`
	FeedTitle   string    `json:"feed_title"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
	Content     string    `json:"content"`
}

func viewEntry(entry store.Entry) entryView {
	return entryView{
		ID:          entry.ID,
		FeedID:      entry.FeedID,
		FeedTitle:   entry.FeedTitle,
		Title:       entry.Title,
		URL:         entry.URL,
		PublishedAt: entry.PublishedAt,
		Content:     entry.Content,
	}
}

// listEntries is the reading list: newest first, one page at a time, optionally
// scoped to a single Feed.
func (h *Handler) listEntries(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	q := store.EntryQuery{Limit: defaultPageSize}
	if raw := query.Get("feed"); raw != "" {
		feedID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			h.writeError(w, r, http.StatusBadRequest, "feed must be a Feed id")
			return
		}
		q.FeedID = feedID
	}
	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			h.writeError(w, r, http.StatusBadRequest, "limit must be a positive number")
			return
		}
		q.Limit = min(limit, maxPageSize)
	}
	if raw := query.Get("cursor"); raw != "" {
		cursor, err := decodeCursor(raw)
		if err != nil {
			h.writeError(w, r, http.StatusBadRequest, "that cursor is not one this server issued")
			return
		}
		q.After = cursor
	}

	// One more than asked for: whether a further page exists is a fact about the
	// data, not a guess from a full page.
	q.Limit++
	entries, err := h.deps.Store.Entries(r.Context(), q)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	var next string
	if len(entries) == q.Limit {
		entries = entries[:len(entries)-1]
		next = encodeCursor(entries[len(entries)-1])
	}

	views := make([]entryView, 0, len(entries))
	for _, entry := range entries {
		views = append(views, viewEntry(entry))
	}
	h.writeJSON(w, r, http.StatusOK, map[string]any{"entries": views, "next_cursor": next})
}

// encodeCursor packs an Entry's position in the newest-first list into a token
// the caller cannot construct, so that paging stays the server's business.
func encodeCursor(entry store.Entry) string {
	position := fmt.Sprintf("%d:%d", entry.PublishedAt.Unix(), entry.ID)
	return base64.RawURLEncoding.EncodeToString([]byte(position))
}

func decodeCursor(raw string) (store.Cursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return store.Cursor{}, fmt.Errorf("decode cursor: %w", err)
	}
	publishedAt, id, found := strings.Cut(string(decoded), ":")
	if !found {
		return store.Cursor{}, errors.New("cursor has no position in it")
	}
	seconds, err := strconv.ParseInt(publishedAt, 10, 64)
	if err != nil {
		return store.Cursor{}, fmt.Errorf("cursor timestamp: %w", err)
	}
	entryID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return store.Cursor{}, fmt.Errorf("cursor entry: %w", err)
	}
	return store.Cursor{PublishedAt: time.Unix(seconds, 0).UTC(), ID: entryID}, nil
}
