package pull

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gabe-santos/yogurt/internal/fetch"
	"github.com/gabe-santos/yogurt/internal/icon"
	"github.com/gabe-santos/yogurt/internal/store"
)

// iconRecheckInterval is how long a Feed's icon check is trusted before it is
// probed again: a site with no usable icon must not be fetched on every poll
// forever, and a site whose icon changes upstream should still eventually be
// noticed. One checked-at timestamp gates both outcomes (ADR-0008).
const iconRecheckInterval = 30 * 24 * time.Hour

// iconDue reports whether a Feed should be checked for its Feed Icon:
// either it has never been checked at all — which covers both a Feed
// subscribed before this feature existed and one just subscribed for the
// first time — or it was last checked at least iconRecheckInterval ago,
// regardless of whether that check found an icon.
func iconDue(feed store.Feed, now time.Time) bool {
	return feed.IconCheckedAt.IsZero() || now.Sub(feed.IconCheckedAt) >= iconRecheckInterval
}

// fetchIconCandidate adapts the pull service's fetch.Client to icon.Fetcher,
// so icon.Choose can retrieve the candidates it finds without knowing about
// fetch.Client itself. It goes through the same client as everything else, so
// the private-network guard applies to publisher-controlled icon addresses
// exactly as it does to Feed URLs — this matters more here, since the site
// and icon addresses come from publisher markup rather than from the reader.
type fetchIconCandidate struct {
	client *fetch.Client
}

// Fetch retrieves rawURL's body, satisfying icon.Fetcher. Any status other
// than 200 is treated as failure to fetch, since there is nothing to choose
// from a candidate the publisher refused.
func (f fetchIconCandidate) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := f.client.Get(ctx, rawURL, fetch.Conditional{})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("icon candidate %s: unexpected status %d", rawURL, resp.StatusCode)
	}
	return resp.Body, nil
}

// discoverIcon finds and stores a Feed's Feed Icon, best-effort: an
// unreachable site, one with no usable icon, or a candidate the
// private-network guard refuses all leave the Feed with no icon rather than
// failing the caller. siteURL is the Feed's own site_url, which may be empty
// for a Feed that never advertised one. Every path ends by recording that the
// Feed was checked, which is what lets a Feed with no usable icon be
// re-probed about once a month instead of on every poll (ADR-0008).
func (s *Service) discoverIcon(ctx context.Context, feedID int64, siteURL string, now time.Time) {
	base, err := fetch.ParseURL(siteURL)
	if err != nil {
		s.markIconChecked(ctx, feedID, now)
		return
	}

	resp, err := s.client.Get(ctx, siteURL, fetch.Conditional{})
	if err != nil || resp.StatusCode != http.StatusOK {
		s.markIconChecked(ctx, feedID, now)
		return
	}

	chosen, ok := icon.Choose(ctx, fetchIconCandidate{client: s.client}, resp.Body, base)
	if !ok {
		s.markIconChecked(ctx, feedID, now)
		return
	}
	if err := s.store.SetFeedIcon(ctx, feedID, chosen.Data, chosen.MediaType, now); err != nil {
		s.logger.WarnContext(ctx, "set feed icon", "feed_id", feedID, "error", err)
	}
}

// markIconChecked records that a Feed was checked for an icon, logging
// rather than propagating a store failure: a failed write here must not turn
// into a failed poll.
func (s *Service) markIconChecked(ctx context.Context, feedID int64, now time.Time) {
	if err := s.store.MarkFeedIconChecked(ctx, feedID, now); err != nil {
		s.logger.WarnContext(ctx, "mark feed icon checked", "feed_id", feedID, "error", err)
	}
}
