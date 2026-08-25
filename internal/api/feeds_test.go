package api_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gabe-santos/rss-reader/internal/apitest"
)

// published is an arbitrary instant the fake publishers date their items from.
var published = time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)

type feedView struct {
	ID          int64  `json:"id"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	SiteURL     string `json:"site_url"`
	GroupID     int64  `json:"group_id"`
	Suspended   bool   `json:"suspended"`
	UnreadCount int    `json:"unread_count"`
}

type entryView struct {
	ID          int64  `json:"id"`
	FeedID      int64  `json:"feed_id"`
	FeedTitle   string `json:"feed_title"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	PublishedAt string `json:"published_at"`
	Content     string `json:"content"`
	Read        bool   `json:"read"`
	Starred     bool   `json:"starred"`
	Archived    bool   `json:"archived"`
}

type entryPage struct {
	Entries    []entryView `json:"entries"`
	NextCursor string      `json:"next_cursor"`
}

// loggedIn boots a harness and signs in, which every Feed test needs first.
func loggedIn(t *testing.T, opts ...apitest.Option) *apitest.Harness {
	t.Helper()
	h := apitest.New(t, opts...)
	h.Login(apitest.Password).ExpectStatus(http.StatusNoContent)
	return h
}

// subscribe adds a Feed and returns it, failing the test if it was refused.
func subscribe(t *testing.T, h *apitest.Harness, url string) feedView {
	t.Helper()
	var body struct {
		Feed feedView `json:"feed"`
	}
	h.Do(http.MethodPost, "/api/feeds", map[string]string{"url": url}).
		ExpectStatus(http.StatusCreated).
		JSON(&body)
	return body.Feed
}

// listFeeds reads the reader's whole collection.
func listFeeds(t *testing.T, h *apitest.Harness) []feedView {
	t.Helper()
	var body struct {
		Feeds []feedView `json:"feeds"`
	}
	h.Do(http.MethodGet, "/api/feeds", nil).ExpectStatus(http.StatusOK).JSON(&body)
	return body.Feeds
}

// listEntries reads one page of Entries, query being everything after "?".
func listEntries(t *testing.T, h *apitest.Harness, query string) entryPage {
	t.Helper()
	path := "/api/entries"
	if query != "" {
		path += "?" + query
	}
	var page entryPage
	h.Do(http.MethodGet, path, nil).ExpectStatus(http.StatusOK).JSON(&page)
	return page
}

func entryTitles(entries []entryView) []string {
	titles := make([]string, 0, len(entries))
	for _, entry := range entries {
		titles = append(titles, entry.Title)
	}
	return titles
}

func feedQuery(feed feedView) string {
	return "feed=" + strconv.FormatInt(feed.ID, 10)
}

// expectErrorMentions fails unless the refusal says something a reader could act
// on, rather than a bare status code.
func expectErrorMentions(t *testing.T, resp *apitest.Response, want string) {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	resp.JSON(&body)
	if !strings.Contains(strings.ToLower(body.Error), strings.ToLower(want)) {
		t.Errorf("error message %q does not mention %q", body.Error, want)
	}
}

func TestSubscribingToAFeedURLStoresItsEntries(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", h.Publisher.URL("/"),
		apitest.Item{
			ID: "one", Title: "First post", Link: h.Publisher.URL("/one"),
			Published: published, Content: "<p>the first</p>",
		},
		apitest.Item{
			ID: "two", Title: "Second post", Link: h.Publisher.URL("/two"),
			Published: published.Add(time.Hour), Content: "<p>the second</p>",
		},
	))

	feed := subscribe(t, h, feedURL)
	if feed.Title != "The Publisher" {
		t.Errorf("feed title = %q, want the publisher's own title", feed.Title)
	}
	if feed.URL != feedURL {
		t.Errorf("feed url = %q, want %q", feed.URL, feedURL)
	}
	if feed.SiteURL != h.Publisher.URL("/") {
		t.Errorf("feed site_url = %q, want the page the Feed belongs to", feed.SiteURL)
	}
	if got := h.Publisher.Hits("/feed.xml"); got != 1 {
		t.Errorf("publisher hits = %d, want one initial fetch", got)
	}

	page := listEntries(t, h, feedQuery(feed))
	if want := []string{"Second post", "First post"}; !equalStrings(entryTitles(page.Entries), want) {
		t.Fatalf("entry titles = %v, want %v (newest first)", entryTitles(page.Entries), want)
	}

	newest := page.Entries[0]
	if newest.Content != "<p>the second</p>" {
		t.Errorf("entry content = %q, want the content the Feed supplied", newest.Content)
	}
	if newest.URL != h.Publisher.URL("/two") {
		t.Errorf("entry url = %q, want the publisher's link", newest.URL)
	}
	if newest.FeedID != feed.ID || newest.FeedTitle != feed.Title {
		t.Errorf("entry feed = (%d, %q), want (%d, %q)", newest.FeedID, newest.FeedTitle, feed.ID, feed.Title)
	}
	at, err := time.Parse(time.RFC3339, newest.PublishedAt)
	if err != nil {
		t.Fatalf("published_at %q: %v", newest.PublishedAt, err)
	}
	if !at.Equal(published.Add(time.Hour)) {
		t.Errorf("published_at = %s, want %s", at, published.Add(time.Hour))
	}
}

func TestSubscribingToASiteURLDiscoversItsFeed(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", h.Publisher.URL("/"),
		apitest.Item{ID: "one", Title: "First post", Published: published},
	))
	// The page advertises its Feed relatively, as publishers do.
	siteURL := h.Publisher.Serve("/", apitest.Page("The Publisher", "/feed.xml"))

	feed := subscribe(t, h, siteURL)
	if feed.URL != feedURL {
		t.Errorf("feed url = %q, want the discovered Feed %q", feed.URL, feedURL)
	}

	page := listEntries(t, h, feedQuery(feed))
	if len(page.Entries) != 1 {
		t.Errorf("entries after discovery = %d, want 1", len(page.Entries))
	}
}

func TestSubscribingToSomethingThatIsNotAFeedIsRefused(t *testing.T) {
	h := loggedIn(t)

	bare := h.Publisher.Serve("/bare", apitest.Page("A page with no Feed"))
	resp := h.Do(http.MethodPost, "/api/feeds", map[string]string{"url": bare}).
		ExpectStatus(http.StatusUnprocessableEntity)
	expectErrorMentions(t, resp, "feed")

	// A URL that answers with an error is a different failure, and says so.
	resp = h.Do(http.MethodPost, "/api/feeds", map[string]string{"url": h.Publisher.URL("/nothing-here")}).
		ExpectStatus(http.StatusBadGateway)
	expectErrorMentions(t, resp, "fetch")

	// So is something that is not an address at all.
	resp = h.Do(http.MethodPost, "/api/feeds", map[string]string{"url": "not an address"}).
		ExpectStatus(http.StatusBadRequest)
	expectErrorMentions(t, resp, "url")

	if feeds := listFeeds(t, h); len(feeds) != 0 {
		t.Errorf("feeds after three refusals = %d, want none", len(feeds))
	}
}

func TestSubscribingToAFeedAlreadySubscribedIsRefused(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", h.Publisher.URL("/"),
		apitest.Item{ID: "one", Title: "First post", Published: published},
	))
	subscribe(t, h, feedURL)

	resp := h.Do(http.MethodPost, "/api/feeds", map[string]string{"url": feedURL}).
		ExpectStatus(http.StatusConflict)
	expectErrorMentions(t, resp, "already")

	// Discovery that lands on a Feed already subscribed is refused just the same.
	siteURL := h.Publisher.Serve("/", apitest.Page("The Publisher", "/feed.xml"))
	h.Do(http.MethodPost, "/api/feeds", map[string]string{"url": siteURL}).
		ExpectStatus(http.StatusConflict)

	if feeds := listFeeds(t, h); len(feeds) != 1 {
		t.Errorf("feeds = %d, want the one Feed subscribed once", len(feeds))
	}
}

func TestAFeedInAtomOrJSONFormIsSubscribedTheSameWay(t *testing.T) {
	h := loggedIn(t)

	atomURL := h.Publisher.Serve("/atom.xml", apitest.Atom("Atom Publisher", h.Publisher.URL("/atom/"),
		apitest.Item{
			ID: "tag:atom,1", Title: "An Atom post", Link: h.Publisher.URL("/atom/one"),
			Published: published, Content: "<p>from Atom</p>",
		},
	))
	jsonURL := h.Publisher.Serve("/feed.json", apitest.JSONFeed("JSON Publisher", h.Publisher.URL("/json/"),
		apitest.Item{
			ID: "json-1", Title: "A JSON post", Link: h.Publisher.URL("/json/one"),
			Published: published, Content: "<p>from JSON Feed</p>",
		},
	))

	for _, feedURL := range []string{atomURL, jsonURL} {
		feed := subscribe(t, h, feedURL)
		page := listEntries(t, h, feedQuery(feed))
		if len(page.Entries) != 1 {
			t.Fatalf("%s: entries = %d, want 1", feedURL, len(page.Entries))
		}
		entry := page.Entries[0]
		if entry.Title == "" || entry.Content == "" || entry.URL == "" {
			t.Errorf("%s: entry = %+v, want title, content and link carried over", feedURL, entry)
		}
	}
}

func TestAnEntrySeenTwiceIsStoredOnceAndAnEditedEntryUpdatesInPlace(t *testing.T) {
	h := loggedIn(t)
	repeated := apitest.Item{
		ID: "one", Title: "First post", Link: h.Publisher.URL("/one"),
		Published: published, Content: "<p>the first</p>",
	}
	second := apitest.Item{
		ID: "two", Title: "Second post", Link: h.Publisher.URL("/two"),
		Published: published.Add(time.Hour), Content: "<p>the second</p>",
	}
	// A sloppy publisher carries the same item twice in one document.
	feedURL := h.Publisher.Serve("/feed.xml",
		apitest.RSS("The Publisher", h.Publisher.URL("/"), repeated, repeated, second))

	feed := subscribe(t, h, feedURL)
	page := listEntries(t, h, feedQuery(feed))
	if len(page.Entries) != 2 {
		t.Fatalf("entry titles = %v, want the duplicate stored once", entryTitles(page.Entries))
	}
	before := page.Entries[1]

	// The publisher then corrects that post rather than publishing a new one.
	edited := repeated
	edited.Title = "First post, corrected"
	edited.Content = "<p>the first, corrected</p>"
	h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", h.Publisher.URL("/"), edited, second))
	h.Do(http.MethodPost, "/api/feeds/refresh", nil).ExpectStatus(http.StatusOK)

	page = listEntries(t, h, feedQuery(feed))
	if len(page.Entries) != 2 {
		t.Fatalf("entry titles = %v, want the edit to update in place", entryTitles(page.Entries))
	}
	after := page.Entries[1]
	if after.ID != before.ID {
		t.Errorf("entry id = %d, want the edited Entry to keep id %d", after.ID, before.ID)
	}
	if after.Title != "First post, corrected" || after.Content != "<p>the first, corrected</p>" {
		t.Errorf("entry after edit = %+v, want the publisher's correction", after)
	}
}

func TestRefreshingAllFeedsFetchesEveryFeedAgain(t *testing.T) {
	h := loggedIn(t)
	first := apitest.Item{ID: "one", Title: "First post", Published: published}
	oneURL := h.Publisher.Serve("/one.xml", apitest.RSS("One", "", first))
	twoURL := h.Publisher.Serve("/two.xml", apitest.RSS("Two", "", first))
	oneFeed := subscribe(t, h, oneURL)
	twoFeed := subscribe(t, h, twoURL)

	later := apitest.Item{ID: "two", Title: "Later post", Published: published.Add(time.Hour)}
	h.Publisher.Serve("/one.xml", apitest.RSS("One", "", first, later))
	h.Publisher.Serve("/two.xml", apitest.RSS("Two", "", first, later))

	h.Do(http.MethodPost, "/api/feeds/refresh", nil).ExpectStatus(http.StatusOK)

	for _, path := range []string{"/one.xml", "/two.xml"} {
		if got := h.Publisher.Hits(path); got != 2 {
			t.Errorf("%s hits = %d, want the initial fetch plus the refresh", path, got)
		}
	}
	for _, feed := range []feedView{oneFeed, twoFeed} {
		page := listEntries(t, h, feedQuery(feed))
		if want := []string{"Later post", "First post"}; !equalStrings(entryTitles(page.Entries), want) {
			t.Errorf("feed %q entries = %v, want %v", feed.Title, entryTitles(page.Entries), want)
		}
	}
}

func TestRefreshingOneFeedLeavesTheOthersAlone(t *testing.T) {
	h := loggedIn(t)
	first := apitest.Item{ID: "one", Title: "First post", Published: published}
	oneURL := h.Publisher.Serve("/one.xml", apitest.RSS("One", "", first))
	h.Publisher.Serve("/two.xml", apitest.RSS("Two", "", first))
	oneFeed := subscribe(t, h, oneURL)
	subscribe(t, h, h.Publisher.URL("/two.xml"))

	h.Do(http.MethodPost, "/api/feeds/"+strconv.FormatInt(oneFeed.ID, 10)+"/refresh", nil).
		ExpectStatus(http.StatusNoContent)

	if got := h.Publisher.Hits("/one.xml"); got != 2 {
		t.Errorf("refreshed Feed hits = %d, want 2", got)
	}
	if got := h.Publisher.Hits("/two.xml"); got != 1 {
		t.Errorf("untouched Feed hits = %d, want the initial fetch only", got)
	}

	h.Do(http.MethodPost, "/api/feeds/9999/refresh", nil).ExpectStatus(http.StatusNotFound)
}

func TestFeedsOnPrivateNetworkAddressesAreRefusedByDefault(t *testing.T) {
	h := loggedIn(t, apitest.BlockPrivateFetch())
	// The fake publisher is on loopback, which is exactly what the guard is for.
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "First post", Published: published},
	))

	resp := h.Do(http.MethodPost, "/api/feeds", map[string]string{"url": feedURL}).
		ExpectStatus(http.StatusBadGateway)
	expectErrorMentions(t, resp, "private")

	if got := h.Publisher.Hits("/feed.xml"); got != 0 {
		t.Errorf("publisher hits = %d, want the connection refused before it was made", got)
	}
}

func TestFeedAndEntryEndpointsRequireASession(t *testing.T) {
	h := apitest.New(t)

	h.Do(http.MethodGet, "/api/feeds", nil).ExpectStatus(http.StatusUnauthorized)
	h.Do(http.MethodPost, "/api/feeds", map[string]string{"url": "https://example.com/feed.xml"}).
		ExpectStatus(http.StatusUnauthorized)
	h.Do(http.MethodPost, "/api/feeds/refresh", nil).ExpectStatus(http.StatusUnauthorized)
	h.Do(http.MethodPost, "/api/feeds/1/refresh", nil).ExpectStatus(http.StatusUnauthorized)
	h.Do(http.MethodGet, "/api/entries", nil).ExpectStatus(http.StatusUnauthorized)
	h.Do(http.MethodPut, "/api/entries/1/state", map[string]any{"read": true}).
		ExpectStatus(http.StatusUnauthorized)
	h.Do(http.MethodGet, "/api/entries/1/article", nil).ExpectStatus(http.StatusUnauthorized)
	h.Do(http.MethodGet, "/api/entries/1/original", nil).ExpectStatus(http.StatusUnauthorized)
	h.Do(http.MethodGet, "/api/settings", nil).ExpectStatus(http.StatusUnauthorized)
	h.Do(http.MethodPut, "/api/settings", map[string]any{"mark_on_open": true}).
		ExpectStatus(http.StatusUnauthorized)
	h.DoRaw(http.MethodPost, "/api/opml/import", "text/x-opml", []byte("<opml/>")).
		ExpectStatus(http.StatusUnauthorized)
	h.Do(http.MethodGet, "/api/opml/export", nil).ExpectStatus(http.StatusUnauthorized)
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
