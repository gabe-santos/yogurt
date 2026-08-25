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

	h.Do(http.MethodPut, "/api/settings", map[string]any{"mark_on_open": false, "entry_view": "feed"}).
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

	h.Do(http.MethodPut, "/api/settings", map[string]any{"mark_on_open": true, "entry_view": "original"}).
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

func TestAnUnknownEntryViewIsRefused(t *testing.T) {
	h := loggedIn(t)

	resp := h.Do(http.MethodPut, "/api/settings",
		map[string]any{"mark_on_open": true, "entry_view": "headless-chromium"}).
		ExpectStatus(http.StatusBadRequest)
	var body struct {
		Error string `json:"error"`
	}
	resp.JSON(&body)
	if body.Error == "" {
		t.Fatal("refusing an unknown entry view said nothing about what is accepted")
	}
}
