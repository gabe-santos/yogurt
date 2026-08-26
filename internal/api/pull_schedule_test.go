package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gabe-santos/rss-reader/internal/apitest"
)

// feedStatusView is a Feed as the API presents it, including the fetch-state
// fields feedView (in feeds_test.go) does not need.
type feedStatusView struct {
	ID                  int64      `json:"id"`
	LastCheckedAt       *time.Time `json:"last_checked_at"`
	LastSuccessAt       *time.Time `json:"last_success_at"`
	LastError           string     `json:"last_error"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
}

// feedStatus reads one Feed's current status from the whole collection.
func feedStatus(t *testing.T, h *apitest.Harness, feedID int64) feedStatusView {
	t.Helper()
	var body struct {
		Feeds []feedStatusView `json:"feeds"`
	}
	h.Do(http.MethodGet, "/api/feeds", nil).ExpectStatus(http.StatusOK).JSON(&body)
	for _, feed := range body.Feeds {
		if feed.ID == feedID {
			return feed
		}
	}
	t.Fatalf("no Feed with id %d in the collection", feedID)
	return feedStatusView{}
}

func TestANewlySubscribedFeedHasNoErrorAndAFreshSuccess(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "First post", Published: published},
	))
	feed := subscribe(t, h, feedURL)

	status := feedStatus(t, h, feed.ID)
	if status.LastError != "" {
		t.Errorf("last_error = %q, want empty after a successful subscribe", status.LastError)
	}
	if status.ConsecutiveFailures != 0 {
		t.Errorf("consecutive_failures = %d, want 0 after a successful subscribe", status.ConsecutiveFailures)
	}
	if status.LastCheckedAt == nil {
		t.Fatal("last_checked_at is nil, want the moment the Feed was first read")
	}
	if status.LastSuccessAt == nil {
		t.Fatal("last_success_at is nil, want the moment the Feed was first read")
	}
	if !status.LastCheckedAt.Equal(*status.LastSuccessAt) {
		t.Errorf("last_checked_at %s != last_success_at %s after a subscribe with no prior failures",
			status.LastCheckedAt, status.LastSuccessAt)
	}
}

func TestConditionalRequestsAvoidReparsingAnUnchangedFeed(t *testing.T) {
	h := loggedIn(t)
	doc := apitest.RSS("The Publisher", "", apitest.Item{ID: "one", Title: "First post", Published: published})
	doc.Headers = map[string]string{"ETag": `"v1"`}
	feedURL := h.Publisher.Serve("/feed.xml", doc)
	feed := subscribe(t, h, feedURL)

	// The publisher now says nothing has changed, and stops repeating the
	// validator, as real publishers often do on a 304.
	h.Publisher.Serve("/feed.xml", apitest.Document{Status: http.StatusNotModified})

	h.Do(http.MethodPost, fmt.Sprintf("/api/feeds/%d/refresh", feed.ID), nil).
		ExpectStatus(http.StatusNoContent)

	if got := h.Publisher.Hits("/feed.xml"); got != 2 {
		t.Errorf("publisher hits = %d, want the initial fetch plus the conditional refresh", got)
	}
	if got := h.Publisher.LastRequest("/feed.xml").Get("If-None-Match"); got != `"v1"` {
		t.Errorf("If-None-Match = %q, want the stored validator %q", got, `"v1"`)
	}

	page := listEntries(t, h, feedQuery(feed))
	if len(page.Entries) != 1 {
		t.Fatalf("entries = %d, want the one Entry from before the unchanged response", len(page.Entries))
	}

	status := feedStatus(t, h, feed.ID)
	if status.LastError != "" {
		t.Errorf("last_error = %q, want empty: an unchanged response is a successful check", status.LastError)
	}
	if status.ConsecutiveFailures != 0 {
		t.Errorf("consecutive_failures = %d, want 0 after an unchanged response", status.ConsecutiveFailures)
	}
	if status.LastSuccessAt == nil {
		t.Fatal("last_success_at is nil, want it advanced by the unchanged response")
	}
}

func TestAFailingFeedBacksOffProgressivelyAndReportsWhy(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "First post", Published: published},
	))
	feed := subscribe(t, h, feedURL)
	successAt := feedStatus(t, h, feed.ID).LastSuccessAt

	h.Publisher.Serve("/feed.xml", apitest.Document{Status: http.StatusInternalServerError, Body: "boom"})

	var failures []int
	for range 3 {
		h.Do(http.MethodPost, fmt.Sprintf("/api/feeds/%d/refresh", feed.ID), nil).
			ExpectStatus(http.StatusBadGateway)

		status := feedStatus(t, h, feed.ID)
		failures = append(failures, status.ConsecutiveFailures)
		if status.LastError == "" {
			t.Fatal("last_error is empty after a failed check")
		}
		if status.LastCheckedAt == nil {
			t.Fatal("last_checked_at is nil after a failed check")
		}
	}

	if want := []int{1, 2, 3}; !equalInts(failures, want) {
		t.Errorf("consecutive_failures across repeated failures = %v, want %v", failures, want)
	}

	// A run of failures does not erase the record of the last success.
	status := feedStatus(t, h, feed.ID)
	if status.LastSuccessAt == nil || !status.LastSuccessAt.Equal(*successAt) {
		t.Errorf("last_success_at = %v, want it unchanged at %v across failures", status.LastSuccessAt, successAt)
	}
}

func TestTheScheduleChecksEveryDueFeedWithoutManualAction(t *testing.T) {
	h := loggedIn(t, apitest.PollInterval(2*time.Second), apitest.PollTick(20*time.Millisecond))

	firstURL := h.Publisher.Serve("/first.xml", apitest.RSS("First", "",
		apitest.Item{ID: "one", Title: "One", Published: published}))
	secondURL := h.Publisher.Serve("/second.xml", apitest.RSS("Second", "",
		apitest.Item{ID: "one", Title: "One", Published: published}))

	subscribe(t, h, firstURL)
	subscribe(t, h, secondURL)

	// The schedule is already ticking, but the fake clock has not moved past
	// either Feed's next check time yet.
	time.Sleep(50 * time.Millisecond)
	if got := h.Publisher.Hits("/first.xml"); got != 1 {
		t.Fatalf("first Feed hits = %d, want only its initial subscribe fetch before it is due", got)
	}
	if got := h.Publisher.Hits("/second.xml"); got != 1 {
		t.Fatalf("second Feed hits = %d, want only its initial subscribe fetch before it is due", got)
	}

	// Cross every Feed's next check time without the reader doing anything.
	h.Clock.Advance(time.Hour)

	waitForHits(t, h.Publisher, "/first.xml", 2, 2*time.Second)
	waitForHits(t, h.Publisher, "/second.xml", 2, 2*time.Second)
}

// waitForHits polls the fake publisher until a path has received at least
// want hits, for asserting on a background goroutine's effect without a fixed
// sleep racing the schedule's own tick.
func waitForHits(t *testing.T, p *apitest.Publisher, path string, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if p.Hits(path) >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s hits = %d after %s, want at least %d", path, p.Hits(path), timeout, want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func equalInts(got, want []int) bool {
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
