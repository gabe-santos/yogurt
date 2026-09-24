package api_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/gabe-santos/yogurt/internal/apitest"
)

// feedPath is one Feed's collection endpoint, with suffix appended (e.g.
// "/icon", "/refresh", or "" for the Feed itself).
func feedPath(id int64, suffix string) string {
	return "/api/feeds/" + strconv.FormatInt(id, 10) + suffix
}

func TestFeedIconServesStoredBytesAndCachesAggressively(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "First post", Published: published},
	))
	feed := subscribe(t, h, feedURL)

	if err := h.Store.SetFeedIcon(t.Context(), feed.ID, []byte("fake-svg-bytes"), "image/svg+xml", h.Clock.Now()); err != nil {
		t.Fatalf("seed feed icon: %v", err)
	}

	resp := h.Do(http.MethodGet, feedPath(feed.ID, "/icon"), nil).ExpectStatus(http.StatusOK)
	if got := string(resp.Body); got != "fake-svg-bytes" {
		t.Errorf("body = %q, want the stored icon bytes", got)
	}
	if got := resp.Header.Get("Content-Type"); got != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", got)
	}
	if got := resp.Header.Get("Cache-Control"); got == "" {
		t.Error("expected an aggressive Cache-Control header")
	}

	// icon_stored_at, read back through the Feed API, is what the frontend
	// versions the icon URL with.
	feeds := listFeeds(t, h)
	if feeds[0].IconStoredAt == nil {
		t.Error("icon_stored_at should be present once an icon is stored")
	}
	if feeds[0].IconCheckedAt == nil {
		t.Error("icon_checked_at should be present once the Feed has been checked")
	}
}

func TestFeedIconOfAFeedWithNoneIs404(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "First post", Published: published},
	))
	feed := subscribe(t, h, feedURL)

	h.Do(http.MethodGet, feedPath(feed.ID, "/icon"), nil).ExpectStatus(http.StatusNotFound)

	feeds := listFeeds(t, h)
	if feeds[0].IconStoredAt != nil {
		t.Error("icon_stored_at should be absent for a Feed with no icon")
	}
}

func TestFeedIconOfANonexistentFeedIs404(t *testing.T) {
	h := loggedIn(t)
	h.Do(http.MethodGet, "/api/feeds/999/icon", nil).ExpectStatus(http.StatusNotFound)
}

func TestDeletingAFeedRemovesItsIcon(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "First post", Published: published},
	))
	feed := subscribe(t, h, feedURL)
	if err := h.Store.SetFeedIcon(t.Context(), feed.ID, []byte("fake-svg-bytes"), "image/svg+xml", h.Clock.Now()); err != nil {
		t.Fatalf("seed feed icon: %v", err)
	}
	h.Do(http.MethodGet, feedPath(feed.ID, "/icon"), nil).ExpectStatus(http.StatusOK)

	h.Do(http.MethodDelete, feedPath(feed.ID, ""), nil).ExpectStatus(http.StatusNoContent)

	h.Do(http.MethodGet, feedPath(feed.ID, "/icon"), nil).ExpectStatus(http.StatusNotFound)
}
