package api_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gabe-santos/rss-reader/internal/apitest"
)

// articleHTML is a realistic publisher page: enough substantive prose that
// readability recognises it as an Article, plus markup a hostile or careless
// publisher might send that extraction must render harmless — a script tag, an
// inline event handler, a one-pixel tracking image, and an insecure image
// source.
const articleHTML = `<!DOCTYPE html>
<html><head><title>A Real Article</title></head>
<body>
<nav>Site navigation the reader does not want</nav>
<article>
<h1>A Real Article</h1>
<p>This is the first paragraph of a genuinely interesting article about
something worth reading, with enough words to satisfy the heuristics
extraction uses to recognise real body text rather than boilerplate
navigation chrome surrounding it.</p>
<script>alert('stolen')</script>
<p onclick="alert('click')">This is the second paragraph, continuing the
discussion with more substantive content and further elaboration on the
topic at hand, again padded out with enough prose to pass the content
density checks extraction performs internally before it decides this is
worth keeping.</p>
<img src="https://example.com/tracking.gif" width="1" height="1" alt="">
<img src="http://example.com/photo.jpg" alt="A photo worth seeing">
<p>A third paragraph wraps up the piece with a concluding thought, some
further detail, and enough additional words to keep the parser satisfied
about the total length of the extracted content block it produces.</p>
</article>
<footer>Footer junk the reader does not want either</footer>
</body></html>`

// blankHTML carries no text extraction could ever recognise as an Article.
const blankHTML = `<!DOCTYPE html><html><head><title>Nothing here</title></head><body></body></html>`

// getArticle requests Reader View for an Entry.
func getArticle(t *testing.T, h *apitest.Harness, entryID int64) *apitest.Response {
	t.Helper()
	return h.Do(http.MethodGet, "/api/entries/"+strconv.FormatInt(entryID, 10)+"/article", nil)
}

func TestReaderViewExtractsSanitisesAndReusesTheArticle(t *testing.T) {
	h := loggedIn(t)
	articleURL := h.Publisher.Serve("/article", apitest.Document{
		ContentType: "text/html; charset=utf-8",
		Body:        articleHTML,
	})
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Link: articleURL, Published: published}))
	subscribe(t, h, feedURL)
	entry := listEntries(t, h, "").Entries[0]

	var body struct {
		Article struct {
			Title      string `json:"title"`
			HTML       string `json:"html"`
			Embeddable bool   `json:"embeddable"`
		} `json:"article"`
	}
	getArticle(t, h, entry.ID).ExpectStatus(http.StatusOK).JSON(&body)

	if want := "first paragraph"; !strings.Contains(body.Article.HTML, want) {
		t.Errorf("extracted HTML = %q, lost the safe text %q", body.Article.HTML, want)
	}
	if strings.Contains(body.Article.HTML, "<script") {
		t.Errorf("extracted HTML = %q, still carries a script tag", body.Article.HTML)
	}
	if strings.Contains(body.Article.HTML, "onclick") {
		t.Errorf("extracted HTML = %q, still carries an event handler", body.Article.HTML)
	}
	if strings.Contains(body.Article.HTML, "tracking.gif") {
		t.Errorf("extracted HTML = %q, still carries the one-pixel tracking image", body.Article.HTML)
	}
	if strings.Contains(body.Article.HTML, "http://example.com/photo.jpg") {
		t.Errorf("extracted HTML = %q, insecure image source was not upgraded", body.Article.HTML)
	}
	if !strings.Contains(body.Article.HTML, "https://example.com/photo.jpg") {
		t.Errorf("extracted HTML = %q, upgraded image source is missing", body.Article.HTML)
	}
	if !strings.Contains(body.Article.HTML, `referrerpolicy="no-referrer"`) {
		t.Errorf("extracted HTML = %q, image carries no no-referrer policy", body.Article.HTML)
	}
	if !body.Article.Embeddable {
		t.Error("article with no framing headers reported as not embeddable")
	}
	if h.Publisher.Hits("/article") != 1 {
		t.Fatalf("publisher hits after first request = %d, want 1", h.Publisher.Hits("/article"))
	}

	// A second request for the same Entry is served from storage, without
	// refetching the publisher.
	var again struct {
		Article struct {
			HTML string `json:"html"`
		} `json:"article"`
	}
	getArticle(t, h, entry.ID).ExpectStatus(http.StatusOK).JSON(&again)
	if again.Article.HTML != body.Article.HTML {
		t.Errorf("reused article HTML = %q, want the stored %q", again.Article.HTML, body.Article.HTML)
	}
	if h.Publisher.Hits("/article") != 1 {
		t.Fatalf("publisher hits after reused request = %d, want still 1", h.Publisher.Hits("/article"))
	}
}

func TestReaderViewRecordsWhenEmbeddingIsForbidden(t *testing.T) {
	h := loggedIn(t)
	articleURL := h.Publisher.Serve("/forbidden", apitest.Document{
		ContentType: "text/html; charset=utf-8",
		Headers:     map[string]string{"X-Frame-Options": "DENY"},
		Body:        articleHTML,
	})
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Link: articleURL, Published: published}))
	subscribe(t, h, feedURL)
	entry := listEntries(t, h, "").Entries[0]

	var body struct {
		Article struct {
			Embeddable bool `json:"embeddable"`
		} `json:"article"`
	}
	getArticle(t, h, entry.ID).ExpectStatus(http.StatusOK).JSON(&body)
	if body.Article.Embeddable {
		t.Error("article served with X-Frame-Options reported as embeddable")
	}
}

func TestReaderViewRecordsEmbeddingFromContentSecurityPolicy(t *testing.T) {
	h := loggedIn(t)
	forbidden := h.Publisher.Serve("/csp-forbidden", apitest.Document{
		ContentType: "text/html; charset=utf-8",
		Headers:     map[string]string{"Content-Security-Policy": "default-src 'self'; frame-ancestors 'none'"},
		Body:        articleHTML,
	})
	allowed := h.Publisher.Serve("/csp-allowed", apitest.Document{
		ContentType: "text/html; charset=utf-8",
		Headers:     map[string]string{"Content-Security-Policy": "default-src 'self'; frame-ancestors *"},
		Body:        articleHTML,
	})
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Link: forbidden, Published: published},
		apitest.Item{ID: "two", Title: "Two", Link: allowed, Published: published.Add(time.Second)}))
	subscribe(t, h, feedURL)
	entries := listEntries(t, h, "").Entries

	var body struct {
		Article struct {
			Embeddable bool `json:"embeddable"`
		} `json:"article"`
	}
	for _, entry := range entries {
		getArticle(t, h, entry.ID).ExpectStatus(http.StatusOK).JSON(&body)
		switch entry.Title {
		case "One":
			if body.Article.Embeddable {
				t.Error("frame-ancestors 'none' reported the page as embeddable")
			}
		case "Two":
			if !body.Article.Embeddable {
				t.Error("frame-ancestors * reported the page as not embeddable")
			}
		}
	}
}

func TestFailedExtractionReturnsAClearMessage(t *testing.T) {
	h := loggedIn(t)
	articleURL := h.Publisher.Serve("/blank", apitest.Document{
		ContentType: "text/html; charset=utf-8",
		Body:        blankHTML,
	})
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Link: articleURL, Published: published}))
	subscribe(t, h, feedURL)
	entry := listEntries(t, h, "").Entries[0]

	resp := getArticle(t, h, entry.ID)
	resp.ExpectStatus(http.StatusBadGateway)
	var body struct {
		Error string `json:"error"`
	}
	resp.JSON(&body)
	if body.Error == "" {
		t.Fatal("failed extraction returned no error message a reader could act on")
	}
}
