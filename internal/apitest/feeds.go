package apitest

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Item is one item a fake publisher's Feed carries. Only the fields a test
// cares about need setting; the renderers omit the empty ones.
type Item struct {
	// ID is the publisher's own identifier for the item: an RSS guid, an Atom
	// id, or a JSON Feed id.
	ID        string
	Title     string
	Link      string
	Published time.Time
	Content   string
}

// RSS renders the items as an RSS 2.0 document.
func RSS(title, siteURL string, items ...Item) Document {
	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	body.WriteString(`<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">` + "\n")
	body.WriteString("<channel>\n")
	element(&body, "title", title)
	element(&body, "link", siteURL)
	for _, item := range items {
		body.WriteString("<item>\n")
		element(&body, "title", item.Title)
		element(&body, "link", item.Link)
		if item.ID != "" {
			body.WriteString(fmt.Sprintf("<guid isPermaLink=\"false\">%s</guid>\n", escape(item.ID)))
		}
		if !item.Published.IsZero() {
			element(&body, "pubDate", item.Published.Format(time.RFC1123Z))
		}
		if item.Content != "" {
			body.WriteString("<content:encoded><![CDATA[" + item.Content + "]]></content:encoded>\n")
		}
		body.WriteString("</item>\n")
	}
	body.WriteString("</channel>\n</rss>\n")

	return Document{ContentType: "application/rss+xml; charset=utf-8", Body: body.String()}
}

// Atom renders the items as an Atom 1.0 document.
func Atom(title, siteURL string, items ...Item) Document {
	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	body.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom">` + "\n")
	element(&body, "title", title)
	if siteURL != "" {
		body.WriteString(fmt.Sprintf("<link rel=\"alternate\" href=%q/>\n", escape(siteURL)))
	}
	for _, item := range items {
		body.WriteString("<entry>\n")
		element(&body, "title", item.Title)
		element(&body, "id", item.ID)
		if item.Link != "" {
			body.WriteString(fmt.Sprintf("<link rel=\"alternate\" href=%q/>\n", escape(item.Link)))
		}
		if !item.Published.IsZero() {
			element(&body, "published", item.Published.Format(time.RFC3339))
		}
		if item.Content != "" {
			body.WriteString(`<content type="html"><![CDATA[` + item.Content + "]]></content>\n")
		}
		body.WriteString("</entry>\n")
	}
	body.WriteString("</feed>\n")

	return Document{ContentType: "application/atom+xml; charset=utf-8", Body: body.String()}
}

// JSONFeed renders the items as a JSON Feed 1.1 document.
func JSONFeed(title, siteURL string, items ...Item) Document {
	doc := map[string]any{
		"version": "https://jsonfeed.org/version/1.1",
		"title":   title,
	}
	if siteURL != "" {
		doc["home_page_url"] = siteURL
	}

	rendered := make([]map[string]any, 0, len(items))
	for _, item := range items {
		entry := map[string]any{"id": item.ID}
		if item.Title != "" {
			entry["title"] = item.Title
		}
		if item.Link != "" {
			entry["url"] = item.Link
		}
		if item.Content != "" {
			entry["content_html"] = item.Content
		}
		if !item.Published.IsZero() {
			entry["date_published"] = item.Published.Format(time.RFC3339)
		}
		rendered = append(rendered, entry)
	}
	doc["items"] = rendered

	encoded, err := json.Marshal(doc)
	if err != nil {
		panic("apitest: render JSON Feed: " + err.Error())
	}
	return Document{ContentType: "application/feed+json", Body: string(encoded)}
}

// Page renders an HTML page that advertises the given Feed URLs the way a
// publisher's home page does, so that autodiscovery has something to find. With
// no URLs it is a page carrying no Feed at all.
func Page(title string, feedURLs ...string) Document {
	var body strings.Builder
	body.WriteString("<!doctype html>\n<html>\n<head>\n")
	body.WriteString(fmt.Sprintf("<title>%s</title>\n", escape(title)))
	for _, feedURL := range feedURLs {
		body.WriteString(fmt.Sprintf(
			"<link rel=\"alternate\" type=\"application/rss+xml\" title=%q href=%q>\n",
			escape(title), escape(feedURL)))
	}
	body.WriteString("</head>\n<body><p>A page, not a Feed.</p></body>\n</html>\n")

	return Document{ContentType: "text/html; charset=utf-8", Body: body.String()}
}

func element(body *strings.Builder, name, value string) {
	if value == "" {
		return
	}
	body.WriteString(fmt.Sprintf("<%s>%s</%s>\n", name, escape(value), name))
}

var escaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
)

func escape(value string) string { return escaper.Replace(value) }
