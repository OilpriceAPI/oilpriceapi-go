# Oil Price API Go SDK

> **Real-time oil and commodity price data for Go** - Professional-grade API at 98% less cost than Bloomberg Terminal

[![Go Reference](https://pkg.go.dev/badge/github.com/OilpriceAPI/oilpriceapi-go.svg)](https://pkg.go.dev/github.com/OilpriceAPI/oilpriceapi-go)
[![Tests](https://github.com/OilpriceAPI/oilpriceapi-go/actions/workflows/test.yml/badge.svg)](https://github.com/OilpriceAPI/oilpriceapi-go/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/OilpriceAPI/oilpriceapi-go)](https://goreportcard.com/report/github.com/OilpriceAPI/oilpriceapi-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**[Get Free API Key](https://www.oilpriceapi.com/signup?utm_source=github&utm_medium=sdk_go&utm_campaign=readme)** | **[Documentation](https://docs.oilpriceapi.com)** | **[Pricing](https://www.oilpriceapi.com/pricing?utm_source=github&utm_medium=sdk_go&utm_campaign=pricing)**

The official Go SDK for [OilPriceAPI](https://www.oilpriceapi.com) - Real-time and historical oil prices for Brent Crude, WTI, Natural Gas, and 100+ commodities.

## Features

- **Simple API** - Idiomatic Go with functional options pattern
- **Context Support** - Full context.Context integration for cancellation and timeouts
- **Typed Errors** - Custom error types for authentication, rate limits, and server errors
- **Automatic Retries** - Configurable retry with exponential backoff
- **Real-Time Streaming** - WebSocket price streaming with auto-reconnect (Professional+)
- **Minimal Dependencies** - Standard library only, plus `gorilla/websocket` for streaming
- **Comprehensive Coverage** - Latest prices, historical data (fixed periods or custom date ranges), forecasts, futures, storage, rig counts, drilling, marine fuels, price alerts, webhooks (full CRUD), analytics, Energy Intelligence (oil inventories, OPEC production, well permits), and real-time streaming

## Installation

```bash
go get github.com/OilpriceAPI/oilpriceapi-go
```

## Quick Start

### Try Demo (No API Key Required)

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/OilpriceAPI/oilpriceapi-go"
)

func main() {
    // Demo endpoint - no API key needed!
    client := oilpriceapi.NewClient("")

    prices, err := client.GetDemoPrices(context.Background())
    if err != nil {
        log.Fatal(err)
    }

    for _, p := range prices.Data.Prices {
        fmt.Printf("%s: $%.2f %s/%s\n", p.Name, p.Price, p.Currency, p.Unit)
    }
}
```

### With API Key

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/OilpriceAPI/oilpriceapi-go"
)

func main() {
    // Create client with API key
    client := oilpriceapi.NewClient("your-api-key",
        oilpriceapi.WithTimeout(10*time.Second),
        oilpriceapi.WithRetries(3),
    )

    // Get all latest prices
    prices, err := client.GetLatestPrices(context.Background())
    if err != nil {
        log.Fatal(err)
    }

    for _, p := range prices.Data.Prices {
        fmt.Printf("%s: $%.2f\n", p.Name, p.Price)
    }

    // Get specific commodity
    brent, err := client.GetLatestPrices(context.Background(),
        oilpriceapi.WithCommodity("BRENT_CRUDE_USD"),
    )
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Brent: $%.2f\n", brent.Data.Prices[0].Price)
}
```

## API Reference

### Creating a Client

```go
// Basic
client := oilpriceapi.NewClient("your-api-key")

// With options
client := oilpriceapi.NewClient("your-api-key",
    oilpriceapi.WithBaseURL("https://api.oilpriceapi.com"),
    oilpriceapi.WithTimeout(30*time.Second),
    oilpriceapi.WithRetries(3),
)
```

### Demo Prices (No Auth)

```go
prices, err := client.GetDemoPrices(ctx)
```

### Latest Prices

```go
// All prices
prices, err := client.GetLatestPrices(ctx)

// Specific commodity
prices, err := client.GetLatestPrices(ctx, oilpriceapi.WithCommodity("WTI_USD"))
```

### Historical Prices

The commodity code is the first positional argument. Use `WithPeriod` for a
fixed lookback window (`"day"`, `"week"`, `"month"`, or `"year"`; defaults to
`"month"`), or `WithStartDate`/`WithEndDate` for a custom range.

```go
// Fixed period (defaults to the past month)
prices, err := client.GetHistoricalPrices(ctx, "BRENT_CRUDE_USD")

// Explicit period
prices, err := client.GetHistoricalPrices(ctx, "BRENT_CRUDE_USD",
    oilpriceapi.WithPeriod("week"),
)

// Custom date range with daily aggregation
prices, err := client.GetHistoricalPrices(ctx, "WTI_USD",
    oilpriceapi.WithStartDate("2024-01-01"),
    oilpriceapi.WithEndDate("2024-12-31"),
    oilpriceapi.WithInterval("daily"),
)

for _, p := range prices.Data.Prices {
    fmt.Printf("%s: $%.2f\n", p.CreatedAt, p.Price)
}
```

### Commodities List

```go
commodities, err := client.GetCommodities(ctx)
for _, c := range commodities.Data.Commodities {
    fmt.Printf("%s: %s (%s)\n", c.Code, c.Name, c.Category)
}
```

### Futures Contracts

The futures contract is selected with `WithContract` (`"BZ"` for Brent or
`"CL"` for WTI; defaults to `"BZ"`).

```go
// Get latest futures contracts (defaults to Brent / BZ)
futures, err := client.GetFuturesLatest(ctx)
for _, c := range futures.Data.Contracts {
    fmt.Printf("%s (%s): $%.2f\n", c.Contract, c.Month, c.Price)
}

// Get the WTI forward curve
curve, err := client.GetFuturesCurve(ctx, oilpriceapi.WithContract("CL"))
for _, c := range curve.Data.Contracts {
    fmt.Printf("%s: $%.2f\n", c.Month, c.Price)
}
```

### Forecasts

```go
// Monthly forecasts for all commodities
forecasts, err := client.GetForecasts(ctx)
for _, f := range forecasts.Data.Commodities {
    fmt.Printf("%s 1-month: $%.2f\n", f.Commodity, f.Forecasts["1_month"].PointEstimate)
}

// Forecast for a single commodity
wti, err := client.GetForecasts(ctx, oilpriceapi.WithForecastCommodity("WTI_USD"))
fmt.Printf("WTI 3-month: $%.2f\n", wti.Data.Forecasts["3_month"].PointEstimate)

// Backtested model accuracy
accuracy, err := client.GetForecastsAccuracy(ctx,
    oilpriceapi.WithLookbackMonths(24),
)
fmt.Printf("1-month MAPE: %.1f%%\n", accuracy.Data.Statistics.AvgMAPE1M)
```

### Storage Levels

```go
// All tracked hubs (Cushing, US SPR, regional)
storage, err := client.GetStorage(ctx)
for _, s := range storage.Data.Storage {
    fmt.Printf("%s: %.1f %s\n", s.Location, s.Value, s.Units)
}

// Cushing hub detail with analytics
cushing, err := client.GetStorageCushing(ctx)
fmt.Printf("Cushing: %.1f %s\n", cushing.Data.Storage.Current.Value, cushing.Data.Storage.Current.Units)

// Strategic Petroleum Reserve
spr, err := client.GetStorageSPR(ctx)
fmt.Printf("SPR: %.1f %s\n", spr.Data.Storage.Current.Value, spr.Data.Storage.Current.Units)
```

### Rig Counts

```go
rigCounts, err := client.GetRigCounts(ctx)
fmt.Printf("Total: %d, Oil: %d, Gas: %d\n",
    rigCounts.Data.Total, rigCounts.Data.Oil, rigCounts.Data.Gas)
```

### Marine Fuels

```go
fuels, err := client.GetMarineFuels(ctx)
for _, p := range fuels.Data.Prices {
    fmt.Printf("%s %s: $%.2f %s/%s\n", p.Port, p.FuelType, p.Price, p.Currency, p.Unit)
}
```

### Drilling Intelligence

```go
drilling, err := client.GetDrillingIntelligence(ctx)
fmt.Printf("Active rigs: %d, total wells: %d\n",
    drilling.Data.ActiveRigs, drilling.Data.TotalWells)
```

### Price Alerts

Create and manage price alerts that fire when a commodity crosses a threshold.

```go
// Create an alert
enabled := true
alert, err := client.CreateAlert(ctx, oilpriceapi.AlertInput{
    Name:              "Brent $85 Alert",
    CommodityCode:     "BRENT_CRUDE_USD",
    ConditionOperator: "greater_than",
    ConditionValue:    85.0,
    WebhookURL:        "https://example.com/webhook",
    Enabled:           &enabled,
    CooldownMinutes:   60,
})

// List alerts
alerts, err := client.GetAlerts(ctx)
for _, a := range alerts {
    fmt.Printf("%s: %s %s %.2f (enabled=%v)\n",
        a.Name, a.CommodityCode, a.ConditionOperator, a.ConditionValue, a.Enabled)
}

// Get a single alert
one, err := client.GetAlert(ctx, "42")

// Update an alert
updated, err := client.UpdateAlert(ctx, "42", oilpriceapi.AlertInput{ConditionValue: 90.0})

// Delete an alert
err = client.DeleteAlert(ctx, "42")

// Trigger history (deprecated server-side; returns a message)
triggers, err := client.GetAlertTriggers(ctx)
fmt.Println(triggers.Message)
```

### Webhooks

Beyond `ListWebhooks`, `CreateWebhook`, and `DeleteWebhook`, the SDK supports the
full management lifecycle (Production Boost tier or higher).

```go
// Get a single webhook (includes signing secret + available options)
wh, err := client.GetWebhook(ctx, "12")
fmt.Printf("%s -> %s (status %s)\n", wh.URL, wh.Status, wh.Status)

// Update a webhook (partial — only set fields are sent)
wh, err = client.UpdateWebhook(ctx, "12", oilpriceapi.WebhookUpdateInput{
    Status:      "paused",
    Description: "Temporarily disabled",
})

// Send a test delivery
result, err := client.TestWebhook(ctx, "12")
fmt.Printf("test: %s (code %d)\n", result.Message, result.ResponseCode)

// List recent delivery events, newest first
events, err := client.GetWebhookEvents(ctx, "12",
    oilpriceapi.WithWebhookPage(1),
    oilpriceapi.WithWebhookPerPage(50),
)
for _, e := range events.Events {
    fmt.Printf("%s: %s (code %d)\n", e.EventType, e.Status, e.ResponseCode)
}
```

### Analytics

Advanced analytics: API usage performance, commodity correlation, trend
analysis, and statistical forecasts (Production Boost / Reservoir Mastery tiers).

```go
// API usage performance for the dashboard
perf, err := client.GetAnalyticsPerformance(ctx, oilpriceapi.WithAnalyticsRange("30d"))
fmt.Printf("Total requests: %d (error rate %.2f%%)\n",
    perf.Overview.TotalRequests, perf.Overview.ErrorRate)

// Correlation between two commodities
corr, err := client.GetAnalyticsCorrelation(ctx,
    oilpriceapi.WithAnalyticsCode1("WTI_USD"),
    oilpriceapi.WithAnalyticsCode2("BRENT_CRUDE_USD"),
    oilpriceapi.WithAnalyticsPeriod(90),
)
var corrData map[string]interface{}
_ = json.Unmarshal(corr.Raw, &corrData)
fmt.Printf("correlation: %v\n", corrData["correlation"])

// Trend / momentum analysis (type can be sma, ema, rsi, levels, or default analysis)
trend, err := client.GetAnalyticsTrend(ctx, "WTI_USD",
    oilpriceapi.WithAnalyticsPeriod(60))

// Statistical forecast
forecast, err := client.GetAnalyticsForecast(ctx, "WTI_USD",
    oilpriceapi.WithAnalyticsMethod("ema"))
fmt.Printf("forecast type=%s tier=%s\n", forecast.Type, forecast.Tier)
```

The correlation, trend, and forecast endpoints return a varying field set, so
`AnalyticsResult` exposes the common `Type`/`Code`/`Tier` fields and preserves
the complete payload in `Raw` (`json.RawMessage`) for fields beyond the typed
set.

### Energy Intelligence (EI)

The `client.EI()` accessor exposes the `/v1/ei/*` energy-intelligence datasets.
Each response carries the dataset-specific payload as a raw JSON `Data` field
(decode it into your own struct) plus typed `Meta`.

```go
ei := client.EI()

// EIA WPSR oil inventories (Reservoir Mastery)
inv, err := ei.GetOilInventories(ctx)
var invData map[string]interface{}
_ = json.Unmarshal(inv.Data, &invData)
fmt.Printf("source: %s, week ending: %v\n", inv.Meta.Source, invData["week_ending"])

// OPEC MOMR production (Reservoir Mastery)
opec, err := ei.GetOpecProduction(ctx, oilpriceapi.WithEIMonths(12))

// State well permits (well_permits addon / enterprise)
permits, err := ei.GetWellPermits(ctx,
    oilpriceapi.WithEIDays(30),
    oilpriceapi.WithEIStates("TX,OK"),
)
```

### Real-Time Streaming (WebSocket)

Stream live price updates over WebSocket using the ActionCable `EnergyPricesChannel`.
`StreamPrices` returns a `*PriceStream`; range over `Updates()` for typed
`Update` values and call `Err()` after the channel closes to see why the stream
ended (`nil` on a clean `Close()` or context cancellation).

> **Streaming requires a Professional+ plan ($99/mo).** The server rejects the
> subscription on lower tiers, which surfaces as a `*StreamRejectedError` on
> `Err()`.

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

stream, err := client.StreamPrices(ctx,
    // Optional client-side filter — accepts upstream codes or streamed slugs.
    oilpriceapi.WithStreamCommodities("BRENT_CRUDE_USD", "WTI_USD"),
)
if err != nil {
    log.Fatal(err)
}
defer stream.Close()

for update := range stream.Updates() {
    switch update.Type {
    case "welcome":
        // Initial snapshot delivered on subscription.
        fmt.Printf("connected @ %s\n", update.Welcome.Data.Timestamp)

    case "price_update":
        if wti := update.Price.Prices.Oil.WTI; wti != nil && wti.OriginalPrice != nil {
            fmt.Printf("WTI  $%.2f @ %s\n", *wti.OriginalPrice, update.Price.Timestamp)
        }
        if brent := update.Price.Prices.Oil.Brent; brent != nil && brent.OriginalPrice != nil {
            fmt.Printf("Brent $%.2f @ %s\n", *brent.OriginalPrice, update.Price.Timestamp)
        }

    case "rig_count_update":
        rc := update.RigCount.RigCount
        fmt.Printf("%s rigs: %.0f (%s)\n", rc.Region, rc.Count, rc.Source)
    }
}

// After the loop, check why the stream ended.
if err := stream.Err(); err != nil {
    log.Printf("stream ended: %v", err)
}
```

The stream auto-reconnects with exponential backoff after transient
disconnects. Cancelling the context (or calling `stream.Close()`) tears the
connection down cleanly and closes `Updates()`. Tune behaviour with
`WithStreamAutoReconnect`, `WithStreamReconnectDelay`, `WithStreamMaxReconnectDelay`,
and `WithStreamMaxReconnectAttempts`.

## Error Handling

The SDK provides typed errors for common API error conditions:

```go
prices, err := client.GetLatestPrices(ctx)
if err != nil {
    switch e := err.(type) {
    case *oilpriceapi.AuthenticationError:
        log.Printf("Invalid API key: %s", e.Message)
    case *oilpriceapi.RateLimitError:
        log.Printf("Rate limited, retry after %d seconds", e.RetryAfter)
    case *oilpriceapi.NotFoundError:
        log.Printf("Resource not found: %s", e.Message)
    case *oilpriceapi.ServerError:
        log.Printf("Server error (%d): %s", e.StatusCode, e.Message)
    default:
        log.Printf("Unknown error: %v", err)
    }
}
```

## Context Support

All methods support Go contexts for cancellation and timeouts:

```go
// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

prices, err := client.GetLatestPrices(ctx)
```

## Available Commodities

**Oil & Gas:**

- `BRENT_CRUDE_USD` - Brent Crude Oil
- `WTI_USD` - WTI Crude Oil
- `NATURAL_GAS_USD` - Natural Gas
- `DIESEL_USD` - Diesel
- `GASOLINE_USD` - Gasoline
- `HEATING_OIL_USD` - Heating Oil

**Coal (8 Endpoints):**

- `CAPP_COAL_USD` - Central Appalachian Coal
- `PRB_COAL_USD` - Powder River Basin Coal
- `NEWCASTLE_COAL_USD` - Newcastle API6
- `COKING_COAL_USD` - Metallurgical Coal

[View all 100+ commodities](https://docs.oilpriceapi.com/commodities)

## Getting an API Key

1. Sign up at [oilpriceapi.com/signup](https://www.oilpriceapi.com/signup?utm_source=github&utm_medium=sdk_go&utm_campaign=readme)
2. Get your API key from the dashboard
3. Start making API calls!

## Support

- Email: support@oilpriceapi.com
- Issues: [GitHub Issues](https://github.com/OilpriceAPI/oilpriceapi-go/issues)
- Docs: [Documentation](https://docs.oilpriceapi.com)

## License

MIT License - see [LICENSE](LICENSE) for details.

## Links

- [OilPriceAPI Website](https://www.oilpriceapi.com)
- [API Documentation](https://docs.oilpriceapi.com)
- [Pricing](https://www.oilpriceapi.com/pricing?utm_source=github&utm_medium=sdk_go&utm_campaign=pricing)
- [Status Page](https://status.oilpriceapi.com)
- [GitHub Repository](https://github.com/OilpriceAPI/oilpriceapi-go)
- [Go Package](https://pkg.go.dev/github.com/OilpriceAPI/oilpriceapi-go)

---

## Why OilPriceAPI?

[OilPriceAPI](https://www.oilpriceapi.com) provides professional-grade commodity price data at **98% less cost than Bloomberg Terminal** ($24,000/year vs $45/month). Trusted by energy traders, financial analysts, and developers worldwide.

### Key Benefits

- **Real-time data** updated every 5 minutes
- **Historical data** for trend analysis and backtesting
- **99.9% uptime** with enterprise-grade reliability
- **5-minute integration** with this Go SDK
- **Free tier** with 100 requests to get started

**[Start Free](https://www.oilpriceapi.com/signup?utm_source=github&utm_medium=sdk_go&utm_campaign=readme)** | **[View Pricing](https://www.oilpriceapi.com/pricing?utm_source=github&utm_medium=sdk_go&utm_campaign=pricing)** | **[Read Docs](https://docs.oilpriceapi.com)**

---

## Also Available As

- **[Python SDK](https://pypi.org/project/oilpriceapi/)** - Python client with Pandas integration
- **[Node.js SDK](https://www.npmjs.com/package/oilpriceapi)** - TypeScript/JavaScript SDK
- **[MCP Server](https://www.npmjs.com/package/oilpriceapi-mcp)** - Model Context Protocol server for Claude, Cursor, and VS Code

---

Made with care by the OilPriceAPI Team
