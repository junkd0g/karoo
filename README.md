[![Go Report Card](https://goreportcard.com/badge/github.com/junkd0g/karoo)](https://goreportcard.com/report/github.com/junkd0g/karoo)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![GoDoc](https://pkg.go.dev/badge/github.com/junkd0g/karoo.svg)](https://pkg.go.dev/github.com/junkd0g/karoo)

# karoo

A lightweight, efficient RSS feed client for Go that provides a simple interface for fetching and parsing RSS feeds with configurable HTTP client options.

## Features

- Simple, clean API
- Configurable HTTP client (timeouts, custom clients)
- Context support for cancellation and deadline control
- Response size limit to prevent OOM from oversized feeds
- Content-Type validation and Atom feed detection
- Comprehensive error handling
- Zero external dependencies (uses only Go standard library)
- Well-tested with 100% statement coverage
- Thread-safe operations

## Installation

```bash
go get -u github.com/junkd0g/karoo
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"

	rss "github.com/junkd0g/karoo"
)

func main() {
	client := rss.NewClient()

	feed, err := client.GetFeed(context.Background(), "https://news.google.com/rss")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Feed Title: %s\n", feed.Channel.Title)
	fmt.Printf("Feed Description: %s\n", feed.Channel.Description)
	fmt.Printf("Number of items: %d\n", len(feed.Channel.Items))

	for _, item := range feed.Channel.Items {
		fmt.Printf("- %s: %s\n", item.Title, item.Link)
	}
}
```

## Configuration Options

### Custom Timeout

```go
client := rss.NewClient(rss.WithTimeout(30 * time.Second))
```

### Custom HTTP Client

```go
httpClient := &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  true,
	},
}

client := rss.NewClient(rss.WithHTTPClient(httpClient))
```

### Multiple Configuration Options

`WithTimeout` is always applied after all other options, so it works correctly regardless of ordering with `WithHTTPClient`:

```go
client := rss.NewClient(
	rss.WithHTTPClient(customHTTPClient),
	rss.WithTimeout(20 * time.Second),
)
```

## Error Handling

```go
feed, err := client.GetFeed(context.Background(), "https://example.com/feed.xml")
if err != nil {
	switch {
	case strings.Contains(err.Error(), "failed to fetch RSS feed"):
		// Handle HTTP errors (404, 500, etc.)
		log.Printf("HTTP error: %v", err)
	case strings.Contains(err.Error(), "unexpected content type"):
		// Handle non-XML responses (e.g., HTML login pages)
		log.Printf("Content type error: %v", err)
	case strings.Contains(err.Error(), "Atom feeds are not supported"):
		// Handle Atom feeds
		log.Printf("Unsupported format: %v", err)
	case strings.Contains(err.Error(), "Client.Timeout"):
		// Handle timeout errors
		log.Printf("Request timeout: %v", err)
	default:
		// Handle other errors (network, XML parsing, etc.)
		log.Printf("Error fetching feed: %v", err)
	}
	return
}
```

## API Reference

### Types

#### RSS
```go
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel Channel  `xml:"channel"`
}
```

#### Channel
```go
type Channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Language    string `xml:"language"`
	Image       *Image `xml:"image"`
	Items       []Item `xml:"item"`
}
```

#### Image
```go
type Image struct {
	URL   string `xml:"url"`
	Title string `xml:"title"`
	Link  string `xml:"link"`
}
```

#### Item
```go
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
```

#### Enclosure
```go
type Enclosure struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Length int64  `xml:"length,attr"`
}
```

#### Client
```go
type Client struct {
	// contains filtered or unexported fields
}
```

### Functions

#### NewClient
```go
func NewClient(opts ...ClientOption) *Client
```
Creates a new RSS client with optional configuration. Default timeout is 10 seconds.

#### WithTimeout
```go
func WithTimeout(timeout time.Duration) ClientOption
```
Sets a custom timeout for HTTP requests. Applied after all other options, so it works correctly regardless of ordering with `WithHTTPClient`.

#### WithHTTPClient
```go
func WithHTTPClient(httpClient *http.Client) ClientOption
```
Sets a custom HTTP client for RSS requests.

### Methods

#### GetFeed
```go
func (c *Client) GetFeed(ctx context.Context, url string) (RSS, error)
```
Fetches and parses an RSS feed from the specified URL. Accepts a context for cancellation and deadline control. Returns the parsed RSS struct or an error. Sets a `User-Agent` header on requests. Validates the response Content-Type is XML-based, limits response body size to 10MB, and detects Atom feeds (returning an error since only RSS is supported).

#### ParsePubDate
```go
func (item *Item) ParsePubDate() (time.Time, error)
```
Attempts to parse the PubDate field into a `time.Time`. Returns the parsed time and nil on success, or a zero `time.Time` and an error if the date is empty or in an unrecognized format. Supports RFC1123, RFC1123Z, RFC822, RFC822Z, ISO 8601, and other common date formats.

#### GetEnclosureURL
```go
func (item *Item) GetEnclosureURL() string
```
Returns the enclosure URL if present, empty string otherwise.

#### IsImageEnclosure
```go
func (item *Item) IsImageEnclosure() bool
```
Returns true if the enclosure MIME type is an image (jpeg, png, gif, or webp).

## Examples

### Basic Usage with Error Handling

```go
package main

import (
	"context"
	"fmt"
	"log"

	rss "github.com/junkd0g/karoo"
)

func main() {
	client := rss.NewClient()

	feeds := []string{
		"https://rss.cnn.com/rss/edition.rss",
		"https://feeds.bbci.co.uk/news/rss.xml",
		"https://rss.nytimes.com/services/xml/rss/nyt/World.xml",
	}

	for _, feedURL := range feeds {
		feed, err := client.GetFeed(context.Background(), feedURL)
		if err != nil {
			log.Printf("Error fetching %s: %v", feedURL, err)
			continue
		}

		fmt.Printf("\n=== %s ===\n", feed.Channel.Title)
		for i, item := range feed.Channel.Items {
			if i >= 3 { // Show only first 3 items
				break
			}
			fmt.Printf("%d. %s\n", i+1, item.Title)
		}
	}
}
```

### Concurrent Feed Fetching

```go
package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	rss "github.com/junkd0g/karoo"
)

func main() {
	client := rss.NewClient(rss.WithTimeout(5 * time.Second))

	feeds := []string{
		"https://rss.cnn.com/rss/edition.rss",
		"https://feeds.bbci.co.uk/news/rss.xml",
		"https://rss.nytimes.com/services/xml/rss/nyt/World.xml",
	}

	var wg sync.WaitGroup
	results := make(chan string, len(feeds))

	ctx := context.Background()
	for _, feedURL := range feeds {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			feed, err := client.GetFeed(ctx, url)
			if err != nil {
				results <- fmt.Sprintf("Error fetching %s: %v", url, err)
				return
			}

			results <- fmt.Sprintf("%s: %d items", feed.Channel.Title, len(feed.Channel.Items))
		}(feedURL)
	}

	wg.Wait()
	close(results)

	for result := range results {
		fmt.Println(result)
	}
}
```

## Testing

Run the test suite:

```bash
go test -v
```

Run tests with coverage:

```bash
go test -v -cover
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Author

**Iordanis Paschalidis** - [junkd0g](https://github.com/junkd0g)
