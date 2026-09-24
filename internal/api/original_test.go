package api_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gabe-santos/yogurt/internal/apitest"
)

// originalView is Original View as the API presents it: the address to embed,
// and whether the publisher permits embedding it.
type originalView struct {
	URL        string `json:"url"`
	Embeddable bool   `json:"embeddable"`
}

// getOriginal asks what Original View needs to know about an Entry.
func getOriginal(t *testing.T, h *apitest.Harness, entryID int64) *apitest.Response {
	t.Helper()
	return h.Do(http.MethodGet, "/api/entries/"+strconv.FormatInt(entryID, 10)+"/original", nil)
}

func readOriginal(t *testing.T, h *apitest.Harness, entryID int64) originalView {
	t.Helper()
	var body struct {
		Original originalView `json:"original"`
	}
	getOriginal(t, h, entryID).ExpectStatus(http.StatusOK).JSON(&body)
	return body.Original
}

func TestOriginalViewReportsTheAddressAndWhetherThePublisherAllowsEmbedding(t *testing.T) {
	h := loggedIn(t)
	allowed := h.Publisher.Serve("/allowed", apitest.Document{
		ContentType: "text/html; charset=utf-8",
		Body:        articleHTML,
	})
	forbidden := h.Publisher.Serve("/forbidden", apitest.Document{
		ContentType: "text/html; charset=utf-8",
		Headers:     map[string]string{"X-Frame-Options": "SAMEORIGIN"},
		Body:        articleHTML,
	})
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "Allowed", Link: allowed, Published: published},
		apitest.Item{ID: "two", Title: "Forbidden", Link: forbidden, Published: published.Add(time.Second)}))
	subscribe(t, h, feedURL)
	entries := listEntries(t, h, "").Entries

	for _, entry := range entries {
		original := readOriginal(t, h, entry.ID)
		switch entry.Title {
		case "Allowed":
			if original.URL != allowed {
				t.Errorf("original URL = %q, want the Entry's own link %q", original.URL, allowed)
			}
			if !original.Embeddable {
				t.Error("page sending no framing headers reported as not embeddable")
			}
		case "Forbidden":
			if original.Embeddable {
				t.Error("page sending X-Frame-Options reported as embeddable")
			}
		}
	}
}

func TestOriginalViewReadsTheFlagRecordedByReaderViewWithoutRefetching(t *testing.T) {
	h := loggedIn(t)
	articleURL := h.Publisher.Serve("/article", apitest.Document{
		ContentType: "text/html; charset=utf-8",
		Headers:     map[string]string{"X-Frame-Options": "DENY"},
		Body:        articleHTML,
	})
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Link: articleURL, Published: published}))
	subscribe(t, h, feedURL)
	entry := listEntries(t, h, "").Entries[0]

	getArticle(t, h, entry.ID).ExpectStatus(http.StatusOK)
	if got := h.Publisher.Hits("/article"); got != 1 {
		t.Fatalf("publisher hits after Reader View = %d, want 1", got)
	}

	if readOriginal(t, h, entry.ID).Embeddable {
		t.Error("Original View ignored the recorded refusal to be embedded")
	}
	if got := h.Publisher.Hits("/article"); got != 1 {
		t.Errorf("publisher hits after Original View = %d, want the recorded flag reused", got)
	}
}

// Original View exists for the pages Reader View cannot read — heavy on
// images, charts, and JavaScript-built layout — so a page with no extractable
// text must still report whether it can be embedded.
func TestOriginalViewWorksForAPageWithNoExtractableText(t *testing.T) {
	h := loggedIn(t)
	articleURL := h.Publisher.Serve("/blank", apitest.Document{
		ContentType: "text/html; charset=utf-8",
		Body:        blankHTML,
	})
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Link: articleURL, Published: published}))
	subscribe(t, h, feedURL)
	entry := listEntries(t, h, "").Entries[0]

	if !readOriginal(t, h, entry.ID).Embeddable {
		t.Error("page with no readable text and no framing headers refused Original View")
	}

	// Reader View still reports the failure it always did, rather than being
	// served an empty Article from whatever Original View left behind.
	getArticle(t, h, entry.ID).ExpectStatus(http.StatusBadGateway)
}

func TestOriginalViewOfAnUnreachablePageSaysSo(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Link: h.Publisher.URL("/gone"), Published: published}))
	subscribe(t, h, feedURL)
	entry := listEntries(t, h, "").Entries[0]

	resp := getOriginal(t, h, entry.ID).ExpectStatus(http.StatusBadGateway)
	var body struct {
		Error string `json:"error"`
	}
	resp.JSON(&body)
	if body.Error == "" {
		t.Fatal("unreachable page returned no message a reader could act on")
	}
}

func TestOriginalViewOfAnUnknownEntryIsNotFound(t *testing.T) {
	h := loggedIn(t)
	getOriginal(t, h, 9999).ExpectStatus(http.StatusNotFound)
}
