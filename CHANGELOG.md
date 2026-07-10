# Changelog

All notable changes to the OilPriceAPI Go SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.2.1] - 2026-07-10

### Changed

- Loosen `source` typing and align examples with the API's masked source labels: the response `source` now returns `market_reporting` for non-government series (government labels like `EIA`/`opec.org` are unchanged). The `Source` struct fields remain free `string` types (no venue enum); test fixtures no longer use venue names such as `ICE`. See oilpriceapi-api#4175.
