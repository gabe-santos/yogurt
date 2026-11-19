package api_test

import (
	"net/http"
	"testing"
)

type aiConfigView struct {
	URL    string `json:"url"`
	Model  string `json:"model"`
	HasKey bool   `json:"has_key"`
}

func TestTheAIConfigDefaultsToUnconfigured(t *testing.T) {
	h := loggedIn(t)

	var body aiConfigView
	h.Do(http.MethodGet, "/api/ai", nil).ExpectStatus(http.StatusOK).JSON(&body)
	if body.URL != "" || body.Model != "" || body.HasKey {
		t.Fatalf("got %+v, want an empty, unconfigured AI config", body)
	}
}

func TestSettingTheAIConfigStoresItWithoutReturningTheKey(t *testing.T) {
	h := loggedIn(t)

	var body aiConfigView
	h.Do(http.MethodPut, "/api/ai", map[string]any{
		"url":   "https://api.openai.com/v1/chat/completions",
		"model": "gpt-4o-mini",
		"key":   "sk-secret",
	}).ExpectStatus(http.StatusOK).JSON(&body)
	if body.URL != "https://api.openai.com/v1/chat/completions" || body.Model != "gpt-4o-mini" || !body.HasKey {
		t.Fatalf("got %+v, want url and model stored and has_key true", body)
	}

	h.Do(http.MethodGet, "/api/ai", nil).ExpectStatus(http.StatusOK).JSON(&body)
	if body.URL != "https://api.openai.com/v1/chat/completions" || body.Model != "gpt-4o-mini" || !body.HasKey {
		t.Fatalf("got %+v after reload, want it to persist", body)
	}
}

func TestOmittingTheKeyOnUpdateKeepsTheStoredOne(t *testing.T) {
	h := loggedIn(t)

	h.Do(http.MethodPut, "/api/ai", map[string]any{
		"url": "https://api.openai.com/v1/chat/completions", "model": "gpt-4o-mini", "key": "sk-secret",
	}).ExpectStatus(http.StatusOK)

	// Changing only the model, with no key field at all, must not disturb the
	// stored key.
	var body aiConfigView
	h.Do(http.MethodPut, "/api/ai", map[string]any{
		"url": "https://api.openai.com/v1/chat/completions", "model": "gpt-4o",
	}).ExpectStatus(http.StatusOK).JSON(&body)
	if body.Model != "gpt-4o" || !body.HasKey {
		t.Fatalf("got %+v, want the new model with the key still set", body)
	}
	stored, err := h.Store.Setting(t.Context(), "ai_key", "")
	if err != nil {
		t.Fatal(err)
	}
	if stored != "sk-secret" {
		t.Fatalf("stored key = %q, want the original key kept", stored)
	}
}

func TestReplacingTheStoredKey(t *testing.T) {
	h := loggedIn(t)

	h.Do(http.MethodPut, "/api/ai", map[string]any{
		"url": "https://api.openai.com/v1/chat/completions", "model": "gpt-4o-mini", "key": "sk-old",
	}).ExpectStatus(http.StatusOK)

	var body aiConfigView
	h.Do(http.MethodPut, "/api/ai", map[string]any{
		"url": "https://api.openai.com/v1/chat/completions", "model": "gpt-4o-mini", "key": "sk-new",
	}).ExpectStatus(http.StatusOK).JSON(&body)
	if !body.HasKey {
		t.Fatalf("got %+v, want has_key true after replacing the key", body)
	}
	stored, err := h.Store.Setting(t.Context(), "ai_key", "")
	if err != nil {
		t.Fatal(err)
	}
	if stored != "sk-new" {
		t.Fatalf("stored key = %q, want the new key to have replaced the old one", stored)
	}
}

func TestClearingTheAIConfigRemovesEverything(t *testing.T) {
	h := loggedIn(t)

	h.Do(http.MethodPut, "/api/ai", map[string]any{
		"url": "https://api.openai.com/v1/chat/completions", "model": "gpt-4o-mini", "key": "sk-secret",
	}).ExpectStatus(http.StatusOK)

	h.Do(http.MethodDelete, "/api/ai", nil).ExpectStatus(http.StatusNoContent)

	var body aiConfigView
	h.Do(http.MethodGet, "/api/ai", nil).ExpectStatus(http.StatusOK).JSON(&body)
	if body.URL != "" || body.Model != "" || body.HasKey {
		t.Fatalf("got %+v, want everything cleared", body)
	}
}

func TestADeviceTokenCanUseTheAIConfigEndpoints(t *testing.T) {
	h := loggedIn(t)
	_, raw := createDeviceToken(t, h, "desktop shell")

	h.DoBearer(http.MethodPut, "/api/ai", raw, map[string]any{
		"url": "https://api.openai.com/v1/chat/completions", "model": "gpt-4o-mini", "key": "sk-secret",
	}).ExpectStatus(http.StatusOK)

	var body aiConfigView
	h.DoBearer(http.MethodGet, "/api/ai", raw, nil).ExpectStatus(http.StatusOK).JSON(&body)
	if body.Model != "gpt-4o-mini" || !body.HasKey {
		t.Fatalf("got %+v, want the device token to read what it wrote", body)
	}

	h.DoBearer(http.MethodDelete, "/api/ai", raw, nil).ExpectStatus(http.StatusNoContent)
}

func TestOrdinarySettingsNeitherExposeNorClobberTheAIConfig(t *testing.T) {
	h := loggedIn(t)

	h.Do(http.MethodPut, "/api/ai", map[string]any{
		"url": "https://api.openai.com/v1/chat/completions", "model": "gpt-4o-mini", "key": "sk-secret",
	}).ExpectStatus(http.StatusOK)

	// The ordinary settings GET is a different JSON shape entirely, so it
	// cannot carry any ai_* value; checking the raw body also guards against
	// a future field accidentally reusing these names.
	raw := h.Do(http.MethodGet, "/api/settings", nil).ExpectStatus(http.StatusOK)
	var rawBody map[string]any
	raw.JSON(&rawBody)
	settings, ok := rawBody["settings"].(map[string]any)
	if !ok {
		t.Fatalf("settings response had no settings object: %+v", rawBody)
	}
	for _, key := range []string{"url", "model", "has_key", "key"} {
		if _, present := settings[key]; present {
			t.Fatalf("ordinary settings exposed %q", key)
		}
	}

	// Replacing the whole set of ordinary preferences must not touch the AI
	// config.
	h.Do(http.MethodPut, "/api/settings",
		map[string]any{"mark_on_open": false, "entry_view": "feed", "reading_font": "sans"}).
		ExpectStatus(http.StatusOK)

	var aiBody aiConfigView
	h.Do(http.MethodGet, "/api/ai", nil).ExpectStatus(http.StatusOK).JSON(&aiBody)
	if aiBody.Model != "gpt-4o-mini" || !aiBody.HasKey {
		t.Fatalf("got %+v, want the AI config untouched by a settings PUT", aiBody)
	}
}
