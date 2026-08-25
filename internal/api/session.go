package api

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// SessionCookie is the name of the browser's session cookie.
const SessionCookie = "reader_session"

// maxLoginBody caps how much of a login request we are willing to read.
const maxLoginBody = 4 << 10

type loginRequest struct {
	Password string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	client := clientKey(r)
	if retryAfter, blocked := h.deps.Limiter.Blocked(client); blocked {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
		h.writeError(w, r, http.StatusTooManyRequests, "too many failed login attempts")
		return
	}

	var body loginRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxLoginBody))
	if err := decoder.Decode(&body); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "expected a JSON object with a password")
		return
	}

	if !h.deps.Password.Matches(body.Password) {
		h.deps.Limiter.Failed(client)
		h.writeError(w, r, http.StatusUnauthorized, "incorrect password")
		return
	}
	h.deps.Limiter.Succeeded(client)

	token, err := h.deps.Sessions.Issue(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	setSessionCookie(w, r, token, int(h.deps.Sessions.TTL().Seconds()))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(SessionCookie); err == nil {
		if err := h.deps.Sessions.Revoke(r.Context(), cookie.Value); err != nil {
			h.serverError(w, r, err)
			return
		}
	} else if !errors.Is(err, http.ErrNoCookie) {
		h.serverError(w, r, err)
		return
	}

	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) currentSession(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, r, http.StatusOK, map[string]bool{"authenticated": true})
}

// setSessionCookie writes the browser's credential. Its attributes live in one
// place so that issuing and clearing cannot drift apart: HttpOnly keeps scripts
// out, SameSite=Lax survives ordinary navigation, and Secure is set whenever the
// request arrived over TLS.
func setSessionCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie expires the browser's credential.
func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	setSessionCookie(w, r, "", -1)
}

// requireAuth rejects any request that does not carry a live credential:
// either the browser's session cookie, or a device token presented as a
// bearer credential, which authenticates anywhere the cookie would per
// ADR-0005.
func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token, ok := bearerToken(r); ok {
			valid, err := h.deps.DeviceTokens.Authenticate(r.Context(), token)
			if err != nil {
				h.serverError(w, r, err)
				return
			}
			if !valid {
				h.writeError(w, r, http.StatusUnauthorized, "not logged in")
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(SessionCookie)
		if err != nil {
			h.writeError(w, r, http.StatusUnauthorized, "not logged in")
			return
		}
		valid, err := h.deps.Sessions.Valid(r.Context(), cookie.Value)
		if err != nil {
			h.serverError(w, r, err)
			return
		}
		if !valid {
			h.writeError(w, r, http.StatusUnauthorized, "not logged in")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// bearerToken extracts a device token from the Authorization header, if the
// request carries one. The scheme is matched case-insensitively per RFC 7235
// §2.1, since some clients and proxies send "bearer" rather than "Bearer".
func bearerToken(r *http.Request) (string, bool) {
	const scheme = "Bearer "
	header := r.Header.Get("Authorization")
	if len(header) <= len(scheme) || !strings.EqualFold(header[:len(scheme)], scheme) {
		return "", false
	}
	return header[len(scheme):], true
}

// clientKey identifies the caller for rate-limiting purposes. Proxy headers are
// not trusted: they are forgeable, and this app is one process on one host.
func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
