/*
Package rss provides a simple client for fetching and parsing RSS feeds using Go's standard libraries.
It supports configurable HTTP client options such as custom timeouts or custom HTTP clients.
The package includes a basic RSS struct that represents the typical XML structure of an RSS feed.
*/
package rss

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// maxResponseSize is the maximum allowed RSS response body size (10MB).
const maxResponseSize = 10 * 1024 * 1024

// RSS represents the structure of an RSS feed.
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel Channel  `xml:"channel"`
}

// Channel represents an RSS channel.
type Channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Language    string `xml:"language"`
	Image       *Image `xml:"image"`
	Items       []Item `xml:"item"`
}

// Image represents a channel image.
type Image struct {
	URL   string `xml:"url"`
	Title string `xml:"title"`
	Link  string `xml:"link"`
}

// Item represents an RSS item.
type Item struct {
	Title       string     `xml:"title"`
	Link        string     `xml:"link"`
	Description string     `xml:"description"`
	PubDate     string     `xml:"pubDate"`
	GUID        string     `xml:"guid"`
	Author      string     `xml:"author"`
	Category    []string   `xml:"category"`
	Enclosure   *Enclosure `xml:"enclosure"`
}

// Enclosure represents a media enclosure in an RSS item.
type Enclosure struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Length int64  `xml:"length,attr"`
}

// ParsePubDate attempts to parse the PubDate field into a time.Time.
// Returns the parsed time and a nil error on success, or a zero time.Time and an error if parsing fails.
func (item *Item) ParsePubDate() (time.Time, error) {
	if item.PubDate == "" {
		return time.Time{}, fmt.Errorf("empty pub date")
	}

	formats := []string{
		time.RFC1123,
		time.RFC1123Z,
		time.RFC822,
		time.RFC822Z,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
		"02 Jan 2006 15:04:05 -0700",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, item.PubDate); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse pub date: %s", item.PubDate)
}

// GetEnclosureURL returns the enclosure URL if present, empty string otherwise.
func (item *Item) GetEnclosureURL() string {
	if item.Enclosure != nil {
		return item.Enclosure.URL
	}
	return ""
}

// IsImageEnclosure returns true if the enclosure is an image type.
func (item *Item) IsImageEnclosure() bool {
	if item.Enclosure == nil {
		return false
	}
	switch item.Enclosure.Type {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	}
	return false
}

// defaultUserAgent is the default User-Agent header sent with requests.
const defaultUserAgent = "karoo/1.0 (+https://github.com/junkd0g/karoo)"

// Client is used for fetching and parsing RSS feeds.
type Client struct {
	httpClient *http.Client
	timeout    *time.Duration
	userAgent  string
}

// ClientOption defines a function type for configuring the RSS Client.
type ClientOption func(*Client)

// NewClient creates a new RSS Client with optional configuration options.
// By default, it uses an HTTP client with a 10-second timeout.
func NewClient(opts ...ClientOption) *Client {
	client := &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		userAgent: defaultUserAgent,
	}

	for _, opt := range opts {
		opt(client)
	}

	// Apply stored timeout after all options, so WithTimeout works
	// correctly regardless of ordering with WithHTTPClient.
	if client.timeout != nil {
		client.httpClient.Timeout = *client.timeout
	}

	return client
}

// WithHTTPClient sets a custom HTTP client for the RSS Client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithTimeout sets a custom timeout for HTTP requests.
// This option is applied after all other options, so it works correctly
// regardless of ordering with WithHTTPClient.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.timeout = &timeout
	}
}

// WithUserAgent sets a custom User-Agent header for HTTP requests.
func WithUserAgent(userAgent string) ClientOption {
	return func(c *Client) {
		c.userAgent = userAgent
	}
}

// GetFeed fetches the RSS feed from the specified URL and parses it.
// It accepts a context for cancellation and deadline control.
// It returns the RSS struct or an error if the request or parsing fails.
func (c *Client) GetFeed(ctx context.Context, url string) (RSS, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return RSS{}, err
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return RSS{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return RSS{}, fmt.Errorf("failed to fetch RSS feed: %s", resp.Status)
	}

	// Validate that the response content type is XML-based.
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !strings.Contains(contentType, "xml") {
		return RSS{}, fmt.Errorf("unexpected content type: %s", contentType)
	}

	// Read response body with a size limit to prevent OOM from oversized responses.
	limitedReader := io.LimitReader(resp.Body, maxResponseSize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return RSS{}, err
	}
	if int64(len(body)) > maxResponseSize {
		return RSS{}, fmt.Errorf("response body exceeds maximum size of %d bytes", maxResponseSize)
	}

	// Detect Atom feeds which would silently produce empty results.
	peek := string(body[:min(len(body), 1024)])
	if strings.Contains(peek, "<feed") {
		return RSS{}, fmt.Errorf("unsupported feed format: Atom feeds are not supported")
	}

	var feed RSS
	if err := xml.Unmarshal(body, &feed); err != nil {
		return RSS{}, err
	}

	return feed, nil
}
