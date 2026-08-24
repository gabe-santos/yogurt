package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gabe-santos/rss-reader/internal/store"
)

// maxGroupBody caps how much of a Group request we are willing to read.
const maxGroupBody = 4 << 10

// groupView is one Group as the API presents it.
type groupView struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	IsDefault   bool      `json:"is_default"`
	UnreadCount int       `json:"unread_count"`
	CreatedAt   time.Time `json:"created_at"`
}

func viewGroup(group store.Group, unreadCounts map[int64]int) groupView {
	return groupView{
		ID:          group.ID,
		Name:        group.Name,
		IsDefault:   group.IsDefault,
		UnreadCount: unreadCounts[group.ID],
		CreatedAt:   group.CreatedAt,
	}
}

// listGroups is the reader's whole set of Groups, with each one's unread
// count summed from the Feeds it holds.
func (h *Handler) listGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.deps.Store.Groups(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	counts, err := h.deps.Store.GroupUnreadCounts(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	views := make([]groupView, 0, len(groups))
	for _, group := range groups {
		views = append(views, viewGroup(group, counts))
	}
	h.writeJSON(w, r, http.StatusOK, map[string]any{"groups": views})
}

type groupRequest struct {
	Name string `json:"name"`
}

// decodeGroupRequest reads a Group name, trimmed, refusing a request that
// does not decode or names a blank Group: per CONTEXT.md a Group is "a named
// set of Feeds", so a nameless one is never valid.
func decodeGroupRequest(r *http.Request) (string, bool) {
	var body groupRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, maxGroupBody)).Decode(&body); err != nil {
		return "", false
	}
	name := strings.TrimSpace(body.Name)
	return name, name != ""
}

// createGroup adds a new Group.
func (h *Handler) createGroup(w http.ResponseWriter, r *http.Request) {
	name, ok := decodeGroupRequest(r)
	if !ok {
		h.writeError(w, r, http.StatusBadRequest, "expected a JSON object with a non-empty name")
		return
	}

	group, err := h.deps.Store.CreateGroup(r.Context(), name, h.deps.Clock.Now())
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	h.writeJSON(w, r, http.StatusCreated, map[string]any{"group": viewGroup(group, nil)})
}

// renameGroup sets a Group's name.
func (h *Handler) renameGroup(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.writeError(w, r, http.StatusNotFound, "no such Group")
		return
	}

	name, ok := decodeGroupRequest(r)
	if !ok {
		h.writeError(w, r, http.StatusBadRequest, "expected a JSON object with a non-empty name")
		return
	}

	group, err := h.deps.Store.RenameGroup(r.Context(), id, name, h.deps.Clock.Now())
	if errors.Is(err, store.ErrNoGroup) {
		h.writeError(w, r, http.StatusNotFound, "no such Group")
		return
	} else if err != nil {
		h.serverError(w, r, err)
		return
	}

	counts, err := h.deps.Store.GroupUnreadCounts(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	h.writeJSON(w, r, http.StatusOK, map[string]any{"group": viewGroup(group, counts)})
}

// deleteGroup removes a Group, reparenting its Feeds to the default Group.
func (h *Handler) deleteGroup(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.writeError(w, r, http.StatusNotFound, "no such Group")
		return
	}

	switch err := h.deps.Store.DeleteGroup(r.Context(), id, h.deps.Clock.Now()); {
	case errors.Is(err, store.ErrNoGroup):
		h.writeError(w, r, http.StatusNotFound, "no such Group")
	case errors.Is(err, store.ErrDefaultGroup):
		h.writeError(w, r, http.StatusConflict, "the default Group cannot be deleted")
	case err != nil:
		h.serverError(w, r, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
