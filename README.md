# Oil Price API Go SDK

Official Go SDK for the [Oil Price API](https://www.oilpriceapi.com).

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

### Commodities List

```go
commodities, err := client.GetCommodities(ctx)
for _, c := range commodities.Data.Commodities {
    fmt.Printf("%s: %s (%s)\n", c.Code, c.Name, c.Category)
}
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

| Code | Name | Category |
|------|------|----------|
| BRENT_CRUDE_USD | Brent Crude Oil | Crude Oil |
| WTI_USD | WTI Crude Oil | Crude Oil |
| NATURAL_GAS_USD | Natural Gas | Natural Gas |
| GOLD_USD | Gold | Precious Metals |
| EUR_USD | EUR/USD | Forex |
| GBP_USD | GBP/USD | Forex |
| HEATING_OIL_USD | Heating Oil | Refined Products |
| GASOLINE_USD | Gasoline | Refined Products |
| DIESEL_USD | Diesel | Refined Products |

See full list: https://www.oilpriceapi.com/commodities

## Getting an API Key

1. Sign up at [oilpriceapi.com/signup](https://www.oilpriceapi.com/auth/signup)
2. Get your API key from the dashboard
3. Start making API calls!

## Support

- Documentation: https://docs.oilpriceapi.com
- Email: support@oilpriceapi.com
- API Status: https://status.oilpriceapi.com

## License

MIT License - see [LICENSE](LICENSE) for details.
