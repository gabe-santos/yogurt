package api_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gabe-santos/rss-reader/internal/apitest"
)

func TestTheEntryListIsNewestFirstAndPagesWithAnOpaqueCursor(t *testing.T) {
	h := loggedIn(t)

	const count = 5
	items := make([]apitest.Item, 0, count)
	wantTitles := make([]string, 0, count)
	for i := range count {
		items = append(items, apitest.Item{
			ID:        "post-" + strconv.Itoa(i),
			Title:     "Post " + strconv.Itoa(i),
			Published: published.Add(time.Duration(i) * time.Hour),
		})
		// Newest first: the last item published is the first one listed.
		wantTitles = append([]string{"Post " + strconv.Itoa(i)}, wantTitles...)
	}
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "", items...))
	feed := subscribe(t, h, feedURL)

	page := listEntries(t, h, feedQuery(feed)+"&limit=2")
	if len(page.Entries) != 2 {
		t.Fatalf("first page = %d entries, want the requested 2", len(page.Entries))
	}
	if page.NextCursor == "" {
		t.Fatal("first page carries no cursor, so the rest is unreachable")
	}
	// The cursor is opaque: it is not an offset the caller could have computed.
	if _, err := strconv.Atoi(page.NextCursor); err == nil {
		t.Errorf("cursor %q is a number, not an opaque cursor", page.NextCursor)
	}

	seen := entryTitles(page.Entries)
	for page.NextCursor != "" {
		page = listEntries(t, h, feedQuery(feed)+"&limit=2&cursor="+page.NextCursor)
		if len(page.Entries) == 0 {
			t.Fatal("a cursor returned an empty page instead of ending the list")
		}
		seen = append(seen, entryTitles(page.Entries)...)
		if len(seen) > count {
			t.Fatalf("paging returned %d entries for %d Entries", len(seen), count)
		}
	}

	if !equalStrings(seen, wantTitles) {
		t.Errorf("paged entries = %v, want %v", seen, wantTitles)
	}
}

func TestAnUnreadableCursorIsRefused(t *testing.T) {
	h := loggedIn(t)

	h.Do(http.MethodGet, "/api/entries?cursor=not-a-cursor", nil).ExpectStatus(http.StatusBadRequest)
}

func TestTheEntryListCanBeScopedToOneFeed(t *testing.T) {
	h := loggedIn(t)
	oneURL := h.Publisher.Serve("/one.xml", apitest.RSS("One", "",
		apitest.Item{ID: "one", Title: "From One", Published: published}))
	twoURL := h.Publisher.Serve("/two.xml", apitest.RSS("Two", "",
		apitest.Item{ID: "two", Title: "From Two", Published: published.Add(time.Hour)}))
	one := subscribe(t, h, oneURL)
	two := subscribe(t, h, twoURL)

	if got := entryTitles(listEntries(t, h, feedQuery(one)).Entries); !equalStrings(got, []string{"From One"}) {
		t.Errorf("entries scoped to One = %v, want only its own", got)
	}
	if got := entryTitles(listEntries(t, h, feedQuery(two)).Entries); !equalStrings(got, []string{"From Two"}) {
		t.Errorf("entries scoped to Two = %v, want only its own", got)
	}
	// Unscoped, the list is every Feed's Entries together, still newest first.
	if got := entryTitles(listEntries(t, h, "").Entries); !equalStrings(got, []string{"From Two", "From One"}) {
		t.Errorf("unscoped entries = %v, want both Feeds newest first", got)
	}
}

// setEntryState declares an Entry's Read state and returns it as stored.
func setEntryState(t *testing.T, h *apitest.Harness, id int64, read bool) *apitest.Response {
	t.Helper()
	path := "/api/entries/" + strconv.FormatInt(id, 10) + "/state"
	return h.Do(http.MethodPut, path, map[string]any{"read": read})
}

func TestOpeningAnEntryMarksItReadByHandAndUnreadReversesIt(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Published: published}))
	feed := subscribe(t, h, feedURL)
	entry := listEntries(t, h, feedQuery(feed)).Entries[0]
	if entry.Read {
		t.Fatal("a freshly stored Entry is already Read")
	}

	var body struct {
		Entry entryView `json:"entry"`
	}
	setEntryState(t, h, entry.ID, true).ExpectStatus(http.StatusOK).JSON(&body)
	if !body.Entry.Read {
		t.Fatal("marking an Entry read did not report it as Read")
	}
	if got := listEntries(t, h, feedQuery(feed)).Entries[0]; !got.Read {
		t.Error("the reading list still reports the Entry as unread")
	}

	// Idempotent: declaring the same state twice is harmless, per ADR-0004.
	setEntryState(t, h, entry.ID, true).ExpectStatus(http.StatusOK).JSON(&body)
	if !body.Entry.Read {
		t.Fatal("replaying the same read declaration lost the state")
	}

	// Manual unread always overrides.
	setEntryState(t, h, entry.ID, false).ExpectStatus(http.StatusOK).JSON(&body)
	if body.Entry.Read {
		t.Fatal("marking an Entry unread did not report it as unread")
	}
	if got := listEntries(t, h, feedQuery(feed)).Entries[0]; got.Read {
		t.Error("the reading list still reports the Entry as read")
	}
}

func TestSettingStateOnAMissingEntryIsRefused(t *testing.T) {
	h := loggedIn(t)
	setEntryState(t, h, 999999, true).ExpectStatus(http.StatusNotFound)
}

func TestTheUnreadFilterScopesTheEntryList(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Published: published},
		apitest.Item{ID: "two", Title: "Two", Published: published.Add(1)}))
	subscribe(t, h, feedURL)

	all := listEntries(t, h, "")
	if len(all.Entries) != 2 {
		t.Fatalf("unscoped list = %d entries, want 2", len(all.Entries))
	}

	setEntryState(t, h, all.Entries[0].ID, true).ExpectStatus(http.StatusOK)

	unread := listEntries(t, h, "unread=true")
	if len(unread.Entries) != 1 {
		t.Fatalf("unread list = %d entries, want 1", len(unread.Entries))
	}
	if unread.Entries[0].ID != all.Entries[1].ID {
		t.Errorf("unread list carried the Read Entry")
	}
}

func TestEntryContentIsSanitisedAgainstDangerousMarkup(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{
			ID:        "one",
			Title:     "One",
			Published: published,
			Content:   `<p>safe text</p><script>alert(1)</script><img src=x onerror=alert(2)>`,
		}))
	feed := subscribe(t, h, feedURL)

	entry := listEntries(t, h, feedQuery(feed)).Entries[0]
	if want := "safe text"; !strings.Contains(entry.Content, want) {
		t.Errorf("sanitised content = %q, lost the safe text %q", entry.Content, want)
	}
	if strings.Contains(entry.Content, "<script") {
		t.Errorf("sanitised content = %q, still carries a script tag", entry.Content)
	}
	if strings.Contains(entry.Content, "onerror") {
		t.Errorf("sanitised content = %q, still carries an event handler", entry.Content)
	}
}
