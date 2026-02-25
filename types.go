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

// HistoricalPrice represents a single historical price point.
type HistoricalPrice struct {
	Price     float64 `json:"price"`
	CreatedAt string  `json:"created_at"`
	Code      string  `json:"code,omitempty"`
}

// HistoricalData contains the data from a historical prices response.
type HistoricalData struct {
	Prices []HistoricalPrice `json:"prices"`
}

// HistoricalResponse represents the response from /v1/prices/past_*.
type HistoricalResponse struct {
	Status string         `json:"status"`
	Data   HistoricalData `json:"data"`
}

// FuturesContract represents a single futures contract.
type FuturesContract struct {
	Contract string  `json:"contract"`
	Month    string  `json:"month"`
	Price    float64 `json:"price"`
	Change   float64 `json:"change,omitempty"`
	Volume   int     `json:"volume,omitempty"`
}

// FuturesData contains the data from a futures response.
type FuturesData struct {
	Contracts []FuturesContract `json:"contracts"`
}

// FuturesResponse represents the response from /v1/futures/*.
type FuturesResponse struct {
	Status string      `json:"status"`
	Data   FuturesData `json:"data"`
}

// MarineFuelPrice represents a single marine fuel price.
type MarineFuelPrice struct {
	Port     string  `json:"port"`
	FuelType string  `json:"fuel_type"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
	Unit     string  `json:"unit"`
	Region   string  `json:"region,omitempty"`
}

// MarineFuelsData contains the data from a marine fuels response.
type MarineFuelsData struct {
	Prices []MarineFuelPrice `json:"prices"`
}

// MarineFuelsResponse represents the response from /v1/marine-fuels/*.
type MarineFuelsResponse struct {
	Status string          `json:"status"`
	Data   MarineFuelsData `json:"data"`
}

// RigCountData contains rig count information.
type RigCountData struct {
	Oil                 int    `json:"oil"`
	Gas                 int    `json:"gas"`
	Total               int    `json:"total"`
	Misc                int    `json:"misc,omitempty"`
	ChangeFromPriorWeek int    `json:"change_from_prior_week,omitempty"`
	Date                string `json:"date"`
	Source              string `json:"source,omitempty"`
}

// RigCountResponse represents the response from /v1/rig-counts/*.
type RigCountResponse struct {
	Status string       `json:"status"`
	Data   RigCountData `json:"data"`
}

// DrillingRegion represents drilling data for a specific region.
type DrillingRegion struct {
	Region string `json:"region"`
	Count  int    `json:"count"`
}

// DrillingData contains drilling intelligence information.
type DrillingData struct {
	TotalWells      int              `json:"total_wells"`
	ActiveRigs      int              `json:"active_rigs"`
	PermitsIssued   int              `json:"permits_issued,omitempty"`
	Completions     int              `json:"completions,omitempty"`
	RegionBreakdown []DrillingRegion `json:"region_breakdown,omitempty"`
	Date            string           `json:"date"`
}

// DrillingResponse represents the response from /v1/drilling/*.
type DrillingResponse struct {
	Status string       `json:"status"`
	Data   DrillingData `json:"data"`
}

// Webhook represents a webhook configuration.
type Webhook struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Active    bool     `json:"active"`
	Secret    string   `json:"secret,omitempty"`
	CreatedAt string   `json:"created_at"`
}

// WebhookCreateInput contains the parameters for creating a webhook.
type WebhookCreateInput struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

// WebhooksData contains the data from a webhooks response.
type WebhooksData struct {
	Webhooks []Webhook `json:"webhooks"`
}

// WebhooksResponse represents the response from /v1/webhooks.
type WebhooksResponse struct {
	Status string       `json:"status"`
	Data   WebhooksData `json:"data"`
}

// WebhookResponse represents the response for a single webhook operation.
type WebhookResponse struct {
	Status string  `json:"status"`
	Data   Webhook `json:"data"`
}

// HistoricalOptions contains options for GetHistoricalPrices.
type HistoricalOptions struct {
	Commodity string
	Period    string
	Page      int
	PerPage   int
}

// HistoricalOption is a functional option for GetHistoricalPrices.
type HistoricalOption func(*HistoricalOptions)

// WithPeriod sets the historical period (day, week, month, year).
func WithPeriod(period string) HistoricalOption {
	return func(o *HistoricalOptions) {
		o.Period = period
	}
}

// WithPage sets the page number for paginated results.
func WithPage(page int) HistoricalOption {
	return func(o *HistoricalOptions) {
		o.Page = page
	}
}

// WithPerPage sets the number of results per page.
func WithPerPage(perPage int) HistoricalOption {
	return func(o *HistoricalOptions) {
		o.PerPage = perPage
	}
}

// FuturesOptions contains options for futures methods.
type FuturesOptions struct {
	Contract string
}

// FuturesOption is a functional option for futures methods.
type FuturesOption func(*FuturesOptions)

// WithContract sets the futures contract code (BZ or CL).
func WithContract(contract string) FuturesOption {
	return func(o *FuturesOptions) {
		o.Contract = contract
	}
}
