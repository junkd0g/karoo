package rss

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
)

// Parse reads an RSS document from r and returns the decoded feed. It uses
// DefaultCharsetReader for non-UTF-8 documents and applies no size limit; the
// caller controls the reader. Use Client.Parse to apply a client's charset
// reader and size limit.
func Parse(r io.Reader) (RSS, error) {
	return parseFeed(r, DefaultCharsetReader)
}

// ParseBytes decodes an RSS document held in memory. See Parse.
func ParseBytes(data []byte) (RSS, error) {
	return Parse(bytes.NewReader(data))
}

// parseFeed streams tokens from r, checks that the root element is <rss>,
// and decodes the document.
func parseFeed(r io.Reader, charsetReader CharsetReader) (RSS, error) {
	dec := xml.NewDecoder(r)
	dec.CharsetReader = charsetReader

	start, err := rootElement(dec)
	if err != nil {
		return RSS{}, wrapParseError(err)
	}

	switch start.Name.Local {
	case "rss":
	case "feed":
		return RSS{}, fmt.Errorf("%w: Atom feeds are not supported", ErrUnsupportedFormat)
	case "RDF":
		return RSS{}, fmt.Errorf("%w: RSS 1.0 (RDF) feeds are not supported", ErrUnsupportedFormat)
	default:
		return RSS{}, fmt.Errorf("%w: unexpected root element <%s>", ErrUnsupportedFormat, start.Name.Local)
	}

	var feed RSS
	if err := dec.DecodeElement(&feed, &start); err != nil {
		return RSS{}, wrapParseError(err)
	}
	return feed, nil
}

// rootElement advances the decoder to the document's first start element.
func rootElement(dec *xml.Decoder) (xml.StartElement, error) {
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return xml.StartElement{}, errors.New("EOF: document contains no elements")
			}
			return xml.StartElement{}, err
		}
		if start, ok := tok.(xml.StartElement); ok {
			return start, nil
		}
	}
}

// wrapParseError attaches ErrParseFeed to decoder errors while letting the
// size-limit sentinel through unchanged.
func wrapParseError(err error) error {
	if errors.Is(err, ErrResponseTooLarge) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrParseFeed, err)
}

// limitedReader returns ErrResponseTooLarge once more than limit bytes have
// been read, instead of the silent EOF that io.LimitReader produces.
type limitedReader struct {
	r         io.Reader
	remaining int64
	limit     int64
}

func newLimitedReader(r io.Reader, limit int64) *limitedReader {
	return &limitedReader{r: r, remaining: limit + 1, limit: limit}
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.remaining <= 0 {
		return 0, fmt.Errorf("%w of %d bytes", ErrResponseTooLarge, l.limit)
	}
	if int64(len(p)) > l.remaining {
		p = p[:l.remaining]
	}
	n, err := l.r.Read(p)
	l.remaining -= int64(n)
	if l.remaining <= 0 {
		// We read the sentinel byte past the limit; report the overflow now
		// rather than handing partial data to the decoder.
		return 0, fmt.Errorf("%w of %d bytes", ErrResponseTooLarge, l.limit)
	}
	return n, err
}
