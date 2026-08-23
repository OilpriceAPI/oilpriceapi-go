# OilPriceAPI Go SDK

The official Go client for source-timestamped oil, gas, refined-product,
futures, and related energy data from [OilPriceAPI](https://www.oilpriceapi.com).

[![Go Reference](https://pkg.go.dev/badge/github.com/OilpriceAPI/oilpriceapi-go.svg)](https://pkg.go.dev/github.com/OilpriceAPI/oilpriceapi-go)
[![Tests](https://github.com/OilpriceAPI/oilpriceapi-go/actions/workflows/test.yml/badge.svg)](https://github.com/OilpriceAPI/oilpriceapi-go/actions/workflows/test.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

[Get an API key](https://www.oilpriceapi.com/auth/signup?utm_source=go-sdk) |
[Documentation](https://docs.oilpriceapi.com) |
[API explorer](https://api.oilpriceapi.com/swagger) |
[Pricing](https://www.oilpriceapi.com/pricing?utm_source=go-sdk-limit)

## Requirements

- Go 1.21 or newer
- API base URL: `https://api.oilpriceapi.com`
- Auth header: `Authorization: Token YOUR_API_KEY`
- Environment variable used by the executable example: `OILPRICEAPI_KEY`

## Install

```bash
go get github.com/OilpriceAPI/oilpriceapi-go
```

## First Request

The canonical authenticated first request is:

```text
GET /v1/prices/latest?by_code=BRENT_CRUDE_USD
```

Run the repository's tested example:

```bash
export OILPRICEAPI_KEY="your-api-key"
go run github.com/OilpriceAPI/oilpriceapi-go/example@latest
```

The same request in application code:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    oilpriceapi "github.com/OilpriceAPI/oilpriceapi-go"
)

func main() {
    client := oilpriceapi.NewClient(os.Getenv("OILPRICEAPI_KEY"))
    response, err := client.GetLatestPrices(
        context.Background(),
        oilpriceapi.WithCommodity("BRENT_CRUDE_USD"),
    )
    if err != nil {
        log.Fatal(err)
    }

    price := response.Data.Prices[0]
    fmt.Printf("%s %.2f %s/%s as of %s\n",
        price.Code, price.Price, price.Currency, price.Unit, price.UpdatedAt)
}
```

Production returns a singleton `data` object for this endpoint. The SDK
normalizes that object to one entry in `response.Data.Prices`. It also accepts
the legacy `data.prices[]` response for backward compatibility and rejects a
successful response that contains no usable price.

For missing configuration and actionable 401, 403, and 429 recovery, use the
exact executable source in [`example/main.go`](example/main.go). CI copies that
file into a clean consumer module and runs every recovery path against fixtures.

## Several Prices In One Request

`WithCommodity` accepts up to **20 comma-separated commodity codes**, and the whole
call counts as **one request** against your quota — not one per code.

```go
resp, err := client.GetLatestPrices(ctx,
    oilpriceapi.WithCommodity("BRENT_CRUDE_USD,WTI_USD,NATURAL_GAS_USD"))
if err != nil {
    log.Fatal(err)
}
for _, p := range resp.Data.Prices {
    fmt.Printf("%s %.2f %s\n", p.Code, p.Price, p.Currency)
}
```

This is worth knowing on the free plan: 50 requests a day carrying 20 codes each is
**1,000 code-reads a day**, not 50.

One code returns a single price; two or more return `Data.Prices`. Asking for more
than 20 returns `400 Too many commodity codes requested (max: 20, requested: N)`,
and an unrecognised code returns `400` with a "did you mean" suggestion — so
validate your code list once rather than on every poll.

See [How Often To Poll](https://docs.oilpriceapi.com/guides/rate-limiting#how-often-to-poll)
for the interval that fits your plan.

## Demo Request

The demo endpoint does not require an API key:

```go
client := oilpriceapi.NewClient("")
response, err := client.GetDemoPrices(context.Background())
if err != nil {
    log.Fatal(err)
}
for _, price := range response.Data.Prices {
    fmt.Printf("%s %.2f %s/%s\n",
        price.Code, price.Price, price.Currency, price.Unit)
}
```

Demo availability and limits are returned by the endpoint. Authenticated
dataset access and limits vary by plan, source, and account entitlement.

## Client Options

```go
client := oilpriceapi.NewClient(apiKey,
    oilpriceapi.WithTimeout(15*time.Second),
    oilpriceapi.WithRetries(2),
)
```

`WithBaseURL` and `WithHTTPClient` are available for testing and custom network
configuration. All client methods accept `context.Context`.

## Core Methods

| Use case | SDK method |
| --- | --- |
| Latest price | `GetLatestPrices` |
| Historical prices | `GetHistoricalPrices` |
| Commodity catalog | `GetCommodities` |
| Futures and curves | `GetFuturesLatest`, `GetFuturesCurve` |
| Forecasts | `GetForecasts` |
| Storage and rig counts | `GetStorage`, `GetRigCounts` |
| Marine fuels and drilling | `GetMarineFuels`, `GetDrillingSummary` |
| Well production | `client.WellProduction()` |
| Alerts | `GetAlerts`, `CreateAlert`, `UpdateAlert`, `DeleteAlert` |
| Webhooks | `ListWebhooks`, `CreateWebhook`, `UpdateWebhook`, `DeleteWebhook` |
| Analytics | `GetAnalyticsPerformance`, `GetAnalyticsCorrelation`, `GetAnalyticsTrend`, `GetAnalyticsForecast` |
| Energy intelligence | `client.EI()` |
| Market brief | `GetMarketBrief` |
| Agent subscriptions | `GetSubscriptions`, `CreateSubscription`, `GetSubscriptionEvents`, `DeleteSubscription` |
| WebSocket stream | `StreamPrices` |

Use the [Go package reference](https://pkg.go.dev/github.com/OilpriceAPI/oilpriceapi-go)
for method parameters and response types. Availability varies by dataset, plan,
source, and account entitlement; the current source is
[pricing](https://www.oilpriceapi.com/pricing).

## Well Production

```go
summary, err := client.WellProduction().GetSummary(ctx)
if err != nil {
    log.Fatal(err)
}
if summary.Data.Coverage == nil {
    log.Fatal("well-production coverage is unavailable")
}

covered := make(map[string]bool)
for _, state := range summary.Data.Coverage.WellLevelStatesWithData {
    covered[state] = true
}

permits, err := client.EI().SearchWellPermits(ctx, oilpriceapi.WellPermitSearchQuery{
    States:   "TX",
    WellName: "Eagle",
})
if err != nil {
    log.Fatal(err)
}

isAPI14 := func(value string) bool {
    if len(value) != 14 {
        return false
    }
    for _, digit := range value {
        if digit < '0' || digit > '9' {
            return false
        }
    }
    return true
}

for _, permit := range permits.WellPermits {
    if !covered[permit.StateCode] || !isAPI14(permit.APINumber) {
        continue
    }
    production, err := client.WellProduction().GetWellDetail(ctx, permit.APINumber)
    if err != nil {
        log.Print(err)
        continue
    }
    fmt.Println(permit.Well.Name, production.Data.Data)
}
```

The accessor also supports state summaries, state and well history, top
producers, and cycle-time analysis. Check the returned coverage metadata
before treating a state or well-level result as complete.

## Historical Prices

```go
response, err := client.GetHistoricalPrices(ctx, "BRENT_CRUDE_USD",
    oilpriceapi.WithPeriod("week"),
)
```

For a custom range, use `WithStartDate`, `WithEndDate`, and `WithInterval`.

## Futures

```go
response, err := client.GetFuturesLatest(ctx,
    oilpriceapi.WithContract("brent"),
)
if err == nil && response.FrontMonth != nil {
    fmt.Printf("%s %.2f\n",
        response.FrontMonth.ContractMonth,
        response.FrontMonth.LastPrice,
    )
}
```

`WithContract` accepts the API slug or a supported short code such as `BZ` or
`CL`. The SDK keeps unknown slugs forward-compatible.

## Typed Errors

```go
response, err := client.GetLatestPrices(ctx,
    oilpriceapi.WithCommodity("BRENT_CRUDE_USD"),
)
if err != nil {
    var authErr *oilpriceapi.AuthenticationError
    var rateErr *oilpriceapi.RateLimitError
    var apiErr *oilpriceapi.APIError

    switch {
    case errors.As(err, &authErr):
        log.Print("replace OILPRICEAPI_KEY with an active key")
    case errors.As(err, &rateErr):
        log.Printf("retry after %d seconds", rateErr.RetryAfter)
    case errors.As(err, &apiErr) && (apiErr.StatusCode == 402 || apiErr.StatusCode == 403):
        log.Print("review dataset access at https://www.oilpriceapi.com/pricing")
    default:
        log.Printf("request failed: %v", err)
    }
}
```

The SDK also exposes `NotFoundError`, `ServerError`, and
`StreamRejectedError`.

## Raw Escape Hatch

Use `Raw` for a versioned API route that does not yet have a typed method:

```go
var result map[string]any
err := client.Raw(ctx, http.MethodGet, "/v1/some/versioned/route",
    url.Values{"days": {"30"}}, &result)
```

## Streaming

`StreamPrices` opens an ActionCable WebSocket and returns typed updates.
Check `stream.Err()` after `Updates()` closes. Availability varies by account
entitlement; a rejected subscription returns `StreamRejectedError` with a
recovery link.

Source timestamps describe the values in each response. They do not imply one
sitewide update interval: refresh cadence varies by source, market hours,
dataset, and plan.

## Reviewed Product Facts

The versioned, reviewed contract is
[`product-facts.json`](https://api.oilpriceapi.com/product-facts.json). Mutable
offer, catalog, freshness, entitlement, and data-rights claims should link to
that contract instead of being copied into SDK documentation.

Current reviewed catalog wording: a broad catalog spanning crude oil, natural
gas, refined products, futures, marine fuels, carbon markets, metals, forex,
and selected energy-intelligence datasets. See the
[commodity catalog](https://www.oilpriceapi.com/commodities) for current
availability.

Standard plans provide API access, normalization, monitoring, and delivery;
they do not grant ownership of source data or unrestricted raw-data
redistribution rights. See the
[data usage policy](https://www.oilpriceapi.com/legal/data-usage).

## Verify This Repository

```bash
go test ./...
go test -race ./...
go vet ./...
./scripts/clean-install-smoke.sh
```

The guarded production suite requires `OILPRICEAPI_TEST_KEY`:

```bash
OILPRICEAPI_TEST_KEY="your-test-key" go test -tags live -run TestLiveGetLatestPrice ./...
```

## Support

- [Documentation](https://docs.oilpriceapi.com)
- [API explorer](https://api.oilpriceapi.com/swagger)
- [Status](https://status.oilpriceapi.com)
- [GitHub issues](https://github.com/OilpriceAPI/oilpriceapi-go/issues)
- support@oilpriceapi.com

MIT licensed. See [LICENSE](LICENSE).
