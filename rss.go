/*
Package rss provides a small client for fetching and parsing RSS 2.0 feeds
using only the Go standard library.

Feeds can be fetched over HTTP with a Client, or parsed from any io.Reader
with Parse. The Client supports timeouts, custom HTTP clients, a response
size limit, conditional requests with ETag and Last-Modified, and pluggable
charset decoding for non-UTF-8 feeds.
*/
package rss

import (
	"encoding/xml"
	"strconv"
	"strings"
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
	Title          string   `xml:"title"`
	Link           string   `xml:"link"`
	Description    string   `xml:"description"`
	Language       string   `xml:"language"`
	Copyright      string   `xml:"copyright"`
	ManagingEditor string   `xml:"managingEditor"`
	WebMaster      string   `xml:"webMaster"`
	PubDate        string   `xml:"pubDate"`
	LastBuildDate  string   `xml:"lastBuildDate"`
	Generator      string   `xml:"generator"`
	Docs           string   `xml:"docs"`
	TTL            int      `xml:"-"`
	Category       []string `xml:"category"`
	Image          *Image   `xml:"image"`
	Items          []Item   `xml:"item"`
}

// UnmarshalXML decodes a channel, tolerating a non-numeric <ttl> value.
func (ch *Channel) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type channelAlias Channel
	var aux struct {
		channelAlias
		TTL string `xml:"ttl"`
	}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	*ch = Channel(aux.channelAlias)
	ch.TTL = int(looseInt(aux.TTL))
	return nil
}

// ParsePubDate parses the channel's <pubDate>. See ParseDate.
func (ch *Channel) ParsePubDate() (time.Time, error) {
	return ParseDate(ch.PubDate)
}

// ParseLastBuildDate parses the channel's <lastBuildDate>. See ParseDate.
func (ch *Channel) ParseLastBuildDate() (time.Time, error) {
	return ParseDate(ch.LastBuildDate)
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
	GUID        string     `xml:"-"`
	Author      string     `xml:"author"`
	Category    []string   `xml:"category"`
	Comments    string     `xml:"comments"`
	Enclosure   *Enclosure `xml:"enclosure"`
	Source      *Source    `xml:"source"`

	// GUIDIsPermaLink reports whether the <guid> is a permalink URL. The RSS
	// specification defaults this to true when the attribute is absent, and
	// it is false when the item has no <guid> at all.
	GUIDIsPermaLink bool `xml:"-"`

	// Content holds the full HTML body from <content:encoded>, used by
	// WordPress and most blogging platforms.
	Content string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`

	// Creator holds the author name from <dc:creator>, which feeds often use
	// instead of <author> because <author> is meant to be an email address.
	Creator string `xml:"http://purl.org/dc/elements/1.1/ creator"`

	// Date holds <dc:date>, used by some feeds instead of <pubDate>.
	Date string `xml:"http://purl.org/dc/elements/1.1/ date"`

	// MediaThumbnails and MediaContents hold Media RSS <media:thumbnail>
	// and <media:content> elements.
	MediaThumbnails []MediaContent `xml:"http://search.yahoo.com/mrss/ thumbnail"`
	MediaContents   []MediaContent `xml:"http://search.yahoo.com/mrss/ content"`
}

// guidElement captures both the text and the isPermaLink attribute of <guid>.
type guidElement struct {
	Value       string `xml:",chardata"`
	IsPermaLink string `xml:"isPermaLink,attr"`
}

// UnmarshalXML decodes an item, capturing the <guid isPermaLink> attribute.
func (item *Item) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type itemAlias Item
	var aux struct {
		itemAlias
		GUID *guidElement `xml:"guid"`
	}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	*item = Item(aux.itemAlias)
	if aux.GUID != nil {
		item.GUID = strings.TrimSpace(aux.GUID.Value)
		item.GUIDIsPermaLink = !strings.EqualFold(strings.TrimSpace(aux.GUID.IsPermaLink), "false")
	}
	return nil
}

// ParsePubDate parses the item's <pubDate>, falling back to <dc:date> when
// <pubDate> is empty. See ParseDate for the supported formats.
func (item *Item) ParsePubDate() (time.Time, error) {
	if strings.TrimSpace(item.PubDate) == "" && strings.TrimSpace(item.Date) != "" {
		return ParseDate(item.Date)
	}
	return ParseDate(item.PubDate)
}

// GetAuthor returns <dc:creator> when present, otherwise <author>.
func (item *Item) GetAuthor() string {
	if item.Creator != "" {
		return item.Creator
	}
	return item.Author
}

// GetContent returns <content:encoded> when present, otherwise <description>.
func (item *Item) GetContent() string {
	if item.Content != "" {
		return item.Content
	}
	return item.Description
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
	return item.Enclosure != nil && item.Enclosure.IsImage()
}

// IsAudioEnclosure returns true if the enclosure is an audio type.
func (item *Item) IsAudioEnclosure() bool {
	return item.Enclosure != nil && item.Enclosure.IsAudio()
}

// IsVideoEnclosure returns true if the enclosure is a video type.
func (item *Item) IsVideoEnclosure() bool {
	return item.Enclosure != nil && item.Enclosure.IsVideo()
}

// Enclosure represents a media enclosure in an RSS item.
type Enclosure struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Length int64  `xml:"-"`
}

// UnmarshalXML decodes an enclosure, tolerating a non-numeric length
// attribute, which many real feeds emit. Unparseable lengths become zero.
func (e *Enclosure) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type enclosureAlias Enclosure
	var aux struct {
		enclosureAlias
		Length string `xml:"length,attr"`
	}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	*e = Enclosure(aux.enclosureAlias)
	e.Length = looseInt(aux.Length)
	return nil
}

// IsImage reports whether the enclosure MIME type is an image.
func (e *Enclosure) IsImage() bool { return hasMediaType(e.Type, "image/") }

// IsAudio reports whether the enclosure MIME type is audio.
func (e *Enclosure) IsAudio() bool { return hasMediaType(e.Type, "audio/") }

// IsVideo reports whether the enclosure MIME type is video.
func (e *Enclosure) IsVideo() bool { return hasMediaType(e.Type, "video/") }

// Source represents the <source> element: the channel an item came from.
type Source struct {
	URL  string `xml:",attr"`
	Name string `xml:",chardata"`
}

// UnmarshalXML decodes a source element. The url attribute is matched
// case-insensitively because feeds are inconsistent about it.
func (s *Source) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, a := range start.Attr {
		if strings.EqualFold(a.Name.Local, "url") {
			s.URL = a.Value
		}
	}
	var name string
	if err := d.DecodeElement(&name, &start); err != nil {
		return err
	}
	s.Name = strings.TrimSpace(name)
	return nil
}

// MediaContent represents a Media RSS <media:content> or <media:thumbnail>.
type MediaContent struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Medium string `xml:"medium,attr"`
	Width  int    `xml:"-"`
	Height int    `xml:"-"`
}

// UnmarshalXML decodes a media element, tolerating non-numeric dimensions.
func (m *MediaContent) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type mediaAlias MediaContent
	var aux struct {
		mediaAlias
		Width  string `xml:"width,attr"`
		Height string `xml:"height,attr"`
	}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	*m = MediaContent(aux.mediaAlias)
	m.Width = int(looseInt(aux.Width))
	m.Height = int(looseInt(aux.Height))
	return nil
}

// looseInt parses an integer, returning zero for anything unparseable.
func looseInt(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// hasMediaType reports whether a MIME type string starts with the given
// prefix, ignoring parameters, whitespace, and case.
func hasMediaType(mimeType, prefix string) bool {
	mimeType, _, _ = strings.Cut(mimeType, ";")
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(mimeType)), prefix)
}
