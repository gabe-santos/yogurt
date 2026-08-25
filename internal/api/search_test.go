package api_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gabe-santos/rss-reader/internal/apitest"
)

type searchEntryView struct {
	entryView
	Snippet string `json:"snippet"`
}

type searchResponse struct {
	Entries []searchEntryView `json:"entries"`
	Feeds   []feedView        `json:"feeds"`
}

// search runs a search, query being everything after "?q=".
func search(t *testing.T, h *apitest.Harness, q string) searchResponse {
	t.Helper()
	var body searchResponse
	h.Do(http.MethodGet, "/api/search?q="+q, nil).ExpectStatus(http.StatusOK).JSON(&body)
	return body
}

func searchEntryTitles(entries []searchEntryView) []string {
	titles := make([]string, 0, len(entries))
	for _, entry := range entries {
		titles = append(titles, entry.Title)
	}
	return titles
}

func TestSearchMatchesEntryTitlesAndFeedSuppliedContent(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{
			ID: "one", Title: "Notes on Beekeeping", Published: published,
			Content: "<p>Advice about hives and honey.</p>",
		},
		apitest.Item{
			ID: "two", Title: "Sourdough Weekend", Published: published.Add(time.Hour),
			Content: "<p>Nothing about insects here.</p>",
		},
	))
	subscribe(t, h, feedURL)

	byTitle := search(t, h, "Beekeeping")
	if got := searchEntryTitles(byTitle.Entries); !equalStrings(got, []string{"Notes on Beekeeping"}) {
		t.Errorf("search by title = %v, want only the matching Entry", got)
	}

	byContent := search(t, h, "hives")
	if got := searchEntryTitles(byContent.Entries); !equalStrings(got, []string{"Notes on Beekeeping"}) {
		t.Errorf("search by content = %v, want only the Entry whose content matched", got)
	}
	if byContent.Entries[0].Snippet == "" {
		t.Error("a content match carried no snippet")
	}

	none := search(t, h, "xylophone")
	if len(none.Entries) != 0 {
		t.Errorf("search for an absent word = %v, want none", none.Entries)
	}
}

func TestSearchIndexesPlainTextNotMarkup(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{
			ID: "one", Title: "Two Words", Published: published,
			Content: `<p><a href="https://example.com/some-path"><img src="x.png"></a>bee<b>keeping</b></p>`,
		}))
	subscribe(t, h, feedURL)

	// A word only ever present as HTML markup — an attribute name, a tag
	// name, part of a URL — must not match: the index is the publisher's
	// words, not their markup.
	for _, word := range []string{"href", "img", "src", "example"} {
		if got := search(t, h, word).Entries; len(got) != 0 {
			t.Errorf("search for markup word %q = %v, want none", word, got)
		}
	}
	// A word split across an inline tag must still match whole.
	if got := search(t, h, "beekeeping").Entries; len(got) != 1 {
		t.Errorf("search for a word split by inline markup = %v, want the Entry", got)
	}
	if snippet := search(t, h, "beekeeping").Entries[0].Snippet; strings.ContainsAny(snippet, "<>") {
		t.Errorf("snippet %q still carries markup", snippet)
	}
}

func TestSearchMatchesFeedNamesForNavigation(t *testing.T) {
	h := loggedIn(t)
	beeURL := h.Publisher.Serve("/bee.xml", apitest.RSS("Beekeeping Weekly", "",
		apitest.Item{ID: "one", Title: "Post", Published: published}))
	bakeURL := h.Publisher.Serve("/bake.xml", apitest.RSS("Sourdough Journal", "",
		apitest.Item{ID: "two", Title: "Post", Published: published}))
	bee := subscribe(t, h, beeURL)
	subscribe(t, h, bakeURL)

	results := search(t, h, "beekeeping")
	if len(results.Feeds) != 1 || results.Feeds[0].ID != bee.ID {
		t.Fatalf("Feed search = %#v, want only %q", results.Feeds, bee.Title)
	}
}

func TestSearchIndexStaysCorrectAfterAnEntryUpdateAndDeletion(t *testing.T) {
	h := loggedIn(t)
	first := apitest.Item{
		ID: "one", Title: "Draft Title", Published: published,
		Content: "<p>original wording</p>",
	}
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "", first))
	feed := subscribe(t, h, feedURL)

	if got := search(t, h, "wording").Entries; len(got) != 1 {
		t.Fatalf("search before the edit = %v, want the Entry", got)
	}

	// Editing the Entry (a re-fetch that changes its content) must move the
	// index: the old word stops matching, the new one starts.
	edited := first
	edited.Title = "Final Title"
	edited.Content = "<p>revised phrasing</p>"
	h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "", edited))
	h.Do(http.MethodPost, "/api/feeds/refresh", nil).ExpectStatus(http.StatusOK)

	if got := search(t, h, "wording").Entries; len(got) != 0 {
		t.Errorf("search after the edit still matches the old content = %v", got)
	}
	if got := search(t, h, "phrasing").Entries; len(got) != 1 {
		t.Fatalf("search after the edit = %v, want the revised Entry", got)
	}

	// Deleting the Feed must remove its Entry from the index too.
	h.Do(http.MethodDelete, "/api/feeds/"+strconv.FormatInt(feed.ID, 10), nil).
		ExpectStatus(http.StatusNoContent)
	if got := search(t, h, "phrasing").Entries; len(got) != 0 {
		t.Errorf("search after deleting the Feed = %v, want none", got)
	}
}

func TestBlankSearchMatchesNothing(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "Something", Published: published}))
	subscribe(t, h, feedURL)

	results := search(t, h, "")
	if len(results.Entries) != 0 || len(results.Feeds) != 0 {
		t.Errorf("blank search = %#v, want no results", results)
	}
}

func TestSelectingASearchResultOpensTheEntryWithinItsListContext(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "Older Post", Published: published},
		apitest.Item{ID: "two", Title: "Findable Post", Published: published.Add(time.Hour)},
		apitest.Item{ID: "three", Title: "Newest Post", Published: published.Add(2 * time.Hour)}))
	feed := subscribe(t, h, feedURL)

	results := search(t, h, "Findable")
	if len(results.Entries) != 1 {
		t.Fatalf("search = %v, want exactly the one matching Entry", results.Entries)
	}
	target := results.Entries[0]

	page := listEntries(t, h, feedQuery(feed)+"&around="+strconv.FormatInt(target.ID, 10))
	if len(page.Entries) == 0 || page.Entries[0].ID != target.ID {
		t.Fatalf("page anchored at the search result = %v, want it first", page.Entries)
	}
	if got := entryTitles(page.Entries); !equalStrings(got, []string{"Findable Post", "Older Post"}) {
		t.Errorf("anchored page = %v, want the result and everything older in its list", got)
	}
}

func TestAroundAnUnknownEntryIsRefused(t *testing.T) {
	h := loggedIn(t)
	h.Do(http.MethodGet, "/api/entries?around=999999", nil).ExpectStatus(http.StatusNotFound)
	h.Do(http.MethodGet, "/api/entries?around=not-a-number", nil).ExpectStatus(http.StatusBadRequest)
}
