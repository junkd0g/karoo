package rss_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	rss "github.com/junkd0g/karoo"
)

func ExampleParseBytes() {
	doc := []byte(`<?xml version="1.0"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel>
    <title>Example Feed</title>
    <item>
      <title>First post</title>
      <dc:creator>Alice</dc:creator>
      <pubDate>Mon, 15 Jan 2024 10:30:00 EST</pubDate>
      <enclosure url="http://example.com/ep1.mp3" type="audio/mpeg" length="unknown"/>
    </item>
  </channel>
</rss>`)

	feed, err := rss.ParseBytes(doc)
	if err != nil {
		log.Fatal(err)
	}

	item := feed.Channel.Items[0]
	published, _ := item.ParsePubDate()
	fmt.Println(feed.Channel.Title)
	fmt.Println(item.Title, "by", item.GetAuthor())
	fmt.Println(published.UTC().Format(time.RFC3339))
	fmt.Println(item.IsAudioEnclosure(), item.Enclosure.Length)
	// Output:
	// Example Feed
	// First post by Alice
	// 2024-01-15T15:30:00Z
	// true 0
}

func ExampleClient_GetFeed() {
	client := rss.NewClient(rss.WithTimeout(15 * time.Second))

	feed, err := client.GetFeed(context.Background(), "https://example.com/feed.xml")
	if err != nil {
		var statusErr *rss.HTTPStatusError
		switch {
		case errors.As(err, &statusErr):
			log.Printf("server returned %d", statusErr.StatusCode)
		case errors.Is(err, rss.ErrUnsupportedFormat):
			log.Print("not an RSS 2.0 feed")
		default:
			log.Print(err)
		}
		return
	}

	for _, item := range feed.Channel.Items {
		fmt.Println(item.Title, item.Link)
	}
}

func ExampleClient_Fetch() {
	client := rss.NewClient()
	ctx := context.Background()
	url := "https://example.com/feed.xml"

	// First poll: no cache headers yet.
	res, err := client.Fetch(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	etag, lastModified := res.ETag, res.LastModified

	// Later polls send the cached validators so unchanged feeds cost a
	// single 304 round trip instead of a full download.
	res, err = client.Fetch(ctx, url, rss.WithETag(etag), rss.WithLastModified(lastModified))
	if errors.Is(err, rss.ErrNotModified) {
		fmt.Println("feed unchanged")
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(res.Feed.Channel.Items), "items")
}
