package apitest

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

// Document is one response the fake publisher will serve.
type Document struct {
	Status      int
	ContentType string
	Headers     map[string]string
	Body        string
}

// Publisher is an in-process HTTP server standing in for a website. It is the
// only network the application is allowed to see in tests.
type Publisher struct {
	t      *testing.T
	server *httptest.Server

	mu       sync.Mutex
	docs     map[string]Document
	hits     map[string]int
	requests map[string]http.Header
}

// NewPublisher starts a fake publisher that serves nothing until documents are
// registered on it.
func NewPublisher(t *testing.T) *Publisher {
	t.Helper()

	p := &Publisher{
		t:        t,
		docs:     make(map[string]Document),
		hits:     make(map[string]int),
		requests: make(map[string]http.Header),
	}
	p.server = httptest.NewServer(http.HandlerFunc(p.serve))
	t.Cleanup(p.server.Close)
	return p
}

// Serve registers a document at a path and returns its absolute URL.
func (p *Publisher) Serve(path string, doc Document) string {
	p.t.Helper()

	p.mu.Lock()
	defer p.mu.Unlock()
	p.docs[path] = doc
	return p.server.URL + path
}

// URL is the absolute URL of a path on the fake publisher, registered or not.
func (p *Publisher) URL(path string) string {
	return p.server.URL + path
}

// Hits is the number of requests the publisher has received for a path.
func (p *Publisher) Hits(path string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.hits[path]
}

// LastRequest is the headers of the most recent request the publisher
// received for a path, so a test can confirm a conditional validator was
// actually sent.
func (p *Publisher) LastRequest(path string) http.Header {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.requests[path]
}

func (p *Publisher) serve(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	doc, ok := p.docs[r.URL.Path]
	p.hits[r.URL.Path]++
	p.requests[r.URL.Path] = r.Header.Clone()
	p.mu.Unlock()

	if !ok {
		http.NotFound(w, r)
		return
	}

	for name, value := range doc.Headers {
		w.Header().Set(name, value)
	}
	if doc.ContentType != "" {
		w.Header().Set("Content-Type", doc.ContentType)
	}
	status := doc.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	if _, err := io.WriteString(w, doc.Body); err != nil {
		p.t.Errorf("write publisher body: %v", err)
	}
}

// testLogger keeps application logs out of test output; tests never assert on
// them.
func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return parsed
}
