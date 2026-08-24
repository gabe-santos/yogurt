package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
)

// maxSettingsBody caps how much of a settings request we are willing to read.
const maxSettingsBody = 1 << 10

// markOnOpenKey is the settings row deciding whether opening an Entry marks it
// Read automatically.
const markOnOpenKey = "mark_on_open"

// settingsView is the reader's preferences as the API presents them.
type settingsView struct {
	// MarkOnOpen is on by default: opening an Entry marks it Read unless the
	// reader has turned this off.
	MarkOnOpen bool `json:"mark_on_open"`
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
	return settingsView{MarkOnOpen: markOnOpen}, nil
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
		h.writeError(w, r, http.StatusBadRequest, "expected a JSON object with mark_on_open")
		return
	}

	if err := h.deps.Store.SetSetting(r.Context(), markOnOpenKey, strconv.FormatBool(body.MarkOnOpen)); err != nil {
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
