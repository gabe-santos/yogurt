package feed

import (
	"bytes"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// feedTypes are the content types a page uses to advertise a Feed.
var feedTypes = map[string]bool{
	"application/rss+xml":   true,
	"application/atom+xml":  true,
	"application/feed+json": true,
	"application/json":      true,
	"application/xml":       true,
	"text/xml":              true,
}

// Discover finds the Feeds a web page advertises, in the order the page lists
// them, resolved against base. A page advertising none yields none.
func Discover(body []byte, base *url.URL) []string {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))

	var found []string
	seen := make(map[string]bool)
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return found
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := tokenizer.TagName()
			if string(name) != "link" || !hasAttr {
				continue
			}
			var rel, linkType, href string
			for {
				key, value, more := tokenizer.TagAttr()
				switch string(key) {
				case "rel":
					rel = string(value)
				case "type":
					linkType = string(value)
				case "href":
					href = string(value)
				}
				if !more {
					break
				}
			}
			if !HasRelToken(rel, "alternate") || !feedTypes[NormalizeType(linkType)] {
				continue
			}
			if resolved := Resolve(base, href); resolved != "" && !seen[resolved] {
				seen[resolved] = true
				found = append(found, resolved)
			}
		}
	}
}

// HasRelToken reports whether a link's rel attribute carries token, per the
// space-separated link-types syntax HTML uses for rel. It is shared by Feed
// autodiscovery (token "alternate") and Feed Icon discovery (token "icon").
func HasRelToken(rel, token string) bool {
	for _, value := range strings.Fields(strings.ToLower(rel)) {
		if value == token {
			return true
		}
	}
	return false
}

// NormalizeType drops the parameters and case from a content type, leaving
// the media type itself.
func NormalizeType(value string) string {
	if index := strings.IndexByte(value, ';'); index >= 0 {
		value = value[:index]
	}
	return strings.ToLower(strings.TrimSpace(value))
}
