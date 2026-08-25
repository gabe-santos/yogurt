package sanitize

import (
	"bytes"
	"net/url"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// articlePolicy is the allowlist extracted Article HTML is run through. It
// starts from the same UGC allowlist as Entry content, plus the
// "referrerpolicy" attribute on images that Article extracts adds to protect
// the reader's IP from the publisher's image host.
var articlePolicy = newArticlePolicy()

func newArticlePolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("referrerpolicy").OnElements("img")
	return p
}

// Article cleans HTML this application extracted from a publisher's page
// before it is stored and rendered: dangerous markup is stripped, one-pixel
// tracking images are removed, every remaining image carries a no-referrer
// policy, and an insecure image source is upgraded to https or, when that is
// not possible, dropped.
func Article(raw string) string {
	return articlePolicy.Sanitize(rewriteImages(raw))
}

// rewriteImages walks the parsed document, dropping tracking pixels and
// fixing up the src and referrerpolicy of every image that remains. It runs
// before the allowlist sanitiser so that the sanitiser has the final say on
// what survives.
func rewriteImages(raw string) string {
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(raw), body)
	if err != nil {
		// Malformed input is left for the allowlist sanitiser to deal with; it
		// strips anything it cannot make sense of.
		return raw
	}
	// ParseFragment does not itself attach the nodes it returns to the context
	// node; doing so here gives every node, including a top-level image, a
	// Parent to remove itself from.
	for _, n := range nodes {
		body.AppendChild(n)
	}

	cleanImages(body)

	var buf bytes.Buffer
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		_ = html.Render(&buf, c)
	}
	return buf.String()
}

// cleanImages walks n and its descendants, removing tracking pixels and
// rewriting the rest in place.
func cleanImages(n *html.Node) {
	var next *html.Node
	for c := n.FirstChild; c != nil; c = next {
		next = c.NextSibling
		cleanImages(c)
	}

	if n.Type != html.ElementNode || n.Data != "img" {
		return
	}
	if isTrackingPixel(n) {
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
		return
	}
	rewriteImageSrc(n)
	setAttr(n, "referrerpolicy", "no-referrer")
}

// isTrackingPixel reports an image declared exactly 1x1, the standard shape
// of a tracking beacon that carries no content of its own.
func isTrackingPixel(img *html.Node) bool {
	return attr(img, "width") == "1" && attr(img, "height") == "1"
}

// rewriteImageSrc upgrades an insecure http source to https. A source this
// application cannot make sense of as a URL is dropped rather than left
// insecure or broken.
func rewriteImageSrc(img *html.Node) {
	src := attr(img, "src")
	if src == "" {
		return
	}
	parsed, err := url.Parse(src)
	if err != nil {
		removeAttr(img, "src")
		return
	}
	if parsed.Scheme == "http" {
		parsed.Scheme = "https"
		setAttr(img, "src", parsed.String())
	}
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func setAttr(n *html.Node, key, value string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr[i].Val = value
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: value})
}

func removeAttr(n *html.Node, key string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr = append(n.Attr[:i], n.Attr[i+1:]...)
			return
		}
	}
}
