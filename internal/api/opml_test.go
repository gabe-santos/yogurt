package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gabe-santos/rss-reader/internal/apitest"
)

type importedFeedView struct {
	Feed  feedView `json:"feed"`
	Group string   `json:"group"`
}

type skippedFeedView struct {
	URL    string `json:"url"`
	Reason string `json:"reason"`
}

type importResult struct {
	Added   []importedFeedView `json:"added"`
	Skipped []skippedFeedView  `json:"skipped"`
}

// importOPML posts an OPML document and decodes the result, failing the test
// unless the server accepted it.
func importOPML(t *testing.T, h *apitest.Harness, doc string) importResult {
	t.Helper()
	var result importResult
	h.DoRaw(http.MethodPost, "/api/opml/import", "text/x-opml", []byte(doc)).
		ExpectStatus(http.StatusOK).
		JSON(&result)
	return result
}

// exportOPML fetches the OPML export as text.
func exportOPML(t *testing.T, h *apitest.Harness) string {
	t.Helper()
	return string(h.Do(http.MethodGet, "/api/opml/export", nil).ExpectStatus(http.StatusOK).Body)
}

// servePlainFeed serves a trivial, valid RSS Feed at path and returns its URL.
func servePlainFeed(h *apitest.Harness, path, title string) string {
	return h.Publisher.Serve(path, apitest.RSS(title, h.Publisher.URL("/"),
		apitest.Item{ID: "one", Title: "Only post", Link: h.Publisher.URL(path + "/one"), Published: published},
	))
}

func TestImportingOPMLCreatesFeedsAndFlattensNestedFoldersIntoOneGroupEach(t *testing.T) {
	h := loggedIn(t)

	techURL := servePlainFeed(h, "/tech.xml", "Tech Publisher")
	golangURL := servePlainFeed(h, "/golang.xml", "Golang Weekly")
	newsURL := servePlainFeed(h, "/news.xml", "News Publisher")
	looseURL := servePlainFeed(h, "/loose.xml", "No Folder")

	// "Programming" is the outermost folder; "Go" nests inside it two levels
	// deep and must not create its own Group.
	doc := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head><title>Subscriptions</title></head>
  <body>
    <outline text="Programming">
      <outline text="Tech" title="Tech" xmlUrl=%q type="rss"/>
      <outline text="Go">
        <outline text="Golang" title="Golang" xmlUrl=%q type="rss"/>
      </outline>
    </outline>
    <outline text="News">
      <outline text="News" title="News" xmlUrl=%q type="rss"/>
    </outline>
    <outline text="Loose" title="Loose" xmlUrl=%q type="rss"/>
  </body>
</opml>`, techURL, golangURL, newsURL, looseURL)

	result := importOPML(t, h, doc)
	if len(result.Skipped) != 0 {
		t.Fatalf("skipped = %+v, want none", result.Skipped)
	}
	if len(result.Added) != 4 {
		t.Fatalf("added %d Feeds, want 4: %+v", len(result.Added), result.Added)
	}

	byURL := make(map[string]importedFeedView, len(result.Added))
	for _, added := range result.Added {
		byURL[added.Feed.URL] = added
	}

	if got := byURL[techURL].Group; got != "Programming" {
		t.Errorf("tech feed group = %q, want %q", got, "Programming")
	}
	if got := byURL[golangURL].Group; got != "Programming" {
		t.Errorf("golang feed group (nested two levels) = %q, want the outermost folder %q", got, "Programming")
	}
	if got := byURL[newsURL].Group; got != "News" {
		t.Errorf("news feed group = %q, want %q", got, "News")
	}
	if got := byURL[looseURL].Group; got != "" {
		t.Errorf("loose feed group = %q, want the default Group (empty)", got)
	}
	if byURL[techURL].Feed.GroupID != byURL[golangURL].Feed.GroupID {
		t.Errorf("tech and golang feeds landed in different Groups despite sharing an outermost folder")
	}

	groups := listGroups(t, h)
	var names []string
	for _, g := range groups {
		names = append(names, g.Name)
	}
	if !containsAll(names, "Programming", "News") {
		t.Errorf("groups = %v, want Programming and News created", names)
	}
	// "Go" never appears as its own Group.
	for _, name := range names {
		if name == "Go" {
			t.Errorf("nested folder \"Go\" created its own Group, want it flattened into Programming")
		}
	}
}

func TestOPMLImportReportsAlreadySubscribedFeedsAsSkippedRatherThanFailing(t *testing.T) {
	h := loggedIn(t)

	feedURL := servePlainFeed(h, "/existing.xml", "Existing")
	subscribe(t, h, feedURL)

	otherURL := servePlainFeed(h, "/other.xml", "Other")
	doc := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0"><body>
  <outline text="Existing" xmlUrl=%q type="rss"/>
  <outline text="Other" xmlUrl=%q type="rss"/>
</body></opml>`, feedURL, otherURL)

	result := importOPML(t, h, doc)
	if len(result.Added) != 1 || result.Added[0].Feed.URL != otherURL {
		t.Fatalf("added = %+v, want only the new Feed", result.Added)
	}
	if len(result.Skipped) != 1 || result.Skipped[0].URL != feedURL {
		t.Fatalf("skipped = %+v, want the already-subscribed Feed", result.Skipped)
	}
	if result.Skipped[0].Reason == "" {
		t.Error("skipped Feed carries no reason")
	}

	if feeds := listFeeds(t, h); len(feeds) != 2 {
		t.Fatalf("feeds = %d, want 2 (no duplicate of the existing Feed)", len(feeds))
	}
}

func TestOPMLExportRoundTripsThroughImportWithoutDuplicating(t *testing.T) {
	h := loggedIn(t)

	newsGroup := createGroup(t, h, "News")
	techURL := servePlainFeed(h, "/tech.xml", "Tech Publisher")
	newsURL := servePlainFeed(h, "/news.xml", "News Publisher")

	subscribe(t, h, techURL)
	news := subscribe(t, h, newsURL)
	h.Do(http.MethodPut, fmt.Sprintf("/api/feeds/%d", news.ID), map[string]any{"group_id": newsGroup.ID}).
		ExpectStatus(http.StatusOK)

	exported := exportOPML(t, h)
	if !strings.Contains(exported, "News") {
		t.Errorf("export does not mention the News Group:\n%s", exported)
	}
	if !strings.Contains(exported, techURL) || !strings.Contains(exported, newsURL) {
		t.Errorf("export does not carry both Feed URLs:\n%s", exported)
	}

	before := listFeeds(t, h)

	result := importOPML(t, h, exported)
	if len(result.Added) != 0 {
		t.Fatalf("re-import added = %+v, want none (every Feed already subscribed)", result.Added)
	}
	if len(result.Skipped) != 2 {
		t.Fatalf("re-import skipped = %d, want 2", len(result.Skipped))
	}

	after := listFeeds(t, h)
	if len(after) != len(before) {
		t.Fatalf("feed count after re-import = %d, want unchanged %d", len(after), len(before))
	}
}

func TestImportingAnInvalidOPMLDocumentIsRefused(t *testing.T) {
	h := loggedIn(t)
	h.DoRaw(http.MethodPost, "/api/opml/import", "text/x-opml", []byte("not xml at all")).
		ExpectStatus(http.StatusBadRequest)
}

func containsAll(haystack []string, wants ...string) bool {
	for _, want := range wants {
		found := false
		for _, got := range haystack {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
