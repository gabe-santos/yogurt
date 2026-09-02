package api_test

import (
	"net/http"
	"testing"
)

func TestTheMarkOnOpenSettingPersists(t *testing.T) {
	h := loggedIn(t)

	var body struct {
		Settings struct {
			MarkOnOpen bool   `json:"mark_on_open"`
			EntryView  string `json:"entry_view"`
		} `json:"settings"`
	}
	h.Do(http.MethodGet, "/api/settings", nil).ExpectStatus(http.StatusOK).JSON(&body)
	if !body.Settings.MarkOnOpen {
		t.Fatal("mark-on-open is not on by default")
	}

	h.Do(http.MethodPut, "/api/settings",
		map[string]any{"mark_on_open": false, "entry_view": "feed", "unread_only": false, "reading_font": "sans"}).
		ExpectStatus(http.StatusOK).JSON(&body)
	if body.Settings.MarkOnOpen {
		t.Fatal("turning mark-on-open off was not reflected in the response")
	}

	h.Do(http.MethodGet, "/api/settings", nil).ExpectStatus(http.StatusOK).JSON(&body)
	if body.Settings.MarkOnOpen {
		t.Fatal("mark-on-open did not persist as off")
	}
}

// The reader's choice between the Feed's own text, Reader View, and Original
// View is a preference, not per-Entry state: it outlives the Entry it was made
// on, and the application it was made in.
func TestTheEntryViewPreferencePersists(t *testing.T) {
	h := loggedIn(t)

	var body struct {
		Settings struct {
			MarkOnOpen bool   `json:"mark_on_open"`
			EntryView  string `json:"entry_view"`
		} `json:"settings"`
	}
	h.Do(http.MethodGet, "/api/settings", nil).ExpectStatus(http.StatusOK).JSON(&body)
	if body.Settings.EntryView != "feed" {
		t.Fatalf("entry view default = %q, want an Entry to open showing what the Feed supplied", body.Settings.EntryView)
	}

	h.Do(http.MethodPut, "/api/settings",
		map[string]any{"mark_on_open": true, "entry_view": "original", "unread_only": false, "reading_font": "sans"}).
		ExpectStatus(http.StatusOK).JSON(&body)
	if body.Settings.EntryView != "original" {
		t.Fatalf("entry view = %q, want the chosen original", body.Settings.EntryView)
	}

	h.Do(http.MethodGet, "/api/settings", nil).ExpectStatus(http.StatusOK).JSON(&body)
	if body.Settings.EntryView != "original" {
		t.Fatalf("entry view after re-reading = %q, want the stored original", body.Settings.EntryView)
	}
	if !body.Settings.MarkOnOpen {
		t.Error("declaring the entry view clobbered mark-on-open")
	}
}

// Unread Only narrows whichever Collection the reader chose rather than being a
// Collection of its own, so it is a preference that outlives the reload: a
// reader who reads unread-first should not have to say so every morning.
func TestTheUnreadOnlyPreferencePersists(t *testing.T) {
	h := loggedIn(t)

	var body struct {
		Settings struct {
			MarkOnOpen bool   `json:"mark_on_open"`
			EntryView  string `json:"entry_view"`
			UnreadOnly bool   `json:"unread_only"`
		} `json:"settings"`
	}
	h.Do(http.MethodGet, "/api/settings", nil).ExpectStatus(http.StatusOK).JSON(&body)
	if body.Settings.UnreadOnly {
		t.Fatal("unread-only is on by default, want a Collection to open showing everything it holds")
	}

	h.Do(http.MethodPut, "/api/settings",
		map[string]any{"mark_on_open": true, "entry_view": "feed", "unread_only": true, "reading_font": "sans"}).
		ExpectStatus(http.StatusOK).JSON(&body)
	if !body.Settings.UnreadOnly {
		t.Fatal("turning unread-only on was not reflected in the response")
	}

	h.Do(http.MethodGet, "/api/settings", nil).ExpectStatus(http.StatusOK).JSON(&body)
	if !body.Settings.UnreadOnly {
		t.Fatal("unread-only did not persist as on")
	}

	if !body.Settings.MarkOnOpen || body.Settings.EntryView != "feed" {
		t.Error("declaring unread-only clobbered the other preferences")
	}

	// Preferences are declared whole: a field the request leaves out is not
	// preserved, it is declared false. The client's typed Settings makes sending
	// a partial body impossible, and this is what the server does if one gets
	// through anyway.
	h.Do(http.MethodPut, "/api/settings",
		map[string]any{"mark_on_open": true, "entry_view": "feed", "reading_font": "sans"}).
		ExpectStatus(http.StatusOK).JSON(&body)
	if body.Settings.UnreadOnly {
		t.Fatal("an omitted unread_only was preserved, want a whole declaration")
	}
}

func TestAnUnknownEntryViewIsRefused(t *testing.T) {
	h := loggedIn(t)

	resp := h.Do(http.MethodPut, "/api/settings",
		map[string]any{"mark_on_open": true, "entry_view": "headless-chromium", "reading_font": "sans"}).
		ExpectStatus(http.StatusBadRequest)
	var body struct {
		Error string `json:"error"`
	}
	resp.JSON(&body)
	if body.Error == "" {
		t.Fatal("refusing an unknown entry view said nothing about what is accepted")
	}
}

// The typeface an Entry is read in belongs to the reader, not to the Entry or
// the session: someone who reads in the serif means it tomorrow morning too.
func TestTheReadingFontPreferencePersists(t *testing.T) {
	h := loggedIn(t)

	var body struct {
		Settings struct {
			MarkOnOpen  bool   `json:"mark_on_open"`
			EntryView   string `json:"entry_view"`
			ReadingFont string `json:"reading_font"`
		} `json:"settings"`
	}
	h.Do(http.MethodGet, "/api/settings", nil).ExpectStatus(http.StatusOK).JSON(&body)
	if body.Settings.ReadingFont != "sans" {
		t.Fatalf("reading font default = %q, want the interface's own face until the reader asks otherwise", body.Settings.ReadingFont)
	}

	h.Do(http.MethodPut, "/api/settings",
		map[string]any{"mark_on_open": true, "entry_view": "feed", "unread_only": false, "reading_font": "serif"}).
		ExpectStatus(http.StatusOK).JSON(&body)
	if body.Settings.ReadingFont != "serif" {
		t.Fatalf("reading font = %q, want the chosen serif", body.Settings.ReadingFont)
	}

	h.Do(http.MethodGet, "/api/settings", nil).ExpectStatus(http.StatusOK).JSON(&body)
	if body.Settings.ReadingFont != "serif" {
		t.Fatalf("reading font after re-reading = %q, want the stored serif", body.Settings.ReadingFont)
	}
	if !body.Settings.MarkOnOpen || body.Settings.EntryView != "feed" {
		t.Error("declaring the reading font clobbered the other preferences")
	}
}

func TestAnUnknownReadingFontIsRefused(t *testing.T) {
	h := loggedIn(t)

	resp := h.Do(http.MethodPut, "/api/settings",
		map[string]any{"mark_on_open": true, "entry_view": "feed", "reading_font": "comic-sans"}).
		ExpectStatus(http.StatusBadRequest)
	var body struct {
		Error string `json:"error"`
	}
	resp.JSON(&body)
	if body.Error == "" {
		t.Fatal("refusing an unknown reading font said nothing about what is accepted")
	}
}
