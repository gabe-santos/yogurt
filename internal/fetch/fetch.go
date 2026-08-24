// Package fetch is the only way this application talks to the outside network.
// It refuses private and link-local destinations unless configured otherwise, so
// that a malicious Feed URL cannot make the app probe the host's own network.
package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"
)

// Defaults for a client the caller has no opinion about.
const (
	DefaultTimeout   = 20 * time.Second
	DefaultMaxBody   = 8 << 20 // 8 MiB, which no sane Feed document exceeds
	DefaultUserAgent = "rss-reader/1.0 (+https://github.com/gabe-santos/rss-reader)"
)

// ErrBodyTooLarge reports a response bigger than the client is willing to read.
var ErrBodyTooLarge = errors.New("response body is too large")

// ErrNotAbsoluteURL reports an address that is not a fetchable http(s) URL.
var ErrNotAbsoluteURL = errors.New("expected an absolute http or https URL")

// Options configure a Client. The zero value is a sensible, safe client.
type Options struct {
	// AllowPrivate lets the client reach private, loopback and link-local
	// addresses. It exists for tests and for self-hosters whose Feeds live on
	// their own LAN, and is off by default.
	AllowPrivate bool
	Timeout      time.Duration
	MaxBody      int64
	UserAgent    string
}

// Client fetches documents over HTTP.
type Client struct {
	http      *http.Client
	maxBody   int64
	userAgent string
}

// Response is a fetched document, already drained.
type Response struct {
	// URL is where the document actually came from, after any redirects.
	URL         *url.URL
	StatusCode  int
	ContentType string
	Header      http.Header
	Body        []byte
}

// New builds a client.
func New(opts Options) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.MaxBody <= 0 {
		opts.MaxBody = DefaultMaxBody
	}
	if opts.UserAgent == "" {
		opts.UserAgent = DefaultUserAgent
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	if !opts.AllowPrivate {
		// The check runs on the address the resolver actually returned, on every
		// connection including redirects, which is what makes DNS rebinding
		// pointless.
		dialer.Control = func(_, address string, _ syscall.RawConn) error {
			return checkPublic(address)
		}
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}

	return &Client{
		http:      &http.Client{Transport: transport, Timeout: opts.Timeout},
		maxBody:   opts.MaxBody,
		userAgent: opts.UserAgent,
	}
}

// accept asks for a Feed, and settles for the web page that might advertise one.
const accept = "application/atom+xml, application/rss+xml, application/feed+json, " +
	"application/xml;q=0.9, text/xml;q=0.9, text/html;q=0.8, */*;q=0.5"

// Get fetches one document. A response the publisher refused — any status — is
// returned rather than treated as an error; only failing to get a response at
// all is an error.
func (c *Client) Get(ctx context.Context, rawURL string) (*Response, error) {
	target, err := ParseURL(rawURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", target, err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", accept)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", target, err)
	}
	if int64(len(body)) > c.maxBody {
		return nil, fmt.Errorf("%s: %w", target, ErrBodyTooLarge)
	}

	final := resp.Request.URL
	if final == nil {
		final = target
	}
	return &Response{
		URL:         final,
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		Header:      resp.Header,
		Body:        body,
	}, nil
}

// ParseURL reads an address the reader supplied, accepting only something this
// client could actually fetch.
func ParseURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("%w: %q", ErrNotAbsoluteURL, rawURL)
	}
	return parsed, nil
}

// checkPublic refuses to connect anywhere but the public internet.
func checkPublic(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("refusing to connect to %s: not an IP address", address)
	}
	if !isPublic(ip) {
		return fmt.Errorf("refusing to connect to the private network address %s", ip)
	}
	return nil
}

// isPublic reports whether an address is somewhere on the public internet, and
// so somewhere a Feed may legitimately live.
func isPublic(ip net.IP) bool {
	switch {
	case ip.IsLoopback(), ip.IsPrivate(), ip.IsUnspecified(),
		ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast(),
		ip.IsInterfaceLocalMulticast(), ip.IsMulticast():
		return false
	case carrierGrade.Contains(ip):
		// Shared address space, which is where a private tailnet lives.
		return false
	}
	return true
}

var carrierGrade = mustCIDR("100.64.0.0/10")

func mustCIDR(notation string) *net.IPNet {
	_, network, err := net.ParseCIDR(notation)
	if err != nil {
		panic("fetch: bad CIDR " + notation)
	}
	return network
}
