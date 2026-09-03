# Changelog

## [v1.3.0] - 2026-09-03

Every change in this release is backward compatible with v1.2.0. Existing error message text is preserved for callers that match on strings.

### Fixed

- **Feeds with a non-numeric enclosure `length` no longer fail to parse.** `Enclosure.Length` stays `int64`; unparseable values become `0`. This was a regression introduced in v1.2.0.
- **FeedBurner feeds were rejected as Atom.** Format detection now inspects the root element instead of searching the first kilobyte for `<feed`, which matched `<feedburner:origLink>`.
- **ISO-8859-1 and Windows-1252 feeds now decode.** Previously they failed with "Decoder.CharsetReader is nil".
- **Timezone abbreviations parse with the correct offset.** `EST`, `PDT`, `CEST` and other common abbreviations were silently parsed as UTC. `IST` and other ambiguous abbreviations are intentionally left alone.
- **`text/plain` and `application/octet-stream` responses are accepted.** Only `text/html` and `application/xhtml+xml` are rejected now.
- **`WithHTTPClient(nil)` no longer panics.** It falls back to the default client.
- **`WithTimeout` no longer mutates the caller's `http.Client`.** The client is copied before the timeout is applied.
- **Dates with surrounding whitespace, no seconds, no weekday, a date only, RFC 3339 with fractional seconds, or the RFC 822 `UT` zone now parse.**

### Added

- **`Parse(io.Reader)`, `ParseBytes([]byte)`, and `Client.Parse(io.Reader)`** for decoding feeds from files, caches, or a custom HTTP layer.
- **`Client.Fetch` with conditional request support.** Returns a `Result` carrying `ETag`, `LastModified`, and `StatusCode`. Pass them back with `WithETag` / `WithLastModified`; a 304 response yields `ErrNotModified` and `Result.NotModified`.
- **Typed errors:** `ErrHTTPStatus` / `*HTTPStatusError`, `ErrUnexpectedContentType`, `ErrResponseTooLarge`, `ErrUnsupportedFormat`, `ErrParseFeed`, `ErrParsePubDate`, `ErrNotModified`. All work with `errors.Is` and `errors.As`.
- **`ParseDate(string)`** exported, plus `Channel.ParsePubDate` and `Channel.ParseLastBuildDate`.
- **New `Channel` fields:** `Copyright`, `ManagingEditor`, `WebMaster`, `PubDate`, `LastBuildDate`, `Generator`, `Docs`, `TTL`, `Category`.
- **New `Item` fields:** `GUIDIsPermaLink`, `Comments`, `Source`, `Content` (`content:encoded`), `Creator` (`dc:creator`), `Date` (`dc:date`), `MediaThumbnails` and `MediaContents` (Media RSS).
- **New helpers:** `Item.GetAuthor`, `Item.GetContent`, `Item.IsAudioEnclosure`, `Item.IsVideoEnclosure`, `Enclosure.IsImage` / `IsAudio` / `IsVideo`. `Item.ParsePubDate` falls back to `dc:date`.
- **`WithMaxResponseSize`** client option. `DefaultMaxResponseSize` and `DefaultTimeout` exported.
- **`WithCharsetReader`** client option, `CharsetReader` type, and `DefaultCharsetReader`.
- **`Accept` header** on requests, listing feed media types.
- **RSS 1.0 (RDF) detection** with a clear `ErrUnsupportedFormat` error instead of a raw XML mismatch.
- **Fuzz targets, a benchmark, and runnable examples** on pkg.go.dev.

### Changed

- The response body is streamed into the decoder instead of being read fully into memory first.
- The default `User-Agent` reports the module version from build info instead of a hardcoded `1.0`.
- MIME type helpers match by prefix (`image/`, `audio/`, `video/`), case-insensitively and ignoring parameters, instead of a fixed allow-list.
- `testify` removed; the module now has no dependencies at all.
- CI tests `oldstable` and `stable` Go, checks `go mod tidy`, runs fuzz smoke tests, and runs `gorelease` on pull requests. Added Dependabot, a `.golangci.yml`, and a tag-triggered release workflow.

## [v1.2.0] - 2026-06-04

### Breaking Changes

- **`ParsePubDate` now returns `(time.Time, error)`** instead of `time.Time`. Previously returned `time.Now()` on failure, silently producing incorrect data. Now returns a zero `time.Time` and an error, giving callers explicit control over parse failures.
- **`GetFeed` now requires a `context.Context` parameter.** Signature changed from `GetFeed(url string)` to `GetFeed(ctx context.Context, url string)`, enabling cancellation and deadline control.
- **`Category` changed from `string` to `[]string`.** RSS items can have multiple `<category>` elements; the previous type silently discarded all but the last one.
- **`Enclosure.Length` changed from `string` to `int64`.** The RSS spec defines enclosure length as file size in bytes. The XML decoder handles the conversion automatically.

### Added

- **Response size limit.** `GetFeed` now caps response bodies at 10MB using `io.LimitReader` to prevent OOM from malicious or broken servers.
- **Content-Type validation.** `GetFeed` rejects responses with non-XML content types (e.g., `text/html` login pages), returning a clear error instead of a confusing XML parse failure.
- **Atom feed detection.** `GetFeed` detects Atom feeds (`<feed>` element) and returns an explicit "unsupported format" error instead of silently returning an empty `RSS` struct.
- **`WithUserAgent` client option.** Allows setting a custom `User-Agent` header. Defaults to `karoo/1.0 (+https://github.com/junkd0g/karoo)`.
- **Context support.** `GetFeed` accepts `context.Context` for request cancellation and deadline control.
- **New tests.** Added tests for invalid URLs, unexpected content types, oversized responses, Atom feed detection, `WithTimeout`/`WithHTTPClient` ordering, context cancellation, and User-Agent header verification.

### Fixed

- **`WithTimeout`/`WithHTTPClient` ordering race.** `WithTimeout` previously mutated the HTTP client directly, so calling `WithHTTPClient` after `WithTimeout` silently discarded the timeout. `WithTimeout` now stores the duration and applies it after all options in `NewClient`, making option ordering irrelevant.
- **Slow `TestGetFeedNetworkError`.** Added `WithTimeout(100ms)` to avoid a 10-second wait on the RFC 5737 TEST-NET address.
- **`User-Agent` header.** Requests now include a `User-Agent` header; some RSS servers block requests without one.

### Changed

- Error formatting uses `fmt.Errorf` instead of `errors.New` with string concatenation.
- README API reference updated to reflect all current types (`Channel`, `Image`, `Item`, `Enclosure`) and methods (`ParsePubDate`, `GetEnclosureURL`, `IsImageEnclosure`). All code examples updated with `context.Context` usage.
- CI action versions bumped: `actions/setup-go` v4 to v5, `actions/cache` v3 to v4, `golangci/golangci-lint-action` v3 to v7. Removed stale "Ensures Go 1.23" comment.
