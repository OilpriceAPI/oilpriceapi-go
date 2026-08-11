# Changelog

All notable changes to the OilPriceAPI Go SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.5.1] - 2026-08-11

### Added

- Add `GetDrillingSummary` for the canonical drilling-intelligence summary
  route while retaining `GetDrillingIntelligence` as a compatibility helper.
- Add typed well-permit search and a coverage-gated permit-to-production
  quickstart.

### Fixed

- Route default futures requests and contract codes through instrument-generic
  Brent, WTI, Gasoil, and EU-carbon aliases while preserving explicit legacy
  slugs as pass-through inputs.

## [1.5.0] - 2026-08-11

### Added

- Add typed well-production access through `WellProduction()` and align the
  drilling response types with the production API payload.

### Fixed

- Decode the current keyless demo contract, preserve its `updated_at`
  timestamp, and fail loudly when a successful response has no usable prices.
- Exercise keyless and authenticated production requests in the release gate.

## [1.4.0] - 2026-07-19

### Fixed

- Decode the production `/v1/prices/latest` singleton payload into one
  `Data.Prices` entry while retaining legacy array-envelope compatibility.
- Reject a successful latest-price response that contains no usable price
  instead of silently returning an empty slice.

### Changed

- Publish an executable `OILPRICEAPI_KEY` first request with actionable missing
  configuration, 401, 403, and 429 recovery.
- Preserve source and production timestamp fields on `Price`.
- Replace mutable product claims with links to the reviewed product-facts and
  pricing contracts; add a claims drift test.
- Add clean consumer-module and strict production first-request release gates.

## [1.2.1] - 2026-07-10

### Changed

- Loosen `source` typing and align examples with the API's masked source labels: the response `source` now returns `market_reporting` for non-government series (government labels like `EIA`/`opec.org` are unchanged). The `Source` struct fields remain free `string` types (no venue enum); test fixtures no longer use venue names such as `ICE`. See oilpriceapi-api#4175.
