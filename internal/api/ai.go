package api

import (
	"encoding/json"
	"io"
	"net/http"
)

// aiURLKey is the settings row holding the Gist model's endpoint URL.
const aiURLKey = "ai_url"

// aiModelKey is the settings row holding the Gist model's name.
const aiModelKey = "ai_model"

// aiAPIKeyKey is the settings row holding the Gist model's API key, stored
// unencrypted (ADR-0018). It is never returned by the API; aiConfigView
// reports only whether one is set.
const aiAPIKeyKey = "ai_key"

// aiConfigView is the Gist model configuration as the API presents it. It
// deliberately has no field for the key itself.
type aiConfigView struct {
	URL    string `json:"url"`
	Model  string `json:"model"`
	HasKey bool   `json:"has_key"`
}

func (h *Handler) readAIConfig(r *http.Request) (aiConfigView, error) {
	url, err := h.deps.Store.Setting(r.Context(), aiURLKey, "")
	if err != nil {
		return aiConfigView{}, err
	}
	model, err := h.deps.Store.Setting(r.Context(), aiModelKey, "")
	if err != nil {
		return aiConfigView{}, err
	}
	key, err := h.deps.Store.Setting(r.Context(), aiAPIKeyKey, "")
	if err != nil {
		return aiConfigView{}, err
	}
	return aiConfigView{URL: url, Model: model, HasKey: key != ""}, nil
}

// getAI is the Gist model configuration, never including the key itself.
func (h *Handler) getAI(w http.ResponseWriter, r *http.Request) {
	config, err := h.readAIConfig(r)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	h.writeJSON(w, r, http.StatusOK, config)
}

// aiUpdate is a PUT /api/ai body. Key is a pointer so an absent field (keep
// the stored key) is distinguishable from an empty one (clear it).
type aiUpdate struct {
	URL   string  `json:"url"`
	Model string  `json:"model"`
	Key   *string `json:"key"`
}

// setAI declares the endpoint URL and model, replacing whatever they held. A
// key that is absent from the request leaves the stored key in place; one
// that is present, even empty, replaces it.
func (h *Handler) setAI(w http.ResponseWriter, r *http.Request) {
	var body aiUpdate
	if err := json.NewDecoder(io.LimitReader(r.Body, maxSettingsBody)).Decode(&body); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "expected a JSON object with url and model, and an optional key")
		return
	}

	if err := h.deps.Store.SetSetting(r.Context(), aiURLKey, body.URL); err != nil {
		h.serverError(w, r, err)
		return
	}
	if err := h.deps.Store.SetSetting(r.Context(), aiModelKey, body.Model); err != nil {
		h.serverError(w, r, err)
		return
	}
	if body.Key != nil {
		if err := h.deps.Store.SetSetting(r.Context(), aiAPIKeyKey, *body.Key); err != nil {
			h.serverError(w, r, err)
			return
		}
	}

	config, err := h.readAIConfig(r)
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	h.writeJSON(w, r, http.StatusOK, config)
}

// deleteAI clears the URL, the model and the key, turning Gists off.
func (h *Handler) deleteAI(w http.ResponseWriter, r *http.Request) {
	for _, key := range []string{aiURLKey, aiModelKey, aiAPIKeyKey} {
		if err := h.deps.Store.SetSetting(r.Context(), key, ""); err != nil {
			h.serverError(w, r, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
