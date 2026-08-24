package pullpolicy_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gabe-santos/rss-reader/internal/pullpolicy"
)

var epoch = time.Date(2026, 1, 2, 15, 0, 0, 0, time.UTC)

func TestNextCheckHonoursTheStrictestHint(t *testing.T) {
	const interval = 30 * time.Minute

	cases := []struct {
		name  string
		hints pullpolicy.Hints
		want  time.Duration
	}{
		{"no hints, falls back to the interval", pullpolicy.Hints{}, interval},
		{
			"a shorter retry hint is ignored: the interval is already stricter",
			pullpolicy.Hints{RetryAfter: 5 * time.Minute},
			interval,
		},
		{
			"a longer retry hint wins over the interval",
			pullpolicy.Hints{RetryAfter: 2 * time.Hour},
			2 * time.Hour,
		},
		{
			"a longer freshness hint wins over the interval",
			pullpolicy.Hints{FreshFor: 3 * time.Hour},
			3 * time.Hour,
		},
		{
			"the longer of two hints wins",
			pullpolicy.Hints{RetryAfter: time.Hour, FreshFor: 4 * time.Hour},
			4 * time.Hour,
		},
		{
			"a hint past the global cap is clamped to it",
			pullpolicy.Hints{FreshFor: 90 * time.Hour},
			pullpolicy.MaxInterval,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := pullpolicy.NextCheck(epoch, interval, c.hints)
			if want := epoch.Add(c.want); !got.Equal(want) {
				t.Errorf("NextCheck(...) = %s, want %s", got, want)
			}
		})
	}
}

func TestNextCheckAfterFailureBacksOffGeometrically(t *testing.T) {
	const interval = 30 * time.Minute

	cases := []struct {
		failures int
		want     time.Duration
	}{
		{0, interval},         // interval * 1.8^0 = interval
		{1, 54 * time.Minute}, // interval * 1.8^1
		{2, time.Duration(float64(interval) * 1.8 * 1.8)}, // interval * 1.8^2
	}

	for _, c := range cases {
		got := pullpolicy.NextCheckAfterFailure(epoch, interval, pullpolicy.Hints{}, c.failures)
		if want := epoch.Add(c.want); !got.Equal(want) {
			t.Errorf("NextCheckAfterFailure(..., failures=%d) = %s, want %s", c.failures, got, want)
		}
	}
}

func TestNextCheckAfterFailureIsCappedGlobally(t *testing.T) {
	got := pullpolicy.NextCheckAfterFailure(epoch, 30*time.Minute, pullpolicy.Hints{}, 100)
	if want := epoch.Add(pullpolicy.MaxInterval); !got.Equal(want) {
		t.Errorf("NextCheckAfterFailure with many failures = %s, want the global cap %s", got, want)
	}
}

func TestNextCheckAfterFailureHonoursARetryHintLongerThanBackoff(t *testing.T) {
	got := pullpolicy.NextCheckAfterFailure(epoch, 30*time.Minute, pullpolicy.Hints{RetryAfter: 10 * time.Hour}, 1)
	if want := epoch.Add(10 * time.Hour); !got.Equal(want) {
		t.Errorf("NextCheckAfterFailure(...) = %s, want the retry hint %s", got, want)
	}
}

func TestNextCheckAfterFailureIgnoresFreshnessHints(t *testing.T) {
	// A failed check read nothing successfully, so a stale freshness hint from
	// a prior success must not extend the wait.
	got := pullpolicy.NextCheckAfterFailure(epoch, 30*time.Minute, pullpolicy.Hints{FreshFor: 10 * time.Hour}, 1)
	if want := epoch.Add(54 * time.Minute); !got.Equal(want) {
		t.Errorf("NextCheckAfterFailure(...) = %s, want backoff alone %s", got, want)
	}
}

func TestHintsFromHeaderPrefersMaxAgeOverExpires(t *testing.T) {
	header := http.Header{}
	header.Set("Cache-Control", "public, max-age=3600")
	header.Set("Expires", epoch.Add(10*time.Hour).Format(http.TimeFormat))

	hints := pullpolicy.HintsFromHeader(header, epoch)
	if hints.FreshFor != time.Hour {
		t.Errorf("FreshFor = %s, want max-age's 1h over Expires' 10h", hints.FreshFor)
	}
}

func TestHintsFromHeaderFallsBackToExpires(t *testing.T) {
	header := http.Header{}
	header.Set("Expires", epoch.Add(2*time.Hour).Format(http.TimeFormat))

	hints := pullpolicy.HintsFromHeader(header, epoch)
	if hints.FreshFor != 2*time.Hour {
		t.Errorf("FreshFor = %s, want 2h from Expires", hints.FreshFor)
	}
}

func TestHintsFromHeaderReadsRetryAfterAsSeconds(t *testing.T) {
	header := http.Header{}
	header.Set("Retry-After", "120")

	hints := pullpolicy.HintsFromHeader(header, epoch)
	if hints.RetryAfter != 2*time.Minute {
		t.Errorf("RetryAfter = %s, want 2m", hints.RetryAfter)
	}
}

func TestHintsFromHeaderReadsRetryAfterAsAnHTTPDate(t *testing.T) {
	header := http.Header{}
	header.Set("Retry-After", epoch.Add(90*time.Minute).Format(http.TimeFormat))

	hints := pullpolicy.HintsFromHeader(header, epoch)
	if hints.RetryAfter != 90*time.Minute {
		t.Errorf("RetryAfter = %s, want 90m", hints.RetryAfter)
	}
}

func TestHintsFromHeaderIgnoresAPastExpiry(t *testing.T) {
	header := http.Header{}
	header.Set("Expires", epoch.Add(-time.Hour).Format(http.TimeFormat))

	hints := pullpolicy.HintsFromHeader(header, epoch)
	if hints.FreshFor != 0 {
		t.Errorf("FreshFor = %s, want 0 for an Expires already in the past", hints.FreshFor)
	}
}

func TestHintsFromHeaderIgnoresAnEmptyHeader(t *testing.T) {
	hints := pullpolicy.HintsFromHeader(http.Header{}, epoch)
	if hints != (pullpolicy.Hints{}) {
		t.Errorf("hints = %+v, want zero value", hints)
	}
}
