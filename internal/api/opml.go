package api

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/gabe-santos/rss-reader/internal/opml"
	"github.com/gabe-santos/rss-reader/internal/pull"
	"github.com/gabe-santos/rss-reader/internal/store"
)

// maxOPMLBody caps how much of an OPML import we are willing to read.
const maxOPMLBody = 1 << 20

// importedFeed is one Feed an OPML import subscribed to.
type importedFeed struct {
	Feed  feedView `json:"feed"`
	Group string   `json:"group"`
}

// skippedFeed is one Feed an OPML import left alone, and why.
type skippedFeed struct {
	URL    string `json:"url"`
	Reason string `json:"reason"`
}

// importOPML reads an OPML document and subscribes to every Feed it names
// that this reader does not already hold. A Feed nested in an OPML folder is
// sorted into the Group that folder names — created if this reader does not
// have it yet — per the flattening opml.Parse documents; a Feed at the
// document's top level goes to the default Group, like any other new
// subscription. A Feed already subscribed is recognised by its URL before it
// is ever fetched, and one that could not be fetched is recognised after; both
// are skipped and reported rather than failing the whole import, and the
// Feeds that do need fetching are fetched concurrently, bounded the way
// SubscribeMany already bounds it.
func (h *Handler) importOPML(w http.ResponseWriter, r *http.Request) {
	subs, err := opml.Parse(io.LimitReader(r.Body, maxOPMLBody))
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, "that is not a valid OPML document")
		return
	}

	existing, err := h.deps.Store.Feeds(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	alreadySubscribed := make(map[string]bool, len(existing))
	for _, feed := range existing {
		alreadySubscribed[feed.URL] = true
	}

	added := make([]importedFeed, 0)
	skipped := make([]skippedFeed, 0)

	// A Feed already subscribed is skipped without fetching it: a re-import
	// of a previously exported collection would otherwise re-fetch every
	// Feed in it only to discard the result.
	pending := make([]opml.FeedImport, 0, len(subs))
	for _, sub := range subs {
		if alreadySubscribed[sub.URL] {
			skipped = append(skipped, skippedFeed{URL: sub.URL, Reason: "already subscribed to this Feed"})
			continue
		}
		pending = append(pending, sub)
	}

	// Groups are resolved only for the Feeds still pending: a folder made up
	// entirely of Feeds this reader already holds never creates an empty
	// Group.
	groupIDs, err := h.resolveOPMLGroups(r.Context(), pending)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	requests := make([]pull.SubscribeRequest, len(pending))
	for i, sub := range pending {
		requests[i] = pull.SubscribeRequest{URL: sub.URL, GroupID: groupIDs[sub.GroupName]}
	}

	for i, outcome := range h.deps.Pull.SubscribeMany(r.Context(), requests) {
		sub := pending[i]
		switch {
		case errors.Is(outcome.Err, store.ErrFeedExists):
			skipped = append(skipped, skippedFeed{URL: sub.URL, Reason: "already subscribed to this Feed"})
		case errors.Is(outcome.Err, pull.ErrInvalidURL):
			skipped = append(skipped, skippedFeed{URL: sub.URL, Reason: "not a valid web address"})
		case errors.Is(outcome.Err, pull.ErrNoFeed):
			skipped = append(skipped, skippedFeed{URL: sub.URL, Reason: "no Feed found at that address"})
		case isFetchFailure(outcome.Err):
			skipped = append(skipped, skippedFeed{URL: sub.URL, Reason: outcome.Err.Error()})
		case outcome.Err != nil:
			h.serverError(w, r, outcome.Err)
			return
		default:
			added = append(added, importedFeed{Feed: viewFeed(outcome.Feed, nil), Group: sub.GroupName})
		}
	}

	h.writeJSON(w, r, http.StatusOK, map[string]any{"added": added, "skipped": skipped})
}

// resolveOPMLGroups maps every FeedImport's GroupName to the id of the
// Group it belongs to, creating a Group this reader does not have yet. An
// empty GroupName maps to the zero id, which SubscribeInGroup already
// resolves to the default Group.
func (h *Handler) resolveOPMLGroups(ctx context.Context, subs []opml.FeedImport) (map[string]int64, error) {
	existing, err := h.deps.Store.Groups(ctx)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]int64, len(existing))
	for _, group := range existing {
		ids[group.Name] = group.ID
	}

	for _, sub := range subs {
		if sub.GroupName == "" {
			continue
		}
		if _, ok := ids[sub.GroupName]; ok {
			continue
		}
		group, err := h.deps.Store.CreateGroup(ctx, sub.GroupName, h.deps.Clock.Now())
		if err != nil {
			return nil, err
		}
		ids[sub.GroupName] = group.ID
	}
	return ids, nil
}

// exportOPML writes the reader's whole collection as an OPML document, one
// folder per Group that holds a Feed, with the default Group's Feeds at the
// document's top level, per opml.Write.
func (h *Handler) exportOPML(w http.ResponseWriter, r *http.Request) {
	groups, err := h.deps.Store.Groups(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}
	feeds, err := h.deps.Store.Feeds(r.Context())
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	byGroup := make(map[int64][]store.Feed)
	for _, feed := range feeds {
		byGroup[feed.GroupID] = append(byGroup[feed.GroupID], feed)
	}

	exports := make([]opml.GroupExport, 0, len(groups))
	for _, group := range groups {
		name := group.Name
		if group.IsDefault {
			name = ""
		}

		groupFeeds := byGroup[group.ID]
		feedExports := make([]opml.FeedExport, 0, len(groupFeeds))
		for _, feed := range groupFeeds {
			feedExports = append(feedExports, opml.FeedExport{
				Title:   feed.Title,
				XMLURL:  feed.URL,
				HTMLURL: feed.SiteURL,
			})
		}
		exports = append(exports, opml.GroupExport{Name: name, Feeds: feedExports})
	}

	w.Header().Set("Content-Type", "text/x-opml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="feeds.opml"`)
	if err := opml.Write(w, exports); err != nil {
		h.deps.Logger.ErrorContext(r.Context(), "write opml export", "error", err)
	}
}
