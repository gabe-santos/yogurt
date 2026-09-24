package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gabe-santos/yogurt/internal/extraction"
	"github.com/gabe-santos/yogurt/internal/sanitize"
	"github.com/gabe-santos/yogurt/internal/store"
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
	Read        bool      `json:"read"`
	Starred     bool      `json:"starred"`
	Archived    bool      `json:"archived"`
}

// viewEntry presents an Entry, sanitising its publisher-supplied content
// against dangerous markup here rather than at ingest: the stored Entry stays
// what the publisher sent, per CONTEXT.md, and every reader — including one
// stored before this sanitisation existed — gets the same guarantee.
func viewEntry(entry store.Entry) entryView {
	return entryView{
		ID:          entry.ID,
		FeedID:      entry.FeedID,
		FeedTitle:   entry.FeedTitle,
		Title:       entry.Title,
		URL:         entry.URL,
		PublishedAt: entry.PublishedAt,
		Content:     sanitize.HTML(entry.Content),
		Read:        entry.Read,
		Starred:     entry.Starred,
		Archived:    entry.Archived,
	}
}

// parseEntrySelection maps the shared scope and filter query onto the Store's
// one selection type for both listing and mark-all-read.
func (h *Handler) parseEntrySelection(w http.ResponseWriter, r *http.Request) (store.EntrySelection, bool) {
	query := r.URL.Query()
	var selection store.EntrySelection
	if raw := query.Get("feed"); raw != "" {
		feedID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			h.writeError(w, r, http.StatusBadRequest, "feed must be a Feed id")
			return store.EntrySelection{}, false
		}
		selection.FeedID = feedID
	}
	if raw := query.Get("unread"); raw != "" {
		unread, err := strconv.ParseBool(raw)
		if err != nil {
			h.writeError(w, r, http.StatusBadRequest, "unread must be true or false")
			return store.EntrySelection{}, false
		}
		selection.UnreadOnly = unread
	}
	if raw := query.Get("starred"); raw != "" {
		starred, err := strconv.ParseBool(raw)
		if err != nil {
			h.writeError(w, r, http.StatusBadRequest, "starred must be true or false")
			return store.EntrySelection{}, false
		}
		selection.StarredOnly = starred
	}
	if raw := query.Get("archived"); raw != "" {
		archived, err := strconv.ParseBool(raw)
		if err != nil {
			h.writeError(w, r, http.StatusBadRequest, "archived must be true or false")
			return store.EntrySelection{}, false
		}
		selection.ArchivedOnly = archived
	}
	return selection, true
}

// listEntries is the reading list in the requested publish-date order,
// newest first by default, one page at a time. With a since query parameter
// present, it is instead the changed-since feed, per ADR-0004: Entries changed
// and Entries removed after that position, oldest first. No since parameter
// at all behaves exactly like the plain list.
func (h *Handler) listEntries(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Has("since") {
		h.listEntriesSince(w, r)
		return
	}

	selection, ok := h.parseEntrySelection(w, r)
	if !ok {
		return
	}
	limit, ok := h.parseLimit(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	q := store.EntryQuery{EntrySelection: selection, Limit: limit}
	switch query.Get("order") {
	case "", "newest":
	case "oldest":
		q.OldestFirst = true
	default:
		h.writeError(w, r, http.StatusBadRequest, "order must be newest or oldest")
		return
	}
	if raw := query.Get("cursor"); raw != "" {
		cursor, err := decodeCursor(raw)
		if err != nil {
			h.writeError(w, r, http.StatusBadRequest, "that cursor is not one this server issued")
			return
		}
		q.After = cursor
	}
	if raw := query.Get("around"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			h.writeError(w, r, http.StatusBadRequest, "around must be an Entry id")
			return
		}
		q.Around = id
	}

	// One more than asked for: whether a further page exists is a fact about the
	// data, not a guess from a full page.
	q.Limit++
	entries, err := h.deps.Store.Entries(r.Context(), q)
	switch {
	case errors.Is(err, store.ErrNoEntry):
		h.writeError(w, r, http.StatusNotFound, "no such Entry")
		return
	case err != nil:
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

// parseLimit reads the shared page-size query parameter, defaulting and
// capping it, for both the reading list and the changed-since feed.
func (h *Handler) parseLimit(w http.ResponseWriter, r *http.Request) (int, bool) {
	limit := defaultPageSize
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			h.writeError(w, r, http.StatusBadRequest, "limit must be a positive number")
			return 0, false
		}
		limit = min(n, maxPageSize)
	}
	return limit, true
}

// sinceScopeParams are the reading-list scope and filter parameters the
// changed-since feed refuses, rather than silently ignoring: it always reads
// the whole collection, so a caller believing it scoped a sync would
// otherwise get every Entry with no signal that the scope was dropped.
var sinceScopeParams = []string{"feed", "unread", "starred", "archived", "order", "around", "cursor"}

// listEntriesSince is the changed-since feed: every Entry changed, and every
// Entry retention removed, after the since cursor, oldest first, over the
// whole collection. An absent value (since=, present but empty) reads from
// the start of history.
func (h *Handler) listEntriesSince(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	for _, param := range sinceScopeParams {
		if query.Has(param) {
			h.writeError(w, r, http.StatusBadRequest,
				"since reads the whole collection and does not accept "+param)
			return
		}
	}

	cursor, err := decodeSinceCursor(query.Get("since"))
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, "that cursor is not one this server issued")
		return
	}
	limit, ok := h.parseLimit(w, r)
	if !ok {
		return
	}

	entries, tombstones, next, err := h.deps.Store.EntriesSince(r.Context(), cursor, limit)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	views := make([]entryView, 0, len(entries))
	for _, entry := range entries {
		views = append(views, viewEntry(entry))
	}
	removed := make([]int64, 0, len(tombstones))
	for _, tombstone := range tombstones {
		removed = append(removed, tombstone.EntryID)
	}
	h.writeJSON(w, r, http.StatusOK, map[string]any{
		"entries": views, "tombstones": removed, "next_since": encodeSinceCursor(next),
	})
}

// encodeCursor packs an Entry's position in a publish-date ordered list into a
// token the caller cannot construct, so that paging stays the server's business.

func encodeCursor(entry store.Entry) string {
	return encodePosition(entry.PublishedAt, entry.ID)
}

func decodeCursor(raw string) (store.Cursor, error) {
	at, id, err := decodePosition(raw)
	if err != nil {
		return store.Cursor{}, err
	}
	return store.Cursor{PublishedAt: at, ID: id}, nil
}

// encodeSinceCursor packs a change sequence into a token the caller cannot
// construct. A zero cursor — the start of history — encodes as the empty
// string, so a bootstrap request can ask for it as since=.
func encodeSinceCursor(cursor store.SinceCursor) string {
	if cursor == 0 {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(int64(cursor), 10)))
}

// decodeSinceCursor reads a since query value; an empty value is the start of
// history, not an error, so a bootstrap request can spell it as since=.
func decodeSinceCursor(raw string) (store.SinceCursor, error) {
	if raw == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return 0, fmt.Errorf("decode cursor: %w", err)
	}
	seq, err := strconv.ParseInt(string(decoded), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("cursor sequence: %w", err)
	}
	return store.SinceCursor(seq), nil
}

// encodePosition packs a (time, id) position into an opaque token.
func encodePosition(at time.Time, id int64) string {
	position := fmt.Sprintf("%d:%d", at.Unix(), id)
	return base64.RawURLEncoding.EncodeToString([]byte(position))
}

// decodePosition reads a (time, id) position back out of an opaque token.
func decodePosition(raw string) (time.Time, int64, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("decode cursor: %w", err)
	}
	at, id, found := strings.Cut(string(decoded), ":")
	if !found {
		return time.Time{}, 0, errors.New("cursor has no position in it")
	}
	seconds, err := strconv.ParseInt(at, 10, 64)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("cursor timestamp: %w", err)
	}
	entryID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("cursor entry: %w", err)
	}
	return time.Unix(seconds, 0).UTC(), entryID, nil
}

// maxEntryStateBody caps how much of a state-mutation request we are willing
// to read.
const maxEntryStateBody = 1 << 10

// entryStateRequest is the complete desired state of an Entry, per ADR-0004.
// Pointers distinguish explicit false from a missing field.
type entryStateRequest struct {
	Read     *bool `json:"read"`
	Starred  *bool `json:"starred"`
	Archived *bool `json:"archived"`
}

// setEntryState declares an Entry's complete reader-owned state. Archived
// implies Read in the store, where that invariant cannot be bypassed.
func (h *Handler) setEntryState(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.writeError(w, r, http.StatusNotFound, "no such Entry")
		return
	}

	var body entryStateRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, maxEntryStateBody)).Decode(&body); err != nil ||
		body.Read == nil || body.Starred == nil || body.Archived == nil {
		h.writeError(w, r, http.StatusBadRequest, "expected read, starred, and archived flags")
		return
	}

	entry, err := h.deps.Store.SetEntryState(r.Context(), id, store.EntryState{
		Read: *body.Read, Starred: *body.Starred, Archived: *body.Archived,
	}, h.deps.Clock.Now())
	switch {
	case errors.Is(err, store.ErrNoEntry):
		h.writeError(w, r, http.StatusNotFound, "no such Entry")
	case err != nil:
		h.serverError(w, r, err)
	default:
		h.writeJSON(w, r, http.StatusOK, map[string]any{"entry": viewEntry(entry)})
	}
}

// setEntriesRead marks exactly the selected filter and scope Read.
func (h *Handler) setEntriesRead(w http.ResponseWriter, r *http.Request) {
	selection, ok := h.parseEntrySelection(w, r)
	if !ok {
		return
	}
	var body struct {
		Read *bool `json:"read"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxEntryStateBody)).Decode(&body); err != nil ||
		body.Read == nil || !*body.Read {
		h.writeError(w, r, http.StatusBadRequest, "read must be true")
		return
	}
	if err := h.deps.Store.MarkEntriesRead(r.Context(), selection, h.deps.Clock.Now()); err != nil {
		h.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// articleView is an extracted Article as the API presents it.
type articleView struct {
	Title      string `json:"title"`
	HTML       string `json:"html"`
	Embeddable bool   `json:"embeddable"`
}

func viewArticle(article store.Article) articleView {
	return articleView{Title: article.Title, HTML: article.HTML, Embeddable: article.Embeddable}
}

// getArticle serves Reader View for an Entry: the publisher's page, reduced
// to its main text. The first request for an Entry's URL extracts and stores
// it; every request after that, for any Entry sharing the same URL, is served
// from storage without refetching the publisher.
func (h *Handler) getArticle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.writeError(w, r, http.StatusNotFound, "no such Entry")
		return
	}

	entry, err := h.deps.Store.Entry(r.Context(), id)
	switch {
	case errors.Is(err, store.ErrNoEntry):
		h.writeError(w, r, http.StatusNotFound, "no such Entry")
		return
	case err != nil:
		h.serverError(w, r, err)
		return
	}

	article, err := h.deps.Store.Article(r.Context(), entry.URL)
	switch {
	case err == nil:
		h.writeJSON(w, r, http.StatusOK, map[string]any{"article": viewArticle(article)})
		return
	case !errors.Is(err, store.ErrNoArticle):
		h.serverError(w, r, err)
		return
	}

	extracted, err := h.deps.Extraction.Extract(r.Context(), entry.URL)
	if err != nil {
		h.deps.Logger.WarnContext(r.Context(), "extract article", "url", entry.URL, "error", err)
		h.writeError(w, r, http.StatusBadGateway, "could not extract the Article from the publisher's page")
		return
	}

	stored := store.Article{
		URL: entry.URL, Title: extracted.Title, HTML: extracted.HTML, Embeddable: extracted.Embeddable,
	}
	if err := h.deps.Store.SaveArticle(r.Context(), stored, h.deps.Clock.Now()); err != nil {
		h.serverError(w, r, err)
		return
	}
	h.writeJSON(w, r, http.StatusOK, map[string]any{"article": viewArticle(stored)})
}

// originalView is what Original View needs to know about an Entry: the address
// to embed, and whether the publisher permits embedding it at all.
type originalView struct {
	URL        string `json:"url"`
	Embeddable bool   `json:"embeddable"`
}

// getOriginal serves Original View for an Entry: the publisher's own page,
// shown as they laid it out. The embedding flag is the one extraction records,
// so a page already read in Reader View costs no second fetch.
//
// A page carrying no extractable text still answers here — that is much of
// what Original View is for, per ADR-0003 — and is not stored as an Article,
// so Reader View keeps reporting the failure rather than serving an empty one.
func (h *Handler) getOriginal(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.writeError(w, r, http.StatusNotFound, "no such Entry")
		return
	}

	entry, err := h.deps.Store.Entry(r.Context(), id)
	switch {
	case errors.Is(err, store.ErrNoEntry):
		h.writeError(w, r, http.StatusNotFound, "no such Entry")
		return
	case err != nil:
		h.serverError(w, r, err)
		return
	}

	article, err := h.deps.Store.Article(r.Context(), entry.URL)
	switch {
	case err == nil:
		h.writeOriginal(w, r, entry.URL, article.Embeddable)
		return
	case !errors.Is(err, store.ErrNoArticle):
		h.serverError(w, r, err)
		return
	}

	extracted, err := h.deps.Extraction.Extract(r.Context(), entry.URL)
	switch {
	case err == nil:
		stored := store.Article{
			URL: entry.URL, Title: extracted.Title, HTML: extracted.HTML, Embeddable: extracted.Embeddable,
		}
		if err := h.deps.Store.SaveArticle(r.Context(), stored, h.deps.Clock.Now()); err != nil {
			h.serverError(w, r, err)
			return
		}
	case errors.Is(err, extraction.ErrNoContent):
		// Nothing to store, but the fetch still answered the only question
		// Original View asked.
	default:
		h.deps.Logger.WarnContext(r.Context(), "check original view", "url", entry.URL, "error", err)
		h.writeError(w, r, http.StatusBadGateway, "could not reach the publisher's page")
		return
	}
	h.writeOriginal(w, r, entry.URL, extracted.Embeddable)
}

func (h *Handler) writeOriginal(w http.ResponseWriter, r *http.Request, url string, embeddable bool) {
	h.writeJSON(w, r, http.StatusOK, map[string]any{
		"original": originalView{URL: url, Embeddable: embeddable},
	})
}
