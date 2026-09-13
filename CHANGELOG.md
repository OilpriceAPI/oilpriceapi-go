# Changelog

All notable changes to the OilPriceAPI Go SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Subscription lifecycle (#31): `GetSubscription`, `UpdateSubscription`,
  `PauseSubscription` and `ResumeSubscription` for the
  `/v1/subscriptions/{id}` member routes. `UpdateSubscription` takes a
  `SubscriptionUpdate` whose pointer fields are sent only when set, so a field
  can be cleared (`""`, `false`) without resending the others. Update, pause and
  resume are writes and are never retried automatically.
- `InvalidInputError`, returned before any request when an ID is blank, `.`,
  `..` or contains a control character, or when an update sets no fields, an
  empty or blank code list, or a non-positive interval.
- `MalformedResponseError`, returned when a lifecycle call gets a 2xx whose
  body is not JSON, has no `data.subscription.id`, or describes a different
  subscription. Previously such a body decoded into a zero-value struct.

### Changed

- `DeleteSubscription` validates its ID with the same rules. A blank ID now
  returns `*InvalidInputError` instead of a plain error, and `.` is refused
  rather than sent as `DELETE /v1/subscriptions/.`.

## [1.7.0] - 2026-09-13

### Changed

- `WithMaxRetryWait` now bounds the **total** time spent waiting across all
  retries of one call, not each individual wait (#42). Previously a server
  answering `Retry-After` just under the budget could hold a call for roughly
  the budget multiplied by the retry count.

### Fixed

- `WithTimeout` no longer mutates a caller-supplied `*http.Client` (#38). Two
  SDK clients sharing one `*http.Client` could overwrite each other's timeout,
  and the write raced with in-flight requests under `-race`. The timeout is now
  applied to a shallow copy after all options run, independent of option order.
- `WithHTTPClient(nil)` no longer panics with a nil pointer dereference on the
  first request (#41).
- Path handling (#40): a space in a path segment is escaped and sent again, as
  it was before 1.6.0, and a percent-encoded `..` segment is rejected.
- `MaxReconnectAttempts` counts **consecutive** failed reconnects, as
  documented (#34). The counter now resets after a healthy streaming session
  instead of accumulating for the life of the stream.

## [1.6.0] - 2026-09-13

### Security

- Reject raw paths that change the authenticated API origin (#36). Six probe
  forms previously reached a foreign host still carrying
  `Authorization: Token <key>`, including a host-suffix append where
  `.evil.invalid` resolved to `api.oilpriceapi.com.evil.invalid` — a fully
  attacker-controlled domain that reads as ours. A path must now be exactly one
  leading `/`, which makes userinfo, host-suffix, scheme-relative and
  absolute-URL forms unreachable by construction rather than by blocklist.
  168,420 hostile forms were brute-forced through the gate: 0 escaped origin.

### Changed

- **Breaking:** `POST`, `PUT`, `PATCH` and `DELETE` are no longer retried (#37).
  A duplicated write is worse than a failed one, and `CreateWebhook` was
  observed sending four POSTs on a single 503. `GET`, `HEAD` and `OPTIONS` are
  unchanged.
- **Breaking:** a path without a leading `/` is now rejected rather than
  concatenated. `Raw("v1/prices")` previously produced the host
  `api.oilpriceapi.comv1`.

### Fixed

- Bound `Retry-After` (#37). A real production response carried
  `retry-after: 28197` — 7h50m — and was honoured uncapped on
  `context.Background()`. Measured: still blocked at the 10s cutoff before,
  961µs after.
- Stop zero and negative `Retry-After` values collapsing the backoff into a hot
  retry loop (four requests in ~1ms), and stop a `math.MaxInt64` value wrapping
  int64 nanoseconds into the same loop.
- Honour context cancellation during retry waits.
- Return a typed `*InvalidPathError` for malformed paths instead of an untyped
  `*url.Error` indistinguishable from a transport failure.
- Return a typed `*ConfigurationError` for a negative retry count instead of
  `request failed after -1 retries: %!w(<nil>)`.

## [1.5.2] - 2026-08-11

### Fixed

- Decode the nested production `{status,data:{well_permits,meta}}` search
  envelope while preserving the legacy top-level response shape.
- Reject successful but unknown permit payloads instead of silently returning
  no typed results, while retaining explicit empty searches as valid data.

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
