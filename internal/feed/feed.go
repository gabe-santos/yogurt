// Package feed reads Feed documents. It parses the three forms a publisher may
// serve — RSS 2.0, Atom and JSON Feed — into one shape, and finds the Feed a web
// page advertises.
package feed

import (
	"bytes"
	"errors"
	"net/url"
	"strings"
	"time"
)

// ErrNotAFeed reports a document that is not a Feed in any form this package
// reads.
var ErrNotAFeed = errors.New("not a Feed")

// Document is a parsed Feed: the publisher's own name for it, the page it
// belongs to, and the items it carried.
type Document struct {
	Title   string
	SiteURL string
	Items   []Item
}

// Item is one item of a Feed, as the publisher supplied it.
type Item struct {
	// ID is the publisher's identifier for the item: an RSS guid, an Atom id or
	// a JSON Feed id, falling back to the item's own link.
	ID          string
	Title       string
	URL         string
	Content     string
	PublishedAt time.Time
}

// Parse reads a Feed document. Relative links are resolved against base, which
// is the address the document was fetched from.
func Parse(body []byte, base *url.URL) (Document, error) {
	trimmed := trimLeadingSpace(body)
	if len(trimmed) == 0 {
		return Document{}, ErrNotAFeed
	}
	if trimmed[0] == '{' {
		return parseJSONFeed(trimmed, base)
	}
	return parseXML(trimmed, base)
}

// trimLeadingSpace drops whitespace and a UTF-8 byte-order mark, so that
// sniffing sees the document's first real byte.
func trimLeadingSpace(body []byte) []byte {
	return bytes.TrimLeft(bytes.TrimPrefix(body, []byte("\uFEFF")), " \t\r\n")
}

// Resolve turns a link the publisher supplied into an absolute URL, dropping
// anything that is neither. Shared by Feed parsing and Feed Icon discovery,
// which both resolve publisher-supplied links against the document's own
// address.
func Resolve(base *url.URL, link string) string {
	link = strings.TrimSpace(link)
	if link == "" {
		return ""
	}
	parsed, err := url.Parse(link)
	if err != nil {
		return ""
	}
	if base != nil {
		parsed = base.ResolveReference(parsed)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return parsed.String()
}

// firstNonEmpty is the first of its arguments with something in it.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
