package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gabe-santos/yogurt/internal/apitest"
	"github.com/gabe-santos/yogurt/internal/store"
)

func TestPollingBackfillsAnIconForAFeedNeverChecked(t *testing.T) {
	h := loggedIn(t)
	feedURL := siteWithIcon(t, h, "/icon.svg", "<svg>backfilled</svg>", "image/svg+xml")

	// A Feed subscribed before this feature existed: created directly, never
	// having gone through Subscribe's own discovery, so icon_checked_at is
	// the zero value.
	created, err := h.Store.CreateFeed(t.Context(), store.Feed{
		URL: feedURL, Title: "The Publisher", SiteURL: h.Publisher.URL("/"),
	}, h.Clock.Now())
	if err != nil {
		t.Fatalf("create feed directly: %v", err)
	}

	h.Do(http.MethodPost, feedPath(created.ID, "/refresh"), nil).ExpectStatus(http.StatusNoContent)

	resp := h.Do(http.MethodGet, feedPath(created.ID, "/icon"), nil).ExpectStatus(http.StatusOK)
	if got := string(resp.Body); got != "<svg>backfilled</svg>" {
		t.Errorf("body = %q, want the icon discovered on the poll that backfilled it", got)
	}
}

func TestPollingDoesNotRecheckAFreshIcon(t *testing.T) {
	h := loggedIn(t)
	feedURL := siteWithIcon(t, h, "/icon.svg", "<svg>original</svg>", "image/svg+xml")
	feed := subscribe(t, h, feedURL)

	// The site changes its icon, but not for another 29 days: within the
	// re-probe window, the stored copy must not change.
	h.Publisher.Serve("/icon.svg", apitest.Document{ContentType: "image/svg+xml", Body: "<svg>changed</svg>"})
	h.Clock.Advance(29 * 24 * time.Hour)

	h.Do(http.MethodPost, feedPath(feed.ID, "/refresh"), nil).ExpectStatus(http.StatusNoContent)

	resp := h.Do(http.MethodGet, feedPath(feed.ID, "/icon"), nil).ExpectStatus(http.StatusOK)
	if got := string(resp.Body); got != "<svg>original</svg>" {
		t.Errorf("body = %q, want the icon untouched within the 30-day window", got)
	}
}

func TestPollingReplacesAStaleIconAfterThirtyDays(t *testing.T) {
	h := loggedIn(t)
	feedURL := siteWithIcon(t, h, "/icon.svg", "<svg>original</svg>", "image/svg+xml")
	feed := subscribe(t, h, feedURL)
	firstStoredAt := feed.IconStoredAt

	h.Publisher.Serve("/icon.svg", apitest.Document{ContentType: "image/svg+xml", Body: "<svg>changed</svg>"})
	h.Clock.Advance(31 * 24 * time.Hour)
	h.Login(apitest.Password)

	h.Do(http.MethodPost, feedPath(feed.ID, "/refresh"), nil).ExpectStatus(http.StatusNoContent)

	resp := h.Do(http.MethodGet, feedPath(feed.ID, "/icon"), nil).ExpectStatus(http.StatusOK)
	if got := string(resp.Body); got != "<svg>changed</svg>" {
		t.Errorf("body = %q, want the icon replaced after 30 days", got)
	}

	feeds := listFeeds(t, h)
	if feeds[0].IconStoredAt == nil || !feeds[0].IconStoredAt.After(*firstStoredAt) {
		t.Errorf("icon_stored_at did not advance: got %v, want after %v", feeds[0].IconStoredAt, firstStoredAt)
	}
}

func TestPollingKeepsAnExistingIconWhenTheSiteBecomesUnreachable(t *testing.T) {
	h := loggedIn(t)
	feedURL := siteWithIcon(t, h, "/icon.svg", "<svg>original</svg>", "image/svg+xml")
	feed := subscribe(t, h, feedURL)

	// The site's home page starts refusing every request, but the Feed
	// document itself is untouched: the poll must still succeed, and the
	// icon already stored must survive rather than being cleared.
	h.Publisher.Serve("/", apitest.Document{Status: http.StatusInternalServerError, Body: "broken"})
	h.Clock.Advance(31 * 24 * time.Hour)
	h.Login(apitest.Password)

	h.Do(http.MethodPost, feedPath(feed.ID, "/refresh"), nil).ExpectStatus(http.StatusNoContent)

	status := feedStatus(t, h, feed.ID)
	if status.LastError != "" {
		t.Errorf("last_error = %q, an icon failure must never fail the poll", status.LastError)
	}
	if status.ConsecutiveFailures != 0 {
		t.Errorf("consecutive_failures = %d, an icon failure must not affect the Feed's own backoff", status.ConsecutiveFailures)
	}

	h.Do(http.MethodGet, feedPath(feed.ID, "/icon"), nil).ExpectStatus(http.StatusOK)
}

func TestPollingDoesNotReprobeARepeatedlyFailingSiteWithinTheWindow(t *testing.T) {
	h := loggedIn(t)
	// No icon link at all: the site has genuinely none, so every check keeps
	// finding nothing.
	h.Publisher.Serve("/", apitest.Document{ContentType: "text/html", Body: `<html><head></head></html>`})
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", h.Publisher.URL("/"),
		apitest.Item{ID: "one", Title: "First post", Published: published},
	))
	feed := subscribe(t, h, feedURL)
	if feed.IconStoredAt != nil {
		t.Fatal("expected no icon at subscribe time")
	}
	hitsAfterSubscribe := h.Publisher.Hits("/")

	// Two more polls inside the 30-day window: the gate must keep the site
	// from being re-fetched at all, not merely keep it iconless.
	h.Do(http.MethodPost, feedPath(feed.ID, "/refresh"), nil).ExpectStatus(http.StatusNoContent)
	h.Clock.Advance(10 * 24 * time.Hour)
	h.Do(http.MethodPost, feedPath(feed.ID, "/refresh"), nil).ExpectStatus(http.StatusNoContent)

	if got := h.Publisher.Hits("/"); got != hitsAfterSubscribe {
		t.Errorf("site hits = %d, want %d: a site with no icon must not be re-probed within the 30-day window",
			got, hitsAfterSubscribe)
	}

	feeds := listFeeds(t, h)
	if feeds[0].IconStoredAt != nil {
		t.Error("icon_stored_at should remain absent")
	}
}
