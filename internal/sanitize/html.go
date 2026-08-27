// Package sanitize cleans HTML a publisher supplied before this application
// renders it, so that a hostile Feed cannot attack the reader's browser.
package sanitize

import (
	"html"

	"github.com/microcosm-cc/bluemonday"
)

// policy is the allowlist every piece of Feed- or Article-supplied HTML is run
// through: the common formatting, list, table and image elements a publisher
// legitimately uses, with scripts, styles, forms, iframes and event handlers
// always stripped.
var policy = bluemonday.UGCPolicy()

// textPolicy strips every tag, keeping only the text a publisher wrote.
var textPolicy = bluemonday.StrictPolicy()

// HTML strips dangerous markup from raw, publisher-supplied HTML, returning
// only what is safe to render, with every heading demoted one level.
// Feed View, like Reader View, nests a publisher's own markup beneath the
// page's own h1 (the Collection) and h2 (the Entry title): see issue #37.
func HTML(raw string) string {
	return policy.Sanitize(demoteHeadings(raw))
}

// PlainText reduces raw, publisher-supplied HTML to plain text: every tag is
// stripped and HTML entities are resolved, so a fragment can stand alone —
// a search result's excerpt, cut from wherever it matched and possibly
// mid-tag — without leaking markup.
func PlainText(raw string) string {
	return html.UnescapeString(textPolicy.Sanitize(raw))
}
