package rss

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultMaxResponseSize is the default cap on response body size (10 MB).
	DefaultMaxResponseSize int64 = 10 * 1024 * 1024

	// DefaultTimeout is the default HTTP client timeout.
	DefaultTimeout = 10 * time.Second

	modulePath = "github.com/junkd0g/karoo"
	acceptFeed = "application/rss+xml, application/rdf+xml;q=0.9, application/xml;q=0.9, text/xml;q=0.8, */*;q=0.7"
)

// defaultUserAgent reports the User-Agent sent when none is configured. The
// version comes from the build info of the binary importing this module.
var defaultUserAgent = sync.OnceValue(func() string {
	version := "dev"
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			if dep.Path == modulePath && dep.Version != "" {
				version = dep.Version
				break
			}
		}
	}
	return fmt.Sprintf("karoo/%s (+https://%s)", version, modulePath)
})

// Client is used for fetching and parsing RSS feeds. A Client is safe for
// concurrent use once constructed.
type Client struct {
	httpClient      *http.Client
	timeout         *time.Duration
	userAgent       string
	maxResponseSize int64
	charsetReader   *CharsetReader // pointer keeps Client comparable, as it was in v1.2.0
}

// ClientOption defines a function type for configuring the RSS Client.
type ClientOption func(*Client)

// NewClient creates a new RSS Client with optional configuration options.
// By default, it uses an HTTP client with a 10-second timeout, a 10 MB
// response size limit, and DefaultCharsetReader.
func NewClient(opts ...ClientOption) *Client {
	defaultCR := CharsetReader(DefaultCharsetReader)
	client := &Client{
		httpClient:      &http.Client{Timeout: DefaultTimeout},
		userAgent:       defaultUserAgent(),
		maxResponseSize: DefaultMaxResponseSize,
		charsetReader:   &defaultCR,
	}

	for _, opt := range opts {
		opt(client)
	}

	if client.httpClient == nil {
		client.httpClient = &http.Client{Timeout: DefaultTimeout}
	}

	// Apply the stored timeout after all options so WithTimeout works
	// regardless of ordering with WithHTTPClient. The http.Client is copied
	// so a caller's shared client is never mutated.
	if client.timeout != nil {
		hc := *client.httpClient
		hc.Timeout = *client.timeout
		client.httpClient = &hc
	}

	return client
}

// WithHTTPClient sets a custom HTTP client for the RSS Client. Passing nil
// keeps the default client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithTimeout sets a custom timeout for HTTP requests. It is applied after
// all other options, so it works regardless of ordering with WithHTTPClient,
// and it never modifies the http.Client passed to WithHTTPClient.
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

// WithMaxResponseSize sets the maximum response body size in bytes. A value
// of zero or less disables the limit.
func WithMaxResponseSize(n int64) ClientOption {
	return func(c *Client) {
		c.maxResponseSize = n
	}
}

// WithCharsetReader sets the function used to decode feeds that declare a
// non-UTF-8 encoding. Passing nil restores DefaultCharsetReader.
func WithCharsetReader(cr CharsetReader) ClientOption {
	return func(c *Client) {
		if cr == nil {
			cr = DefaultCharsetReader
		}
		c.charsetReader = &cr
	}
}

// FetchOption configures a single Fetch call.
type FetchOption func(*fetchOptions)

type fetchOptions struct {
	etag         string
	lastModified string
}

// WithETag sends an If-None-Match header so the server can answer 304 Not
// Modified. Pass the ETag from a previous Result.
func WithETag(etag string) FetchOption {
	return func(o *fetchOptions) {
		o.etag = etag
	}
}

// WithLastModified sends an If-Modified-Since header. Pass the LastModified
// value from a previous Result verbatim; servers compare it as a string.
func WithLastModified(lastModified string) FetchOption {
	return func(o *fetchOptions) {
		o.lastModified = lastModified
	}
}

// Result is the outcome of a Fetch call.
type Result struct {
	// Feed is the parsed feed. It is empty when NotModified is true.
	Feed RSS

	// StatusCode is the HTTP status the server returned.
	StatusCode int

	// ETag and LastModified echo the response headers, for use with
	// WithETag and WithLastModified on the next Fetch.
	ETag         string
	LastModified string

	// NotModified is true when the server answered 304 to a conditional
	// request, in which case Feed is empty.
	NotModified bool
}

// Fetch retrieves and parses a feed, with support for conditional requests.
// When the server answers 304 Not Modified, Fetch returns a Result with
// NotModified set and an error wrapping ErrNotModified.
func (c *Client) Fetch(ctx context.Context, url string, opts ...FetchOption) (*Result, error) {
	var fo fetchOptions
	for _, opt := range opts {
		opt(&fo)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", acceptFeed)
	if fo.etag != "" {
		req.Header.Set("If-None-Match", fo.etag)
	}
	if fo.lastModified != "" {
		req.Header.Set("If-Modified-Since", fo.lastModified)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	result := &Result{
		StatusCode:   resp.StatusCode,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotModified:
		result.NotModified = true
		return result, ErrNotModified
	default:
		return result, &HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status}
	}

	if err := checkContentType(resp.Header.Get("Content-Type")); err != nil {
		return result, err
	}

	feed, err := c.Parse(resp.Body)
	if err != nil {
		return result, err
	}
	result.Feed = feed
	return result, nil
}

// GetFeed fetches the RSS feed from the specified URL and parses it.
// It accepts a context for cancellation and deadline control.
// It returns the RSS struct or an error if the request or parsing fails.
func (c *Client) GetFeed(ctx context.Context, url string) (RSS, error) {
	result, err := c.Fetch(ctx, url)
	if err != nil {
		return RSS{}, err
	}
	return result.Feed, nil
}

// Parse decodes a feed from r using the client's charset reader and response
// size limit. It is useful for feeds obtained through other means, such as a
// cache or a custom HTTP layer.
func (c *Client) Parse(r io.Reader) (RSS, error) {
	if c.maxResponseSize > 0 {
		r = newLimitedReader(r, c.maxResponseSize)
	}
	return parseFeed(r, *c.charsetReader)
}

// checkContentType rejects HTML responses, which are almost always login or
// error pages. Every other type is accepted because feeds are routinely
// served as text/plain or application/octet-stream by misconfigured hosts.
func checkContentType(contentType string) error {
	if contentType == "" {
		return nil
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType, _, _ = strings.Cut(contentType, ";")
		mediaType = strings.TrimSpace(mediaType)
	}
	if strings.EqualFold(mediaType, "text/html") || strings.EqualFold(mediaType, "application/xhtml+xml") {
		return fmt.Errorf("%w: %s", ErrUnexpectedContentType, contentType)
	}
	return nil
}
