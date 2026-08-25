// Package opml reads and writes the reader's Feed collection as OPML, the
// format most Feed readers use to exchange subscription lists. It knows
// nothing of the store or the application; it only turns an OPML document
// into the FeedImports it names, and Groups of Feeds into an OPML document.
package opml

import (
	"encoding/xml"
	"io"
	"strings"
)

// FeedImport is one Feed an OPML document names, together with the Group it
// belongs to once nested folders are flattened.
type FeedImport struct {
	// URL is the Feed's address, as the OPML document gave it.
	URL string
	// GroupName is the outermost folder this Feed was nested inside, trimmed,
	// or empty when the Feed sat at the document's top level. A folder nested
	// inside that outermost one contributes its Feeds to the same Group; its
	// own name is never used, so how deep an outline was nested never
	// invents an extra Group.
	GroupName string
}

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
	Text     string      `xml:"text,attr"`
	Title    string      `xml:"title,attr"`
	XMLURL   string      `xml:"xmlUrl,attr"`
	Outlines []inOutline `xml:"outline"`
}

// name is this outline's folder name: its title when present, its text
// otherwise, per the OPML spec where text is required and title is not.
func (o inOutline) name() string {
	if strings.TrimSpace(o.Title) != "" {
		return strings.TrimSpace(o.Title)
	}
	return strings.TrimSpace(o.Text)
}

// Parse reads an OPML document and returns the Feeds it names, in document
// order. Nested folders flatten to one Group per outermost folder: a Feed two
// folders deep belongs to the Group named by the first folder it was nested
// in, and every folder below that contributes nothing but its Feeds.
func Parse(r io.Reader) ([]FeedImport, error) {
	decoder := xml.NewDecoder(r)
	// Publishers and other readers are not careful producing OPML either:
	// accept documents a strict parser would refuse.
	decoder.Strict = false
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }

	var doc document
	if err := decoder.Decode(&doc); err != nil {
		return nil, err
	}

	var subs []FeedImport
	flatten(doc.Body.Outlines, "", &subs)
	return subs, nil
}

func flatten(outlines []inOutline, groupName string, subs *[]FeedImport) {
	for _, o := range outlines {
		url := strings.TrimSpace(o.XMLURL)
		if url != "" {
			*subs = append(*subs, FeedImport{URL: url, GroupName: groupName})
			continue
		}

		folderName := groupName
		if folderName == "" {
			folderName = o.name()
		}
		flatten(o.Outlines, folderName, subs)
	}
}

// FeedExport is one Feed as an exported OPML document names it.
type FeedExport struct {
	Title   string
	XMLURL  string
	HTMLURL string
}

// GroupExport is one Group's Feeds, ready to render as OPML. A Group with no
// Feeds is deliberately left out of the document: OPML names Feed
// subscriptions, and a Group the reader has not put anything in yet is not a
// subscription to round-trip, so re-importing an export never has to decide
// whether to recreate an empty Group.
type GroupExport struct {
	// Name is the Group's name, or empty for the default Group: OPML has no
	// notion of a default folder, so its Feeds are written at the document's
	// top level rather than nested in one.
	Name  string
	Feeds []FeedExport
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
	Text     string       `xml:"text,attr"`
	Title    string       `xml:"title,attr,omitempty"`
	Type     string       `xml:"type,attr,omitempty"`
	XMLURL   string       `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string       `xml:"htmlUrl,attr,omitempty"`
	Outlines []outOutline `xml:"outline,omitempty"`
}

// Write renders Groups and their Feeds as an OPML 2.0 document: one folder
// outline per named Group, its Feeds as leaf outlines inside it, and the
// default Group's Feeds as leaf outlines at the top level.
func Write(w io.Writer, groups []GroupExport) error {
	doc := outDocument{
		Version: "2.0",
		Head:    outHead{Title: "Feeds"},
	}

	for _, group := range groups {
		if len(group.Feeds) == 0 {
			continue
		}

		feedOutlines := make([]outOutline, 0, len(group.Feeds))
		for _, feed := range group.Feeds {
			feedOutlines = append(feedOutlines, outOutline{
				Text:    feed.Title,
				Title:   feed.Title,
				Type:    "rss",
				XMLURL:  feed.XMLURL,
				HTMLURL: feed.HTMLURL,
			})
		}

		if group.Name == "" {
			doc.Body.Outlines = append(doc.Body.Outlines, feedOutlines...)
			continue
		}
		doc.Body.Outlines = append(doc.Body.Outlines, outOutline{
			Text:     group.Name,
			Title:    group.Name,
			Outlines: feedOutlines,
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
