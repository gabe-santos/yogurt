package api_test

import (
	"net/http"
	"testing"
)

func TestTheMarkOnOpenSettingPersists(t *testing.T) {
	h := loggedIn(t)

	var body struct {
		Settings struct {
			MarkOnOpen bool `json:"mark_on_open"`
		} `json:"settings"`
	}
	h.Do(http.MethodGet, "/api/settings", nil).ExpectStatus(http.StatusOK).JSON(&body)
	if !body.Settings.MarkOnOpen {
		t.Fatal("mark-on-open is not on by default")
	}

	h.Do(http.MethodPut, "/api/settings", map[string]any{"mark_on_open": false}).
		ExpectStatus(http.StatusOK).JSON(&body)
	if body.Settings.MarkOnOpen {
		t.Fatal("turning mark-on-open off was not reflected in the response")
	}

	h.Do(http.MethodGet, "/api/settings", nil).ExpectStatus(http.StatusOK).JSON(&body)
	if body.Settings.MarkOnOpen {
		t.Fatal("mark-on-open did not persist as off")
	}
}
