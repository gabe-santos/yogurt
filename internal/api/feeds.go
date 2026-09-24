package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gabe-santos/yogurt/internal/pull"
	"github.com/gabe-santos/yogurt/internal/store"
)

// maxFeedBody caps how much of a create-Feed request we are willing to read.
const maxFeedBody = 8 << 10

// feedView is one Feed as the API presents it. LastCheckedAt and
// LastSuccessAt are nil until the Feed's first check, so silence before any
// check is distinguishable from a Feed that keeps failing.
type feedView struct {
	ID                  int64      `json:"id"`
	URL                 string     `json:"url"`
	Title               string     `json:"title"`
	SiteURL             string     `json:"site_url"`
	UnreadCount         int        `json:"unread_count"`
	CreatedAt           time.Time  `json:"created_at"`
	LastCheckedAt       *time.Time `json:"last_checked_at"`
	LastSuccessAt       *time.Time `json:"last_success_at"`
	LastError           string     `json:"last_error"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	// IconStoredAt is absent for a Feed with no Feed Icon, and doubles as the
	// icon endpoint's cache-busting version. IconCheckedAt is absent until
	// the Feed's first icon check, present regardless of outcome after that.
	IconStoredAt  *time.Time `json:"icon_stored_at"`
	IconCheckedAt *time.Time `json:"icon_checked_at"`
}

func viewFeed(feed store.Feed, unreadCounts map[int64]int) feedView {
	return feedView{
		ID:                  feed.ID,
		URL:                 feed.URL,
		Title:               feed.Title,
		SiteURL:             feed.SiteURL,
		UnreadCount:         unreadCounts[feed.ID],
		CreatedAt:           feed.CreatedAt,
		LastCheckedAt:       zeroToNil(feed.LastCheckedAt),
		LastSuccessAt:       zeroToNil(feed.LastSuccessAt),
		LastError:           feed.LastError,
		ConsecutiveFailures: feed.ConsecutiveFailures,
		IconStoredAt:        zeroToNil(feed.IconStoredAt),
		IconCheckedAt:       zeroToNil(feed.IconCheckedAt),
	}
}

// zeroToNil renders a Feed's never-checked zero Time as absent rather than as
// the misleading instant 1970-01-01.
func zeroToNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

type createFeedRequest struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// createFeed subscribes to a Feed: the address is validated, a Feed is
// discovered on it when the address is a web page, and the Feed is read once so
// that a new subscription is not empty.
func (h *Handler) createFeed(w http.ResponseWriter, r *http.Request) {
	var body createFeedRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, maxFeedBody)).Decode(&body); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "expected a JSON object with a url")
		return
	}

	feed, err := h.deps.Pull.Subscribe(r.Context(), body.URL, strings.TrimSpace(body.Title))
	switch {
	case errors.Is(err, pull.ErrInvalidURL):
		h.writeError(w, r, http.StatusBadRequest, "that url is not a web address")
	case errors.Is(err, store.ErrFeedExists):
		h.writeError(w, r, http.StatusConflict, "you already have this Feed")
	case errors.Is(err, pull.ErrNoFeed):
		h.writeError(w, r, http.StatusUnprocessableEntity,
			"that address is not a Feed, and carries no link to one")
	case isFetchFailure(err):
		h.writeError(w, r, http.StatusBadGateway, err.Error())
	case err != nil:
		h.serverError(w, r, err)
	default:
		h.writeJSON(w, r, http.StatusCreated, map[string]any{"feed": viewFeed(feed, nil)})
	}
}

// listFeeds is the reader's whole collection, with each one's unread count.
func (h *Handler) listFeeds(w http.ResponseWriter, r *http.Request) {
	feeds, err := h.deps.Store.Feeds(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	counts, err := h.deps.Store.FeedUnreadCounts(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	views := make([]feedView, 0, len(feeds))
	for _, feed := range feeds {
		views = append(views, viewFeed(feed, counts))
	}
	h.writeJSON(w, r, http.StatusOK, map[string]any{"feeds": views})
}

// refreshFeeds re-reads every Feed on demand. Feeds that fail are reported
// rather than failing the whole refresh.
func (h *Handler) refreshFeeds(w http.ResponseWriter, r *http.Request) {
	failures, err := h.deps.Pull.RefreshAll(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	reported := make([]map[string]any, 0, len(failures))
	for id, cause := range failures {
		reported = append(reported, map[string]any{"feed_id": id, "error": cause.Error()})
	}
	h.writeJSON(w, r, http.StatusOK, map[string]any{"failures": reported})
}

// refreshFeed re-reads one Feed on demand.
func (h *Handler) refreshFeed(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.writeError(w, r, http.StatusNotFound, "no such Feed")
		return
	}

	switch err := h.deps.Pull.Refresh(r.Context(), id); {
	case errors.Is(err, store.ErrNoFeed):
		h.writeError(w, r, http.StatusNotFound, "no such Feed")
	case isFetchFailure(err):
		h.writeError(w, r, http.StatusBadGateway, err.Error())
	case err != nil:
		h.serverError(w, r, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// updateFeedRequest declares the fields of a Feed the reader wants to
// change; an absent field is left as stored.
type updateFeedRequest struct {
	Title *string `json:"title"`
}

// updateFeed changes a Feed's title. Only fields present in the request are
// changed.
func (h *Handler) updateFeed(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.writeError(w, r, http.StatusNotFound, "no such Feed")
		return
	}

	var body updateFeedRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, maxFeedBody)).Decode(&body); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "expected a JSON object with a title")
		return
	}
	if body.Title == nil {
		h.writeError(w, r, http.StatusBadRequest, "expected a title")
		return
	}

	feed, err := h.deps.Store.UpdateFeed(r.Context(), id, store.FeedPatch{
		Title: body.Title,
	}, h.deps.Clock.Now())
	switch {
	case errors.Is(err, store.ErrNoFeed):
		h.writeError(w, r, http.StatusNotFound, "no such Feed")
		return
	case err != nil:
		h.serverError(w, r, err)
		return
	}

	counts, err := h.deps.Store.FeedUnreadCounts(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	h.writeJSON(w, r, http.StatusOK, map[string]any{"feed": viewFeed(feed, counts)})
}

// deleteFeed removes a Feed and every Entry it carried.
func (h *Handler) deleteFeed(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.writeError(w, r, http.StatusNotFound, "no such Feed")
		return
	}

	switch err := h.deps.Store.DeleteFeed(r.Context(), id); {
	case errors.Is(err, store.ErrNoFeed):
		h.writeError(w, r, http.StatusNotFound, "no such Feed")
	case err != nil:
		h.serverError(w, r, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// getFeedIcon serves a Feed's stored Feed Icon from this app's own origin,
// rather than the caller hotlinking the publisher's copy — which would tell
// every publisher this reader saw their headline, for every Entry, whether
// it was opened or not (ADR-0008). The response is cached aggressively but
// marked private, since it sits behind this app's own session rather than
// being safe for a shared cache to store; callers version the URL with the
// Feed's icon_stored_at so a changed icon invalidates the cache and an
// unchanged one is never refetched.
func (h *Handler) getFeedIcon(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.writeError(w, r, http.StatusNotFound, "no such Feed Icon")
		return
	}

	data, mediaType, err := h.deps.Store.FeedIcon(r.Context(), id)
	switch {
	case errors.Is(err, store.ErrNoFeed), errors.Is(err, store.ErrNoIcon):
		h.writeError(w, r, http.StatusNotFound, "no such Feed Icon")
		return
	case err != nil:
		h.serverError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		h.deps.Logger.WarnContext(r.Context(), "write response", "path", r.URL.Path, "error", err)
	}
}

// isFetchFailure reports whether the publisher, and not this application, is
// what went wrong.
func isFetchFailure(err error) bool {
	var failure *pull.FetchError
	return errors.As(err, &failure)
}
