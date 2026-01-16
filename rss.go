/*
Package rss provides a simple client for fetching and parsing RSS feeds using Go's standard libraries.
It supports configurable HTTP client options such as custom timeouts or custom HTTP clients.
The package includes a basic RSS struct that represents the typical XML structure of an RSS feed.
*/
package rss

import (
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"time"
)

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
	Category    string     `xml:"category"`
	Enclosure   *Enclosure `xml:"enclosure"`
}

// Enclosure represents a media enclosure in an RSS item.
type Enclosure struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Length string `xml:"length,attr"`
}

// ParsePubDate attempts to parse the PubDate field into a time.Time.
// Returns the parsed time or the current time if parsing fails.
func (item *Item) ParsePubDate() time.Time {
	if item.PubDate == "" {
		return time.Now()
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
			return t
		}
	}

	return time.Now()
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

// Client is used for fetching and parsing RSS feeds.
type Client struct {
	httpClient *http.Client
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
	}

	for _, opt := range opts {
		opt(client)
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
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// GetFeed fetches the RSS feed from the specified URL and parses it.
// It returns the RSS struct or an error if the request or parsing fails.
func (c *Client) GetFeed(url string) (RSS, error) {
	// Perform an HTTP GET request to retrieve the RSS feed.
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return RSS{}, err
	}
	defer resp.Body.Close()

	// Ensure the HTTP response status is OK.
	if resp.StatusCode != http.StatusOK {
		return RSS{}, errors.New("failed to fetch RSS feed: " + resp.Status)
	}

	// Read the response body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return RSS{}, err
	}

	// Unmarshal the XML response into the RSS struct.
	var feed RSS
	if err := xml.Unmarshal(body, &feed); err != nil {
		return RSS{}, err
	}

	return feed, nil
}
