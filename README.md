# Oil Price API Go SDK

> **Real-time oil and commodity price data for Go** - Professional-grade API at 98% less cost than Bloomberg Terminal

[![Go Reference](https://pkg.go.dev/badge/github.com/OilpriceAPI/oilpriceapi-go.svg)](https://pkg.go.dev/github.com/OilpriceAPI/oilpriceapi-go)
[![Tests](https://github.com/OilpriceAPI/oilpriceapi-go/actions/workflows/test.yml/badge.svg)](https://github.com/OilpriceAPI/oilpriceapi-go/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/OilpriceAPI/oilpriceapi-go)](https://goreportcard.com/report/github.com/OilpriceAPI/oilpriceapi-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**[Get Free API Key](https://www.oilpriceapi.com/signup)** | **[Documentation](https://docs.oilpriceapi.com)** | **[Pricing](https://www.oilpriceapi.com/pricing)**

The official Go SDK for [OilPriceAPI](https://www.oilpriceapi.com) - Real-time and historical oil prices for Brent Crude, WTI, Natural Gas, and 100+ commodities.

## Features

- **Simple API** - Idiomatic Go with functional options pattern
- **Context Support** - Full context.Context integration for cancellation and timeouts
- **Typed Errors** - Custom error types for authentication, rate limits, and server errors
- **Automatic Retries** - Configurable retry with exponential backoff
- **Zero Dependencies** - Uses only the Go standard library
- **Comprehensive Coverage** - Latest prices, historical data, futures, storage, rig counts, drilling, marine fuels, and webhooks

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

```go
// Past week
prices, err := client.GetHistoricalPrices(ctx,
    oilpriceapi.WithPeriod("past_week"),
    oilpriceapi.WithCommodity("BRENT_CRUDE_USD"),
)

// Custom date range with daily aggregation
prices, err := client.GetHistoricalPrices(ctx,
    oilpriceapi.WithStartDate("2024-01-01"),
    oilpriceapi.WithEndDate("2024-12-31"),
    oilpriceapi.WithCommodity("WTI_USD"),
    oilpriceapi.WithInterval("daily"),
)
```

### Commodities List

```go
commodities, err := client.GetCommodities(ctx)
for _, c := range commodities.Data.Commodities {
    fmt.Printf("%s: %s (%s)\n", c.Code, c.Name, c.Category)
}
```

### Futures Contracts

```go
// Get latest front month futures price
futures, err := client.GetFuturesLatest(ctx, "CL.1")
fmt.Printf("WTI Front Month: $%.2f\n", futures.Price)

// Get futures curve
curve, err := client.GetFuturesCurve(ctx, "CL")
for _, point := range curve.Curve {
    fmt.Printf("%d months out: $%.2f\n", point.MonthsOut, point.Price)
}
```

### Storage Levels

```go
// Cushing hub levels
cushing, err := client.GetStorageCushing(ctx)
fmt.Printf("Cushing: %s %s\n", cushing.Level, cushing.Unit)

// Strategic Petroleum Reserve
spr, err := client.GetStorageSPR(ctx)
fmt.Printf("SPR: %s %s\n", spr.Level, spr.Unit)
```

### Rig Counts

```go
rigCounts, err := client.GetRigCountsLatest(ctx)
fmt.Printf("Total: %d, Oil: %d, Gas: %d\n", rigCounts.Total, rigCounts.Oil, rigCounts.Gas)
```

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

1. Sign up at [oilpriceapi.com/signup](https://www.oilpriceapi.com/signup)
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
- [Pricing](https://www.oilpriceapi.com/pricing)
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

**[Start Free](https://www.oilpriceapi.com/signup)** | **[View Pricing](https://www.oilpriceapi.com/pricing)** | **[Read Docs](https://docs.oilpriceapi.com)**

---

Made with care by the OilPriceAPI Team
