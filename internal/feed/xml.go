package feed

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/url"
	"strings"
)

// parseXML reads an RSS 2.0 or Atom document, deciding which by its root
// element.
func parseXML(body []byte, base *url.URL) (Document, error) {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	// Publishers are not careful: accept documents a strict parser would refuse,
	// and treat an unknown charset declaration as the UTF-8 it almost always is.
	decoder.Strict = false
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }

	for {
		token, err := decoder.Token()
		if err != nil {
			return Document{}, ErrNotAFeed
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}

		switch start.Name.Local {
		case "rss":
			var doc rssDocument
			if err := decoder.DecodeElement(&doc, &start); err != nil {
				return Document{}, ErrNotAFeed
			}
			return doc.document(base), nil
		case "feed":
			var doc atomDocument
			if err := decoder.DecodeElement(&doc, &start); err != nil {
				return Document{}, ErrNotAFeed
			}
			return doc.document(base), nil
		default:
			// Some other XML document: not a Feed we read.
			return Document{}, ErrNotAFeed
		}
	}
}

type rssDocument struct {
	Channel struct {
		Title string `xml:"title"`
		// A channel's own <link> is the site; an <atom:link> alongside it has no
		// text, hence the slice and the first-non-empty rule.
		Links []string  `xml:"link"`
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Date        string `xml:"http://purl.org/dc/elements/1.1/ date"`
	Description string `xml:"description"`
	Encoded     string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
}

func (d rssDocument) document(base *url.URL) Document {
	doc := Document{
		Title:   strings.TrimSpace(d.Channel.Title),
		SiteURL: resolve(base, firstNonEmpty(d.Channel.Links...)),
		Items:   make([]Item, 0, len(d.Channel.Items)),
	}
	for _, item := range d.Channel.Items {
		link := resolve(base, item.Link)
		doc.Items = append(doc.Items, Item{
			ID:          firstNonEmpty(item.GUID, link),
			Title:       strings.TrimSpace(item.Title),
			URL:         link,
			Content:     firstNonEmpty(item.Encoded, item.Description),
			PublishedAt: parseTime(firstNonEmpty(item.PubDate, item.Date)),
		})
	}
	return doc
}

type atomDocument struct {
	Title   atomText    `xml:"title"`
	Links   []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
	Href string `xml:"href,attr"`
}

type atomEntry struct {
	ID        string     `xml:"id"`
	Title     atomText   `xml:"title"`
	Links     []atomLink `xml:"link"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
	Content   atomText   `xml:"content"`
	Summary   atomText   `xml:"summary"`
}

// atomText is an Atom text construct, which carries either escaped text or
// inline XHTML depending on its type attribute.
type atomText struct {
	Type  string `xml:"type,attr"`
	Text  string `xml:",chardata"`
	Inner string `xml:",innerxml"`
}

func (t atomText) value() string {
	if t.Type == "xhtml" {
		return strings.TrimSpace(t.Inner)
	}
	return strings.TrimSpace(t.Text)
}

func (d atomDocument) document(base *url.URL) Document {
	doc := Document{
		Title:   d.Title.value(),
		SiteURL: resolve(base, alternateLink(d.Links)),
		Items:   make([]Item, 0, len(d.Entries)),
	}
	for _, entry := range d.Entries {
		link := resolve(base, alternateLink(entry.Links))
		doc.Items = append(doc.Items, Item{
			ID:          firstNonEmpty(entry.ID, link),
			Title:       entry.Title.value(),
			URL:         link,
			Content:     firstNonEmpty(entry.Content.value(), entry.Summary.value()),
			PublishedAt: parseTime(firstNonEmpty(entry.Published, entry.Updated)),
		})
	}
	return doc
}

// alternateLink is the human-readable page a set of Atom links points at.
func alternateLink(links []atomLink) string {
	for _, link := range links {
		if link.Rel == "" || link.Rel == "alternate" {
			return link.Href
		}
	}
	return ""
}
