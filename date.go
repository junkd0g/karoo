package rss

import (
	"fmt"
	"strings"
	"time"
)

// dateFormats lists the layouts tried, in order, by the date parsing helpers.
var dateFormats = []string{
	time.RFC1123Z,
	time.RFC1123,
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 MST",
	"Mon, 02 Jan 2006 15:04 MST",
	"Mon, 2 Jan 2006 15:04 MST",
	"Mon, 02 Jan 2006 15:04 -0700",
	"Mon, 2 Jan 2006 15:04 -0700",
	"02 Jan 2006 15:04:05 -0700",
	"02 Jan 2006 15:04:05 MST",
	"2 Jan 2006 15:04:05 -0700",
	"2 Jan 2006 15:04:05 MST",
	time.RFC822Z,
	time.RFC822,
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05-07:00",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05 -0700",
	"2006-01-02 15:04:05 MST",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// zoneOffsets maps common timezone abbreviations to their UTC offset in
// seconds. Go's time.Parse records an unknown abbreviation with a zero offset,
// which silently shifts the result by hours. Abbreviations that are ambiguous
// across regions (for example IST) are deliberately omitted.
var zoneOffsets = map[string]int{
	"UT":   0,
	"UTC":  0,
	"GMT":  0,
	"WET":  1 * 3600,
	"WEST": 1 * 3600,
	"BST":  1 * 3600,
	"CET":  1 * 3600,
	"CEST": 2 * 3600,
	"EET":  2 * 3600,
	"EEST": 3 * 3600,
	"MSK":  3 * 3600,
	"JST":  9 * 3600,
	"KST":  9 * 3600,
	"AEST": 10 * 3600,
	"AEDT": 11 * 3600,
	"NZST": 12 * 3600,
	"NZDT": 13 * 3600,
	"AST":  -4 * 3600,
	"ADT":  -3 * 3600,
	"EST":  -5 * 3600,
	"EDT":  -4 * 3600,
	"CST":  -6 * 3600,
	"CDT":  -5 * 3600,
	"MST":  -7 * 3600,
	"MDT":  -6 * 3600,
	"PST":  -8 * 3600,
	"PDT":  -7 * 3600,
	"AKST": -9 * 3600,
	"AKDT": -8 * 3600,
	"HST":  -10 * 3600,
}

// ParseDate parses a date string in any of the formats commonly found in RSS
// feeds: RFC 822, RFC 1123, RFC 3339 / ISO 8601, and several loose variants.
// Timezone abbreviations such as EST or CEST are resolved to their real
// offsets. It returns a zero time.Time and an error wrapping ErrParsePubDate
// when the value is empty or unrecognised.
func ParseDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("%w: empty pub date", ErrParsePubDate)
	}
	// RFC 822 allows "UT", which Go's parser rejects as too short.
	if strings.HasSuffix(value, " UT") {
		value += "C"
	}

	for _, format := range dateFormats {
		t, err := time.Parse(format, value)
		if err != nil {
			continue
		}
		return fixZone(t), nil
	}

	return time.Time{}, fmt.Errorf("%w: %s", ErrParsePubDate, value)
}

// fixZone corrects times whose zone abbreviation was parsed with a zero
// offset because Go did not recognise it.
func fixZone(t time.Time) time.Time {
	name, offset := t.Zone()
	if offset != 0 || name == "" {
		return t
	}
	want, ok := zoneOffsets[name]
	if !ok || want == 0 {
		return t
	}
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(),
		time.FixedZone(name, want))
}
