# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added

- `Request.DecodeJSON(v, limit...)`: optional second argument for custom body size limit.
- `Request.DecodeJSON`: rejects non-`application/json` Content-Type.

### Changed

- `Request.DecodeJSON`: uses deterministic body read up to limit+1 bytes before decoding; rejects body exceeding limit and trailing JSON data reliably.
- `newRequestID`: returns error on `crypto/rand` failure instead of empty string.
- `RequestID()`: on `crypto/rand` failure, returns the error propagated to caller instead of falling back to a weak deterministic ID.
- `Adapt`: explicit nil check on Response before `Write` call for consistency.

### Fixed

- `Request.DecodeJSON`: respects caller-specified limit by reading body first then decoding; rejects body exceeding it and trailing JSON data.
- `RequestID()`: never returns empty or weak request ID; `crypto/rand` failures are surfaced as errors.

## [1.0.0] - 2026-11-04

Initial release.
