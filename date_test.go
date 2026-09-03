package rss_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	rss "github.com/junkd0g/karoo"
)

func TestParseDate(t *testing.T) {
	utc := func(y int, m time.Month, d, h, mi, s int) time.Time {
		return time.Date(y, m, d, h, mi, s, 0, time.UTC)
	}
	cases := []struct {
		name  string
		input string
		want  time.Time // compared as an instant
	}{
		{"RFC1123", "Mon, 15 Jan 2024 10:30:00 GMT", utc(2024, 1, 15, 10, 30, 0)},
		{"RFC1123Z", "Mon, 15 Jan 2024 10:30:00 +0000", utc(2024, 1, 15, 10, 30, 0)},
		{"RFC1123Z offset", "Mon, 15 Jan 2024 10:30:00 +0200", utc(2024, 1, 15, 8, 30, 0)},
		{"RFC822", "15 Jan 24 10:30 GMT", utc(2024, 1, 15, 10, 30, 0)},
		{"single digit day", "Mon, 1 Jan 2024 10:30:00 GMT", utc(2024, 1, 1, 10, 30, 0)},
		{"no seconds", "Mon, 15 Jan 2024 10:30 GMT", utc(2024, 1, 15, 10, 30, 0)},
		{"no weekday", "15 Jan 2024 10:30:00 +0000", utc(2024, 1, 15, 10, 30, 0)},
		{"wrong weekday is ignored", "Tue, 15 Jan 2024 10:30:00 GMT", utc(2024, 1, 15, 10, 30, 0)},
		{"EST abbreviation", "Mon, 15 Jan 2024 10:30:00 EST", utc(2024, 1, 15, 15, 30, 0)},
		{"PDT abbreviation", "Mon, 15 Jul 2024 10:30:00 PDT", utc(2024, 7, 15, 17, 30, 0)},
		{"CEST abbreviation", "Mon, 15 Jul 2024 10:30:00 CEST", utc(2024, 7, 15, 8, 30, 0)},
		{"UT abbreviation", "Mon, 15 Jan 2024 10:30:00 UT", utc(2024, 1, 15, 10, 30, 0)},
		{"RFC3339", "2024-01-15T10:30:00Z", utc(2024, 1, 15, 10, 30, 0)},
		{"RFC3339 offset", "2024-01-15T10:30:00-05:00", utc(2024, 1, 15, 15, 30, 0)},
		{"RFC3339 fractional", "2024-01-15T10:30:00.123Z", utc(2024, 1, 15, 10, 30, 0).Add(123 * time.Millisecond)},
		{"ISO no zone", "2024-01-15T10:30:00", utc(2024, 1, 15, 10, 30, 0)},
		{"space separated", "2024-01-15 10:30:00", utc(2024, 1, 15, 10, 30, 0)},
		{"date only", "2024-01-15", utc(2024, 1, 15, 0, 0, 0)},
		{"surrounding whitespace", "  Mon, 15 Jan 2024 10:30:00 GMT\n", utc(2024, 1, 15, 10, 30, 0)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rss.ParseDate(tc.input)
			if err != nil {
				t.Fatalf("ParseDate(%q): %v", tc.input, err)
			}
			if !got.Equal(tc.want) {
				t.Errorf("ParseDate(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseDateErrors(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got, err := rss.ParseDate("   ")
		if !errors.Is(err, rss.ErrParsePubDate) || !got.IsZero() {
			t.Fatalf("got %v, %v", got, err)
		}
		if !strings.Contains(err.Error(), "empty pub date") {
			t.Errorf("legacy message substring missing: %q", err)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		got, err := rss.ParseDate("not a date")
		if !errors.Is(err, rss.ErrParsePubDate) || !got.IsZero() {
			t.Fatalf("got %v, %v", got, err)
		}
		if !strings.Contains(err.Error(), "unable to parse pub date: not a date") {
			t.Errorf("legacy message substring missing: %q", err)
		}
	})
}

func TestParseDateKeepsZoneName(t *testing.T) {
	got, err := rss.ParseDate("Mon, 15 Jan 2024 10:30:00 EST")
	if err != nil {
		t.Fatal(err)
	}
	name, offset := got.Zone()
	if name != "EST" || offset != -5*3600 {
		t.Errorf("zone = %s %d, want EST -18000", name, offset)
	}
	if got.Hour() != 10 {
		t.Errorf("wall clock hour = %d, want 10", got.Hour())
	}
}

func TestItemParsePubDate(t *testing.T) {
	t.Run("pubDate", func(t *testing.T) {
		item := rss.Item{PubDate: "Mon, 15 Jan 2024 10:30:00 GMT", Date: "2020-01-01"}
		got, err := item.ParsePubDate()
		if err != nil || got.Year() != 2024 {
			t.Fatalf("got %v, %v", got, err)
		}
	})
	t.Run("dc:date fallback", func(t *testing.T) {
		item := rss.Item{Date: "2024-01-15T10:30:00Z"}
		got, err := item.ParsePubDate()
		if err != nil || got.Year() != 2024 {
			t.Fatalf("got %v, %v", got, err)
		}
	})
	t.Run("neither", func(t *testing.T) {
		var item rss.Item
		if _, err := item.ParsePubDate(); !errors.Is(err, rss.ErrParsePubDate) {
			t.Fatalf("expected ErrParsePubDate, got %v", err)
		}
	})
}

func TestChannelDates(t *testing.T) {
	ch := rss.Channel{PubDate: "Mon, 15 Jan 2024 09:00:00 GMT", LastBuildDate: "2024-01-15T10:30:00Z"}
	pub, err := ch.ParsePubDate()
	if err != nil || pub.Hour() != 9 {
		t.Errorf("ParsePubDate = %v, %v", pub, err)
	}
	build, err := ch.ParseLastBuildDate()
	if err != nil || build.Hour() != 10 {
		t.Errorf("ParseLastBuildDate = %v, %v", build, err)
	}
	if _, err := (&rss.Channel{}).ParseLastBuildDate(); !errors.Is(err, rss.ErrParsePubDate) {
		t.Errorf("expected ErrParsePubDate, got %v", err)
	}
}

func FuzzParseDate(f *testing.F) {
	for _, s := range []string{"Mon, 15 Jan 2024 10:30:00 GMT", "2024-01-15T10:30:00Z", "", "garbage", "Mon, 15 Jan 2024 10:30:00 EST"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got, err := rss.ParseDate(s)
		if err != nil && !got.IsZero() {
			t.Errorf("error with non-zero time: %v %v", got, err)
		}
		if err != nil && !errors.Is(err, rss.ErrParsePubDate) {
			t.Errorf("error does not wrap ErrParsePubDate: %v", err)
		}
	})
}
