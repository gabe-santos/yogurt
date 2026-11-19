package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gabe-santos/yogurt/internal/aiclient"
	"github.com/gabe-santos/yogurt/internal/apitest"
	"github.com/gabe-santos/yogurt/internal/store"
)

type gistView struct {
	Text      string    `json:"text"`
	Source    string    `json:"source"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
}

// chatCompletion renders a minimal OpenAI-compatible chat-completions reply
// carrying content as its one choice.
func chatCompletion(content string) string {
	body, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{
			{"message": map[string]any{"content": content}},
		},
	})
	return string(body)
}

// chatCompletionsPath is where every test's fake model lives on the fake
// publisher: any path works, since the Gist model client takes whatever
// endpoint the reader configured.
const chatCompletionsPath = "/v1/chat/completions"

// configureAI points the Gist model at the fake publisher's chat-completions
// route and returns that route's path, for assertions on what was sent to
// it.
func configureAI(t *testing.T, h *apitest.Harness, key string) string {
	t.Helper()
	body := map[string]any{"url": h.Publisher.URL(chatCompletionsPath), "model": "test-model"}
	if key != "" {
		body["key"] = key
	}
	h.Do(http.MethodPut, "/api/ai", body).ExpectStatus(http.StatusOK)
	return chatCompletionsPath
}

func getGist(t *testing.T, h *apitest.Harness, entryID int64) *apitest.Response {
	t.Helper()
	return h.Do(http.MethodGet, "/api/entries/"+strconv.FormatInt(entryID, 10)+"/gist", nil)
}

func requestGist(t *testing.T, h *apitest.Harness, entryID int64) *apitest.Response {
	t.Helper()
	return h.Do(http.MethodPost, "/api/entries/"+strconv.FormatInt(entryID, 10)+"/gist", nil)
}

func TestGistIsRefusedUntilGistsAreConfigured(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Content: "<p>Some feed body text.</p>", Published: published}))
	subscribe(t, h, feedURL)
	entry := listEntries(t, h, "").Entries[0]

	getGist(t, h, entry.ID).ExpectStatus(http.StatusNotFound)
	requestGist(t, h, entry.ID).ExpectStatus(http.StatusBadRequest)
}

func TestGistCondensesTheArticleAndSendsTheKeyAsABearerCredential(t *testing.T) {
	h := loggedIn(t)
	articleURL := h.Publisher.Serve("/article", apitest.Document{
		Status: http.StatusOK, ContentType: "text/html", Body: articleHTML,
	})
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Link: articleURL, Published: published}))
	subscribe(t, h, feedURL)
	entry := listEntries(t, h, "").Entries[0]

	chatPath := configureAI(t, h, "sk-test-key")
	h.Publisher.Serve(chatPath, apitest.Document{
		Status: http.StatusOK, ContentType: "application/json",
		Body: chatCompletion("- bullet one\n- bullet two\n- bullet three"),
	})

	var body struct {
		Gist gistView `json:"gist"`
	}
	requestGist(t, h, entry.ID).ExpectStatus(http.StatusOK).JSON(&body)
	if body.Gist.Source != "article" {
		t.Errorf("source = %q, want article", body.Gist.Source)
	}
	if body.Gist.Model != "test-model" {
		t.Errorf("model = %q, want test-model", body.Gist.Model)
	}
	if !strings.Contains(body.Gist.Text, "bullet one") {
		t.Errorf("gist text = %q, want the model's reply", body.Gist.Text)
	}

	if got := h.Publisher.LastRequest(chatPath).Get("Authorization"); got != "Bearer sk-test-key" {
		t.Errorf("Authorization header = %q, want the configured key as a bearer credential", got)
	}
	sent := h.Publisher.LastRequestBody(chatPath)
	if !strings.Contains(sent, "first paragraph of a genuinely interesting article") {
		t.Errorf("request body = %q, want the extracted Article's own text", sent)
	}
}

func TestGistFallsBackToTheEntrysOwnBodyWhenTheArticleCannotBeExtracted(t *testing.T) {
	h := loggedIn(t)
	articleURL := h.Publisher.Serve("/blank", apitest.Document{
		ContentType: "text/html; charset=utf-8", Body: blankHTML,
	})
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{
			ID: "one", Title: "One", Link: articleURL,
			Content: "<p>The Feed's own body text, distinct from the Article.</p>", Published: published,
		}))
	subscribe(t, h, feedURL)
	entry := listEntries(t, h, "").Entries[0]

	chatPath := configureAI(t, h, "")
	h.Publisher.Serve(chatPath, apitest.Document{
		Status: http.StatusOK, ContentType: "application/json", Body: chatCompletion("- a bullet"),
	})

	var body struct {
		Gist gistView `json:"gist"`
	}
	requestGist(t, h, entry.ID).ExpectStatus(http.StatusOK).JSON(&body)
	if body.Gist.Source != "feed" {
		t.Errorf("source = %q, want feed", body.Gist.Source)
	}
	if got := h.Publisher.LastRequest(chatPath).Get("Authorization"); got != "" {
		t.Errorf("Authorization header = %q, want none without a configured key", got)
	}
	sent := h.Publisher.LastRequestBody(chatPath)
	if !strings.Contains(sent, "Feed's own body text") {
		t.Errorf("request body = %q, want the Entry's own body text", sent)
	}
}

func TestGistIsStoredAndReusedWithoutANewModelCallThenRegenerateReplacesIt(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Content: "<p>Body text to condense.</p>", Published: published}))
	subscribe(t, h, feedURL)
	entry := listEntries(t, h, "").Entries[0]

	chatPath := configureAI(t, h, "")
	h.Publisher.Serve(chatPath, apitest.Document{Status: http.StatusOK, ContentType: "application/json", Body: chatCompletion("- first")})

	var first struct {
		Gist gistView `json:"gist"`
	}
	requestGist(t, h, entry.ID).ExpectStatus(http.StatusOK).JSON(&first)
	if got := h.Publisher.Hits(chatPath); got != 1 {
		t.Fatalf("model hits after first request = %d, want 1", got)
	}

	var again struct {
		Gist gistView `json:"gist"`
	}
	getGist(t, h, entry.ID).ExpectStatus(http.StatusOK).JSON(&again)
	if again.Gist.Text != first.Gist.Text {
		t.Errorf("reused gist text = %q, want the stored %q", again.Gist.Text, first.Gist.Text)
	}
	if got := h.Publisher.Hits(chatPath); got != 1 {
		t.Errorf("model hits after GET = %d, want still 1 (no new model call)", got)
	}

	h.Publisher.Serve(chatPath, apitest.Document{Status: http.StatusOK, ContentType: "application/json", Body: chatCompletion("- regenerated")})
	var regenerated struct {
		Gist gistView `json:"gist"`
	}
	requestGist(t, h, entry.ID).ExpectStatus(http.StatusOK).JSON(&regenerated)
	if !strings.Contains(regenerated.Gist.Text, "regenerated") {
		t.Errorf("regenerated gist text = %q, want the new reply", regenerated.Gist.Text)
	}
	if got := h.Publisher.Hits(chatPath); got != 2 {
		t.Errorf("model hits after regenerate = %d, want 2", got)
	}
}

func TestGistFailuresStoreNothing(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Content: "<p>Body text.</p>", Published: published},
		apitest.Item{ID: "two", Title: "Two", Content: "<p>More body text.</p>", Published: published.Add(time.Second)}))
	subscribe(t, h, feedURL)
	entries := listEntries(t, h, "").Entries

	chatPath := configureAI(t, h, "")

	// A provider failure (non-2xx).
	h.Publisher.Serve(chatPath, apitest.Document{Status: http.StatusInternalServerError, Body: "oops"})
	requestGist(t, h, entries[0].ID).ExpectStatus(http.StatusBadGateway)
	getGist(t, h, entries[0].ID).ExpectStatus(http.StatusNotFound)

	// An empty reply.
	h.Publisher.Serve(chatPath, apitest.Document{Status: http.StatusOK, ContentType: "application/json", Body: chatCompletion("")})
	requestGist(t, h, entries[1].ID).ExpectStatus(http.StatusBadGateway)
	getGist(t, h, entries[1].ID).ExpectStatus(http.StatusNotFound)
}

func TestASlowModelTimesOutRatherThanHangingAndStoresNothing(t *testing.T) {
	h := loggedIn(t, apitest.AIClient(aiclient.New(50*time.Millisecond)))
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Content: "<p>Body text.</p>", Published: published}))
	subscribe(t, h, feedURL)
	entry := listEntries(t, h, "").Entries[0]

	chatPath := configureAI(t, h, "")
	h.Publisher.Serve(chatPath, apitest.Document{
		Status: http.StatusOK, ContentType: "application/json",
		Body:  chatCompletion("- too late"),
		Delay: 300 * time.Millisecond,
	})

	requestGist(t, h, entry.ID).ExpectStatus(http.StatusBadGateway)
	getGist(t, h, entry.ID).ExpectStatus(http.StatusNotFound)
}

func TestTurningGistsOffHidesStoredGistsRatherThanDeletingThem(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Content: "<p>Body text.</p>", Published: published}))
	subscribe(t, h, feedURL)
	entry := listEntries(t, h, "").Entries[0]

	chatPath := configureAI(t, h, "")
	h.Publisher.Serve(chatPath, apitest.Document{Status: http.StatusOK, ContentType: "application/json", Body: chatCompletion("- kept")})
	requestGist(t, h, entry.ID).ExpectStatus(http.StatusOK)

	h.Do(http.MethodDelete, "/api/ai", nil).ExpectStatus(http.StatusNoContent)
	getGist(t, h, entry.ID).ExpectStatus(http.StatusNotFound)
	requestGist(t, h, entry.ID).ExpectStatus(http.StatusBadRequest)

	configureAI(t, h, "")
	var body struct {
		Gist gistView `json:"gist"`
	}
	getGist(t, h, entry.ID).ExpectStatus(http.StatusOK).JSON(&body)
	if !strings.Contains(body.Gist.Text, "kept") {
		t.Errorf("gist after re-enabling = %q, want the previously stored reply", body.Gist.Text)
	}
	if got := h.Publisher.Hits(chatPath); got != 1 {
		t.Errorf("model hits after re-enabling = %d, want still 1 (GET made no call)", got)
	}
}

func TestGistRequestsLeaveReadStarredAndArchivedUnchanged(t *testing.T) {
	h := loggedIn(t)
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Content: "<p>Body text.</p>", Published: published}))
	subscribe(t, h, feedURL)
	entry := listEntries(t, h, "").Entries[0]
	if entry.Read || entry.Starred || entry.Archived {
		t.Fatalf("entry state before any Gist request = %+v, want all false", entry)
	}

	chatPath := configureAI(t, h, "")
	h.Publisher.Serve(chatPath, apitest.Document{Status: http.StatusOK, ContentType: "application/json", Body: chatCompletion("- bullet")})

	requestGist(t, h, entry.ID).ExpectStatus(http.StatusOK)
	getGist(t, h, entry.ID).ExpectStatus(http.StatusOK)

	after := listEntries(t, h, "").Entries[0]
	if after.Read || after.Starred || after.Archived {
		t.Errorf("entry state after Gist requests = %+v, want unchanged", after)
	}
}

func TestRetentionDeletingAnEntryTakesItsGistWithIt(t *testing.T) {
	h := loggedIn(t, apitest.RetentionAge(time.Hour), apitest.RetentionTick(10*time.Millisecond))
	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Content: "<p>Body text.</p>", Published: h.Clock.Now()}))
	subscribe(t, h, feedURL)
	entry := listEntries(t, h, "").Entries[0]

	chatPath := configureAI(t, h, "")
	h.Publisher.Serve(chatPath, apitest.Document{Status: http.StatusOK, ContentType: "application/json", Body: chatCompletion("- bullet")})
	requestGist(t, h, entry.ID).ExpectStatus(http.StatusOK)

	if _, err := h.Store.Gist(context.Background(), entry.ID); err != nil {
		t.Fatalf("gist missing before retention: %v", err)
	}

	h.Clock.Advance(2 * time.Hour)
	waitForRemoval(t, h, entry.ID, 2*time.Second)

	if _, err := h.Store.Gist(context.Background(), entry.ID); !errors.Is(err, store.ErrNoGist) {
		t.Errorf("gist after retention removed its Entry: err = %v, want store.ErrNoGist", err)
	}
}

func TestGistModelReachesLoopbackWhileFeedFetchingStillRefusesPrivateAddresses(t *testing.T) {
	h := loggedIn(t, apitest.BlockPrivateFetch())

	feedURL := h.Publisher.Serve("/feed.xml", apitest.RSS("The Publisher", "",
		apitest.Item{ID: "one", Title: "One", Published: published}))
	h.Do(http.MethodPost, "/api/feeds", map[string]string{"url": feedURL}).
		ExpectStatus(http.StatusBadGateway)

	// An Entry seeded directly, bypassing the blocked Feed fetch, so there is
	// something for the Gist model to condense.
	feed, err := h.Store.CreateFeed(context.Background(), store.Feed{URL: feedURL, Title: "The Publisher"}, h.Clock.Now())
	if err != nil {
		t.Fatalf("create feed directly: %v", err)
	}
	if err := h.Store.SaveEntries(context.Background(), feed.ID, []store.Entry{
		{GUID: "one", Title: "One", URL: h.Publisher.URL("/article"), Content: "<p>Body text.</p>", PublishedAt: published},
	}, h.Clock.Now()); err != nil {
		t.Fatalf("seed entry directly: %v", err)
	}
	entry := listEntries(t, h, "").Entries[0]

	chatPath := configureAI(t, h, "")
	h.Publisher.Serve(chatPath, apitest.Document{Status: http.StatusOK, ContentType: "application/json", Body: chatCompletion("- bullet")})

	requestGist(t, h, entry.ID).ExpectStatus(http.StatusOK)
}
