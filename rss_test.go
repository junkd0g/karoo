package rss_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	rss "github.com/junkd0g/karoo"
	"github.com/stretchr/testify/assert"
)

// TestGetFeedSuccess verifies that GetFeed correctly fetches and parses a valid RSS feed.
func TestGetFeedSuccess(t *testing.T) {
	validXML := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>http://example.com</link>
    <description>Test Description</description>
    <item>
      <title>Item 1</title>
      <link>http://example.com/item1</link>
      <description>Item 1 Description</description>
    </item>
    <item>
      <title>Item 2</title>
      <link>http://example.com/item2</link>
      <description>Item 2 Description</description>
    </item>
  </channel>
</rss>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validXML))
	}))
	defer ts.Close()

	client := rss.NewClient()
	feed, err := client.GetFeed(ts.URL)
	assert.NoError(t, err)
	assert.Equal(t, "2.0", feed.Version)
	assert.Equal(t, "Test Feed", feed.Channel.Title)
	assert.Equal(t, "http://example.com", feed.Channel.Link)
	assert.Equal(t, "Test Description", feed.Channel.Description)
	assert.Len(t, feed.Channel.Items, 2)
	assert.Equal(t, "Item 1", feed.Channel.Items[0].Title)
	assert.Equal(t, "Item 2", feed.Channel.Items[1].Title)
}

// TestGetFeedNonOK verifies that GetFeed returns an error when the HTTP status is not OK.
func TestGetFeedNonOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer ts.Close()

	client := rss.NewClient()
	_, err := client.GetFeed(ts.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch RSS feed")
}

// TestGetFeedInvalidXML verifies that GetFeed returns an error when the XML is invalid.
func TestGetFeedInvalidXML(t *testing.T) {
	invalidXML := `this is not xml`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(invalidXML))
	}))
	defer ts.Close()

	client := rss.NewClient()
	_, err := client.GetFeed(ts.URL)
	assert.Error(t, err)
}

// errReader is an io.ReadCloser that always returns an error on Read.
type errReader struct{}

func (er errReader) Read(p []byte) (int, error) {
	return 0, errors.New("read error")
}

func (er errReader) Close() error {
	return nil
}

// errorRoundTripper is a custom RoundTripper that returns a response with a body that errors on read.
type errorRoundTripper struct{}

func (ert errorRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       errReader{},
		Header:     make(http.Header),
	}, nil
}

// TestGetFeedReadError verifies that GetFeed returns an error when reading the response body fails.
func TestGetFeedReadError(t *testing.T) {
	customHTTPClient := &http.Client{
		Transport: errorRoundTripper{},
		Timeout:   5 * time.Second,
	}
	client := rss.NewClient(rss.WithHTTPClient(customHTTPClient))
	_, err := client.GetFeed("http://example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read error")
}

// TestGetFeedTimeout verifies that GetFeed returns a timeout error when the request exceeds the configured timeout.
func TestGetFeedTimeout(t *testing.T) {
	// Create a test server that delays its response.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>Delayed Feed</title></channel></rss>`))
	}))
	defer ts.Close()

	// Set a client timeout shorter than the server's delay.
	client := rss.NewClient(rss.WithTimeout(50 * time.Millisecond))
	_, err := client.GetFeed(ts.URL)
	assert.Error(t, err)
	// The error message may vary, but should indicate a timeout.
	assert.Contains(t, err.Error(), "Client.Timeout")
}

// TestGetFeedNetworkError verifies that GetFeed returns an error when the host is unreachable.
func TestGetFeedNetworkError(t *testing.T) {
	client := rss.NewClient()
	_, err := client.GetFeed("http://192.0.2.1:12345/nonexistent") // RFC 3330 test address
	assert.Error(t, err)
}

// TestGetFeedDifferentStatusCodes verifies that GetFeed returns appropriate errors for different HTTP status codes.
func TestGetFeedDifferentStatusCodes(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
	}{
		{"404 Not Found", http.StatusNotFound},
		{"403 Forbidden", http.StatusForbidden},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte("Error response"))
			}))
			defer ts.Close()

			client := rss.NewClient()
			_, err := client.GetFeed(ts.URL)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to fetch RSS feed")
		})
	}
}

// TestGetFeedEmptyResponse verifies that GetFeed handles empty response bodies correctly.
func TestGetFeedEmptyResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Don't write anything to the response body
	}))
	defer ts.Close()

	client := rss.NewClient()
	_, err := client.GetFeed(ts.URL)
	assert.Error(t, err)
}

// TestGetFeedEmptyFeed verifies that GetFeed handles feeds with no items correctly.
func TestGetFeedEmptyFeed(t *testing.T) {
	emptyFeedXML := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Empty Feed</title>
    <link>http://example.com</link>
    <description>A feed with no items</description>
  </channel>
</rss>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(emptyFeedXML))
	}))
	defer ts.Close()

	client := rss.NewClient()
	feed, err := client.GetFeed(ts.URL)
	assert.NoError(t, err)
	assert.Equal(t, "2.0", feed.Version)
	assert.Equal(t, "Empty Feed", feed.Channel.Title)
	assert.Equal(t, "http://example.com", feed.Channel.Link)
	assert.Equal(t, "A feed with no items", feed.Channel.Description)
	assert.Len(t, feed.Channel.Items, 0)
}

// TestGetFeedMissingOptionalFields verifies that GetFeed handles RSS feeds with missing optional fields.
func TestGetFeedMissingOptionalFields(t *testing.T) {
	minimalXML := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Minimal Feed</title>
    <item>
      <title>Item with minimal fields</title>
    </item>
    <item>
      <link>http://example.com/item2</link>
      <description>Item with different fields</description>
    </item>
  </channel>
</rss>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(minimalXML))
	}))
	defer ts.Close()

	client := rss.NewClient()
	feed, err := client.GetFeed(ts.URL)
	assert.NoError(t, err)
	assert.Equal(t, "2.0", feed.Version)
	assert.Equal(t, "Minimal Feed", feed.Channel.Title)
	assert.Equal(t, "", feed.Channel.Link)        // Missing link should be empty
	assert.Equal(t, "", feed.Channel.Description) // Missing description should be empty
	assert.Len(t, feed.Channel.Items, 2)

	// First item has only title
	assert.Equal(t, "Item with minimal fields", feed.Channel.Items[0].Title)
	assert.Equal(t, "", feed.Channel.Items[0].Link)
	assert.Equal(t, "", feed.Channel.Items[0].Description)

	// Second item has link and description but no title
	assert.Equal(t, "", feed.Channel.Items[1].Title)
	assert.Equal(t, "http://example.com/item2", feed.Channel.Items[1].Link)
	assert.Equal(t, "Item with different fields", feed.Channel.Items[1].Description)
}

// TestGetFeedExtendedFields verifies that GetFeed correctly parses extended RSS fields.
func TestGetFeedExtendedFields(t *testing.T) {
	extendedXML := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Extended Feed</title>
    <link>http://example.com</link>
    <description>Feed with extended fields</description>
    <language>en-us</language>
    <image>
      <url>http://example.com/logo.png</url>
      <title>Feed Logo</title>
      <link>http://example.com</link>
    </image>
    <item>
      <title>Item with all fields</title>
      <link>http://example.com/item1</link>
      <description>Full description</description>
      <pubDate>Mon, 15 Jan 2024 10:30:00 GMT</pubDate>
      <guid>unique-id-123</guid>
      <author>john@example.com</author>
      <category>Technology</category>
      <enclosure url="http://example.com/image.jpg" type="image/jpeg" length="12345"/>
    </item>
  </channel>
</rss>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(extendedXML))
	}))
	defer ts.Close()

	client := rss.NewClient()
	feed, err := client.GetFeed(ts.URL)
	assert.NoError(t, err)

	// Channel extended fields
	assert.Equal(t, "en-us", feed.Channel.Language)
	assert.NotNil(t, feed.Channel.Image)
	assert.Equal(t, "http://example.com/logo.png", feed.Channel.Image.URL)
	assert.Equal(t, "Feed Logo", feed.Channel.Image.Title)

	// Item extended fields
	assert.Len(t, feed.Channel.Items, 1)
	item := feed.Channel.Items[0]
	assert.Equal(t, "Mon, 15 Jan 2024 10:30:00 GMT", item.PubDate)
	assert.Equal(t, "unique-id-123", item.GUID)
	assert.Equal(t, "john@example.com", item.Author)
	assert.Equal(t, "Technology", item.Category)
	assert.NotNil(t, item.Enclosure)
	assert.Equal(t, "http://example.com/image.jpg", item.Enclosure.URL)
	assert.Equal(t, "image/jpeg", item.Enclosure.Type)
	assert.Equal(t, "12345", item.Enclosure.Length)
}

// TestParsePubDate verifies that ParsePubDate correctly parses various date formats.
func TestParsePubDate(t *testing.T) {
	testCases := []struct {
		name     string
		pubDate  string
		expected time.Time
		useNow   bool
	}{
		{
			name:     "RFC1123",
			pubDate:  "Mon, 15 Jan 2024 10:30:00 GMT",
			expected: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:     "RFC1123Z",
			pubDate:  "Mon, 15 Jan 2024 10:30:00 +0000",
			expected: time.Date(2024, 1, 15, 10, 30, 0, 0, time.FixedZone("", 0)),
		},
		{
			name:     "RFC822",
			pubDate:  "15 Jan 24 10:30 GMT",
			expected: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:     "ISO8601 with Z",
			pubDate:  "2024-01-15T10:30:00Z",
			expected: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:     "ISO8601 with offset",
			pubDate:  "2024-01-15T10:30:00-05:00",
			expected: time.Date(2024, 1, 15, 10, 30, 0, 0, time.FixedZone("", -5*3600)),
		},
		{
			name:    "empty string",
			pubDate: "",
			useNow:  true,
		},
		{
			name:    "invalid format",
			pubDate: "not a date",
			useNow:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			item := rss.Item{PubDate: tc.pubDate}
			result := item.ParsePubDate()

			if tc.useNow {
				// Should be close to current time
				assert.WithinDuration(t, time.Now(), result, 2*time.Second)
			} else {
				assert.Equal(t, tc.expected.Year(), result.Year())
				assert.Equal(t, tc.expected.Month(), result.Month())
				assert.Equal(t, tc.expected.Day(), result.Day())
				assert.Equal(t, tc.expected.Hour(), result.Hour())
				assert.Equal(t, tc.expected.Minute(), result.Minute())
			}
		})
	}
}

// TestGetEnclosureURL verifies that GetEnclosureURL returns the correct URL.
func TestGetEnclosureURL(t *testing.T) {
	t.Run("with enclosure", func(t *testing.T) {
		item := rss.Item{
			Enclosure: &rss.Enclosure{
				URL:  "http://example.com/media.mp3",
				Type: "audio/mpeg",
			},
		}
		assert.Equal(t, "http://example.com/media.mp3", item.GetEnclosureURL())
	})

	t.Run("without enclosure", func(t *testing.T) {
		item := rss.Item{}
		assert.Equal(t, "", item.GetEnclosureURL())
	})
}

// TestIsImageEnclosure verifies that IsImageEnclosure correctly identifies image types.
func TestIsImageEnclosure(t *testing.T) {
	testCases := []struct {
		name     string
		mimeType string
		expected bool
	}{
		{"jpeg", "image/jpeg", true},
		{"png", "image/png", true},
		{"gif", "image/gif", true},
		{"webp", "image/webp", true},
		{"mp3", "audio/mpeg", false},
		{"mp4", "video/mp4", false},
		{"pdf", "application/pdf", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			item := rss.Item{
				Enclosure: &rss.Enclosure{
					URL:  "http://example.com/file",
					Type: tc.mimeType,
				},
			}
			assert.Equal(t, tc.expected, item.IsImageEnclosure())
		})
	}

	t.Run("nil enclosure", func(t *testing.T) {
		item := rss.Item{}
		assert.False(t, item.IsImageEnclosure())
	})
}
