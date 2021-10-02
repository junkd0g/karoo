package karoo

import (
	"encoding/xml"
	"io/ioutil"
	"net/http"
)

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

type Client struct{}

type ClientInterface interface {
	request(url string) ([]byte, error)
	GetFeed(url string) (RSS, error)
}

func NewClient() (Client, error) {
	return Client{}, nil
}

func (c Client) request(url string) ([]byte, error) {
	client := &http.Client{}
	req, reqError := http.NewRequest(http.MethodGet, url, nil)

	if reqError != nil {
		return nil, reqError
	}

	res, resError := client.Do(req)
	if resError != nil {
		return nil, resError

	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// GetFeed does a GET request to the url passed
// and return an RSS strut of the response
func (c Client) GetFeed(url string) (RSS, error) {

	body, err := c.request(url)
	if err != nil {
		return RSS{}, err
	}
	var feed RSS

	unmarshallError := xml.Unmarshal(body, &feed)
	if unmarshallError != nil {
		return RSS{}, unmarshallError
	}

	return feed, nil
}
