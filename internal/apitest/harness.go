// Package apitest is the API seam: it boots the real application against a
// temporary database with an injected clock and a fake publisher, and drives it
// over real HTTP requests. Tests assert on what a caller can observe.
package apitest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gabe-santos/rss-reader/internal/app"
	"github.com/gabe-santos/rss-reader/internal/clock"
	"github.com/gabe-santos/rss-reader/internal/config"
)

// Password is the configured password every harness boots with.
const Password = "correct horse battery staple"

// Harness is one running application plus the clients that talk to it.
type Harness struct {
	t         *testing.T
	Clock     *clock.Fake
	Publisher *Publisher
	Server    *httptest.Server
	Client    *http.Client
	DataDir   string

	stop func()
}

// Option adjusts the configuration a harness boots with.
type Option func(*config.Config)

// BlockPrivateFetch keeps the application's default refusal to fetch private
// network addresses in place. Every harness relaxes it by default, because the
// fake publisher listens on loopback.
func BlockPrivateFetch() Option {
	return func(cfg *config.Config) { cfg.AllowPrivateFetch = false }
}

// PollInterval sets how often a Feed is checked when nothing else says
// otherwise, for tests that want to observe the schedule without a real
// interval's wait.
func PollInterval(d time.Duration) Option {
	return func(cfg *config.Config) { cfg.PollInterval = d }
}

// PollTick sets how often the background schedule wakes to look for a due
// Feed, for tests that want to observe scheduled polling within a bounded
// real-time wait instead of a production-sized tick.
func PollTick(d time.Duration) Option {
	return func(cfg *config.Config) { cfg.PollTick = d }
}

// RetentionAge sets how old an unstarred Entry may get before automatic
// cleanup removes it, for tests that want to observe retention without a
// production-sized age.
func RetentionAge(d time.Duration) Option {
	return func(cfg *config.Config) { cfg.RetentionAge = d }
}

// RetentionTick sets how often the background schedule wakes to look for
// expired Entries, for tests that want to observe cleanup within a bounded
// real-time wait instead of a production-sized tick.
func RetentionTick(d time.Duration) Option {
	return func(cfg *config.Config) { cfg.RetentionTick = d }
}

// New boots the application against a fresh temporary data directory. The
// returned harness is torn down when the test ends.
func New(t *testing.T, opts ...Option) *Harness {
	t.Helper()
	return NewInDir(t, t.TempDir(), opts...)
}

// NewInDir boots the application against a given data directory, so that a test
// can restart the application over data it already wrote.
func NewInDir(t *testing.T, dataDir string, opts ...Option) *Harness {
	t.Helper()

	fake := clock.NewFake(time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC))
	publisher := NewPublisher(t)

	cfg := config.Config{
		DataDir:  dataDir,
		Password: Password,
		// The fake publisher is on loopback, which the application refuses to
		// fetch unless told otherwise.
		AllowPrivateFetch: true,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	application, err := app.New(cfg, app.Deps{Clock: fake, Logger: testLogger(t)})
	if err != nil {
		t.Fatalf("boot application: %v", err)
	}

	server := httptest.NewServer(application.Handler())

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("new cookie jar: %v", err)
	}

	var once sync.Once
	h := &Harness{
		t:         t,
		Clock:     fake,
		Publisher: publisher,
		Server:    server,
		Client:    &http.Client{Jar: jar},
		DataDir:   dataDir,
		stop: func() {
			once.Do(func() {
				server.Close()
				if err := application.Close(); err != nil {
					t.Errorf("close application: %v", err)
				}
			})
		},
	}
	t.Cleanup(h.stop)
	return h
}

// Stop shuts the application down, releasing its database.
func (h *Harness) Stop() { h.stop() }

// Response is one API response, already drained.
type Response struct {
	t       *testing.T
	Status  int
	Header  http.Header
	Cookies []*http.Cookie
	Body    []byte
}

// Do sends a request to the application. A non-nil body is sent as JSON.
func (h *Harness) Do(method, path string, body any) *Response {
	h.t.Helper()
	return h.send(h.jsonRequest(method, path, body))
}

// DoBearer sends a request carrying token as a bearer credential, instead of
// whatever session cookie the client holds.
func (h *Harness) DoBearer(method, path, token string, body any) *Response {
	h.t.Helper()
	req := h.jsonRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer "+token)
	return h.send(req)
}

// jsonRequest builds a request against the running server, sending a
// non-nil body as JSON, for Do and DoBearer to send as-is or add a header
// to first.
func (h *Harness) jsonRequest(method, path string, body any) *http.Request {
	h.t.Helper()

	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("encode request body: %v", err)
		}
		payload = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, h.Server.URL+path, payload)
	if err != nil {
		h.t.Fatalf("build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// DoRaw sends a request whose body is not JSON, such as an OPML document, at
// the given Content-Type.
func (h *Harness) DoRaw(method, path, contentType string, body []byte) *Response {
	h.t.Helper()

	req, err := http.NewRequest(method, h.Server.URL+path, bytes.NewReader(body))
	if err != nil {
		h.t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", contentType)

	return h.send(req)
}

// send issues a request already built and drains its response, the tail
// every request-sending method shares once they differ only in how the
// request itself is built.
func (h *Harness) send(req *http.Request) *Response {
	h.t.Helper()

	resp, err := h.Client.Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()

	read, err := io.ReadAll(resp.Body)
	if err != nil {
		h.t.Fatalf("read response body: %v", err)
	}

	return &Response{
		t:       h.t,
		Status:  resp.StatusCode,
		Header:  resp.Header,
		Cookies: resp.Cookies(),
		Body:    read,
	}
}

// Login posts a password to the session endpoint.
func (h *Harness) Login(password string) *Response {
	h.t.Helper()
	return h.Do(http.MethodPost, "/api/session", map[string]string{"password": password})
}

// Logout ends the current session.
func (h *Harness) Logout() *Response {
	h.t.Helper()
	return h.Do(http.MethodDelete, "/api/session", nil)
}

// UseSessionToken makes the client present this session token on later
// requests, replacing whatever session it held.
func (h *Harness) UseSessionToken(token string) {
	h.t.Helper()
	h.Client.Jar.SetCookies(mustParse(h.t, h.Server.URL), []*http.Cookie{{
		Name:  "reader_session",
		Value: token,
		Path:  "/",
	}})
}

// ExpectStatus fails the test unless the response carries the wanted status.
func (r *Response) ExpectStatus(want int) *Response {
	r.t.Helper()
	if r.Status != want {
		r.t.Fatalf("status = %d, want %d (body: %s)", r.Status, want, r.Body)
	}
	return r
}

// JSON decodes the response body into target.
func (r *Response) JSON(target any) {
	r.t.Helper()
	if err := json.Unmarshal(r.Body, target); err != nil {
		r.t.Fatalf("decode response body %q: %v", r.Body, err)
	}
}

// Cookie returns the named cookie the response set, or nil.
func (r *Response) Cookie(name string) *http.Cookie {
	r.t.Helper()
	for _, c := range r.Cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}
