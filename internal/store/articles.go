package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNoArticle reports an Article that has not been extracted yet.
var ErrNoArticle = errors.New("no such Article")

// Article is a publisher's page, reduced to its main text by extraction and
// kept by URL: extraction.Service produces the content, and the store only
// persists it. Embeddable and FetchedAt come from the same fetch that
// extracted HTML, per ADR-0003.
type Article struct {
	URL        string
	Title      string
	HTML       string
	Embeddable bool
	FetchedAt  time.Time
}

// Article reads a previously extracted Article by its URL. It returns
// ErrNoArticle when this URL has not been extracted yet.
func (s *Store) Article(ctx context.Context, url string) (Article, error) {
	var article Article
	var embeddable int64
	var fetchedAt int64
	err := s.db.QueryRowContext(ctx,
		`SELECT url, title, html, embeddable, fetched_at FROM articles WHERE url = ?`, url,
	).Scan(&article.URL, &article.Title, &article.HTML, &embeddable, &fetchedAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Article{}, ErrNoArticle
	case err != nil:
		return Article{}, fmt.Errorf("read article %s: %w", url, err)
	}
	article.Embeddable = embeddable != 0
	article.FetchedAt = time.Unix(fetchedAt, 0).UTC()
	return article, nil
}

// SaveArticle stores an Article extraction result, replacing whatever this
// URL held before: a reader who asks for Reader View again after this
// application starts recognising more, or fixing a bug in extraction, gets
// the improved result rather than being stuck with the first one forever.
func (s *Store) SaveArticle(ctx context.Context, article Article, now time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO articles (url, title, html, embeddable, fetched_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (url) DO UPDATE SET
			title = excluded.title, html = excluded.html,
			embeddable = excluded.embeddable, fetched_at = excluded.fetched_at`,
		article.URL, article.Title, article.HTML, boolToInt(article.Embeddable), now.Unix())
	if err != nil {
		return fmt.Errorf("save article %s: %w", article.URL, err)
	}
	return nil
}
