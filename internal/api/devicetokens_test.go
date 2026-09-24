package api_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/gabe-santos/yogurt/internal/apitest"
)

type deviceTokenView struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
}

// createDeviceToken issues a new device token and returns its view and raw
// value, failing the test if refused.
func createDeviceToken(t *testing.T, h *apitest.Harness, name string) (deviceTokenView, string) {
	t.Helper()
	var body struct {
		DeviceToken deviceTokenView `json:"device_token"`
		Token       string          `json:"token"`
	}
	h.Do(http.MethodPost, "/api/device-tokens", map[string]string{"name": name}).
		ExpectStatus(http.StatusCreated).
		JSON(&body)
	return body.DeviceToken, body.Token
}

func listDeviceTokens(t *testing.T, h *apitest.Harness) []deviceTokenView {
	t.Helper()
	var body struct {
		DeviceTokens []deviceTokenView `json:"device_tokens"`
	}
	h.Do(http.MethodGet, "/api/device-tokens", nil).ExpectStatus(http.StatusOK).JSON(&body)
	return body.DeviceTokens
}

func TestCreatingADeviceTokenShowsItsRawValueOnlyOnce(t *testing.T) {
	h := loggedIn(t)

	token, raw := createDeviceToken(t, h, "desktop shell")
	if token.Name != "desktop shell" {
		t.Errorf("name = %q, want %q", token.Name, "desktop shell")
	}
	if raw == "" {
		t.Fatal("create response carried no raw token value")
	}
	if token.LastUsedAt != nil {
		t.Errorf("LastUsedAt = %v, want nil for a token that has never authenticated a request", *token.LastUsedAt)
	}

	tokens := listDeviceTokens(t, h)
	if len(tokens) != 1 {
		t.Fatalf("got %d device tokens, want 1", len(tokens))
	}
}

func TestADeviceTokenAuthenticatesRequestsAndRecordsItsLastUse(t *testing.T) {
	h := loggedIn(t)
	_, raw := createDeviceToken(t, h, "desktop shell")

	// A fresh client presenting only the token, no session cookie.
	h.DoBearer(http.MethodGet, "/api/feeds", raw, nil).ExpectStatus(http.StatusOK)

	tokens := listDeviceTokens(t, h)
	if tokens[0].LastUsedAt == nil {
		t.Error("LastUsedAt is still nil after the token authenticated a request")
	}
}

func TestARevokedDeviceTokenStopsAuthenticatingImmediately(t *testing.T) {
	h := loggedIn(t)
	token, raw := createDeviceToken(t, h, "desktop shell")

	h.DoBearer(http.MethodGet, "/api/feeds", raw, nil).ExpectStatus(http.StatusOK)
	h.Do(http.MethodDelete, "/api/device-tokens/"+strconv.FormatInt(token.ID, 10), nil).ExpectStatus(http.StatusNoContent)

	h.DoBearer(http.MethodGet, "/api/feeds", raw, nil).ExpectStatus(http.StatusUnauthorized)
}

func TestAnInvalidBearerTokenIsRejected(t *testing.T) {
	h := loggedIn(t)
	h.DoBearer(http.MethodGet, "/api/feeds", "not-a-real-token", nil).ExpectStatus(http.StatusUnauthorized)
}

func TestCreatingADeviceTokenWithABlankNameIsRejected(t *testing.T) {
	h := loggedIn(t)
	h.Do(http.MethodPost, "/api/device-tokens", map[string]string{"name": "  "}).
		ExpectStatus(http.StatusBadRequest)
}

func TestRevokingAnUnknownDeviceTokenIs404(t *testing.T) {
	h := loggedIn(t)
	h.Do(http.MethodDelete, "/api/device-tokens/999999", nil).ExpectStatus(http.StatusNotFound)
}
