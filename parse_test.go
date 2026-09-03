package rss_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	rss "github.com/junkd0g/karoo"
)

const extendedFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"
     xmlns:content="http://purl.org/rss/1.0/modules/content/"
     xmlns:dc="http://purl.org/dc/elements/1.1/"
     xmlns:media="http://search.yahoo.com/mrss/"
     xmlns:feedburner="http://rssnamespace.org/feedburner/ext/1.0">
  <channel>
    <title>Extended Feed</title>
    <link>http://example.com</link>
    <description>Feed with extended fields</description>
    <language>en-us</language>
    <copyright>2024 Example</copyright>
    <managingEditor>editor@example.com</managingEditor>
    <webMaster>web@example.com</webMaster>
    <pubDate>Mon, 15 Jan 2024 09:00:00 GMT</pubDate>
    <lastBuildDate>Mon, 15 Jan 2024 10:30:00 EST</lastBuildDate>
    <generator>karoo-test</generator>
    <docs>https://www.rssboard.org/rss-specification</docs>
    <ttl>60</ttl>
    <category>News</category>
    <category>Tech</category>
    <image>
      <url>http://example.com/logo.png</url>
      <title>Feed Logo</title>
      <link>http://example.com</link>
    </image>
    <item>
      <title>Item with all fields</title>
      <link>http://example.com/item1</link>
      <description>Full description</description>
      <pubDate>Mon, 15 Jan 2024 10:30:00 GMT</pubDate>
      <guid isPermaLink="false">unique-id-123</guid>
      <author>john@example.com</author>
      <dc:creator>John Doe</dc:creator>
      <category>Technology</category>
      <category>Science</category>
      <comments>http://example.com/item1#comments</comments>
      <enclosure url="http://example.com/image.jpg" type="image/jpeg" length="12345"/>
      <source url="http://source.example.com/feed">Source Feed</source>
      <content:encoded><![CDATA[<p>Full <b>HTML</b> body</p>]]></content:encoded>
      <media:thumbnail url="http://example.com/thumb.jpg" width="150" height="100"/>
      <media:content url="http://example.com/video.mp4" type="video/mp4" medium="video" width="1920" height="1080"/>
      <media:content url="http://example.com/still.jpg" type="image/jpeg" medium="image"/>
      <feedburner:origLink>http://example.com/original</feedburner:origLink>
    </item>
  </channel>
</rss>`

func TestParseExtendedFields(t *testing.T) {
	feed, err := rss.ParseBytes([]byte(extendedFeed))
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}

	ch := feed.Channel
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"Language", ch.Language, "en-us"},
		{"Copyright", ch.Copyright, "2024 Example"},
		{"ManagingEditor", ch.ManagingEditor, "editor@example.com"},
		{"WebMaster", ch.WebMaster, "web@example.com"},
		{"PubDate", ch.PubDate, "Mon, 15 Jan 2024 09:00:00 GMT"},
		{"LastBuildDate", ch.LastBuildDate, "Mon, 15 Jan 2024 10:30:00 EST"},
		{"Generator", ch.Generator, "karoo-test"},
		{"Docs", ch.Docs, "https://www.rssboard.org/rss-specification"},
		{"TTL", ch.TTL, 60},
		{"Category", strings.Join(ch.Category, ","), "News,Tech"},
		{"Image.URL", ch.Image.URL, "http://example.com/logo.png"},
		{"Image.Title", ch.Image.Title, "Feed Logo"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("Channel.%s = %v, want %v", c.name, c.got, c.want)
		}
	}

	if len(ch.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(ch.Items))
	}
	item := ch.Items[0]
	itemChecks := []struct {
		name string
		got  any
		want any
	}{
		{"PubDate", item.PubDate, "Mon, 15 Jan 2024 10:30:00 GMT"},
		{"GUID", item.GUID, "unique-id-123"},
		{"GUIDIsPermaLink", item.GUIDIsPermaLink, false},
		{"Author", item.Author, "john@example.com"},
		{"Creator", item.Creator, "John Doe"},
		{"GetAuthor", item.GetAuthor(), "John Doe"},
		{"Category", strings.Join(item.Category, ","), "Technology,Science"},
		{"Comments", item.Comments, "http://example.com/item1#comments"},
		{"Enclosure.URL", item.Enclosure.URL, "http://example.com/image.jpg"},
		{"Enclosure.Type", item.Enclosure.Type, "image/jpeg"},
		{"Enclosure.Length", item.Enclosure.Length, int64(12345)},
		{"Source.URL", item.Source.URL, "http://source.example.com/feed"},
		{"Source.Name", item.Source.Name, "Source Feed"},
		{"Content", item.Content, "<p>Full <b>HTML</b> body</p>"},
		{"GetContent", item.GetContent(), "<p>Full <b>HTML</b> body</p>"},
		{"MediaThumbnails len", len(item.MediaThumbnails), 1},
		{"MediaThumbnails[0].Width", item.MediaThumbnails[0].Width, 150},
		{"MediaContents len", len(item.MediaContents), 2},
		{"MediaContents[0].Medium", item.MediaContents[0].Medium, "video"},
		{"MediaContents[0].Height", item.MediaContents[0].Height, 1080},
		{"MediaContents[1].Width", item.MediaContents[1].Width, 0},
	}
	for _, c := range itemChecks {
		if c.got != c.want {
			t.Errorf("Item.%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestParseMissingOptionalFields(t *testing.T) {
	feed, err := rss.ParseBytes([]byte(`<rss version="2.0"><channel><title>Minimal</title>
<item><title>Only title</title></item>
<item><link>http://example.com/2</link><description>No title</description></item>
</channel></rss>`))
	if err != nil {
		t.Fatal(err)
	}
	if feed.Channel.Link != "" || feed.Channel.Image != nil || feed.Channel.TTL != 0 {
		t.Errorf("unexpected channel: %+v", feed.Channel)
	}
	items := feed.Channel.Items
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Link != "" || items[0].Enclosure != nil || items[0].Source != nil || items[0].GUIDIsPermaLink {
		t.Errorf("unexpected item 0: %+v", items[0])
	}
	if items[1].Title != "" || items[1].Link != "http://example.com/2" || items[1].GetContent() != "No title" {
		t.Errorf("unexpected item 1: %+v", items[1])
	}
}

func TestParseGUIDIsPermaLink(t *testing.T) {
	cases := []struct {
		name      string
		guid      string
		wantGUID  string
		wantPerma bool
	}{
		{"absent", "", "", false},
		{"no attribute", `<guid>http://example.com/1</guid>`, "http://example.com/1", true},
		{"true", `<guid isPermaLink="true">http://example.com/1</guid>`, "http://example.com/1", true},
		{"false", `<guid isPermaLink="false">id-1</guid>`, "id-1", false},
		{"FALSE uppercase", `<guid isPermaLink="FALSE">id-1</guid>`, "id-1", false},
		{"whitespace", `<guid> id-1 </guid>`, "id-1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			feed, err := rss.ParseBytes([]byte(`<rss version="2.0"><channel><item>` + tc.guid + `</item></channel></rss>`))
			if err != nil {
				t.Fatal(err)
			}
			item := feed.Channel.Items[0]
			if item.GUID != tc.wantGUID || item.GUIDIsPermaLink != tc.wantPerma {
				t.Errorf("GUID = %q perma = %v, want %q %v", item.GUID, item.GUIDIsPermaLink, tc.wantGUID, tc.wantPerma)
			}
		})
	}
}

func TestParseLenientNumbers(t *testing.T) {
	feed, err := rss.ParseBytes([]byte(`<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/"><channel><ttl>soon</ttl>
<item><enclosure url="http://example.com/a.mp3" type="audio/mpeg" length="unknown"/><media:thumbnail url="http://t" width="12.5" height=""/></item>
<item><enclosure url="http://example.com/b.mp3" type="audio/mpeg" length=" 42 "/></item>
<item><enclosure url="http://example.com/c.mp3" type="audio/mpeg"/></item>
</channel></rss>`))
	if err != nil {
		t.Fatalf("lenient parse should succeed, got %v", err)
	}
	if feed.Channel.TTL != 0 {
		t.Errorf("TTL = %d, want 0", feed.Channel.TTL)
	}
	items := feed.Channel.Items
	if items[0].Enclosure.Length != 0 || items[1].Enclosure.Length != 42 || items[2].Enclosure.Length != 0 {
		t.Errorf("lengths = %d %d %d", items[0].Enclosure.Length, items[1].Enclosure.Length, items[2].Enclosure.Length)
	}
	if th := items[0].MediaThumbnails[0]; th.Width != 0 || th.Height != 0 || th.URL != "http://t" {
		t.Errorf("thumbnail = %+v", th)
	}
}

func TestParseCharsets(t *testing.T) {
	cases := []struct {
		name     string
		encoding string
		raw      string
		want     string
	}{
		{"utf-8", "UTF-8", "café", "café"},
		{"utf8 alias", "utf8", "café", "café"},
		{"us-ascii", "US-ASCII", "cafe", "cafe"},
		{"iso-8859-1", "ISO-8859-1", "caf\xe9", "café"},
		{"latin1 alias", "latin1", "caf\xe9", "café"},
		{"windows-1252", "windows-1252", "\x93caf\xe9\x94 \x80", "“café” €"},
		{"cp1252 undefined byte", "cp1252", "a\x81b", "a�b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := `<?xml version="1.0" encoding="` + tc.encoding + `"?><rss version="2.0"><channel><title>` + tc.raw + `</title></channel></rss>`
			feed, err := rss.ParseBytes([]byte(doc))
			if err != nil {
				t.Fatalf("ParseBytes: %v", err)
			}
			if feed.Channel.Title != tc.want {
				t.Errorf("title = %q, want %q", feed.Channel.Title, tc.want)
			}
		})
	}

	t.Run("unsupported", func(t *testing.T) {
		_, err := rss.ParseBytes([]byte(`<?xml version="1.0" encoding="KOI8-R"?><rss/>`))
		if !errors.Is(err, rss.ErrParseFeed) {
			t.Fatalf("expected ErrParseFeed, got %v", err)
		}
	})
}

func TestDefaultCharsetReaderSmallBuffers(t *testing.T) {
	r, err := rss.DefaultCharsetReader("windows-1252", strings.NewReader("\x80\x80"))
	if err != nil {
		t.Fatal(err)
	}
	var out []byte
	buf := make([]byte, 1)
	for {
		n, err := r.Read(buf)
		out = append(out, buf[:n]...)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if string(out) != "€€" {
		t.Errorf("got %q", out)
	}

	if _, err := rss.DefaultCharsetReader("EBCDIC", strings.NewReader("")); err == nil {
		t.Error("expected error for unknown charset")
	}
}

func TestParseUnsupportedFormat(t *testing.T) {
	cases := map[string]string{
		"atom": `<feed xmlns="http://www.w3.org/2005/Atom"><title>A</title></feed>`,
		"rdf":  `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><channel/></rdf:RDF>`,
		"html": `<html><body>nope</body></html>`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := rss.ParseBytes([]byte(`<?xml version="1.0"?>` + doc))
			if !errors.Is(err, rss.ErrUnsupportedFormat) {
				t.Fatalf("expected ErrUnsupportedFormat, got %v", err)
			}
		})
	}
}

func TestParseFeedBurnerNotMistakenForAtom(t *testing.T) {
	feed, err := rss.ParseBytes([]byte(`<rss version="2.0" xmlns:feedburner="http://rssnamespace.org/feedburner/ext/1.0"><channel><item><feedburner:origLink>x</feedburner:origLink><title>a</title></item></channel></rss>`))
	if err != nil {
		t.Fatalf("FeedBurner feed rejected: %v", err)
	}
	if len(feed.Channel.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(feed.Channel.Items))
	}
}

func TestParseLeadingContent(t *testing.T) {
	feed, err := rss.ParseBytes([]byte("\xEF\xBB\xBF<?xml version=\"1.0\"?>\n<!-- generated -->\n<?xml-stylesheet href=\"x.xsl\"?>\n<rss version=\"2.0\"><channel><title>ok</title></channel></rss>"))
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if feed.Channel.Title != "ok" {
		t.Errorf("title = %q", feed.Channel.Title)
	}
}

func TestParseErrors(t *testing.T) {
	for name, doc := range map[string]string{
		"empty":                  "",
		"truncated":              `<rss version="2.0"><channel><title>t`,
		"truncated in item":      `<rss version="2.0"><channel><item><title>t`,
		"truncated in enclosure": `<rss version="2.0"><channel><item><enclosure url="x">`,
		"truncated in source":    `<rss version="2.0"><channel><item><source url="x">n`,
		"truncated in media":     `<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/"><channel><item><media:content url="x">`,
		"garbage":                "not xml at all",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := rss.Parse(strings.NewReader(doc))
			if !errors.Is(err, rss.ErrParseFeed) {
				t.Fatalf("expected ErrParseFeed, got %v", err)
			}
		})
	}
}

func TestEnclosureHelpers(t *testing.T) {
	cases := []struct {
		mimeType            string
		image, audio, video bool
	}{
		{"image/jpeg", true, false, false},
		{"image/png", true, false, false},
		{"image/webp", true, false, false},
		{"IMAGE/GIF", true, false, false},
		{"audio/mpeg", false, true, false},
		{"audio/x-m4a; codecs=mp4a", false, true, false},
		{"video/mp4", false, false, true},
		{"video/webm", false, false, true},
		{"application/pdf", false, false, false},
		{"", false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.mimeType, func(t *testing.T) {
			item := rss.Item{Enclosure: &rss.Enclosure{URL: "http://example.com/f", Type: tc.mimeType}}
			if item.IsImageEnclosure() != tc.image || item.IsAudioEnclosure() != tc.audio || item.IsVideoEnclosure() != tc.video {
				t.Errorf("image/audio/video = %v/%v/%v", item.IsImageEnclosure(), item.IsAudioEnclosure(), item.IsVideoEnclosure())
			}
			if item.GetEnclosureURL() != "http://example.com/f" {
				t.Errorf("GetEnclosureURL = %q", item.GetEnclosureURL())
			}
		})
	}

	t.Run("nil enclosure", func(t *testing.T) {
		var item rss.Item
		if item.IsImageEnclosure() || item.IsAudioEnclosure() || item.IsVideoEnclosure() || item.GetEnclosureURL() != "" {
			t.Error("nil enclosure should report false and empty URL")
		}
	})
}

func TestItemFallbackGetters(t *testing.T) {
	item := rss.Item{Author: "a@example.com", Description: "short"}
	if item.GetAuthor() != "a@example.com" || item.GetContent() != "short" {
		t.Errorf("fallbacks: author %q content %q", item.GetAuthor(), item.GetContent())
	}
	item.Creator = "Alice"
	item.Content = "<p>long</p>"
	if item.GetAuthor() != "Alice" || item.GetContent() != "<p>long</p>" {
		t.Errorf("preferred: author %q content %q", item.GetAuthor(), item.GetContent())
	}
}

func FuzzParseBytes(f *testing.F) {
	f.Add([]byte(extendedFeed))
	f.Add([]byte(minimalFeed))
	f.Add([]byte(`<?xml version="1.0" encoding="ISO-8859-1"?><rss version="2.0"><channel><title>caf` + "\xe9" + `</title></channel></rss>`))
	f.Add([]byte(`<feed/>`))
	f.Add([]byte(``))
	f.Fuzz(func(_ *testing.T, data []byte) {
		feed, err := rss.ParseBytes(data)
		if err != nil {
			return
		}
		for _, item := range feed.Channel.Items {
			_, _ = item.ParsePubDate()
			_ = item.IsImageEnclosure()
			_ = item.GetContent()
		}
	})
}

func BenchmarkParseBytes(b *testing.B) {
	items := strings.Repeat(`<item><title>Title</title><link>http://example.com/1</link><description>Desc</description><pubDate>Mon, 15 Jan 2024 10:30:00 GMT</pubDate><guid isPermaLink="false">id</guid><enclosure url="http://example.com/a.mp3" type="audio/mpeg" length="123"/></item>`, 100)
	doc := []byte(`<rss version="2.0"><channel><title>Bench</title>` + items + `</channel></rss>`)
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := rss.ParseBytes(doc); err != nil {
			b.Fatal(err)
		}
	}
}
