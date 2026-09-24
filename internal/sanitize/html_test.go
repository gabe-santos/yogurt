package sanitize_test

import (
	"strings"
	"testing"

	"github.com/gabe-santos/yogurt/internal/sanitize"
)

// TestHTMLDemotesHeadings confirms a publisher's own h1-h6 in Feed content
// never survive into Feed View, so the page's single h1 stays the Collection
// title and the Entry title stays its only h2: see issue #37.
func TestHTMLDemotesHeadings(t *testing.T) {
	raw := `<h1>A Feed heading</h1><p>Body text.</p>`
	got := sanitize.HTML(raw)

	if strings.Contains(got, "<h1") {
		t.Errorf("HTML(%q) = %q, still carries an h1", raw, got)
	}
	if want := "<h2>A Feed heading</h2>"; !strings.Contains(got, want) {
		t.Errorf("HTML(%q) = %q, want the heading demoted to %q", raw, got, want)
	}
}
