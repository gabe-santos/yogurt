// Package pull subscribes to Feeds and reads them: fetch, parse, and store what
// the publisher carried. It is the only place that turns a Feed document into
// Entries.
package pull

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gabe-santos/rss-reader/internal/clock"
	"github.com/gabe-santos/rss-reader/internal/feed"
	"github.com/gabe-santos/rss-reader/internal/fetch"
	"github.com/gabe-santos/rss-reader/internal/store"
)

// concurrency bounds how many Feeds are fetched at once, so that refreshing
// hundreds of Feeds is quick without opening hundreds of connections.
const concurrency = 8

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

// Service subscribes to Feeds and refreshes them.
type Service struct {
	store  *store.Store
	client *fetch.Client
	clock  clock.Clock
	logger *slog.Logger
}

// New wires a pull service.
func New(db *store.Store, client *fetch.Client, now clock.Clock, logger *slog.Logger) *Service {
	return &Service{store: db, client: client, clock: now, logger: logger}
}

// Subscribe validates an address, discovering the Feed on it when the address is
// a web page rather than a Feed, saves the Feed, and stores what it carries.
func (s *Service) Subscribe(ctx context.Context, rawURL string) (store.Feed, error) {
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
	saved, err := s.store.CreateFeed(ctx, store.Feed{
		URL:     feedURL,
		Title:   feedTitle(document, feedURL),
		SiteURL: document.SiteURL,
	}, now)
	if err != nil {
		return store.Feed{}, err
	}

	if err := s.store.SaveEntries(ctx, saved.ID, entriesOf(document, now), now); err != nil {
		return store.Feed{}, err
	}
	return saved, nil
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
// ones that failed. A Feed that fails does not stop the others.
func (s *Service) RefreshAll(ctx context.Context) (map[int64]error, error) {
	feeds, err := s.store.Feeds(ctx)
	if err != nil {
		return nil, err
	}

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

	return failures, nil
}

func (s *Service) refresh(ctx context.Context, subscribed store.Feed) error {
	resp, err := s.get(ctx, subscribed.URL)
	if err != nil {
		return err
	}
	document, err := feed.Parse(resp.Body, resp.URL)
	if err != nil {
		return fmt.Errorf("%s: %w", subscribed.URL, err)
	}

	now := s.clock.Now()
	return s.store.SaveEntries(ctx, subscribed.ID, entriesOf(document, now), now)
}

// get fetches a document, treating a refusal from the publisher as a failure to
// fetch rather than as content.
func (s *Service) get(ctx context.Context, rawURL string) (*fetch.Response, error) {
	resp, err := s.client.Get(ctx, rawURL)
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
