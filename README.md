# karoo

Package junkd0g/karoo is a simple rss feed client

## Installing

go get -u github.com/junkd0g/karoo

## Example

```go
package main

import (
	"fmt"

	rss "github.com/junkd0g/karoo"
)

func main() {
	client, clientError := rss.NewClient()
	if clientError != nil {
		panic(clientError.Error())
	}

	feed, getFeedError := client.GetFeed("https://news.google.com/rss")
	if getFeedError != nil {
		panic(getFeedError)
	}
	fmt.Println(feed)

}
```

## Return feed struct

```go
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Text    string   `xml:",chardata"`
	Version string   `xml:"version,attr"`
	Channel struct {
		Text        string `xml:",chardata"`
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		Description string `xml:"description"`
		Item        []struct {
			Text        string `xml:",chardata"`
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
		} `xml:"item"`
	} `xml:"channel"`
}
```

## Authors

* **Iordanis Paschalidis** -[junkd0g](https://github.com/junkd0g)
