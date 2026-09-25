package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gabe-santos/yogurt/internal/apitest"
)

func TestLoggingInWithTheConfiguredPasswordStartsASession(t *testing.T) {
	h := apitest.New(t)

	resp := h.Login(apitest.Password).ExpectStatus(http.StatusNoContent)

	cookie := resp.Cookie("yogurt_session")
	if cookie == nil {
		t.Fatal("login set no session cookie")
	}
	if cookie.Value == "" {
		t.Fatal("session cookie is empty")
	}
	if !cookie.HttpOnly {
		t.Error("session cookie is not HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("session cookie SameSite = %v, want Lax", cookie.SameSite)
	}
	if cookie.Secure {
		t.Error("session cookie is Secure over plain HTTP")
	}
	if cookie.MaxAge <= 0 {
		t.Errorf("session cookie MaxAge = %d, want a persistent cookie", cookie.MaxAge)
	}

	h.Do(http.MethodGet, "/api/session", nil).ExpectStatus(http.StatusOK)
}

func TestLoggingInThroughAnHTTPSProxyMarksTheSessionCookieSecure(t *testing.T) {
	h := apitest.New(t)

	// Caddy and Traefik end HTTPS themselves and forward plain HTTP, saying so
	// in this header.
	resp := h.LoginWithHeader(apitest.Password, http.Header{"X-Forwarded-Proto": {"https"}}).
		ExpectStatus(http.StatusNoContent)

	cookie := resp.Cookie("yogurt_session")
	if cookie == nil {
		t.Fatal("login set no session cookie")
	}
	if !cookie.Secure {
		t.Error("session cookie is not Secure")
	}
}

func TestLoggingInWithTheWrongPasswordIsRejected(t *testing.T) {
	h := apitest.New(t)

	resp := h.Login("hunter2").ExpectStatus(http.StatusUnauthorized)
	if cookie := resp.Cookie("yogurt_session"); cookie != nil {
		t.Fatalf("rejected login set a session cookie: %q", cookie.Value)
	}

	h.Do(http.MethodGet, "/api/session", nil).ExpectStatus(http.StatusUnauthorized)
}

func TestLoggingOutEndsTheSessionOnTheServer(t *testing.T) {
	h := apitest.New(t)
	token := h.Login(apitest.Password).ExpectStatus(http.StatusNoContent).Cookie("yogurt_session").Value

	h.Logout().ExpectStatus(http.StatusNoContent)
	h.Do(http.MethodGet, "/api/session", nil).ExpectStatus(http.StatusUnauthorized)

	// The token itself is dead, not merely dropped by this browser.
	h.UseSessionToken(token)
	h.Do(http.MethodGet, "/api/session", nil).ExpectStatus(http.StatusUnauthorized)
}

func TestASessionExpiresAfterItsLifetime(t *testing.T) {
	h := apitest.New(t)
	h.Login(apitest.Password).ExpectStatus(http.StatusNoContent)

	h.Clock.Advance(29 * 24 * time.Hour)
	h.Do(http.MethodGet, "/api/session", nil).ExpectStatus(http.StatusOK)

	h.Clock.Advance(2 * 24 * time.Hour)
	h.Do(http.MethodGet, "/api/session", nil).ExpectStatus(http.StatusUnauthorized)
}

func TestRepeatedFailedLoginsAreRateLimited(t *testing.T) {
	h := apitest.New(t)

	for range 5 {
		h.Login("hunter2").ExpectStatus(http.StatusUnauthorized)
	}

	// The correct password is refused too: the client, not the guess, is blocked.
	h.Login(apitest.Password).ExpectStatus(http.StatusTooManyRequests)
	h.Do(http.MethodGet, "/api/session", nil).ExpectStatus(http.StatusUnauthorized)

	h.Clock.Advance(15 * time.Minute)
	h.Login(apitest.Password).ExpectStatus(http.StatusNoContent)
}

// Behind a reverse proxy every login arrives from the proxy's own address, so
// failures from anyone block the Reader too. The client address the proxy
// forwards does not tell them apart, because a caller can forge it.
func TestBehindAProxyFailedLoginsFromAnyoneBlockTheReader(t *testing.T) {
	h := apitest.New(t)
	forwardedFor := func(client string) http.Header {
		return http.Header{"X-Forwarded-For": {client}}
	}

	for range 5 {
		h.LoginWithHeader("hunter2", forwardedFor("203.0.113.7")).ExpectStatus(http.StatusUnauthorized)
	}

	h.LoginWithHeader(apitest.Password, forwardedFor("198.51.100.9")).ExpectStatus(http.StatusTooManyRequests)
}

func TestASuccessfulLoginClearsEarlierFailures(t *testing.T) {
	h := apitest.New(t)

	for range 4 {
		h.Login("hunter2").ExpectStatus(http.StatusUnauthorized)
	}
	h.Login(apitest.Password).ExpectStatus(http.StatusNoContent)
	h.Logout().ExpectStatus(http.StatusNoContent)

	for range 4 {
		h.Login("hunter2").ExpectStatus(http.StatusUnauthorized)
	}
	h.Login(apitest.Password).ExpectStatus(http.StatusNoContent)
}
