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
// policy, an insecure image source is upgraded to https or, when that is
// not possible, dropped, and every heading is demoted one level — all in
// one parse and one render, rather than the two round trips running each
// step as its own pass would cost. Reader View nests an Article beneath the
// page's own h1 (the Collection) and h2 (the Entry title); left alone, a
// publisher's own <h1> inside the Article body would read as a second h1
// on the page.
func Article(raw string) string {
	rewritten := rewriteFragment(raw, func(n *html.Node) {
		cleanImages(n)
		lowerHeadings(n)
	})
	return articlePolicy.Sanitize(rewritten)
}

// rewriteFragment parses raw as an HTML fragment, runs walk over the parsed
// tree, and renders the result back to a string. Malformed input is
// returned unchanged, left for the allowlist sanitiser that follows to deal
// with — it strips anything it cannot make sense of.
func rewriteFragment(raw string, walk func(*html.Node)) string {
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(raw), body)
	if err != nil {
		return raw
	}
	// ParseFragment does not itself attach the nodes it returns to the context
	// node; doing so here gives every node, including a top-level image, a
	// Parent to remove itself from.
	for _, n := range nodes {
		body.AppendChild(n)
	}

	walk(body)

	var buf bytes.Buffer
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		_ = html.Render(&buf, c)
	}
	return buf.String()
}

// headingLevel maps each heading atom to its numeric level.
var headingLevel = map[atom.Atom]int{
	atom.H1: 1,
	atom.H2: 2,
	atom.H3: 3,
	atom.H4: 4,
	atom.H5: 5,
	atom.H6: 6,
}

// demotedHeading maps a heading level to the tag one level lower, capping at
// h6 rather than inventing a level HTML has no tag for.
var demotedHeading = map[int]atom.Atom{
	1: atom.H2,
	2: atom.H3,
	3: atom.H4,
	4: atom.H5,
	5: atom.H6,
	6: atom.H6,
}

// demoteHeadings demotes every heading in raw by one level: Feed View, like
// Reader View, nests a publisher's own markup beneath the page's own h1 (the
// Collection) and h2 (the Entry title), so neither may carry a heading of
// its own that outranks them.
func demoteHeadings(raw string) string {
	return rewriteFragment(raw, lowerHeadings)
}

// lowerHeadings walks n and its descendants, demoting every heading it finds
// in place.
func lowerHeadings(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		lowerHeadings(c)
	}
	if n.Type != html.ElementNode {
		return
	}
	level, ok := headingLevel[n.DataAtom]
	if !ok {
		return
	}
	demoted := demotedHeading[level]
	n.DataAtom = demoted
	n.Data = demoted.String()
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
