package api

import (
	"errors"
	"io"
	"net/http"

	"github.com/gabe-santos/yogurt/internal/opml"
	"github.com/gabe-santos/yogurt/internal/pull"
	"github.com/gabe-santos/yogurt/internal/store"
)

// maxOPMLBody caps how much of an OPML import we are willing to read.
const maxOPMLBody = 1 << 20

// importedFeed is one Feed an OPML import subscribed to.
type importedFeed struct {
	Feed feedView `json:"feed"`
}

// skippedFeed is one Feed an OPML import left alone, and why.
type skippedFeed struct {
	URL    string `json:"url"`
	Reason string `json:"reason"`
}

// importOPML reads an OPML document and subscribes to every Feed it names
// that Yogurt does not already hold, ignoring whatever folders the
// document nested them in. A Feed already subscribed is recognised by its
// URL before it is ever fetched, and one that could not be fetched is
// recognised after; both are skipped and reported rather than failing the
// whole import, and the Feeds that do need fetching are fetched
// concurrently, bounded the way SubscribeMany already bounds it.
func (h *Handler) importOPML(w http.ResponseWriter, r *http.Request) {
	urls, err := opml.Parse(io.LimitReader(r.Body, maxOPMLBody))
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, "that is not a valid OPML document")
		return
	}

	existing, err := h.deps.Store.Feeds(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	alreadySubscribed := make(map[string]bool, len(existing))
	for _, feed := range existing {
		alreadySubscribed[feed.URL] = true
	}

	added := make([]importedFeed, 0)
	skipped := make([]skippedFeed, 0)

	// A Feed already subscribed is skipped without fetching it: a re-import
	// of a previously exported collection would otherwise re-fetch every
	// Feed in it only to discard the result.
	pending := make([]string, 0, len(urls))
	for _, url := range urls {
		if alreadySubscribed[url] {
			skipped = append(skipped, skippedFeed{URL: url, Reason: "already subscribed to this Feed"})
			continue
		}
		pending = append(pending, url)
	}

	for i, outcome := range h.deps.Pull.SubscribeMany(r.Context(), pending) {
		url := pending[i]
		switch {
		case errors.Is(outcome.Err, store.ErrFeedExists):
			skipped = append(skipped, skippedFeed{URL: url, Reason: "already subscribed to this Feed"})
		case errors.Is(outcome.Err, pull.ErrInvalidURL):
			skipped = append(skipped, skippedFeed{URL: url, Reason: "not a valid web address"})
		case errors.Is(outcome.Err, pull.ErrNoFeed):
			skipped = append(skipped, skippedFeed{URL: url, Reason: "no Feed found at that address"})
		case isFetchFailure(outcome.Err):
			skipped = append(skipped, skippedFeed{URL: url, Reason: outcome.Err.Error()})
		case outcome.Err != nil:
			h.serverError(w, r, outcome.Err)
			return
		default:
			added = append(added, importedFeed{Feed: viewFeed(outcome.Feed, nil)})
		}
	}

	h.writeJSON(w, r, http.StatusOK, map[string]any{"added": added, "skipped": skipped})
}

// exportOPML writes the reader's whole collection as a flat OPML document,
// per opml.Write.
func (h *Handler) exportOPML(w http.ResponseWriter, r *http.Request) {
	feeds, err := h.deps.Store.Feeds(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	exports := make([]opml.FeedExport, 0, len(feeds))
	for _, feed := range feeds {
		exports = append(exports, opml.FeedExport{
			Title:   feed.Title,
			XMLURL:  feed.URL,
			HTMLURL: feed.SiteURL,
		})
	}

	w.Header().Set("Content-Type", "text/x-opml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="feeds.opml"`)
	if err := opml.Write(w, exports); err != nil {
		h.deps.Logger.ErrorContext(r.Context(), "write opml export", "error", err)
	}
}
