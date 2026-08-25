package api

import (
	"net/http"
	"strings"

	"github.com/gabe-santos/rss-reader/internal/store"
)

// maxSearchQuery caps how much of a search's own text this application will
// look at, well past anything a reader would plausibly type into a search box.
const maxSearchQuery = 200

// entrySearchView is one Entry a search matched, as the API presents it.
// Snippet is already plain text: the search index stores plain text, not
// the publisher's markup.
type entrySearchView struct {
	entryView
	Snippet string `json:"snippet"`
}

func viewEntrySearchResult(result store.EntrySearchResult) entrySearchView {
	return entrySearchView{
		entryView: viewEntry(result.Entry),
		Snippet:   result.Snippet,
	}
}

// search finds what the reader's text matches: Entries by title or
// Feed-supplied content, and Feeds by name, so one shortcut both finds an
// Entry and navigates to a Feed.
func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if runes := []rune(q); len(runes) > maxSearchQuery {
		q = string(runes[:maxSearchQuery])
	}

	entries, err := h.deps.Store.SearchEntries(r.Context(), q)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	feeds, err := h.deps.Store.SearchFeeds(r.Context(), q)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	unreadCounts, err := h.deps.Store.FeedUnreadCounts(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	entryViews := make([]entrySearchView, 0, len(entries))
	for _, result := range entries {
		entryViews = append(entryViews, viewEntrySearchResult(result))
	}
	feedViews := make([]feedView, 0, len(feeds))
	for _, feed := range feeds {
		feedViews = append(feedViews, viewFeed(feed, unreadCounts))
	}
	h.writeJSON(w, r, http.StatusOK, map[string]any{"entries": entryViews, "feeds": feedViews})
}
