// Command example runs the canonical OilPriceAPI authenticated first request.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	oilpriceapi "github.com/OilpriceAPI/oilpriceapi-go"
)

const pricingURL = "https://www.oilpriceapi.com/pricing"

func main() {
	apiKey := os.Getenv("OILPRICEAPI_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OILPRICEAPI_KEY is required; create a key at https://www.oilpriceapi.com/auth/signup")
		os.Exit(2)
	}

	options := []oilpriceapi.ClientOption{
		oilpriceapi.WithTimeout(15 * time.Second),
		oilpriceapi.WithRetries(0),
	}
	if baseURL := os.Getenv("OILPRICEAPI_BASE_URL"); baseURL != "" {
		options = append(options, oilpriceapi.WithBaseURL(baseURL))
	}

	client := oilpriceapi.NewClient(apiKey, options...)
	prices, err := client.GetLatestPrices(
		context.Background(),
		oilpriceapi.WithCommodity("BRENT_CRUDE_USD"),
	)
	if err != nil {
		exitForError(err)
	}

	price := prices.Data.Prices[0]
	observedAt := price.UpdatedAt
	if observedAt == "" {
		observedAt = price.CreatedAt
	}
	fmt.Printf("%s %.2f %s/%s as of %s (source: %s)\n",
		price.Code, price.Price, price.Currency, price.Unit, observedAt, price.Source)
}

func exitForError(err error) {
	var authErr *oilpriceapi.AuthenticationError
	var rateErr *oilpriceapi.RateLimitError
	var apiErr *oilpriceapi.APIError

	switch {
	case errors.As(err, &authErr):
		fmt.Fprintln(os.Stderr, "authentication failed; replace OILPRICEAPI_KEY with an active key")
	case errors.As(err, &rateErr):
		if rateErr.RetryAfter > 0 {
			fmt.Fprintf(os.Stderr, "request limit reached; retry after %d seconds or review %s\n", rateErr.RetryAfter, pricingURL)
		} else {
			fmt.Fprintf(os.Stderr, "request limit reached; retry later or review %s\n", pricingURL)
		}
	case errors.As(err, &apiErr) && (apiErr.StatusCode == 402 || apiErr.StatusCode == 403):
		fmt.Fprintf(os.Stderr, "this account cannot access the requested dataset; review %s\n", pricingURL)
	default:
		fmt.Fprintf(os.Stderr, "latest-price request failed: %v\n", err)
	}
	os.Exit(1)
}
