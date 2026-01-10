// Example usage of the Oil Price API Go SDK
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	oilpriceapi "github.com/OilpriceAPI/oilpriceapi-go"
)

func main() {
	// Try demo endpoint first (no API key needed)
	fmt.Println("=== Demo Mode (No API Key) ===")
	demoClient := oilpriceapi.NewClient("")

	demoPrices, err := demoClient.GetDemoPrices(context.Background())
	if err != nil {
		log.Printf("Demo error: %v\n", err)
	} else {
		fmt.Printf("Demo mode: %v\n", demoPrices.Data.Meta.DemoMode)
		fmt.Printf("Rate limit: %s\n", demoPrices.Data.Meta.RateLimit)
		fmt.Println("\nDemo Prices:")
		for _, p := range demoPrices.Data.Prices {
			fmt.Printf("  %s: $%.2f %s/%s\n", p.Name, p.Price, p.Currency, p.Unit)
		}
	}

	// If API key is provided, show authenticated examples
	apiKey := os.Getenv("OILPRICEAPI_KEY")
	if apiKey == "" {
		fmt.Println("\n=== Set OILPRICEAPI_KEY to see authenticated examples ===")
		return
	}

	fmt.Println("\n=== Authenticated Mode ===")
	client := oilpriceapi.NewClient(apiKey)

	// Get all latest prices
	prices, err := client.GetLatestPrices(context.Background())
	if err != nil {
		log.Fatalf("Error getting prices: %v", err)
	}

	fmt.Println("\nLatest Prices:")
	for _, p := range prices.Data.Prices {
		fmt.Printf("  %s: $%.2f %s/%s (updated: %s)\n",
			p.Name, p.Price, p.Currency, p.Unit, p.UpdatedAt)
	}

	// Get specific commodity
	brent, err := client.GetLatestPrices(context.Background(),
		oilpriceapi.WithCommodity("BRENT_CRUDE_USD"))
	if err != nil {
		log.Fatalf("Error getting Brent: %v", err)
	}

	fmt.Printf("\nBrent Crude: $%.2f\n", brent.Data.Prices[0].Price)

	// Get commodities list
	commodities, err := client.GetCommodities(context.Background())
	if err != nil {
		log.Fatalf("Error getting commodities: %v", err)
	}

	fmt.Printf("\nAvailable Commodities: %d\n", len(commodities.Data.Commodities))
}
