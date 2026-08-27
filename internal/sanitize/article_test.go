package sanitize_test

import (
	"strings"
	"testing"

	"github.com/gabe-santos/rss-reader/internal/sanitize"
)

// TestArticleDemotesHeadings confirms extraction's own h1-h6 never survive
// into Reader View, so the page's single h1 stays the Collection title and
// the Entry title stays its only h2: see issue #37.
func TestArticleDemotesHeadings(t *testing.T) {
	raw := `<h1>A Real Article</h1><p>First paragraph.</p><h2>A section</h2><p>More.</p><h3>A subsection</h3><h6>Already at the floor</h6>`
	got := sanitize.Article(raw)

	if strings.Contains(got, "<h1") {
		t.Errorf("Article(%q) = %q, still carries an h1", raw, got)
	}
	for tag, want := range map[string]string{
		"A Real Article":       "<h2>A Real Article</h2>",
		"A section":            "<h3>A section</h3>",
		"A subsection":         "<h4>A subsection</h4>",
		"Already at the floor": "<h6>Already at the floor</h6>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Article(%q) = %q, want %q containing demoted %q", raw, got, want, tag)
		}
	}
}
