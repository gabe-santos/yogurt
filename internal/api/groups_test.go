package api_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gabe-santos/rss-reader/internal/apitest"
)

type groupView struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	IsDefault   bool   `json:"is_default"`
	UnreadCount int    `json:"unread_count"`
}

// listGroups reads the reader's whole set of Groups.
func listGroups(t *testing.T, h *apitest.Harness) []groupView {
	t.Helper()
	var body struct {
		Groups []groupView `json:"groups"`
	}
	h.Do(http.MethodGet, "/api/groups", nil).ExpectStatus(http.StatusOK).JSON(&body)
	return body.Groups
}

// createGroup adds a new Group and returns it, failing the test if refused.
func createGroup(t *testing.T, h *apitest.Harness, name string) groupView {
	t.Helper()
	var body struct {
		Group groupView `json:"group"`
	}
	h.Do(http.MethodPost, "/api/groups", map[string]string{"name": name}).
		ExpectStatus(http.StatusCreated).
		JSON(&body)
	return body.Group
}

func TestANewReaderHasOneDefaultGroupAndEveryFeedStartsThere(t *testing.T) {
	h := loggedIn(t)

	groups := listGroups(t, h)
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want exactly the default Group", len(groups))
	}
	if !groups[0].IsDefault {
		t.Errorf("the only Group is not marked default")
	}

	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "First post", Published: published}))
	feed := subscribe(t, h, feedURL)
	if feed.GroupID != groups[0].ID {
		t.Errorf("new Feed group = %d, want the default Group %d", feed.GroupID, groups[0].ID)
	}
}

func TestGroupsCanBeCreatedRenamedAndDeleted(t *testing.T) {
	h := loggedIn(t)

	group := createGroup(t, h, "Tech")
	if group.Name != "Tech" {
		t.Fatalf("created group name = %q, want Tech", group.Name)
	}
	if group.IsDefault {
		t.Errorf("a newly created Group must not be the default")
	}

	var renamed struct {
		Group groupView `json:"group"`
	}
	h.Do(http.MethodPut, "/api/groups/"+strconv.FormatInt(group.ID, 10), map[string]string{"name": "Technology"}).
		ExpectStatus(http.StatusOK).
		JSON(&renamed)
	if renamed.Group.Name != "Technology" {
		t.Errorf("renamed group name = %q, want Technology", renamed.Group.Name)
	}

	h.Do(http.MethodDelete, "/api/groups/"+strconv.FormatInt(group.ID, 10), nil).
		ExpectStatus(http.StatusNoContent)

	groups := listGroups(t, h)
	for _, g := range groups {
		if g.ID == group.ID {
			t.Fatalf("deleted Group %d is still listed", group.ID)
		}
	}
}

func TestTheDefaultGroupCannotBeDeleted(t *testing.T) {
	h := loggedIn(t)
	groups := listGroups(t, h)

	resp := h.Do(http.MethodDelete, "/api/groups/"+strconv.FormatInt(groups[0].ID, 10), nil).
		ExpectStatus(http.StatusConflict)
	expectErrorMentions(t, resp, "default")

	if got := listGroups(t, h); len(got) != 1 {
		t.Fatalf("groups after refused delete = %d, want the default Group still there", len(got))
	}
}

func TestDeletingAGroupReparentsItsFeedsToTheDefault(t *testing.T) {
	h := loggedIn(t)
	defaultGroup := listGroups(t, h)[0]
	group := createGroup(t, h, "Tech")

	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "First post", Published: published}))
	feed := subscribe(t, h, feedURL)

	h.Do(http.MethodPut, "/api/feeds/"+strconv.FormatInt(feed.ID, 10),
		map[string]any{"group_id": group.ID}).
		ExpectStatus(http.StatusOK)

	h.Do(http.MethodDelete, "/api/groups/"+strconv.FormatInt(group.ID, 10), nil).
		ExpectStatus(http.StatusNoContent)

	var body struct {
		Feeds []feedView `json:"feeds"`
	}
	h.Do(http.MethodGet, "/api/feeds", nil).ExpectStatus(http.StatusOK).JSON(&body)
	if body.Feeds[0].GroupID != defaultGroup.ID {
		t.Errorf("feed group after deleting its Group = %d, want the default Group %d",
			body.Feeds[0].GroupID, defaultGroup.ID)
	}
}

func TestAFeedCanBeRenamedMovedAndSuspendedIndependently(t *testing.T) {
	h := loggedIn(t)
	group := createGroup(t, h, "Tech")
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "First post", Published: published}))
	feed := subscribe(t, h, feedURL)

	var renamed struct {
		Feed feedView `json:"feed"`
	}
	h.Do(http.MethodPut, "/api/feeds/"+strconv.FormatInt(feed.ID, 10),
		map[string]any{"title": "My Feed"}).
		ExpectStatus(http.StatusOK).
		JSON(&renamed)
	if renamed.Feed.Title != "My Feed" {
		t.Errorf("renamed feed title = %q, want My Feed", renamed.Feed.Title)
	}
	if renamed.Feed.Suspended {
		t.Errorf("renaming must not disturb suspension")
	}
	defaultGroupID := listGroups(t, h)[0].ID
	if renamed.Feed.GroupID != defaultGroupID {
		t.Errorf("renaming must not disturb the Group")
	}

	var moved struct {
		Feed feedView `json:"feed"`
	}
	h.Do(http.MethodPut, "/api/feeds/"+strconv.FormatInt(feed.ID, 10),
		map[string]any{"group_id": group.ID}).
		ExpectStatus(http.StatusOK).
		JSON(&moved)
	if moved.Feed.GroupID != group.ID {
		t.Errorf("moved feed group = %d, want %d", moved.Feed.GroupID, group.ID)
	}
	if moved.Feed.Title != "My Feed" {
		t.Errorf("moving to a Group must not disturb the title")
	}

	var suspended struct {
		Feed feedView `json:"feed"`
	}
	h.Do(http.MethodPut, "/api/feeds/"+strconv.FormatInt(feed.ID, 10),
		map[string]any{"suspended": true}).
		ExpectStatus(http.StatusOK).
		JSON(&suspended)
	if !suspended.Feed.Suspended {
		t.Errorf("feed was not suspended")
	}
	if suspended.Feed.GroupID != group.ID {
		t.Errorf("suspending must not disturb the Group")
	}
}

func TestMovingAFeedToAMissingGroupIsRefused(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "First post", Published: published}))
	feed := subscribe(t, h, feedURL)

	resp := h.Do(http.MethodPut, "/api/feeds/"+strconv.FormatInt(feed.ID, 10),
		map[string]any{"group_id": 99999}).
		ExpectStatus(http.StatusUnprocessableEntity)
	expectErrorMentions(t, resp, "Group")
}

func TestDeletingAFeedRemovesItsEntriesButSuspendingDoesNot(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "First post", Published: published}))
	feed := subscribe(t, h, feedURL)

	h.Do(http.MethodPut, "/api/feeds/"+strconv.FormatInt(feed.ID, 10),
		map[string]any{"suspended": true}).
		ExpectStatus(http.StatusOK)
	page := listEntries(t, h, feedQuery(feed))
	if len(page.Entries) != 1 {
		t.Fatalf("entries after suspending = %d, want the Entry still there", len(page.Entries))
	}

	h.Do(http.MethodDelete, "/api/feeds/"+strconv.FormatInt(feed.ID, 10), nil).
		ExpectStatus(http.StatusNoContent)

	page = listEntries(t, h, feedQuery(feed))
	if len(page.Entries) != 0 {
		t.Fatalf("entries after deleting the Feed = %d, want none", len(page.Entries))
	}

	h.Do(http.MethodDelete, "/api/feeds/9999", nil).ExpectStatus(http.StatusNotFound)
}

func TestManualRefreshDoesNotSkipSuspendedFeeds(t *testing.T) {
	h := loggedIn(t)
	first := apitest.Item{ID: "one", Title: "First post", Published: published}
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "", first))
	feed := subscribe(t, h, feedURL)

	h.Do(http.MethodPut, "/api/feeds/"+strconv.FormatInt(feed.ID, 10),
		map[string]any{"suspended": true}).
		ExpectStatus(http.StatusOK)

	later := apitest.Item{ID: "two", Title: "Later post", Published: published}
	h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "", first, later))

	h.Do(http.MethodPost, "/api/feeds/refresh", nil).ExpectStatus(http.StatusOK)

	if got := h.Publisher.Hits("/feed.xml"); got != 2 {
		t.Errorf("suspended feed hits = %d, want manual refresh to bypass suspension too", got)
	}
}

func TestUpdatingAFeedWithAMissingGroupChangesNothing(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "First post", Published: published}))
	feed := subscribe(t, h, feedURL)

	h.Do(http.MethodPut, "/api/feeds/"+strconv.FormatInt(feed.ID, 10),
		map[string]any{"title": "Renamed", "group_id": 99999}).
		ExpectStatus(http.StatusUnprocessableEntity)

	var body struct {
		Feeds []feedView `json:"feeds"`
	}
	h.Do(http.MethodGet, "/api/feeds", nil).ExpectStatus(http.StatusOK).JSON(&body)
	if body.Feeds[0].Title != "The Publisher" {
		t.Errorf("title = %q, want the refused update to have changed nothing", body.Feeds[0].Title)
	}
}

func TestTheReadingListCanBeScopedToAGroup(t *testing.T) {
	h := loggedIn(t)
	defaultGroup := listGroups(t, h)[0]
	group := createGroup(t, h, "Tech")

	inGroupURL := h.Publisher.Serve("/tech.xml", apitest.RSS("Tech Site", "",
		apitest.Item{ID: "one", Title: "Tech post", Published: published}))
	inGroup := subscribe(t, h, inGroupURL)
	h.Do(http.MethodPut, "/api/feeds/"+strconv.FormatInt(inGroup.ID, 10),
		map[string]any{"group_id": group.ID}).
		ExpectStatus(http.StatusOK)

	otherURL := h.Publisher.Serve("/other.xml", apitest.RSS("Other Site", "",
		apitest.Item{ID: "two", Title: "Other post", Published: published}))
	subscribe(t, h, otherURL)

	page := listEntries(t, h, "group="+strconv.FormatInt(group.ID, 10))
	if want := []string{"Tech post"}; !equalStrings(entryTitles(page.Entries), want) {
		t.Errorf("group-scoped entries = %v, want %v", entryTitles(page.Entries), want)
	}

	defaultPage := listEntries(t, h, "group="+strconv.FormatInt(defaultGroup.ID, 10))
	if want := []string{"Other post"}; !equalStrings(entryTitles(defaultPage.Entries), want) {
		t.Errorf("default group entries = %v, want %v", entryTitles(defaultPage.Entries), want)
	}
}

func TestUnreadCountsPerFeedAndGroupStayCorrectAsEntriesAreRead(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "First post", Published: published},
		apitest.Item{ID: "two", Title: "Second post", Published: published.Add(time.Hour)},
	))
	feed := subscribe(t, h, feedURL)

	feeds := listFeeds(t, h)
	if feeds[0].UnreadCount != 2 {
		t.Fatalf("unread count before reading = %d, want 2", feeds[0].UnreadCount)
	}
	groups := listGroups(t, h)
	if groups[0].UnreadCount != 2 {
		t.Fatalf("group unread count before reading = %d, want 2", groups[0].UnreadCount)
	}

	page := listEntries(t, h, feedQuery(feed))
	setEntryState(t, h, page.Entries[0].ID, true)

	feeds = listFeeds(t, h)
	if feeds[0].UnreadCount != 1 {
		t.Errorf("unread count after reading one = %d, want 1", feeds[0].UnreadCount)
	}
	groups = listGroups(t, h)
	if groups[0].UnreadCount != 1 {
		t.Errorf("group unread count after reading one = %d, want 1", groups[0].UnreadCount)
	}
}
