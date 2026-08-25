package api_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gabe-santos/rss-reader/internal/apitest"
)

// sincePage is one response from the changed-since feed.
type sincePage struct {
	Entries    []entryView `json:"entries"`
	Tombstones []int64     `json:"tombstones"`
	NextSince  string      `json:"next_since"`
}

// readSince reads the changed-since feed from a since cursor, "" meaning the
// start of history.
func readSince(t *testing.T, h *apitest.Harness, since string) sincePage {
	t.Helper()
	var page sincePage
	h.Do(http.MethodGet, "/api/entries?since="+since, nil).ExpectStatus(http.StatusOK).JSON(&page)
	return page
}

// waitForRemoval polls the reading list until id no longer appears among
// every Entry, or fails the test after timeout — for asserting on the
// background retention schedule without a fixed sleep racing its tick.
func waitForRemoval(t *testing.T, h *apitest.Harness, id int64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		gone := true
		for _, view := range []string{"", "archived=true"} {
			for _, entry := range listEntries(t, h, view).Entries {
				if entry.ID == id {
					gone = false
				}
			}
		}
		if gone {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("Entry %d still listed after %s, want retention to have removed it", id, timeout)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestRetentionRemovesExpiredEntriesExemptsStarredAndTombstonesTheDeltaFeed(t *testing.T) {
	h := loggedIn(t, apitest.RetentionAge(time.Hour), apitest.RetentionTick(10*time.Millisecond))

	articleURL := h.Publisher.Serve("/article", apitest.Document{
		Status: http.StatusOK, ContentType: "text/html", Body: articleHTML,
	})
	base := h.Clock.Now()
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "keep-starred", Title: "Keep Starred", Link: articleURL, Published: base},
		apitest.Item{ID: "drop-archived", Title: "Drop Archived", Published: base.Add(time.Second)},
		apitest.Item{ID: "drop-unread", Title: "Drop Unread", Published: base.Add(2 * time.Second)}))
	subscribe(t, h, feedURL)

	entries := listEntries(t, h, "").Entries
	var keepID, dropArchivedID, dropUnreadID int64
	for _, entry := range entries {
		switch entry.Title {
		case "Keep Starred":
			keepID = entry.ID
		case "Drop Archived":
			dropArchivedID = entry.ID
		case "Drop Unread":
			dropUnreadID = entry.ID
		}
	}
	if keepID == 0 || dropArchivedID == 0 || dropUnreadID == 0 {
		t.Fatalf("subscribing did not produce all three Entries: %#v", entries)
	}

	setEntryState(t, h, keepID, entryState{Starred: true}).ExpectStatus(http.StatusOK)
	setEntryState(t, h, dropArchivedID, entryState{Read: true, Archived: true}).ExpectStatus(http.StatusOK)

	// Extract the Starred Entry's Article before cleanup, so its exemption can
	// be told apart from an Article that was never fetched.
	var article struct {
		Article struct{ HTML string } `json:"article"`
	}
	getArticle(t, h, keepID).ExpectStatus(http.StatusOK).JSON(&article)
	if h.Publisher.Hits("/article") != 1 {
		t.Fatalf("publisher hits after extracting = %d, want 1", h.Publisher.Hits("/article"))
	}

	// A delta reader catching up from here should see the two removals but
	// not the Starred Entry, once retention runs.
	before := readSince(t, h, "")
	if before.NextSince == "" {
		t.Fatal("delta feed carried no cursor after the initial Entries, so a catch-up position is unreachable")
	}

	// Age every Entry's Published time past the configured retention age.
	h.Clock.Advance(2 * time.Hour)

	waitForRemoval(t, h, dropArchivedID, 2*time.Second)
	waitForRemoval(t, h, dropUnreadID, 2*time.Second)

	// The Starred Entry survives, unaffected, in the ordinary list.
	kept := listEntries(t, h, "").Entries
	found := false
	for _, entry := range kept {
		if entry.ID == keepID {
			found = true
		}
		if entry.ID == dropArchivedID || entry.ID == dropUnreadID {
			t.Errorf("removed Entry %d still appears in the reading list", entry.ID)
		}
	}
	if !found {
		t.Fatal("Starred Entry was removed by retention, want it exempt")
	}

	// Its Article is still served from storage, without a second fetch.
	getArticle(t, h, keepID).ExpectStatus(http.StatusOK).JSON(&article)
	if h.Publisher.Hits("/article") != 1 {
		t.Fatalf("publisher hits after cleanup = %d, want still 1: retention refetched or dropped a Starred Article",
			h.Publisher.Hits("/article"))
	}

	// Calling with no since parameter at all still behaves like the plain
	// list: no tombstones key, an ordinary next_cursor.
	var plain struct {
		Entries    []entryView `json:"entries"`
		NextCursor string      `json:"next_cursor"`
		Tombstones []int64     `json:"tombstones"`
	}
	h.Do(http.MethodGet, "/api/entries", nil).ExpectStatus(http.StatusOK).JSON(&plain)
	if plain.Tombstones != nil {
		t.Errorf("plain list carried tombstones %v, want none: since was not requested", plain.Tombstones)
	}

	// A delta reader resuming from before cleanup sees exactly the two
	// removals as tombstones, and no Entry changes: nothing was, only
	// removed.
	after := readSince(t, h, before.NextSince)
	if len(after.Entries) != 0 {
		t.Errorf("delta read after cleanup returned changed Entries %v, want none", after.Entries)
	}
	gotTombstones := map[int64]bool{}
	for _, id := range after.Tombstones {
		gotTombstones[id] = true
	}
	if !gotTombstones[dropArchivedID] || !gotTombstones[dropUnreadID] {
		t.Errorf("tombstones = %v, want both %d and %d", after.Tombstones, dropArchivedID, dropUnreadID)
	}
	if gotTombstones[keepID] {
		t.Errorf("tombstones = %v, want the Starred Entry %d absent", after.Tombstones, keepID)
	}
	if len(after.Tombstones) != 2 {
		t.Errorf("tombstones = %v, want exactly the two removed Entries", after.Tombstones)
	}
}

func TestASinceCursorFromAnotherServerIsRefused(t *testing.T) {
	h := loggedIn(t)
	resp := h.Do(http.MethodGet, "/api/entries?since=not-a-real-cursor", nil)
	resp.ExpectStatus(http.StatusBadRequest)
	expectErrorMentions(t, resp, "cursor")
}

func TestDeltaFeedPagesWithLimit(t *testing.T) {
	h := loggedIn(t)
	items := make([]apitest.Item, 0, 3)
	for i := range 3 {
		items = append(items, apitest.Item{
			ID: "item-" + strconv.Itoa(i), Title: "Item " + strconv.Itoa(i),
			Published: published.Add(time.Duration(i) * time.Second),
		})
	}
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "", items...))
	subscribe(t, h, feedURL)

	page := readSince(t, h, "")
	if len(page.Entries) != 3 {
		t.Fatalf("bootstrap delta read = %d Entries, want all 3", len(page.Entries))
	}

	limited := h.Do(http.MethodGet, "/api/entries?since=&limit=1", nil)
	var limitedPage sincePage
	limited.ExpectStatus(http.StatusOK).JSON(&limitedPage)
	if len(limitedPage.Entries) != 1 {
		t.Fatalf("limited delta read = %d Entries, want 1", len(limitedPage.Entries))
	}
	if limitedPage.NextSince == "" {
		t.Fatal("limited delta read carried no cursor, so the rest is unreachable")
	}
	rest := readSince(t, h, limitedPage.NextSince)
	if len(rest.Entries) != 2 {
		t.Fatalf("remaining delta read = %d Entries, want 2", len(rest.Entries))
	}
}

// TestDeltaFeedOrdersBySequenceNotByEntryIdOnATiedClock proves the fix for a
// real ordering bug: an Entry's id is assigned once at creation and never
// renumbered by a later state change, so a since cursor built from
// (timestamp, id) can silently skip an update that lands in the same second
// as the cursor but belongs to a lower-id Entry. change_seq orders by actual
// write order instead, so this cannot happen even with a clock that never
// advances between writes.
func TestDeltaFeedOrdersBySequenceNotByEntryIdOnATiedClock(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "low-id", Title: "Low Id", Published: published},
		apitest.Item{ID: "high-id", Title: "High Id", Published: published.Add(time.Second)}))
	subscribe(t, h, feedURL)

	entries := listEntries(t, h, "").Entries
	var lowID, highID int64
	for _, entry := range entries {
		switch entry.Title {
		case "Low Id":
			lowID = entry.ID
		case "High Id":
			highID = entry.ID
		}
	}
	if lowID == 0 || highID == 0 || lowID >= highID {
		t.Fatalf("want Low Id assigned a lower id than High Id, got %d and %d", lowID, highID)
	}

	// A cursor positioned exactly after the higher-id Entry's own creation,
	// with the fake clock never advancing.
	cursor := readSince(t, h, "").NextSince

	// The higher-id Entry changes first, then the lower-id one — both within
	// the same, unmoved instant of the fake clock. A (timestamp, id) cursor
	// sitting past the higher id would drop the second change entirely.
	setEntryState(t, h, highID, entryState{Starred: true}).ExpectStatus(http.StatusOK)
	setEntryState(t, h, lowID, entryState{Starred: true}).ExpectStatus(http.StatusOK)

	page := readSince(t, h, cursor)
	seen := map[int64]bool{}
	for _, entry := range page.Entries {
		seen[entry.ID] = true
	}
	if !seen[lowID] {
		t.Errorf("delta read after both changes = %v, want the lower-id Entry %d present", page.Entries, lowID)
	}
	if !seen[highID] {
		t.Errorf("delta read after both changes = %v, want the higher-id Entry %d present", page.Entries, highID)
	}
}
