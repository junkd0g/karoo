# Changelog

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
