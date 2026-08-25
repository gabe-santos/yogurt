// Package extraction fetches a publisher's own page for an Entry and reduces
// it to its main text and images: Reader View. Extraction happens on demand,
// the first time the reader asks for it, not on ingest — the fetch package
// already runs every outbound request through the same private-network guard
// Feed fetches use.
package extraction

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	readability "codeberg.org/readeck/go-readability/v2"

	"github.com/gabe-santos/rss-reader/internal/fetch"
	"github.com/gabe-santos/rss-reader/internal/sanitize"
)

// ErrNoContent reports a page with nothing extraction could recognise as an
// Article: too little text, or none at all.
var ErrNoContent = errors.New("no readable content found on that page")

// Accept is what a fetch.Client extracting Articles should ask for: an
// Entry's own page, not a Feed, so a publisher that content-negotiates on
// Accept should be offered HTML ahead of anything else.
const Accept = "text/html, application/xhtml+xml;q=0.9, */*;q=0.5"

// Article is what extraction produced from a publisher's page.
type Article struct {
	Title string
	// HTML is the reduced main text and images, already sanitised and safe to
	// render.
	HTML string
	// Embeddable reports whether the response permits this page to be shown
	// in an iframe, per ADR-0003. It is read once, from the same fetch that
	// extracted the Article, so the UI never has to discover this by watching
	// a frame fail.
	Embeddable bool
}

// Service extracts Articles from Entry links.
type Service struct {
	client *fetch.Client
}

// New wires an extraction service over a fetch Client.
func New(client *fetch.Client) *Service {
	return &Service{client: client}
}

// Extract fetches rawURL and reduces the response to an Article.
//
// Every error raised after the fetch itself succeeded — ErrNoContent among
// them — is returned together with an Article whose Embeddable flag is already
// meaningful, because Original View needs that flag for exactly the pages
// Reader View cannot read: the ones built by JavaScript, or carrying nothing
// but images and charts.
func (s *Service) Extract(ctx context.Context, rawURL string) (Article, error) {
	resp, err := s.client.Get(ctx, rawURL, fetch.Conditional{})
	if err != nil {
		return Article{}, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Article{}, fmt.Errorf("fetch %s: publisher returned status %d", rawURL, resp.StatusCode)
	}
	article := Article{Embeddable: canEmbed(resp.Header)}

	parsed, err := readability.FromReader(bytes.NewReader(resp.Body), resp.URL)
	if err != nil {
		return article, fmt.Errorf("extract %s: %w", rawURL, err)
	}
	if parsed.Node == nil {
		return article, fmt.Errorf("extract %s: %w", rawURL, ErrNoContent)
	}

	var buf bytes.Buffer
	if err := parsed.RenderHTML(&buf); err != nil {
		return article, fmt.Errorf("render %s: %w", rawURL, err)
	}

	article.Title = parsed.Title()
	article.HTML = sanitize.Article(buf.String())
	return article, nil
}

// canEmbed reports whether a response's framing headers permit this page to
// be embedded from an origin other than its own. X-Frame-Options has no value
// that permits a third-party origin, so its mere presence refuses embedding.
// A Content-Security-Policy frame-ancestors directive refuses unless it names
// the wildcard source; this application does not know in advance which origin
// a future consumer would embed from, so anything narrower is treated as a
// refusal.
func canEmbed(header http.Header) bool {
	if header.Get("X-Frame-Options") != "" {
		return false
	}

	// Multiple Content-Security-Policy headers are all enforced together, and
	// a single header may itself carry several comma-separated policies, so
	// every directive in every policy must be checked before this page can be
	// called embeddable.
	for _, policy := range header.Values("Content-Security-Policy") {
		for _, part := range strings.Split(policy, ",") {
			for _, directive := range strings.Split(part, ";") {
				fields := strings.Fields(directive)
				if len(fields) == 0 || !strings.EqualFold(fields[0], "frame-ancestors") {
					continue
				}
				if !slices.Contains(fields[1:], "*") {
					return false
				}
			}
		}
	}
	return true
}
