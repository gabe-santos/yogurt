package api

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// maxSettingsBody caps how much of a settings request we are willing to read.
const maxSettingsBody = 1 << 10

// markOnOpenKey is the settings row deciding whether opening an Entry marks it
// Read automatically.
const markOnOpenKey = "mark_on_open"

// entryViewKey is the settings row remembering which view an Entry opens in.
// The choice belongs to the reader rather than to an Entry: someone who reads
// in Original View means it for the next Entry too.
const entryViewKey = "entry_view"

// The views an Entry can open in: the text the Feed itself carried, the
// Article reduced to its main text, or the publisher's own page embedded.
const (
	feedEntryView     = "feed"
	readerEntryView   = "reader"
	originalEntryView = "original"
)

// entryViews is every accepted entry_view value, in the order a reader meets
// them. feedEntryView is first and is the default: an Entry opens showing what
// the Feed supplied, and fetching an Article stays something the reader asks
// for.
var entryViews = []string{feedEntryView, readerEntryView, originalEntryView}

// settingsView is the reader's preferences as the API presents them.
type settingsView struct {
	// MarkOnOpen is on by default: opening an Entry marks it Read unless the
	// reader has turned this off.
	MarkOnOpen bool `json:"mark_on_open"`
	// EntryView is one of entryViews.
	EntryView string `json:"entry_view"`
}

func (h *Handler) readSettings(r *http.Request) (settingsView, error) {
	raw, err := h.deps.Store.Setting(r.Context(), markOnOpenKey, strconv.FormatBool(true))
	if err != nil {
		return settingsView{}, err
	}
	markOnOpen, err := strconv.ParseBool(raw)
	if err != nil {
		markOnOpen = true
	}

	entryView, err := h.deps.Store.Setting(r.Context(), entryViewKey, feedEntryView)
	if err != nil {
		return settingsView{}, err
	}
	if !slices.Contains(entryViews, entryView) {
		entryView = feedEntryView
	}

	return settingsView{MarkOnOpen: markOnOpen, EntryView: entryView}, nil
}

// getSettings is the reader's whole set of preferences.
func (h *Handler) getSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.readSettings(r)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	h.writeJSON(w, r, http.StatusOK, map[string]any{"settings": settings})
}

// setSettings declares the reader's preferences, replacing whatever they held,
// and returns them as stored.
func (h *Handler) setSettings(w http.ResponseWriter, r *http.Request) {
	var body settingsView
	if err := json.NewDecoder(io.LimitReader(r.Body, maxSettingsBody)).Decode(&body); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "expected a JSON object with mark_on_open and entry_view")
		return
	}
	if !slices.Contains(entryViews, body.EntryView) {
		h.writeError(w, r, http.StatusBadRequest,
			"entry_view must be one of "+strings.Join(entryViews, ", "))
		return
	}

	if err := h.deps.Store.SetSetting(r.Context(), markOnOpenKey, strconv.FormatBool(body.MarkOnOpen)); err != nil {
		h.serverError(w, r, err)
		return
	}
	if err := h.deps.Store.SetSetting(r.Context(), entryViewKey, body.EntryView); err != nil {
		h.serverError(w, r, err)
		return
	}

	settings, err := h.readSettings(r)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	h.writeJSON(w, r, http.StatusOK, map[string]any{"settings": settings})
}
