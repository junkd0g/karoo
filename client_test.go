package rss_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	rss "github.com/junkd0g/karoo"
)

const minimalFeed = `<?xml version="1.0"?><rss version="2.0"><channel><title>Test</title></channel></rss>`

// serve starts a test server that answers every request with the given
// content type (empty to let net/http sniff) and body.
func serve(t *testing.T, contentType, body string) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestGetFeedSuccess(t *testing.T) {
	ts := serve(t, "", `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>http://example.com</link>
    <description>Test Description</description>
    <item><title>Item 1</title><link>http://example.com/item1</link><description>Item 1 Description</description></item>
    <item><title>Item 2</title><link>http://example.com/item2</link><description>Item 2 Description</description></item>
  </channel>
</rss>`)

	feed, err := rss.NewClient().GetFeed(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("GetFeed: %v", err)
	}
	if feed.Version != "2.0" || feed.Channel.Title != "Test Feed" || feed.Channel.Link != "http://example.com" {
		t.Errorf("unexpected channel: %+v", feed.Channel)
	}
	if len(feed.Channel.Items) != 2 || feed.Channel.Items[0].Title != "Item 1" || feed.Channel.Items[1].Title != "Item 2" {
		t.Errorf("unexpected items: %+v", feed.Channel.Items)
	}
}

func TestGetFeedHTTPStatusError(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
				_, _ = w.Write([]byte("Error response"))
			}))
			defer ts.Close()

			_, err := rss.NewClient().GetFeed(context.Background(), ts.URL)
			if err == nil {
				t.Fatal("expected error")
			}
			var statusErr *rss.HTTPStatusError
			if !errors.As(err, &statusErr) || statusErr.StatusCode != code {
				t.Fatalf("expected HTTPStatusError with code %d, got %v", code, err)
			}
			if !errors.Is(err, rss.ErrHTTPStatus) {
				t.Error("expected errors.Is(err, ErrHTTPStatus)")
			}
			if !strings.Contains(err.Error(), "failed to fetch RSS feed") {
				t.Errorf("legacy message substring missing: %q", err)
			}
		})
	}
}

func TestGetFeedInvalidXML(t *testing.T) {
	ts := serve(t, "text/xml", "this is not xml")
	_, err := rss.NewClient().GetFeed(context.Background(), ts.URL)
	if !errors.Is(err, rss.ErrParseFeed) {
		t.Fatalf("expected ErrParseFeed, got %v", err)
	}
}

func TestGetFeedEmptyResponse(t *testing.T) {
	ts := serve(t, "text/xml", "")
	_, err := rss.NewClient().GetFeed(context.Background(), ts.URL)
	if !errors.Is(err, rss.ErrParseFeed) {
		t.Fatalf("expected ErrParseFeed, got %v", err)
	}
}

// errReader is an io.ReadCloser that always fails.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read error") }
func (errReader) Close() error             { return nil }

// roundTripperFunc adapts a function to http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGetFeedReadError(t *testing.T) {
	hc := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: errReader{}, Header: make(http.Header)}, nil
	})}
	_, err := rss.NewClient(rss.WithHTTPClient(hc)).GetFeed(context.Background(), "http://example.com")
	if err == nil || !strings.Contains(err.Error(), "read error") {
		t.Fatalf("expected read error, got %v", err)
	}
}

func TestGetFeedTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(minimalFeed))
	}))
	defer ts.Close()

	_, err := rss.NewClient(rss.WithTimeout(50*time.Millisecond)).GetFeed(context.Background(), ts.URL)
	if err == nil || !strings.Contains(err.Error(), "Client.Timeout") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestGetFeedNetworkError(t *testing.T) {
	// Grab a free local port and close it so the connection is refused.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	_, err = rss.NewClient(rss.WithTimeout(time.Second)).GetFeed(context.Background(), "http://"+addr+"/feed")
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestGetFeedInvalidURL(t *testing.T) {
	if _, err := rss.NewClient().GetFeed(context.Background(), "://invalid"); err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestGetFeedContentType(t *testing.T) {
	cases := []struct {
		contentType string
		wantErr     bool
	}{
		{"text/html; charset=utf-8", true},
		{"TEXT/HTML", true},
		{"application/xhtml+xml", true},
		{"text/html;;malformed", true},
		{"application/rss+xml", false},
		{"text/xml; charset=ISO-8859-1", false},
		{"text/plain", false},
		{"application/octet-stream", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.contentType, func(t *testing.T) {
			ts := serve(t, tc.contentType, minimalFeed)
			_, err := rss.NewClient().GetFeed(context.Background(), ts.URL)
			if tc.wantErr {
				if !errors.Is(err, rss.ErrUnexpectedContentType) {
					t.Fatalf("expected ErrUnexpectedContentType, got %v", err)
				}
				if !strings.Contains(err.Error(), "unexpected content type") {
					t.Errorf("legacy message substring missing: %q", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// infiniteReader generates unlimited data.
type infiniteReader struct{}

func (infiniteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}

func TestGetFeedOversizedResponse(t *testing.T) {
	hc := &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(infiniteReader{}), Header: make(http.Header)}, nil
	})}

	t.Run("default limit", func(t *testing.T) {
		_, err := rss.NewClient(rss.WithHTTPClient(hc)).GetFeed(context.Background(), "http://example.com")
		if !errors.Is(err, rss.ErrResponseTooLarge) {
			t.Fatalf("expected ErrResponseTooLarge, got %v", err)
		}
		if !strings.Contains(err.Error(), "response body exceeds maximum size") {
			t.Errorf("legacy message substring missing: %q", err)
		}
	})

	t.Run("custom limit", func(t *testing.T) {
		ts := serve(t, "", minimalFeed)
		_, err := rss.NewClient(rss.WithMaxResponseSize(16)).GetFeed(context.Background(), ts.URL)
		if !errors.Is(err, rss.ErrResponseTooLarge) {
			t.Fatalf("expected ErrResponseTooLarge, got %v", err)
		}
	})

	t.Run("exactly at limit", func(t *testing.T) {
		ts := serve(t, "", minimalFeed)
		_, err := rss.NewClient(rss.WithMaxResponseSize(int64(len(minimalFeed)))).GetFeed(context.Background(), ts.URL)
		if err != nil {
			t.Fatalf("feed at exactly the limit should parse, got %v", err)
		}
	})

	t.Run("limit disabled", func(t *testing.T) {
		big := strings.Repeat("<item><title>x</title></item>", 1000)
		ts := serve(t, "", `<rss version="2.0"><channel>`+big+`</channel></rss>`)
		feed, err := rss.NewClient(rss.WithMaxResponseSize(0)).GetFeed(context.Background(), ts.URL)
		if err != nil || len(feed.Channel.Items) != 1000 {
			t.Fatalf("expected 1000 items, got %d, err %v", len(feed.Channel.Items), err)
		}
	})
}

func TestGetFeedAtomFeed(t *testing.T) {
	ts := serve(t, "application/atom+xml", `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><title>Atom</title></feed>`)
	_, err := rss.NewClient().GetFeed(context.Background(), ts.URL)
	if !errors.Is(err, rss.ErrUnsupportedFormat) {
		t.Fatalf("expected ErrUnsupportedFormat, got %v", err)
	}
	if !strings.Contains(err.Error(), "Atom feeds are not supported") {
		t.Errorf("legacy message substring missing: %q", err)
	}
}

func TestWithTimeoutOrdering(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(minimalFeed))
	}))
	defer ts.Close()

	client := rss.NewClient(rss.WithTimeout(50*time.Millisecond), rss.WithHTTPClient(&http.Client{}))
	_, err := client.GetFeed(context.Background(), ts.URL)
	if err == nil || !strings.Contains(err.Error(), "Client.Timeout") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestWithTimeoutDoesNotMutateCallerClient(t *testing.T) {
	hc := &http.Client{Timeout: 99 * time.Second}
	rss.NewClient(rss.WithHTTPClient(hc), rss.WithTimeout(time.Second))
	if hc.Timeout != 99*time.Second {
		t.Fatalf("caller's http.Client was mutated: timeout is now %v", hc.Timeout)
	}
}

func TestWithHTTPClientNil(t *testing.T) {
	ts := serve(t, "", minimalFeed)
	if _, err := rss.NewClient(rss.WithHTTPClient(nil)).GetFeed(context.Background(), ts.URL); err != nil {
		t.Fatalf("nil http client should fall back to default, got %v", err)
	}
}

func TestGetFeedContextCancellation(t *testing.T) {
	ts := serve(t, "", minimalFeed)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := rss.NewClient().GetFeed(ctx, ts.URL)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestGetFeedRequestHeaders(t *testing.T) {
	var got http.Header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte(minimalFeed))
	}))
	defer ts.Close()

	t.Run("defaults", func(t *testing.T) {
		if _, err := rss.NewClient().GetFeed(context.Background(), ts.URL); err != nil {
			t.Fatal(err)
		}
		if ua := got.Get("User-Agent"); !strings.HasPrefix(ua, "karoo/") {
			t.Errorf("User-Agent = %q, want karoo/ prefix", ua)
		}
		if accept := got.Get("Accept"); !strings.Contains(accept, "application/rss+xml") {
			t.Errorf("Accept = %q, want rss+xml", accept)
		}
	})

	t.Run("custom user agent", func(t *testing.T) {
		if _, err := rss.NewClient(rss.WithUserAgent("my-app/2.0")).GetFeed(context.Background(), ts.URL); err != nil {
			t.Fatal(err)
		}
		if ua := got.Get("User-Agent"); ua != "my-app/2.0" {
			t.Errorf("User-Agent = %q", ua)
		}
	})
}

func TestFetchConditional(t *testing.T) {
	const etag = `"abc123"`
	const lastMod = "Mon, 15 Jan 2024 10:30:00 GMT"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", etag)
		w.Header().Set("Last-Modified", lastMod)
		if r.Header.Get("If-None-Match") == etag || r.Header.Get("If-Modified-Since") == lastMod {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write([]byte(minimalFeed))
	}))
	defer ts.Close()

	client := rss.NewClient()
	first, err := client.Fetch(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if first.NotModified || first.StatusCode != http.StatusOK || first.ETag != etag || first.LastModified != lastMod {
		t.Fatalf("unexpected first result: %+v", first)
	}
	if first.Feed.Channel.Title != "Test" {
		t.Errorf("feed not parsed: %+v", first.Feed)
	}

	for name, opt := range map[string]rss.FetchOption{
		"etag":          rss.WithETag(first.ETag),
		"last-modified": rss.WithLastModified(first.LastModified),
	} {
		t.Run(name, func(t *testing.T) {
			second, err := client.Fetch(context.Background(), ts.URL, opt)
			if !errors.Is(err, rss.ErrNotModified) {
				t.Fatalf("expected ErrNotModified, got %v", err)
			}
			if second == nil || !second.NotModified || second.StatusCode != http.StatusNotModified {
				t.Fatalf("unexpected result: %+v", second)
			}
			if len(second.Feed.Channel.Items) != 0 || second.Feed.Channel.Title != "" {
				t.Errorf("expected empty feed on 304, got %+v", second.Feed)
			}
		})
	}
}

func TestFetchErrorStillReturnsResult(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	res, err := rss.NewClient().Fetch(context.Background(), ts.URL)
	if err == nil {
		t.Fatal("expected error")
	}
	if res == nil || res.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected result with status 429, got %+v", res)
	}
}

func TestClientParse(t *testing.T) {
	client := rss.NewClient(rss.WithMaxResponseSize(1024))

	feed, err := client.Parse(strings.NewReader(`<?xml version="1.0" encoding="ISO-8859-1"?><rss version="2.0"><channel><title>caf` + "\xe9" + `</title></channel></rss>`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if feed.Channel.Title != "café" {
		t.Errorf("title = %q", feed.Channel.Title)
	}

	_, err = client.Parse(strings.NewReader(`<rss version="2.0"><channel>` + strings.Repeat("<item/>", 500) + `</channel></rss>`))
	if !errors.Is(err, rss.ErrResponseTooLarge) {
		t.Fatalf("expected ErrResponseTooLarge, got %v", err)
	}
}

func TestWithCharsetReader(t *testing.T) {
	called := false
	custom := func(_ string, input io.Reader) (io.Reader, error) {
		called = true
		return input, nil
	}
	client := rss.NewClient(rss.WithCharsetReader(custom))
	if _, err := client.Parse(strings.NewReader(`<?xml version="1.0" encoding="KOI8-R"?>` + minimalFeed[21:])); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !called {
		t.Error("custom charset reader was not used")
	}

	// nil restores the default, which rejects KOI8-R.
	client = rss.NewClient(rss.WithCharsetReader(nil))
	if _, err := client.Parse(strings.NewReader(`<?xml version="1.0" encoding="KOI8-R"?>` + minimalFeed[21:])); !errors.Is(err, rss.ErrParseFeed) {
		t.Fatalf("expected ErrParseFeed for unsupported charset, got %v", err)
	}
}
