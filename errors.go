package rss

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by this package. Use errors.Is to test for them.
// Every error keeps the same message text as previous releases, so callers
// that match on strings continue to work.
var (
	// ErrHTTPStatus is matched by any *HTTPStatusError.
	ErrHTTPStatus = errors.New("failed to fetch RSS feed")

	// ErrUnexpectedContentType is returned when the server responds with
	// text/html, which almost always means a login or error page.
	ErrUnexpectedContentType = errors.New("unexpected content type")

	// ErrResponseTooLarge is returned when the response body exceeds the
	// client's maximum response size.
	ErrResponseTooLarge = errors.New("response body exceeds maximum size")

	// ErrUnsupportedFormat is returned when the document is well-formed XML
	// but is not an RSS feed (for example Atom or RSS 1.0 RDF).
	ErrUnsupportedFormat = errors.New("unsupported feed format")

	// ErrParseFeed wraps XML decoding failures.
	ErrParseFeed = errors.New("failed to parse feed")

	// ErrParsePubDate is returned by the date parsing helpers when the value
	// is empty or in an unrecognised format.
	ErrParsePubDate = errors.New("unable to parse pub date")

	// ErrNotModified is returned by Fetch when the server answers a
	// conditional request with 304 Not Modified.
	ErrNotModified = errors.New("feed not modified")
)

// HTTPStatusError is returned when the server responds with a status other
// than 200 OK or 304 Not Modified. It matches ErrHTTPStatus with errors.Is.
type HTTPStatusError struct {
	StatusCode int
	Status     string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("failed to fetch RSS feed: %s", e.Status)
}

// Is reports whether target is ErrHTTPStatus.
func (e *HTTPStatusError) Is(target error) bool {
	return target == ErrHTTPStatus
}
