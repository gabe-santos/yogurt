package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gabe-santos/rss-reader/internal/pull"
	"github.com/gabe-santos/rss-reader/internal/store"
)

// maxFeedBody caps how much of a create-Feed request we are willing to read.
const maxFeedBody = 8 << 10

// feedView is one Feed as the API presents it.
type feedView struct {
	ID        int64     `json:"id"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	SiteURL   string    `json:"site_url"`
	CreatedAt time.Time `json:"created_at"`
}

func viewFeed(feed store.Feed) feedView {
	return feedView{
		ID:        feed.ID,
		URL:       feed.URL,
		Title:     feed.Title,
		SiteURL:   feed.SiteURL,
		CreatedAt: feed.CreatedAt,
	}
}

type createFeedRequest struct {
	URL string `json:"url"`
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

	feed, err := h.deps.Pull.Subscribe(r.Context(), body.URL)
	switch {
	case errors.Is(err, pull.ErrInvalidURL):
		h.writeError(w, r, http.StatusBadRequest, "that url is not a web address")
	case errors.Is(err, store.ErrFeedExists):
		h.writeError(w, r, http.StatusConflict, "you are already subscribed to this Feed")
	case errors.Is(err, pull.ErrNoFeed):
		h.writeError(w, r, http.StatusUnprocessableEntity,
			"that address is not a Feed, and carries no link to one")
	case isFetchFailure(err):
		h.writeError(w, r, http.StatusBadGateway, err.Error())
	case err != nil:
		h.serverError(w, r, err)
	default:
		h.writeJSON(w, r, http.StatusCreated, map[string]any{"feed": viewFeed(feed)})
	}
}

// listFeeds is the reader's whole collection.
func (h *Handler) listFeeds(w http.ResponseWriter, r *http.Request) {
	feeds, err := h.deps.Store.Feeds(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	views := make([]feedView, 0, len(feeds))
	for _, feed := range feeds {
		views = append(views, viewFeed(feed))
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

// isFetchFailure reports whether the publisher, and not this application, is
// what went wrong.
func isFetchFailure(err error) bool {
	var failure *pull.FetchError
	return errors.As(err, &failure)
}
