// Package pullpolicy decides when a Feed should next be checked. It is a pure
// function of the current time, the reader's configured interval, whatever
// hints the publisher's last response carried, and — after a failure — how
// many checks in a row have failed. It touches no store, no clock, and no
// network, so every case can be proven with a table instead of a fake
// publisher and a fake clock.
package pullpolicy

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DefaultInterval is how often a Feed is checked when the reader has not
// configured anything else.
const DefaultInterval = 30 * time.Minute

// MaxInterval is the longest this application will ever wait before checking
// a Feed again, however loudly a publisher's hint or a run of failures asks
// for longer, per ADR-0001.
const MaxInterval = 48 * time.Hour

// backoffBase grows the wait between checks geometrically with each
// consecutive failure, per ADR-0001: interval * 1.8^failures.
const backoffBase = 1.8

// Hints are the publisher's own preferences about when to be asked again,
// each a duration measured from the response that carried it. A zero value
// means the publisher expressed no opinion.
type Hints struct {
	// RetryAfter is how long the publisher asked to be left alone, from a
	// Retry-After header.
	RetryAfter time.Duration
	// FreshFor is how long the publisher says its response stays good, from
	// Cache-Control's max-age or, failing that, Expires.
	FreshFor time.Duration
}

// HintsFromHeader reads Retry-After, Cache-Control and Expires from a
// publisher's response. Cache-Control's max-age is preferred over Expires
// when both are present, per RFC 9111 §5.3.
func HintsFromHeader(header http.Header, now time.Time) Hints {
	var hints Hints

	if v := header.Get("Retry-After"); v != "" {
		hints.RetryAfter = parseRetryAfter(v, now)
	}

	if maxAge, ok := parseMaxAge(header.Get("Cache-Control")); ok {
		hints.FreshFor = maxAge
	} else if v := header.Get("Expires"); v != "" {
		if when, err := http.ParseTime(v); err == nil {
			if d := when.Sub(now); d > 0 {
				hints.FreshFor = d
			}
		}
	}

	return hints
}

// NextCheck computes when a Feed should be checked again after a check that
// succeeded, whether or not the Feed had changed. The wait is the strictest
// (longest) of the configured interval and every publisher hint, capped at
// MaxInterval so a publisher's hint alone can never stop this Feed being
// checked at all.
func NextCheck(now time.Time, interval time.Duration, hints Hints) time.Time {
	return now.Add(capped(strictest(interval, hints.RetryAfter, hints.FreshFor)))
}

// NextCheckAfterFailure computes when a Feed should be checked again after a
// check that failed. Consecutive failures grow the wait geometrically; the
// result is still the strictest of that backoff, the configured interval and
// the publisher's retry hint (a failure carries no freshness hint, since
// nothing was successfully read), capped at MaxInterval.
func NextCheckAfterFailure(now time.Time, interval time.Duration, hints Hints, consecutiveFailures int) time.Time {
	if consecutiveFailures < 0 {
		consecutiveFailures = 0
	}
	backoff := time.Duration(float64(interval) * math.Pow(backoffBase, float64(consecutiveFailures)))
	return now.Add(capped(strictest(interval, backoff, hints.RetryAfter)))
}

// strictest is the longest of the given waits: the one that would make the
// reader wait the longest before checking again.
func strictest(waits ...time.Duration) time.Duration {
	var longest time.Duration
	for _, w := range waits {
		if w > longest {
			longest = w
		}
	}
	return longest
}

// capped bounds a wait to MaxInterval, and gives non-positive waits the
// interval's floor rather than leaving a Feed uncheckable or checked in a
// tight loop.
func capped(d time.Duration) time.Duration {
	switch {
	case d <= 0:
		return DefaultInterval
	case d > MaxInterval:
		return MaxInterval
	default:
		return d
	}
}

// parseRetryAfter reads a Retry-After header, which RFC 9110 §10.2.3 allows to
// be either a number of seconds or an HTTP date.
func parseRetryAfter(v string, now time.Time) time.Duration {
	if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if when, err := http.ParseTime(v); err == nil {
		if d := when.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}

// parseMaxAge reads the max-age directive from a Cache-Control header.
func parseMaxAge(cacheControl string) (time.Duration, bool) {
	for _, directive := range strings.Split(cacheControl, ",") {
		name, value, found := strings.Cut(strings.TrimSpace(directive), "=")
		if !found || !strings.EqualFold(strings.TrimSpace(name), "max-age") {
			continue
		}
		secs, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || secs < 0 {
			continue
		}
		return time.Duration(secs) * time.Second, true
	}
	return 0, false
}
