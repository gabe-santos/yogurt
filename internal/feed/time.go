package feed

import (
	"strings"
	"time"
)

// timeLayouts are the timestamp forms publishers actually serve, in the order
// they are tried. RSS asks for RFC 1123 and Atom for RFC 3339; the rest are what
// real Feeds do instead.
var timeLayouts = []string{
	time.RFC1123Z,
	time.RFC1123,
	time.RFC3339,
	time.RFC822Z,
	time.RFC822,
	time.ANSIC,
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 MST",
	"Mon, 2 Jan 2006 15:04:05",
	"2006-01-02T15:04:05.999999999Z07:00",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// parseTime reads a publisher's timestamp, returning the zero time when it is
// missing or in no form we recognise. The caller decides what to do about that;
// a Feed with unreadable dates is still worth reading.
func parseTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	for _, layout := range timeLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}
