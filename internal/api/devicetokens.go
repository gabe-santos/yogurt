package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gabe-santos/yogurt/internal/store"
)

// maxDeviceTokenBody caps how much of a device token request we are willing
// to read.
const maxDeviceTokenBody = 4 << 10

// deviceTokenView is one device token as the API presents it. Its raw value
// is never included here: that appears once, only in the response that
// created it.
type deviceTokenView struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

func viewDeviceToken(t store.DeviceToken) deviceTokenView {
	return deviceTokenView{ID: t.ID, Name: t.Name, CreatedAt: t.CreatedAt, LastUsedAt: t.LastUsedAt}
}

// listDeviceTokens is the reader's whole set of device tokens.
func (h *Handler) listDeviceTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.deps.DeviceTokens.List(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	views := make([]deviceTokenView, 0, len(tokens))
	for _, t := range tokens {
		views = append(views, viewDeviceToken(t))
	}
	h.writeJSON(w, r, http.StatusOK, map[string]any{"device_tokens": views})
}

type deviceTokenRequest struct {
	Name string `json:"name"`
}

// decodeDeviceTokenRequest reads a device token name, trimmed, refusing a
// request that does not decode or names a blank token.
func decodeDeviceTokenRequest(r *http.Request) (string, bool) {
	var body deviceTokenRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, maxDeviceTokenBody)).Decode(&body); err != nil {
		return "", false
	}
	name := strings.TrimSpace(body.Name)
	return name, name != ""
}

// createDeviceToken issues a new device token. Its raw value is returned
// once, in the token field of this response, and is not recoverable again.
func (h *Handler) createDeviceToken(w http.ResponseWriter, r *http.Request) {
	name, ok := decodeDeviceTokenRequest(r)
	if !ok {
		h.writeError(w, r, http.StatusBadRequest, "expected a JSON object with a non-empty name")
		return
	}

	token, raw, err := h.deps.DeviceTokens.Issue(r.Context(), name)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	h.writeJSON(w, r, http.StatusCreated, map[string]any{
		"device_token": viewDeviceToken(token),
		"token":        raw,
	})
}

// deleteDeviceToken revokes a device token, ending it immediately.
func (h *Handler) deleteDeviceToken(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.writeError(w, r, http.StatusNotFound, "no such device token")
		return
	}

	switch err := h.deps.DeviceTokens.Revoke(r.Context(), id); {
	case errors.Is(err, store.ErrNoDeviceToken):
		h.writeError(w, r, http.StatusNotFound, "no such device token")
	case err != nil:
		h.serverError(w, r, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
