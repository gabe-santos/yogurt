// Package api is the application's HTTP surface: the JSON API under /api and
// the embedded single-page app under everything else.
package api

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/gabe-santos/rss-reader/internal/auth"
	"github.com/gabe-santos/rss-reader/internal/clock"
	"github.com/gabe-santos/rss-reader/internal/pull"
	"github.com/gabe-santos/rss-reader/internal/store"
)

// Deps are the collaborators the HTTP surface needs.
type Deps struct {
	Password *auth.Password
	Sessions *auth.Sessions
	Limiter  *auth.Limiter
	Store    *store.Store
	Pull     *pull.Service
	Clock    clock.Clock
	Logger   *slog.Logger
	// SPA is the compiled frontend, or nil when the binary carries none.
	SPA fs.FS
}

// Handler is the API plus the SPA.
type Handler struct {
	deps Deps
	mux  *http.ServeMux
}

// New wires the routes.
func New(deps Deps) *Handler {
	h := &Handler{deps: deps, mux: http.NewServeMux()}

	h.mux.HandleFunc("POST /api/session", h.login)
	h.mux.HandleFunc("DELETE /api/session", h.logout)
	h.mux.Handle("GET /api/session", h.requireSession(http.HandlerFunc(h.currentSession)))

	h.mux.Handle("GET /api/feeds", h.requireSession(http.HandlerFunc(h.listFeeds)))
	h.mux.Handle("POST /api/feeds", h.requireSession(http.HandlerFunc(h.createFeed)))
	h.mux.Handle("PUT /api/feeds/{id}", h.requireSession(http.HandlerFunc(h.updateFeed)))
	h.mux.Handle("DELETE /api/feeds/{id}", h.requireSession(http.HandlerFunc(h.deleteFeed)))
	h.mux.Handle("POST /api/feeds/refresh", h.requireSession(http.HandlerFunc(h.refreshFeeds)))
	h.mux.Handle("POST /api/feeds/{id}/refresh", h.requireSession(http.HandlerFunc(h.refreshFeed)))

	h.mux.Handle("GET /api/groups", h.requireSession(http.HandlerFunc(h.listGroups)))
	h.mux.Handle("POST /api/groups", h.requireSession(http.HandlerFunc(h.createGroup)))
	h.mux.Handle("PUT /api/groups/{id}", h.requireSession(http.HandlerFunc(h.renameGroup)))
	h.mux.Handle("DELETE /api/groups/{id}", h.requireSession(http.HandlerFunc(h.deleteGroup)))

	h.mux.Handle("GET /api/entries", h.requireSession(http.HandlerFunc(h.listEntries)))
	h.mux.Handle("PUT /api/entries/{id}/state", h.requireSession(http.HandlerFunc(h.setEntryState)))

	h.mux.Handle("GET /api/settings", h.requireSession(http.HandlerFunc(h.getSettings)))
	h.mux.Handle("PUT /api/settings", h.requireSession(http.HandlerFunc(h.setSettings)))

	h.mux.HandleFunc("GET /", h.spa)

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// writeJSON sends a JSON body, or logs and gives up if the client has gone.
func (h *Handler) writeJSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		h.deps.Logger.WarnContext(r.Context(), "write response", "path", r.URL.Path, "error", err)
	}
}

// writeError sends a machine-readable error body.
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, status int, message string) {
	h.writeJSON(w, r, status, map[string]string{"error": message})
}

// serverError logs the cause and tells the caller nothing about it.
func (h *Handler) serverError(w http.ResponseWriter, r *http.Request, err error) {
	h.deps.Logger.ErrorContext(r.Context(), "request failed",
		"method", r.Method, "path", r.URL.Path, "error", err)
	h.writeError(w, r, http.StatusInternalServerError, "internal error")
}
