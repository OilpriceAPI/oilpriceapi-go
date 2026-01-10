// Package oilpriceapi provides a Go client for the Oil Price API.
//
// The Oil Price API provides real-time and historical oil price data
// for various commodities including Brent Crude, WTI, Natural Gas, and more.
//
// Example usage:
//
//	client := oilpriceapi.NewClient("your-api-key")
//	prices, err := client.GetLatestPrices(context.Background())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Brent: $%.2f\n", prices.Data.Prices[0].Price)
package oilpriceapi

// DemoPrice represents a price in demo mode.
type DemoPrice struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
	Unit     string  `json:"unit"`
}

// DemoMeta contains metadata for demo responses.
type DemoMeta struct {
	DemoMode             bool   `json:"demo_mode"`
	RateLimit            string `json:"rate_limit"`
	CommoditiesAvailable int    `json:"commodities_available,omitempty"`
}

// DemoPricesData contains the data from a demo prices response.
type DemoPricesData struct {
	Prices []DemoPrice `json:"prices"`
	Meta   DemoMeta    `json:"meta"`
}

// DemoPricesResponse represents the response from /v1/demo/prices.
type DemoPricesResponse struct {
	Status string         `json:"status"`
	Data   DemoPricesData `json:"data"`
}

// Price represents a single price entry.
type Price struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Currency  string  `json:"currency"`
	Unit      string  `json:"unit"`
	UpdatedAt string  `json:"updated_at"`
}

// PriceData contains the data from a prices response.
type PriceData struct {
	Prices []Price `json:"prices"`
}

// PricesResponse represents the response from /v1/prices/latest.
type PricesResponse struct {
	Status string    `json:"status"`
	Data   PriceData `json:"data"`
}

// Commodity represents a supported commodity.
type Commodity struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

// CommoditiesData contains the data from a commodities response.
type CommoditiesData struct {
	Commodities []Commodity `json:"commodities"`
}

// CommoditiesResponse represents the response from /v1/commodities.
type CommoditiesResponse struct {
	Status string          `json:"status"`
	Data   CommoditiesData `json:"data"`
}

// LatestPricesOptions contains options for GetLatestPrices.
type LatestPricesOptions struct {
	Commodity string
}

// LatestPricesOption is a functional option for GetLatestPrices.
type LatestPricesOption func(*LatestPricesOptions)

// WithCommodity filters prices by commodity code.
func WithCommodity(code string) LatestPricesOption {
	return func(o *LatestPricesOptions) {
		o.Commodity = code
	}
}
