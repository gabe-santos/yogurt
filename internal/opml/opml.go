// Package opml reads and writes the reader's Feed collection as OPML, the
// format most Feed readers use to exchange subscription lists. It knows
// nothing of the store or the application; it only turns an OPML document
// into the Feed addresses it names, and Feeds into an OPML document.
package opml

import (
	"encoding/xml"
	"io"
	"strings"
)

// document, body, and outline mirror only the parts of the OPML/XML outline
// format this application reads: a Feed is any outline that carries an
// xmlUrl, and a folder is any outline that does not.
type document struct {
	XMLName xml.Name `xml:"opml"`
	Body    xmlBody  `xml:"body"`
}

type xmlBody struct {
	Outlines []inOutline `xml:"outline"`
}

type inOutline struct {
	XMLURL   string      `xml:"xmlUrl,attr"`
	Outlines []inOutline `xml:"outline"`
}

// Parse reads an OPML document and returns the Feed addresses it names, in
// document order. A Feed nested inside folders is returned the same as one
// at the document's top level: this application does not sort Feeds into
// folders, so how an OPML document organised them is not preserved.
func Parse(r io.Reader) ([]string, error) {
	decoder := xml.NewDecoder(r)
	// Publishers and other readers are not careful producing OPML either:
	// accept documents a strict parser would refuse.
	decoder.Strict = false
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }

	var doc document
	if err := decoder.Decode(&doc); err != nil {
		return nil, err
	}

	var urls []string
	flatten(doc.Body.Outlines, &urls)
	return urls, nil
}

func flatten(outlines []inOutline, urls *[]string) {
	for _, o := range outlines {
		url := strings.TrimSpace(o.XMLURL)
		if url != "" {
			*urls = append(*urls, url)
			continue
		}
		flatten(o.Outlines, urls)
	}
}

// FeedExport is one Feed as an exported OPML document names it.
type FeedExport struct {
	Title   string
	XMLURL  string
	HTMLURL string
}

type outDocument struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    outHead  `xml:"head"`
	Body    outBody  `xml:"body"`
}

type outHead struct {
	Title string `xml:"title"`
}

type outBody struct {
	Outlines []outOutline `xml:"outline"`
}

type outOutline struct {
	Text    string `xml:"text,attr"`
	Title   string `xml:"title,attr,omitempty"`
	Type    string `xml:"type,attr,omitempty"`
	XMLURL  string `xml:"xmlUrl,attr,omitempty"`
	HTMLURL string `xml:"htmlUrl,attr,omitempty"`
}

// Write renders every Feed as a flat OPML 2.0 document, one leaf outline per
// Feed at the document's top level, with no folders.
func Write(w io.Writer, feeds []FeedExport) error {
	doc := outDocument{
		Version: "2.0",
		Head:    outHead{Title: "Feeds"},
	}

	doc.Body.Outlines = make([]outOutline, 0, len(feeds))
	for _, feed := range feeds {
		doc.Body.Outlines = append(doc.Body.Outlines, outOutline{
			Text:    feed.Title,
			Title:   feed.Title,
			Type:    "rss",
			XMLURL:  feed.XMLURL,
			HTMLURL: feed.HTMLURL,
		})
	}

	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")
	if err := encoder.Encode(doc); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}
