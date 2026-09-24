package api_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gabe-santos/yogurt/internal/apitest"
)

// siteWithIcon serves a home page advertising a Feed Icon at iconPath, and
// the Feed itself pointing at that home page as its site_url.
func siteWithIcon(t *testing.T, h *apitest.Harness, iconPath, iconBody, iconContentType string) (feedURL string) {
	t.Helper()
	iconURL := h.Publisher.Serve(iconPath, apitest.Document{
		ContentType: iconContentType,
		Body:        iconBody,
	})
	h.Publisher.Serve("/", apitest.Document{
		ContentType: "text/html",
		Body:        `<html><head><link rel="icon" href="` + iconURL + `"></head></html>`,
	})
	return h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", h.Publisher.URL("/"),
		apitest.Item{ID: "one", Title: "First post", Published: published},
	))
}

func TestSubscribingDiscoversAndStoresFeedIconAsSVG(t *testing.T) {
	h := loggedIn(t)
	feedURL := siteWithIcon(t, h, "/icon.svg", "<svg>fake</svg>", "image/svg+xml")

	feed := subscribe(t, h, feedURL)
	if feed.IconStoredAt == nil {
		t.Fatal("expected icon_stored_at to be set once discovery finds an icon")
	}
	if feed.IconCheckedAt == nil {
		t.Fatal("expected icon_checked_at to be set")
	}

	resp := h.Do(http.MethodGet, feedPath(feed.ID, "/icon"), nil).ExpectStatus(http.StatusOK)
	if got := string(resp.Body); got != "<svg>fake</svg>" {
		t.Errorf("stored icon body = %q, want the SVG byte-for-byte", got)
	}
	if got := resp.Header.Get("Content-Type"); got != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", got)
	}
}

func TestSubscribingSucceedsWhenSiteHasNoUsableIcon(t *testing.T) {
	h := loggedIn(t)
	h.Publisher.Serve("/", apitest.Document{
		ContentType: "text/html",
		Body:        `<html><head><title>No icons</title></head></html>`,
	})
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", h.Publisher.URL("/"),
		apitest.Item{ID: "one", Title: "First post", Published: published},
	))

	feed := subscribe(t, h, feedURL)
	if feed.IconStoredAt != nil {
		t.Error("icon_stored_at should be absent: the site has no icon")
	}
	if feed.IconCheckedAt == nil {
		t.Error("icon_checked_at should be set even when no icon was found")
	}
}

func TestSubscribingSucceedsWhenTheSiteIsUnreachable(t *testing.T) {
	h := loggedIn(t)
	// The RSS advertises a site the fake publisher never registers a
	// document for, so fetching it 404s.
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", h.Publisher.URL("/does-not-exist"),
		apitest.Item{ID: "one", Title: "First post", Published: published},
	))

	feed := subscribe(t, h, feedURL)
	if feed.IconStoredAt != nil {
		t.Error("icon_stored_at should be absent when the site cannot be read")
	}
	if feed.IconCheckedAt == nil {
		t.Error("icon_checked_at should still be set after a failed check")
	}
}

func TestSubscribingSucceedsWhenTheOnlyIconExceedsTheSizeCap(t *testing.T) {
	h := loggedIn(t)
	oversized := strings.Repeat("x", 256<<10+1)
	feedURL := siteWithIcon(t, h, "/huge.png", oversized, "image/png")

	feed := subscribe(t, h, feedURL)
	if feed.IconStoredAt != nil {
		t.Error("icon_stored_at should be absent: the only candidate exceeds the size cap")
	}

	h.Do(http.MethodGet, feedPath(feed.ID, "/icon"), nil).ExpectStatus(http.StatusNotFound)
}
