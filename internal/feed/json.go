package feed

import (
	"encoding/json"
	"net/url"
	"strings"
)

// jsonFeedVersion prefixes the version of every JSON Feed document, and is how a
// JSON Feed is told apart from any other JSON a URL might answer with.
const jsonFeedVersion = "https://jsonfeed.org/version/"

type jsonFeed struct {
	Version     string         `json:"version"`
	Title       string         `json:"title"`
	HomePageURL string         `json:"home_page_url"`
	Items       []jsonFeedItem `json:"items"`
}

type jsonFeedItem struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	Title         string `json:"title"`
	ContentHTML   string `json:"content_html"`
	ContentText   string `json:"content_text"`
	DatePublished string `json:"date_published"`
	DateModified  string `json:"date_modified"`
}

func parseJSONFeed(body []byte, base *url.URL) (Document, error) {
	var parsed jsonFeed
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Document{}, ErrNotAFeed
	}
	if !strings.HasPrefix(parsed.Version, jsonFeedVersion) {
		return Document{}, ErrNotAFeed
	}

	doc := Document{
		Title:   strings.TrimSpace(parsed.Title),
		SiteURL: Resolve(base, parsed.HomePageURL),
		Items:   make([]Item, 0, len(parsed.Items)),
	}
	for _, item := range parsed.Items {
		link := Resolve(base, item.URL)
		doc.Items = append(doc.Items, Item{
			ID:          firstNonEmpty(item.ID, link),
			Title:       strings.TrimSpace(item.Title),
			URL:         link,
			Content:     firstNonEmpty(item.ContentHTML, item.ContentText),
			PublishedAt: parseTime(firstNonEmpty(item.DatePublished, item.DateModified)),
		})
	}
	return doc, nil
}
