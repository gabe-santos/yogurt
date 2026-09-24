// Package pull subscribes to Feeds and reads them: fetch, parse, and store what
// the publisher carried. It is the only place that turns a Feed document into
// Entries, and the only place a Feed is checked — on demand or on its own
// schedule.
package pull

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gabe-santos/yogurt/internal/clock"
	"github.com/gabe-santos/yogurt/internal/feed"
	"github.com/gabe-santos/yogurt/internal/fetch"
	"github.com/gabe-santos/yogurt/internal/pullpolicy"
	"github.com/gabe-santos/yogurt/internal/store"
)

// concurrency bounds how many Feeds are fetched at once, so that refreshing
// hundreds of Feeds is quick without opening hundreds of connections.
const concurrency = 8

// defaultTick is how often the schedule wakes to look for a due Feed, when the
// caller has no opinion. It only needs to be finer than the shortest interval
// a reader could plausibly configure.
const defaultTick = time.Minute

// ErrNoFeed reports an address that is reachable but carries no Feed, and
// advertises none.
var ErrNoFeed = errors.New("no Feed found at that address")

// ErrInvalidURL reports an address the reader typed that cannot be fetched.
var ErrInvalidURL = fetch.ErrNotAbsoluteURL

// FetchError reports that an address could not be read at all: the connection
// failed, was refused by the guard, or the publisher answered with an error.
type FetchError struct {
	URL string
	Err error
}

func (e *FetchError) Error() string { return fmt.Sprintf("could not fetch %s: %v", e.URL, e.Err) }
func (e *FetchError) Unwrap() error { return e.Err }

// Service subscribes to Feeds, refreshes them on demand, and polls them on
// their own schedule.
type Service struct {
	store    *store.Store
	client   *fetch.Client
	clock    clock.Clock
	logger   *slog.Logger
	interval time.Duration
}

// New wires a pull service. interval is how often a Feed is checked when
// nothing else — a publisher's hint, or a run of failures — says otherwise; a
// non-positive interval falls back to pullpolicy.DefaultInterval.
func New(db *store.Store, client *fetch.Client, now clock.Clock, logger *slog.Logger, interval time.Duration) *Service {
	if interval <= 0 {
		interval = pullpolicy.DefaultInterval
	}
	return &Service{store: db, client: client, clock: now, logger: logger, interval: interval}
}

// Subscribe validates an address, discovering the Feed on it when the address is
// a web page rather than a Feed, saves the Feed, and stores what it carries.
// title, once trimmed, becomes the Feed's stored title; an empty or
// all-whitespace title falls back to the publisher's own title.
func (s *Service) Subscribe(ctx context.Context, rawURL string, title string) (store.Feed, error) {
	target, err := fetch.ParseURL(strings.TrimSpace(rawURL))
	if err != nil {
		return store.Feed{}, err
	}

	resp, err := s.get(ctx, target.String())
	if err != nil {
		return store.Feed{}, err
	}

	document, err := feed.Parse(resp.Body, resp.URL)
	feedURL := resp.URL.String()
	if err != nil {
		// Not a Feed: it may still be a page that advertises one.
		discovered := feed.Discover(resp.Body, resp.URL)
		if len(discovered) == 0 {
			return store.Feed{}, ErrNoFeed
		}
		feedURL = discovered[0]
		if resp, err = s.get(ctx, feedURL); err != nil {
			return store.Feed{}, err
		}
		if document, err = feed.Parse(resp.Body, resp.URL); err != nil {
			return store.Feed{}, ErrNoFeed
		}
		feedURL = resp.URL.String()
	}

	now := s.clock.Now()
	resolvedTitle := strings.TrimSpace(title)
	if resolvedTitle == "" {
		resolvedTitle = feedTitle(document, feedURL)
	}
	saved, err := s.store.CreateFeed(ctx, store.Feed{
		URL:     feedURL,
		Title:   resolvedTitle,
		SiteURL: document.SiteURL,
	}, now)
	if err != nil {
		return store.Feed{}, err
	}

	if err := s.store.SaveEntries(ctx, saved.ID, entriesOf(document, now), now); err != nil {
		return store.Feed{}, err
	}
	s.recordSuccess(ctx, saved.ID, store.Feed{}, resp.Header, now)
	s.discoverIcon(ctx, saved.ID, document.SiteURL, now)
	if saved, err = s.store.Feed(ctx, saved.ID); err != nil {
		return store.Feed{}, err
	}
	return saved, nil
}

// SubscribeOutcome is what came of one address OPML import asked to
// subscribe to: the Feed on success, or the error Subscribe returned.
type SubscribeOutcome struct {
	Feed store.Feed
	Err  error
}

// SubscribeMany subscribes to a set of addresses concurrently, bounded by
// concurrency, and returns one SubscribeOutcome per address in the same
// order — so a slow or dead publisher among hundreds an OPML import names
// does not hold the whole import open, the way refreshMany already bounds
// refreshing many Feeds at once.
func (s *Service) SubscribeMany(ctx context.Context, urls []string) []SubscribeOutcome {
	results := make([]SubscribeOutcome, len(urls))
	var wait sync.WaitGroup
	slots := make(chan struct{}, concurrency)
	for i, url := range urls {
		wait.Add(1)
		go func(i int, url string) {
			defer wait.Done()
			slots <- struct{}{}
			defer func() { <-slots }()

			feed, err := s.Subscribe(ctx, url, "")
			results[i] = SubscribeOutcome{Feed: feed, Err: err}
		}(i, url)
	}
	wait.Wait()
	return results
}

// Refresh re-reads one Feed, whatever the schedule would have said.
func (s *Service) Refresh(ctx context.Context, feedID int64) error {
	subscribed, err := s.store.Feed(ctx, feedID)
	if err != nil {
		return err
	}
	return s.refresh(ctx, subscribed)
}

// RefreshAll re-reads every Feed, a bounded number at a time, and returns the
// ones that failed. A Feed that fails does not stop the others. It reads every
// Feed regardless of schedule, because "refresh" means refresh.
func (s *Service) RefreshAll(ctx context.Context) (map[int64]error, error) {
	feeds, err := s.store.Feeds(ctx)
	if err != nil {
		return nil, err
	}
	return s.refreshMany(ctx, feeds), nil
}

// PollDue re-reads every Feed whose schedule says it is due. This is what the
// background schedule calls; RefreshAll is what a reader's own "refresh
// everything" asks for, and does not wait for the schedule.
func (s *Service) PollDue(ctx context.Context) (map[int64]error, error) {
	due, err := s.store.DueFeeds(ctx, s.clock.Now())
	if err != nil {
		return nil, err
	}
	return s.refreshMany(ctx, due), nil
}

// Run polls due Feeds on a schedule until ctx is cancelled. It is the only
// place a Feed is checked without the reader asking. tick sets how often the
// schedule wakes to look for a due Feed; a non-positive tick falls back to
// defaultTick.
func (s *Service) Run(ctx context.Context, tick time.Duration) {
	if tick <= 0 {
		tick = defaultTick
	}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.PollDue(ctx); err != nil {
				s.logger.ErrorContext(ctx, "poll due feeds", "error", err)
			}
		}
	}
}

// refreshMany re-reads a set of Feeds concurrently, bounded by concurrency,
// and returns the ones that failed.
func (s *Service) refreshMany(ctx context.Context, feeds []store.Feed) map[int64]error {
	var (
		mu       sync.Mutex
		failures = make(map[int64]error)
		wait     sync.WaitGroup
		slots    = make(chan struct{}, concurrency)
	)
	for _, subscribed := range feeds {
		wait.Add(1)
		go func() {
			defer wait.Done()
			slots <- struct{}{}
			defer func() { <-slots }()

			if err := s.refresh(ctx, subscribed); err != nil {
				s.logger.WarnContext(ctx, "refresh feed",
					"feed", subscribed.ID, "url", subscribed.URL, "error", err)
				mu.Lock()
				failures[subscribed.ID] = err
				mu.Unlock()
			}
		}()
	}
	wait.Wait()
	return failures
}

// refresh re-reads one Feed, sending whatever validators it holds from a
// previous check so an unchanged Feed can be confirmed without resending its
// whole document, and records the outcome — success or failure — as this
// Feed's new fetch state.
func (s *Service) refresh(ctx context.Context, subscribed store.Feed) error {
	resp, err := s.client.Get(ctx, subscribed.URL, fetch.Conditional{
		ETag:         subscribed.ETag,
		LastModified: subscribed.LastModified,
	})
	now := s.clock.Now()
	if err != nil {
		s.recordFailure(ctx, subscribed, err.Error(), nil, now)
		return &FetchError{URL: subscribed.URL, Err: err}
	}

	if resp.StatusCode == http.StatusNotModified {
		// Nothing to reparse: the publisher confirmed this Feed is unchanged,
		// which is what a conditional request is for.
		s.recordSuccess(ctx, subscribed.ID, subscribed, resp.Header, now)
		s.maybeDiscoverIcon(ctx, subscribed, now)
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		cause := fmt.Errorf("the publisher answered %d", resp.StatusCode)
		s.recordFailure(ctx, subscribed, cause.Error(), resp.Header, now)
		return &FetchError{URL: subscribed.URL, Err: cause}
	}

	document, err := feed.Parse(resp.Body, resp.URL)
	if err != nil {
		cause := fmt.Errorf("%s: %w", subscribed.URL, err)
		s.recordFailure(ctx, subscribed, cause.Error(), resp.Header, now)
		return cause
	}

	if err := s.store.SaveEntries(ctx, subscribed.ID, entriesOf(document, now), now); err != nil {
		return err
	}
	s.recordSuccess(ctx, subscribed.ID, subscribed, resp.Header, now)
	s.maybeDiscoverIcon(ctx, subscribed, now)
	return nil
}

// maybeDiscoverIcon runs icon discovery for a Feed that just polled
// successfully, when it is due per iconDue. It never affects whether the
// poll itself succeeded: it runs after the fetch state is already recorded,
// and every path inside discoverIcon is best-effort.
func (s *Service) maybeDiscoverIcon(ctx context.Context, subscribed store.Feed, now time.Time) {
	if iconDue(subscribed, now) {
		s.discoverIcon(ctx, subscribed.ID, subscribed.SiteURL, now)
	}
}

// recordSuccess stores the fetch state after a check that succeeded, whether
// or not the Feed had changed. A validator the response did not repeat (a 304
// commonly omits both) is kept from the previous check rather than erased.
func (s *Service) recordSuccess(ctx context.Context, feedID int64, previous store.Feed, header http.Header, now time.Time) {
	etag := header.Get("ETag")
	if etag == "" {
		etag = previous.ETag
	}
	lastModified := header.Get("Last-Modified")
	if lastModified == "" {
		lastModified = previous.LastModified
	}

	next := pullpolicy.NextCheck(now, s.interval, pullpolicy.HintsFromHeader(header, now))
	if err := s.store.RecordFetchResult(ctx, feedID, store.FetchResult{
		ETag:         etag,
		LastModified: lastModified,
		Success:      true,
		NextCheckAt:  next,
	}, now); err != nil {
		s.logger.WarnContext(ctx, "record fetch success", "feed", feedID, "error", err)
	}
}

// recordFailure stores the fetch state after a check that failed. header is
// nil when the failure happened before a response arrived at all.
func (s *Service) recordFailure(ctx context.Context, previous store.Feed, message string, header http.Header, now time.Time) {
	hints := pullpolicy.Hints{RetryAfter: pullpolicy.HintsFromHeader(header, now).RetryAfter}
	failures := previous.ConsecutiveFailures + 1
	next := pullpolicy.NextCheckAfterFailure(now, s.interval, hints, failures)
	if err := s.store.RecordFetchResult(ctx, previous.ID, store.FetchResult{
		ETag:                previous.ETag,
		LastModified:        previous.LastModified,
		Success:             false,
		Error:               message,
		ConsecutiveFailures: failures,
		NextCheckAt:         next,
	}, now); err != nil {
		s.logger.WarnContext(ctx, "record fetch failure", "feed", previous.ID, "error", err)
	}
}

// get fetches a document unconditionally, treating a refusal from the
// publisher as a failure to fetch rather than as content. It is used only by
// Subscribe, which holds no validators yet.
func (s *Service) get(ctx context.Context, rawURL string) (*fetch.Response, error) {
	resp, err := s.client.Get(ctx, rawURL, fetch.Conditional{})
	if err != nil {
		return nil, &FetchError{URL: rawURL, Err: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &FetchError{URL: rawURL, Err: fmt.Errorf("the publisher answered %d", resp.StatusCode)}
	}
	return resp, nil
}

// feedTitle is the publisher's own name for the Feed, falling back to its
// address so that no Feed is nameless in the sidebar.
func feedTitle(document feed.Document, feedURL string) string {
	if title := strings.TrimSpace(document.Title); title != "" {
		return title
	}
	return feedURL
}

// entriesOf converts a parsed document into Entries. An item the publisher gave
// no identifier and no link is dropped: there would be no way to recognise it
// again, and it would arrive afresh on every fetch.
func entriesOf(document feed.Document, now time.Time) []store.Entry {
	entries := make([]store.Entry, 0, len(document.Items))
	for _, item := range document.Items {
		if item.ID == "" {
			continue
		}
		published := item.PublishedAt
		if published.IsZero() {
			// A publisher who dates nothing still gets an ordering: when we
			// first saw the item.
			published = now
		}
		entries = append(entries, store.Entry{
			GUID:        item.ID,
			Title:       item.Title,
			URL:         item.URL,
			Content:     item.Content,
			PublishedAt: published,
		})
	}
	return entries
}
