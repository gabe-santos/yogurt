package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gabe-santos/yogurt/internal/sanitize"
	"github.com/gabe-santos/yogurt/internal/store"
)

// gistView is a Gist as the API presents it.
type gistView struct {
	Text      string    `json:"text"`
	Source    string    `json:"source"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
}

func viewGist(g store.Gist) gistView {
	return gistView{Text: g.Text, Source: g.Source, Model: g.Model, CreatedAt: g.CreatedAt}
}

// aiCredentials reads the Gist model configuration as the model client needs
// it, including the key itself (never returned by GET /api/ai). Gists count
// as turned on once url and model are both set (GABE-202).
func (h *Handler) aiCredentials(r *http.Request) (url, model, key string, enabled bool, err error) {
	url, err = h.deps.Store.Setting(r.Context(), aiURLKey, "")
	if err != nil {
		return "", "", "", false, err
	}
	model, err = h.deps.Store.Setting(r.Context(), aiModelKey, "")
	if err != nil {
		return "", "", "", false, err
	}
	key, err = h.deps.Store.Setting(r.Context(), aiAPIKeyKey, "")
	if err != nil {
		return "", "", "", false, err
	}
	return url, model, key, url != "" && model != "", nil
}

// entryForGist looks up the Entry a Gist request names, answering 404 for an
// id that does not parse or does not exist.
func (h *Handler) entryForGist(w http.ResponseWriter, r *http.Request) (store.Entry, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.writeError(w, r, http.StatusNotFound, "no such Entry")
		return store.Entry{}, false
	}
	entry, err := h.deps.Store.Entry(r.Context(), id)
	switch {
	case errors.Is(err, store.ErrNoEntry):
		h.writeError(w, r, http.StatusNotFound, "no such Entry")
		return store.Entry{}, false
	case err != nil:
		h.serverError(w, r, err)
		return store.Entry{}, false
	}
	return entry, true
}

// getGist serves the stored Gist for an Entry, making no model call. While
// Gists are turned off it answers exactly as if none existed, so stored rows
// stay hidden rather than deleted.
func (h *Handler) getGist(w http.ResponseWriter, r *http.Request) {
	entry, ok := h.entryForGist(w, r)
	if !ok {
		return
	}

	_, _, _, enabled, err := h.aiCredentials(r)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	if !enabled {
		h.writeError(w, r, http.StatusNotFound, "no such Gist")
		return
	}

	gist, err := h.deps.Store.Gist(r.Context(), entry.ID)
	switch {
	case errors.Is(err, store.ErrNoGist):
		h.writeError(w, r, http.StatusNotFound, "no such Gist")
		return
	case err != nil:
		h.serverError(w, r, err)
		return
	}
	h.writeJSON(w, r, http.StatusOK, map[string]any{"gist": viewGist(gist)})
}

// postGist makes or regenerates an Entry's Gist: the Article as Reader View
// would extract it, falling back to the Entry's own body when extraction
// fails or finds nothing usable. It never changes Read, Starred or Archived,
// and a failed model call stores nothing.
func (h *Handler) postGist(w http.ResponseWriter, r *http.Request) {
	entry, ok := h.entryForGist(w, r)
	if !ok {
		return
	}

	url, model, key, enabled, err := h.aiCredentials(r)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	if !enabled {
		h.writeError(w, r, http.StatusBadRequest, "Gists are turned off; configure a model in the Yogurt Menu first")
		return
	}

	// Preferred source: the Article as Reader View would extract it, reusing
	// its cache. If none is cached, extraction is attempted exactly as
	// Reader View would attempt it; a failure here simply falls through to
	// the Entry's own body below, rather than refusing the request.
	article, err := h.deps.Store.Article(r.Context(), entry.URL)
	switch {
	case err == nil:
		// already cached
	case errors.Is(err, store.ErrNoArticle):
		if extracted, extractErr := h.deps.Extraction.Extract(r.Context(), entry.URL); extractErr == nil {
			article = store.Article{URL: entry.URL, Title: extracted.Title, HTML: extracted.HTML, Embeddable: extracted.Embeddable}
			if err := h.deps.Store.SaveArticle(r.Context(), article, h.deps.Clock.Now()); err != nil {
				h.serverError(w, r, err)
				return
			}
		}
	default:
		h.serverError(w, r, err)
		return
	}

	text, source := strings.TrimSpace(sanitize.PlainText(article.HTML)), "article"
	if text == "" {
		text, source = strings.TrimSpace(sanitize.PlainText(entry.Content)), "feed"
	}
	if text == "" {
		h.writeError(w, r, http.StatusBadRequest, "this Entry has no text to condense")
		return
	}

	reply, err := h.deps.AIClient.Gist(r.Context(), url, model, key, text)
	if err != nil {
		h.deps.Logger.WarnContext(r.Context(), "gist model call failed", "error", err)
		h.writeError(w, r, http.StatusBadGateway, "could not reach the Gist model")
		return
	}

	now := h.deps.Clock.Now()
	gist := store.Gist{EntryID: entry.ID, Text: reply, Source: source, Model: model, CreatedAt: now}
	if err := h.deps.Store.SaveGist(r.Context(), gist, now); err != nil {
		h.serverError(w, r, err)
		return
	}
	h.writeJSON(w, r, http.StatusOK, map[string]any{"gist": viewGist(gist)})
}
