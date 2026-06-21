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

	// Date-range historical prices with daily aggregation
	history, err := client.GetHistoricalPrices(context.Background(), "WTI_USD",
		oilpriceapi.WithStartDate("2024-01-01"),
		oilpriceapi.WithEndDate("2024-03-31"),
		oilpriceapi.WithInterval("daily"))
	if err != nil {
		log.Printf("Error getting historical range: %v\n", err)
	} else {
		fmt.Printf("\nWTI history points (Q1 2024): %d\n", len(history.Data.Prices))
	}

	// Monthly forecasts
	forecasts, err := client.GetForecasts(context.Background())
	if err != nil {
		log.Printf("Error getting forecasts: %v\n", err)
	} else {
		fmt.Printf("\nForecasts for %s:\n", forecasts.Data.Period)
		for _, f := range forecasts.Data.Commodities {
			fmt.Printf("  %s 1-month: $%.2f\n", f.Commodity, f.Forecasts["1_month"].PointEstimate)
		}
	}

	// Storage levels
	storage, err := client.GetStorage(context.Background())
	if err != nil {
		log.Printf("Error getting storage: %v\n", err)
	} else {
		fmt.Println("\nStorage Levels:")
		for _, s := range storage.Data.Storage {
			fmt.Printf("  %s: %.1f %s\n", s.Location, s.Value, s.Units)
		}
	}

	// Price alerts
	alerts, err := client.GetAlerts(context.Background())
	if err != nil {
		log.Printf("Error getting alerts: %v\n", err)
	} else {
		fmt.Printf("\nPrice Alerts: %d configured\n", len(alerts))
		for _, a := range alerts {
			fmt.Printf("  %s: %s %s %.2f\n", a.Name, a.CommodityCode, a.ConditionOperator, a.ConditionValue)
		}
	}

	// Analytics: API usage performance
	perf, err := client.GetAnalyticsPerformance(context.Background(),
		oilpriceapi.WithAnalyticsRange("30d"))
	if err != nil {
		log.Printf("Error getting analytics performance: %v\n", err)
	} else {
		fmt.Printf("\nAnalytics (30d): %d requests, %.2f%% error rate\n",
			perf.Overview.TotalRequests, perf.Overview.ErrorRate)
	}

	// Energy Intelligence: OPEC production
	opec, err := client.EI().GetOpecProduction(context.Background())
	if err != nil {
		log.Printf("Error getting OPEC production: %v\n", err)
	} else {
		fmt.Printf("\nEI OPEC production source: %s\n", opec.Meta.Source)
	}
}
