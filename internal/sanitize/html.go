// Package sanitize cleans HTML a publisher supplied before this application
// renders it, so that a hostile Feed cannot attack the reader's browser.
package sanitize

import "github.com/microcosm-cc/bluemonday"

// policy is the allowlist every piece of Feed- or Article-supplied HTML is run
// through: the common formatting, list, table and image elements a publisher
// legitimately uses, with scripts, styles, forms, iframes and event handlers
// always stripped.
var policy = bluemonday.UGCPolicy()

// HTML strips dangerous markup from raw, publisher-supplied HTML, returning
// only what is safe to render.
func HTML(raw string) string {
	return policy.Sanitize(raw)
}
