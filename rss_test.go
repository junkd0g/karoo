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
