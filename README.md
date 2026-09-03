[![Go Report Card](https://goreportcard.com/badge/github.com/junkd0g/karoo)](https://goreportcard.com/report/github.com/junkd0g/karoo)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![GoDoc](https://pkg.go.dev/badge/github.com/junkd0g/karoo.svg)](https://pkg.go.dev/github.com/junkd0g/karoo)

# karoo

A lightweight RSS 2.0 feed client for Go. It fetches feeds over HTTP or parses them from any reader, using only the standard library.

## Features

- Simple, clean API with functional options
- Parse from HTTP (`GetFeed`, `Fetch`) or from any `io.Reader` / `[]byte` (`Parse`, `ParseBytes`)
- Conditional requests with `ETag` and `Last-Modified` so unchanged feeds cost a single 304 round trip
- Typed errors that work with `errors.Is` and `errors.As`
- Context support for cancellation and deadlines
- Streaming decode with a configurable response size limit
- Built-in decoding of ISO-8859-1 and Windows-1252 feeds, pluggable for anything else
- Tolerant of real-world feeds: junk `length` attributes, FeedBurner namespaces, `text/plain` content types, timezone abbreviations like `EST`
- Extension elements: `content:encoded`, `dc:creator`, `dc:date`, `media:thumbnail`, `media:content`
- Zero external dependencies, including in tests
- Safe for concurrent use

## Installation

```bash
go get github.com/junkd0g/karoo
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
	fmt.Printf("Number of items: %d\n", len(feed.Channel.Items))

	for _, item := range feed.Channel.Items {
		published, _ := item.ParsePubDate()
		fmt.Printf("- %s (%s) by %s\n", item.Title, published.Format("2006-01-02"), item.GetAuthor())
	}
}
```

## Parsing Without HTTP

`Parse` and `ParseBytes` decode a feed from memory or any reader, which is useful for cached files, test fixtures, or a custom fetch layer.

```go
data, _ := os.ReadFile("feed.xml")
feed, err := rss.ParseBytes(data)
```

`Client.Parse` does the same but applies the client's charset reader and size limit.

## Conditional Requests

Pollers should use `Fetch`, which exposes the response validators and sends them back on the next request. When the server answers `304 Not Modified`, `Fetch` returns `ErrNotModified` and a `Result` with `NotModified` set.

```go
res, err := client.Fetch(ctx, url)
if err != nil {
	return err
}
etag, lastModified := res.ETag, res.LastModified

// Later:
res, err = client.Fetch(ctx, url, rss.WithETag(etag), rss.WithLastModified(lastModified))
if errors.Is(err, rss.ErrNotModified) {
	// Nothing changed. res.StatusCode == 304, res.Feed is empty.
}
```

## Configuration Options

```go
client := rss.NewClient(
	rss.WithTimeout(30*time.Second),          // default 10s
	rss.WithHTTPClient(customHTTPClient),      // never mutated by karoo
	rss.WithUserAgent("my-app/2.0"),           // default "karoo/<version> (+https://github.com/junkd0g/karoo)"
	rss.WithMaxResponseSize(50*1024*1024),     // default 10 MB, 0 disables the limit
	rss.WithCharsetReader(charset.NewReaderLabel), // e.g. golang.org/x/net/html/charset for every encoding
)
```

`WithTimeout` is applied after all other options, so it works regardless of ordering with `WithHTTPClient`, and it copies the HTTP client rather than modifying yours.

### Character Encodings

Feeds that declare UTF-8, US-ASCII, ISO-8859-1, or Windows-1252 are decoded out of the box. For other encodings, supply a `CharsetReader`. The one from `golang.org/x/net/html/charset` covers everything:

```go
import "golang.org/x/net/html/charset"

client := rss.NewClient(rss.WithCharsetReader(charset.NewReaderLabel))
```

## Error Handling

All errors wrap a sentinel, so use `errors.Is` and `errors.As` rather than string matching. The message text from earlier versions is preserved for callers that still match on it.

```go
feed, err := client.GetFeed(ctx, url)
if err != nil {
	var statusErr *rss.HTTPStatusError
	switch {
	case errors.As(err, &statusErr):
		log.Printf("HTTP %d: %s", statusErr.StatusCode, statusErr.Status)
	case errors.Is(err, rss.ErrUnexpectedContentType):
		log.Print("server returned an HTML page, not a feed")
	case errors.Is(err, rss.ErrUnsupportedFormat):
		log.Print("Atom or RSS 1.0 feed; only RSS 2.0 is supported")
	case errors.Is(err, rss.ErrResponseTooLarge):
		log.Print("feed exceeds the size limit")
	case errors.Is(err, rss.ErrParseFeed):
		log.Printf("malformed XML: %v", err)
	case errors.Is(err, context.DeadlineExceeded):
		log.Print("timed out")
	default:
		log.Printf("network error: %v", err)
	}
	return
}
```

| Error | Meaning |
|---|---|
| `*HTTPStatusError` (matches `ErrHTTPStatus`) | Status other than 200 or 304. Carries `StatusCode` and `Status`. |
| `ErrNotModified` | Server answered 304 to a conditional `Fetch`. |
| `ErrUnexpectedContentType` | Response was `text/html`, almost always a login or error page. |
| `ErrResponseTooLarge` | Body exceeded `WithMaxResponseSize`. |
| `ErrUnsupportedFormat` | Well-formed XML that is Atom, RSS 1.0 (RDF), or not a feed. |
| `ErrParseFeed` | XML decoding failed, including unsupported charsets. |
| `ErrParsePubDate` | A date helper got an empty or unrecognised value. |

## API Reference

### Types

```go
type RSS struct {
	XMLName xml.Name
	Version string
	Channel Channel
}

type Channel struct {
	Title, Link, Description, Language string
	Copyright, ManagingEditor, WebMaster string
	PubDate, LastBuildDate               string
	Generator, Docs                      string
	TTL                                  int      // minutes; 0 when absent or unparseable
	Category                             []string
	Image                                *Image
	Items                                []Item
}

type Image struct {
	URL, Title, Link string
}

type Item struct {
	Title, Link, Description, PubDate string
	GUID                              string
	GUIDIsPermaLink                   bool   // true when <guid> is present without isPermaLink="false"
	Author                            string // <author>, by spec an email address
	Creator                           string // <dc:creator>, usually a display name
	Date                              string // <dc:date>
	Content                           string // <content:encoded>, full HTML body
	Category                          []string
	Comments                          string
	Enclosure                         *Enclosure
	Source                            *Source
	MediaThumbnails                   []MediaContent // <media:thumbnail>
	MediaContents                     []MediaContent // <media:content>
}

type Enclosure struct {
	URL, Type string
	Length    int64 // 0 when absent or unparseable
}

type Source struct {
	URL, Name string
}

type MediaContent struct {
	URL, Type, Medium string
	Width, Height     int // 0 when absent or unparseable
}

type Result struct {
	Feed         RSS
	StatusCode   int
	ETag         string
	LastModified string
	NotModified  bool
}
```

### Functions

| Function | Description |
|---|---|
| `NewClient(opts ...ClientOption) *Client` | Creates a client. Defaults: 10s timeout, 10 MB limit, `DefaultCharsetReader`. |
| `Parse(r io.Reader) (RSS, error)` | Decodes a feed from a reader with no size limit. |
| `ParseBytes(data []byte) (RSS, error)` | Decodes a feed held in memory. |
| `ParseDate(value string) (time.Time, error)` | Parses RFC 822, RFC 1123, RFC 3339 and common loose variants. Resolves timezone abbreviations. |
| `DefaultCharsetReader(charset string, r io.Reader) (io.Reader, error)` | Handles UTF-8, US-ASCII, ISO-8859-1, Windows-1252. |

### Client Options

| Option | Description |
|---|---|
| `WithTimeout(d)` | HTTP timeout. Applied last, never mutates a supplied client. |
| `WithHTTPClient(c)` | Custom `*http.Client`. `nil` keeps the default. |
| `WithUserAgent(s)` | Custom `User-Agent` header. |
| `WithMaxResponseSize(n)` | Response body cap in bytes. `0` or less disables it. |
| `WithCharsetReader(f)` | Decoder for non-UTF-8 feeds. `nil` restores the default. |

### Client Methods

| Method | Description |
|---|---|
| `GetFeed(ctx, url) (RSS, error)` | Fetches and parses a feed. |
| `Fetch(ctx, url, opts ...FetchOption) (*Result, error)` | Like `GetFeed` but returns validators and supports `WithETag` / `WithLastModified`. |
| `Parse(r io.Reader) (RSS, error)` | Decodes with the client's charset reader and size limit. |

### Item and Channel Methods

| Method | Description |
|---|---|
| `Item.ParsePubDate()` | Parses `<pubDate>`, falling back to `<dc:date>`. |
| `Item.GetAuthor()` | `<dc:creator>` if set, otherwise `<author>`. |
| `Item.GetContent()` | `<content:encoded>` if set, otherwise `<description>`. |
| `Item.GetEnclosureURL()` | Enclosure URL or empty string. |
| `Item.IsImageEnclosure()` / `IsAudioEnclosure()` / `IsVideoEnclosure()` | MIME type prefix checks, case-insensitive, parameters ignored. |
| `Enclosure.IsImage()` / `IsAudio()` / `IsVideo()` | Same checks on the enclosure itself. |
| `Channel.ParsePubDate()` / `ParseLastBuildDate()` | Channel-level date parsing. |

## Limitations

Only RSS 2.0 (and the compatible 0.9x family) is parsed. Atom and RSS 1.0 (RDF) documents are detected and rejected with `ErrUnsupportedFormat`.

## Testing

```bash
go test -race -cover ./...
```

Fuzz targets are included:

```bash
go test -run='^$' -fuzz=FuzzParseBytes -fuzztime=30s ./...
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

CI runs formatting, vet, race tests, fuzz smoke tests, golangci-lint, govulncheck, and an API compatibility check against the last release.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Author

**Iordanis Paschalidis** - [junkd0g](https://github.com/junkd0g)
