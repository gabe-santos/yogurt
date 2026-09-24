package api_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gabe-santos/yogurt/internal/apitest"
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

func TestTheEntryListCanBeOldestFirstAcrossPages(t *testing.T) {
	h := loggedIn(t)

	const count = 5
	items := make([]apitest.Item, 0, count)
	wantTitles := make([]string, 0, count)
	for i := range count {
		title := "Post " + strconv.Itoa(i)
		items = append(items, apitest.Item{
			ID:        "post-" + strconv.Itoa(i),
			Title:     title,
			Published: published.Add(time.Duration(i) * time.Hour),
		})
		wantTitles = append(wantTitles, title)
	}
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "", items...))
	feed := subscribe(t, h, feedURL)

	query := feedQuery(feed) + "&order=oldest&limit=2"
	page := listEntries(t, h, query)
	seen := entryTitles(page.Entries)
	for page.NextCursor != "" {
		page = listEntries(t, h, query+"&cursor="+page.NextCursor)
		seen = append(seen, entryTitles(page.Entries)...)
		if len(seen) > count {
			t.Fatalf("paging returned %d entries for %d Entries", len(seen), count)
		}
	}

	if !equalStrings(seen, wantTitles) {
		t.Errorf("oldest-first paged entries = %v, want %v", seen, wantTitles)
	}
}

func TestAnUnknownEntryOrderIsRefused(t *testing.T) {
	h := loggedIn(t)

	h.Do(http.MethodGet, "/api/entries?order=sideways", nil).ExpectStatus(http.StatusBadRequest)
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

type entryState struct {
	Read     bool `json:"read"`
	Starred  bool `json:"starred"`
	Archived bool `json:"archived"`
}

// setEntryState declares an Entry's complete state and returns it as stored.
func setEntryState(t *testing.T, h *apitest.Harness, id int64, state entryState) *apitest.Response {
	t.Helper()
	path := "/api/entries/" + strconv.FormatInt(id, 10) + "/state"
	return h.Do(http.MethodPut, path, state)
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
	setEntryState(t, h, entry.ID, entryState{Read: true}).ExpectStatus(http.StatusOK).JSON(&body)
	if !body.Entry.Read {
		t.Fatal("marking an Entry read did not report it as Read")
	}
	if got := listEntries(t, h, feedQuery(feed)).Entries[0]; !got.Read {
		t.Error("the reading list still reports the Entry as unread")
	}

	// Idempotent: declaring the same state twice is harmless, per ADR-0004.
	setEntryState(t, h, entry.ID, entryState{Read: true}).ExpectStatus(http.StatusOK).JSON(&body)
	if !body.Entry.Read {
		t.Fatal("replaying the same read declaration lost the state")
	}

	// Manual unread always overrides.
	setEntryState(t, h, entry.ID, entryState{Read: false}).ExpectStatus(http.StatusOK).JSON(&body)
	if body.Entry.Read {
		t.Fatal("marking an Entry unread did not report it as unread")
	}
	if got := listEntries(t, h, feedQuery(feed)).Entries[0]; got.Read {
		t.Error("the reading list still reports the Entry as read")
	}
}

func TestSettingStateOnAMissingEntryOrWithMissingFieldsIsRefused(t *testing.T) {
	h := loggedIn(t)
	setEntryState(t, h, 999999, entryState{Read: true}).ExpectStatus(http.StatusNotFound)
	entry := listEntries(t, h, "").Entries
	if len(entry) != 0 {
		t.Fatal("new reader unexpectedly has Entries")
	}
	h.Do(http.MethodPut, "/api/entries/1/state", map[string]bool{"read": true}).
		ExpectStatus(http.StatusBadRequest)
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

	setEntryState(t, h, all.Entries[0].ID, entryState{Read: true}).ExpectStatus(http.StatusOK)

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

func TestAnEntryCanBeStarredUnstarredAndListedAsStarred(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Published: published},
		apitest.Item{ID: "two", Title: "Two", Published: published.Add(time.Second)}))
	subscribe(t, h, feedURL)
	entries := listEntries(t, h, "").Entries
	starredID := entries[0].ID

	state := entryState{Starred: true}
	var body struct {
		Entry entryView `json:"entry"`
	}
	setEntryState(t, h, starredID, state).ExpectStatus(http.StatusOK).JSON(&body)
	if !body.Entry.Starred {
		t.Fatal("starring an Entry did not report it as Starred")
	}

	// Replaying the same declaration is harmless.
	setEntryState(t, h, starredID, state).ExpectStatus(http.StatusOK).JSON(&body)
	if !body.Entry.Starred {
		t.Fatal("replaying the Starred declaration lost the state")
	}
	// Read changes carry the complete current state, so they cannot silently
	// clear Starred while declaring Read.
	state.Read = true
	setEntryState(t, h, starredID, state).ExpectStatus(http.StatusOK).JSON(&body)
	if !body.Entry.Read || !body.Entry.Starred {
		t.Fatalf("read declaration returned (Read %t, Starred %t), want both true", body.Entry.Read, body.Entry.Starred)
	}
	starred := listEntries(t, h, "starred=true").Entries
	if len(starred) != 1 || starred[0].ID != starredID {
		t.Fatalf("Starred view after marking Read = %#v, want only Entry %d", starred, starredID)
	}

	state.Starred = false
	setEntryState(t, h, starredID, state).ExpectStatus(http.StatusOK).JSON(&body)
	if body.Entry.Starred {
		t.Fatal("unstarring an Entry still reports it as Starred")
	}
	if got := listEntries(t, h, "starred=true").Entries; len(got) != 0 {
		t.Fatalf("Starred view after unstarring = %#v, want empty", got)
	}
}

func TestArchivingAnEntryImpliesReadAndLeavesOnlyTheArchiveView(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Published: published},
		apitest.Item{ID: "two", Title: "Two", Published: published.Add(time.Second)}))
	subscribe(t, h, feedURL)
	entries := listEntries(t, h, "").Entries
	archivedID := entries[0].ID

	var body struct {
		Entry entryView `json:"entry"`
	}
	setEntryState(t, h, archivedID, entryState{
		Read: false, Starred: true, Archived: true,
	}).ExpectStatus(http.StatusOK).JSON(&body)
	if !body.Entry.Archived || !body.Entry.Read {
		t.Fatalf("archived state = (Archived %t, Read %t), want both true", body.Entry.Archived, body.Entry.Read)
	}

	for _, query := range []string{"", "unread=true", "starred=true"} {
		for _, entry := range listEntries(t, h, query).Entries {
			if entry.ID == archivedID {
				t.Errorf("Entry %d still appears in view %q after archiving", archivedID, query)
			}
		}
	}
	archived := listEntries(t, h, "archived=true").Entries
	if len(archived) != 1 || archived[0].ID != archivedID {
		t.Fatalf("Archive view = %#v, want only Entry %d", archived, archivedID)
	}
	h.Do(http.MethodPut, "/api/entries/state?archived=true", map[string]bool{"read": false}).
		ExpectStatus(http.StatusBadRequest)
	if got := listEntries(t, h, "archived=true").Entries[0]; !got.Read {
		t.Fatal("rejected bulk unread declaration still made an Archived Entry unread")
	}
}

func TestUnarchivingAnEntryReturnsItToTheOrdinaryReadingListAndOutOfTheArchive(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Published: published}))
	subscribe(t, h, feedURL)
	id := listEntries(t, h, "").Entries[0].ID

	setEntryState(t, h, id, entryState{Read: true, Archived: true}).
		ExpectStatus(http.StatusOK)

	var body struct {
		Entry entryView `json:"entry"`
	}
	// Declaring the complete state with archived false, and read left true
	// as an already-Archived Entry is always Read, is the whole of
	// unarchiving: no dedicated endpoint exists for it.
	setEntryState(t, h, id, entryState{Read: true, Archived: false}).
		ExpectStatus(http.StatusOK).JSON(&body)
	if body.Entry.Archived {
		t.Fatalf("unarchived Entry Archived = %t, want false", body.Entry.Archived)
	}
	if !body.Entry.Read {
		t.Fatalf("unarchived Entry Read = %t, want true", body.Entry.Read)
	}

	if got := listEntries(t, h, "archived=true").Entries; len(got) != 0 {
		t.Fatalf("Archive view = %#v, want empty after unarchiving", got)
	}
	found := false
	for _, entry := range listEntries(t, h, "").Entries {
		if entry.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("Entry %d missing from the ordinary reading list after unarchiving", id)
	}
}

func TestMarkAllReadAffectsOnlyTheCurrentFilterAndScope(t *testing.T) {
	h := loggedIn(t)
	oneURL := h.Publisher.Serve("/one.xml", apitest.RSS("One", "",
		apitest.Item{ID: "one-a", Title: "One A", Published: published},
		apitest.Item{ID: "one-b", Title: "One B", Published: published.Add(time.Second)}))
	twoURL := h.Publisher.Serve("/two.xml", apitest.RSS("Two", "",
		apitest.Item{ID: "two-a", Title: "Two A", Published: published.Add(2 * time.Second)}))
	one := subscribe(t, h, oneURL)
	two := subscribe(t, h, twoURL)
	oneEntries := listEntries(t, h, feedQuery(one)).Entries
	twoEntry := listEntries(t, h, feedQuery(two)).Entries[0]

	star := func(entry entryView) {
		setEntryState(t, h, entry.ID, entryState{Starred: true}).ExpectStatus(http.StatusOK)
	}
	star(oneEntries[0])
	star(twoEntry)

	path := "/api/entries/state?" + feedQuery(one) + "&starred=true"
	h.Do(http.MethodPut, path, map[string]bool{"read": true}).ExpectStatus(http.StatusNoContent)

	oneEntries = listEntries(t, h, feedQuery(one)).Entries
	for _, entry := range oneEntries {
		if entry.ID == oneEntries[0].ID && entry.Title == "One B" && !entry.Read {
			t.Error("the Starred Entry in the selected Feed is still unread")
		}
		if entry.Title == "One A" && entry.Read {
			t.Error("the unstarred Entry in the selected Feed was marked Read")
		}
	}
	if got := listEntries(t, h, feedQuery(two)).Entries[0]; got.Read {
		t.Error("a Starred Entry outside the selected Feed was marked Read")
	}
}
