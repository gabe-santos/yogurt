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

// unreadOnlyKey is the settings row remembering whether the Entry List is
// narrowed to unread Entries. Unread Only is a modifier over whichever
// Collection the reader chose rather than a Collection of its own, so it
// belongs with the reader's other preferences and survives a reload.
const unreadOnlyKey = "unread_only"

// readingFontKey is the settings row remembering which typeface the Reading
// Pane sets its own text in. It governs Reader View and Feed View, which are
// this app's markup; Original View is the publisher's own layout and their
// typography, and is never restyled.
const readingFontKey = "reading_font"

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

// The typefaces the Reading Pane can set an Entry in: the interface's own
// grotesque, or a serif cut for long-form screen reading.
const (
	sansReadingFont  = "sans"
	serifReadingFont = "serif"
)

// readingFonts is every accepted reading_font value. sansReadingFont is first
// and is the default: reading in the same face as the rest of the interface is
// what the app did before the choice existed.
var readingFonts = []string{sansReadingFont, serifReadingFont}

// settingsView is the reader's preferences as the API presents them.
type settingsView struct {
	// MarkOnOpen is on by default: opening an Entry marks it Read unless the
	// reader has turned this off.
	MarkOnOpen bool `json:"mark_on_open"`
	// EntryView is one of entryViews.
	EntryView string `json:"entry_view"`
	// UnreadOnly is off by default: a Collection opens showing everything it
	// holds until the reader narrows it.
	UnreadOnly bool `json:"unread_only"`
	// ReadingFont is one of readingFonts.
	ReadingFont string `json:"reading_font"`
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

	rawUnreadOnly, err := h.deps.Store.Setting(r.Context(), unreadOnlyKey, strconv.FormatBool(false))
	if err != nil {
		return settingsView{}, err
	}
	unreadOnly, err := strconv.ParseBool(rawUnreadOnly)
	if err != nil {
		unreadOnly = false
	}

	readingFont, err := h.deps.Store.Setting(r.Context(), readingFontKey, sansReadingFont)
	if err != nil {
		return settingsView{}, err
	}
	if !slices.Contains(readingFonts, readingFont) {
		readingFont = sansReadingFont
	}

	return settingsView{
		MarkOnOpen:  markOnOpen,
		EntryView:   entryView,
		UnreadOnly:  unreadOnly,
		ReadingFont: readingFont,
	}, nil
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
		h.writeError(w, r, http.StatusBadRequest, "expected a JSON object with mark_on_open, entry_view, unread_only and reading_font")
		return
	}
	if !slices.Contains(entryViews, body.EntryView) {
		h.writeError(w, r, http.StatusBadRequest,
			"entry_view must be one of "+strings.Join(entryViews, ", "))
		return
	}
	if !slices.Contains(readingFonts, body.ReadingFont) {
		h.writeError(w, r, http.StatusBadRequest,
			"reading_font must be one of "+strings.Join(readingFonts, ", "))
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
	if err := h.deps.Store.SetSetting(r.Context(), unreadOnlyKey, strconv.FormatBool(body.UnreadOnly)); err != nil {
		h.serverError(w, r, err)
		return
	}
	if err := h.deps.Store.SetSetting(r.Context(), readingFontKey, body.ReadingFont); err != nil {
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
