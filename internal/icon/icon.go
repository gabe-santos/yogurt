// Package icon chooses and normalizes a publisher site's Feed Icon. Given a
// site's home-page HTML and a way to fetch URLs, it decides which declared
// icon link (or, failing that, OpenGraph image) should become the Feed Icon,
// fetches it, and returns it ready to store — or reports that the site has
// none.
//
// Icons are found by reading the site's declared icon links, never by
// guessing conventional paths: a bare /favicon.ico was measured returning 404
// on two of the three subscribed sites at the time this package was written.
// See ADR-0008.
package icon

import (
	"bytes"
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"net/url"
	"strconv"
	"strings"

	"github.com/gabe-santos/rss-reader/internal/feed"
	"golang.org/x/net/html"
)

// MaxBodySize is the largest response this package will read for an icon
// candidate or the OpenGraph fallback image. A 170x170, 58KB PNG has already
// been measured as one site's only icon; 256KB leaves headroom above that
// while still refusing anything trying to be an unbounded download.
const MaxBodySize = 256 << 10

// targetSize is the edge length, in pixels, a Feed Icon is downscaled to. A
// Feed Icon renders at roughly 16px; 32px keeps it sharp on high-density
// displays without shipping a publisher's full-size asset.
const targetSize = 32

// Icon is a Feed Icon ready to store.
type Icon struct {
	// Data is the icon's bytes: byte-for-byte as fetched for SVG and ICO,
	// re-encoded as PNG when a raster was downscaled.
	Data []byte
	// MediaType is Data's content type, for the serving endpoint's
	// Content-Type header.
	MediaType string
}

// Fetcher retrieves a URL's body. icon.Choose never talks to the network
// itself, so tests can exercise it against a fake with no real network
// access; production wires it to internal/fetch, which applies the
// private-network guard.
type Fetcher interface {
	Fetch(ctx context.Context, rawURL string) ([]byte, error)
}

// Choose reads a site's home-page HTML and decides which image should become
// its Feed Icon. base is the address the HTML was fetched from, used to
// resolve relative links. It returns the icon and true, or false when the
// site has no usable icon — malformed HTML, missing icon links, a candidate
// that cannot be fetched, and an undecodable image all produce false rather
// than an error, since none of them should stop a Feed from being subscribed
// or polled.
func Choose(ctx context.Context, fetcher Fetcher, siteHTML []byte, base *url.URL) (Icon, bool) {
	links := parseLinks(siteHTML, base)

	for _, candidate := range links.svg {
		if icon, ok := fetchPassthrough(ctx, fetcher, candidate, "image/svg+xml"); ok {
			return icon, true
		}
	}
	for _, candidate := range links.raster {
		if icon, ok := fetchRaster(ctx, fetcher, candidate); ok {
			return icon, true
		}
	}
	for _, candidate := range links.ico {
		if icon, ok := fetchPassthrough(ctx, fetcher, candidate, "image/x-icon"); ok {
			return icon, true
		}
	}
	if links.openGraph != "" {
		if icon, ok := fetchRaster(ctx, fetcher, links.openGraph); ok {
			return icon, true
		}
	}
	return Icon{}, false
}

// rasterCandidate is a raster icon link, kept with its declared size so
// candidates with a known small size can be preferred over a much larger one
// without fetching every candidate first.
type rasterCandidate struct {
	href string
	// size is the declared width in pixels from a sizes="WxH" attribute, or 0
	// when the site declared none.
	size int
}

// linkSet is every icon-related link a site's home page declared, in
// selection order: SVG first, then raster (smallest declared size first),
// then ICO, with the OpenGraph image kept aside as the last resort.
type linkSet struct {
	svg       []string
	raster    []string
	ico       []string
	openGraph string
}

// parseLinks reads a site's declared icon links and its OpenGraph image,
// resolving every href against base. Malformed HTML tokenizes as best it can
// and simply yields fewer links; it is never an error.
func parseLinks(siteHTML []byte, base *url.URL) linkSet {
	tokenizer := html.NewTokenizer(bytes.NewReader(siteHTML))

	var rasters []rasterCandidate
	var set linkSet
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			set.raster = rankRasters(rasters)
			return set
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := tokenizer.TagName()
			if !hasAttr {
				continue
			}
			switch string(name) {
			case "link":
				rel, linkType, href, sizes := readLinkAttrs(tokenizer)
				if !feed.HasRelToken(rel, "icon") {
					continue
				}
				resolved := feed.Resolve(base, href)
				if resolved == "" {
					continue
				}
				switch classify(linkType, resolved) {
				case kindSVG:
					set.svg = append(set.svg, resolved)
				case kindICO:
					set.ico = append(set.ico, resolved)
				default:
					rasters = append(rasters, rasterCandidate{href: resolved, size: parseSize(sizes)})
				}
			case "meta":
				property, content := readMetaAttrs(tokenizer)
				if set.openGraph == "" && property == "og:image" {
					set.openGraph = feed.Resolve(base, content)
				}
			}
		}
	}
}

// readLinkAttrs reads the attributes Feed Icon discovery cares about off a
// <link> tag the tokenizer is positioned on.
func readLinkAttrs(tokenizer *html.Tokenizer) (rel, linkType, href, sizes string) {
	for {
		key, value, more := tokenizer.TagAttr()
		switch string(key) {
		case "rel":
			rel = string(value)
		case "type":
			linkType = string(value)
		case "href":
			href = string(value)
		case "sizes":
			sizes = string(value)
		}
		if !more {
			return
		}
	}
}

// readMetaAttrs reads the attributes Feed Icon discovery cares about off a
// <meta> tag the tokenizer is positioned on.
func readMetaAttrs(tokenizer *html.Tokenizer) (property, content string) {
	for {
		key, value, more := tokenizer.TagAttr()
		switch string(key) {
		case "property":
			property = string(value)
		case "content":
			content = string(value)
		}
		if !more {
			return
		}
	}
}

type kind int

const (
	kindRaster kind = iota
	kindSVG
	kindICO
)

// classify decides what an icon link is, from its declared type first and its
// href's extension when the type is absent or generic.
func classify(linkType, href string) kind {
	switch feed.NormalizeType(linkType) {
	case "image/svg+xml":
		return kindSVG
	case "image/x-icon", "image/vnd.microsoft.icon":
		return kindICO
	}
	path := href
	if idx := strings.IndexAny(path, "?#"); idx >= 0 {
		path = path[:idx]
	}
	switch {
	case strings.HasSuffix(path, ".svg"):
		return kindSVG
	case strings.HasSuffix(path, ".ico"):
		return kindICO
	default:
		return kindRaster
	}
}

// parseSize reads the first WxH pair from a sizes attribute (e.g. "32x32", or
// "16x16 32x32" for multiple), returning 0 when it declares none or "any".
func parseSize(sizes string) int {
	first := strings.Fields(sizes)
	if len(first) == 0 {
		return 0
	}
	dims := strings.SplitN(strings.ToLower(first[0]), "x", 2)
	if len(dims) != 2 {
		return 0
	}
	width, err := strconv.Atoi(dims[0])
	if err != nil || width <= 0 {
		return 0
	}
	return width
}

// rankRasters orders raster candidates so the smallest declared size that is
// still at least targetSize is tried first — a Feed Icon renders sharpest
// from a source no smaller than its target, so a site declaring both 16x16
// and 32x32 must yield the 32, not the smaller one. Candidates declared
// smaller than targetSize follow, smallest first, as a fallback when nothing
// large enough exists. Candidates with no declared size are tried last, in
// document order, since an unknown size might turn out to be far larger than
// any declared one.
func rankRasters(candidates []rasterCandidate) []string {
	var atLeastTarget, belowTarget []rasterCandidate
	var unknown []string
	for _, c := range candidates {
		switch {
		case c.size >= targetSize:
			atLeastTarget = append(atLeastTarget, c)
		case c.size > 0:
			belowTarget = append(belowTarget, c)
		default:
			unknown = append(unknown, c.href)
		}
	}
	sortBySizeAscending(atLeastTarget)
	sortBySizeAscending(belowTarget)

	ordered := make([]string, 0, len(candidates))
	for _, c := range atLeastTarget {
		ordered = append(ordered, c.href)
	}
	for _, c := range belowTarget {
		ordered = append(ordered, c.href)
	}
	return append(ordered, unknown...)
}

// sortBySizeAscending is a stable insertion sort: the candidate counts per
// site are tiny, and stability preserves document order among equal declared
// sizes.
func sortBySizeAscending(candidates []rasterCandidate) {
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && candidates[j].size < candidates[j-1].size; j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}
}

// fetchPassthrough fetches an SVG or ICO candidate and, when it is within the
// size cap, stores it byte-for-byte: there is no pure-Go SVG rasteriser and no
// standard-library ICO decoder, and browsers render both natively.
func fetchPassthrough(ctx context.Context, fetcher Fetcher, rawURL, mediaType string) (Icon, bool) {
	body, err := fetcher.Fetch(ctx, rawURL)
	if err != nil || len(body) == 0 || len(body) > MaxBodySize {
		return Icon{}, false
	}
	return Icon{Data: body, MediaType: mediaType}, true
}

// fetchRaster fetches a raster candidate, decodes it to learn its real
// dimensions and format, and downscales it when it exceeds targetSize. A
// raster already at or under targetSize is stored as fetched rather than
// re-encoded, since nothing needs to change about it.
func fetchRaster(ctx context.Context, fetcher Fetcher, rawURL string) (Icon, bool) {
	body, err := fetcher.Fetch(ctx, rawURL)
	if err != nil || len(body) == 0 || len(body) > MaxBodySize {
		return Icon{}, false
	}
	img, format, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return Icon{}, false
	}
	bounds := img.Bounds()
	if bounds.Dx() <= targetSize && bounds.Dy() <= targetSize {
		return Icon{Data: body, MediaType: rasterMediaType(format)}, true
	}
	scaled := downscale(img, targetSize)
	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return Icon{}, false
	}
	return Icon{Data: buf.Bytes(), MediaType: "image/png"}, true
}

// rasterMediaType maps the format image.Decode reports to the media type an
// unmodified raster is served with.
func rasterMediaType(format string) string {
	switch format {
	case "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	default:
		return "image/png"
	}
}
