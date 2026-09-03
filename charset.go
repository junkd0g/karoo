package rss

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// CharsetReader converts input in the named charset to UTF-8. It has the same
// signature as xml.Decoder.CharsetReader and can be supplied with
// WithCharsetReader, for example using golang.org/x/net/html/charset.NewReaderLabel
// to support every encoding.
type CharsetReader func(charset string, input io.Reader) (io.Reader, error)

// DefaultCharsetReader handles the encodings found in the vast majority of
// real-world feeds without any external dependency: UTF-8 (and its aliases),
// US-ASCII, ISO-8859-1 / Latin-1, and Windows-1252. Any other declared
// encoding returns an error.
func DefaultCharsetReader(charset string, input io.Reader) (io.Reader, error) {
	switch normalizeCharset(charset) {
	case "utf-8", "utf8", "us-ascii", "ascii":
		return input, nil
	case "iso-8859-1", "iso8859-1", "iso_8859-1", "latin1", "latin-1", "l1", "cp819":
		return &singleByteReader{r: bufio.NewReader(input), table: &latin1Table}, nil
	case "windows-1252", "cp1252", "win-1252", "x-cp1252":
		return &singleByteReader{r: bufio.NewReader(input), table: &cp1252Table}, nil
	}
	return nil, fmt.Errorf("unsupported charset %q", charset)
}

func normalizeCharset(charset string) string {
	return strings.ToLower(strings.TrimSpace(charset))
}

// singleByteReader transcodes a single-byte encoding to UTF-8 using a
// 256-entry rune table.
type singleByteReader struct {
	r     io.ByteReader
	table *[256]rune
	buf   [utf8.UTFMax]byte
	pend  []byte
	err   error
}

func (s *singleByteReader) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		if len(s.pend) > 0 {
			c := copy(p[n:], s.pend)
			s.pend = s.pend[c:]
			n += c
			continue
		}
		if s.err != nil {
			break
		}
		b, err := s.r.ReadByte()
		if err != nil {
			s.err = err
			break
		}
		size := utf8.EncodeRune(s.buf[:], s.table[b])
		s.pend = s.buf[:size]
	}
	if n == 0 && len(s.pend) == 0 && s.err != nil {
		return 0, s.err
	}
	return n, nil
}

// latin1Table maps ISO-8859-1 bytes to runes: an identity mapping.
var latin1Table = func() [256]rune {
	var t [256]rune
	for i := range t {
		t[i] = rune(i)
	}
	return t
}()

// cp1252Table maps Windows-1252 bytes to runes. It differs from Latin-1 only
// in the 0x80-0x9F range. Undefined positions map to U+FFFD.
var cp1252Table = func() [256]rune {
	t := latin1Table
	high := [32]rune{
		0x20AC, 0xFFFD, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
		0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0xFFFD, 0x017D, 0xFFFD,
		0xFFFD, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
		0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0xFFFD, 0x017E, 0x0178,
	}
	for i, r := range high {
		t[0x80+i] = r
	}
	return t
}()
