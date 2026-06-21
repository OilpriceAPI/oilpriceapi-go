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
//
// The Currency, Formatted, Type, PriceType, Unit, and Source fields are only
// populated by the flexible date-range endpoint (/v1/prices/historical); the
// fixed-period endpoints leave them empty.
type HistoricalPrice struct {
	Price     float64 `json:"price"`
	CreatedAt string  `json:"created_at"`
	Code      string  `json:"code,omitempty"`
	Currency  string  `json:"currency,omitempty"`
	Formatted string  `json:"formatted,omitempty"`
	Type      string  `json:"type,omitempty"`
	PriceType string  `json:"price_type,omitempty"`
	Unit      string  `json:"unit,omitempty"`
	Source    string  `json:"source,omitempty"`
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
	StartDate string
	EndDate   string
	Interval  string
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

// WithStartDate sets the start of a custom date range (YYYY-MM-DD).
//
// Setting a start or end date routes the request to /v1/prices/historical
// instead of the fixed-period endpoints.
func WithStartDate(date string) HistoricalOption {
	return func(o *HistoricalOptions) {
		o.StartDate = date
	}
}

// WithEndDate sets the end of a custom date range (YYYY-MM-DD).
//
// Setting a start or end date routes the request to /v1/prices/historical
// instead of the fixed-period endpoints.
func WithEndDate(date string) HistoricalOption {
	return func(o *HistoricalOptions) {
		o.EndDate = date
	}
}

// WithInterval sets the aggregation interval for date-range historical queries
// (e.g. "daily", "weekly", "monthly", "hourly", "raw").
func WithInterval(interval string) HistoricalOption {
	return func(o *HistoricalOptions) {
		o.Interval = interval
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

// ForecastHorizon represents a single forecast horizon (e.g. 1-month, 3-month).
type ForecastHorizon struct {
	Period        string  `json:"period"`
	PointEstimate float64 `json:"point_estimate"`
	LowBound      float64 `json:"low_bound"`
	HighBound     float64 `json:"high_bound"`
	Confidence    float64 `json:"confidence"`
}

// ForecastModelInputs contains the model signals used to generate a forecast.
type ForecastModelInputs struct {
	EMATrendValue   float64 `json:"ema_trend_value"`
	EIASTEOValue    float64 `json:"eia_steo_value"`
	InventorySignal float64 `json:"inventory_signal"`
	RigCountSignal  float64 `json:"rig_count_signal"`
}

// ForecastAccuracy contains backtested accuracy information for a forecast.
type ForecastAccuracy struct {
	ActualPrice1M      float64 `json:"actual_price_1m"`
	ActualPrice3M      float64 `json:"actual_price_3m"`
	MAPE1M             float64 `json:"mape_1m"`
	MAPE3M             float64 `json:"mape_3m"`
	DirectionCorrect1M bool    `json:"direction_correct_1m"`
	DirectionCorrect3M bool    `json:"direction_correct_3m"`
}

// Forecast represents a monthly price forecast for a single commodity.
type Forecast struct {
	Commodity         string                     `json:"commodity"`
	GeneratedAt       string                     `json:"generated_at"`
	GeneratedForMonth string                     `json:"generated_for_month"`
	Forecasts         map[string]ForecastHorizon `json:"forecasts"`
	ModelInputs       ForecastModelInputs        `json:"model_inputs"`
	Drivers           []string                   `json:"drivers,omitempty"`
	Accuracy          *ForecastAccuracy          `json:"accuracy,omitempty"`
	ModelVersion      string                     `json:"model_version,omitempty"`
	Disclaimer        string                     `json:"disclaimer,omitempty"`
}

// ForecastsData contains the data from a forecasts response.
//
// When no commodity is requested the API returns the full set in Commodities
// along with the top-level Period and GeneratedAt. When a single commodity is
// requested, the API returns that forecast directly, so the Commodity,
// Forecasts, ModelInputs, Drivers, and Accuracy fields are populated and
// Commodities is empty.
type ForecastsData struct {
	// Multi-commodity fields.
	Period      string     `json:"period,omitempty"`
	Commodities []Forecast `json:"commodities,omitempty"`

	// Single-commodity fields (also covers fields shared with Forecast).
	Commodity         string                     `json:"commodity,omitempty"`
	GeneratedAt       string                     `json:"generated_at,omitempty"`
	GeneratedForMonth string                     `json:"generated_for_month,omitempty"`
	Forecasts         map[string]ForecastHorizon `json:"forecasts,omitempty"`
	ModelInputs       ForecastModelInputs        `json:"model_inputs,omitempty"`
	Drivers           []string                   `json:"drivers,omitempty"`
	Accuracy          *ForecastAccuracy          `json:"accuracy,omitempty"`
	ModelVersion      string                     `json:"model_version,omitempty"`
	Disclaimer        string                     `json:"disclaimer,omitempty"`
}

// ForecastsResponse represents the response from /v1/forecasts/monthly.
type ForecastsResponse struct {
	Data ForecastsData `json:"data"`
}

// ForecastAccuracyStatistics contains aggregate accuracy statistics.
type ForecastAccuracyStatistics struct {
	SampleSize            int     `json:"sample_size"`
	AvgMAPE1M             float64 `json:"avg_mape_1m"`
	AvgMAPE3M             float64 `json:"avg_mape_3m"`
	DirectionalAccuracy1M float64 `json:"directional_accuracy_1m"`
	DirectionalAccuracy3M float64 `json:"directional_accuracy_3m"`
}

// ForecastAccuracyData contains the data from a forecast accuracy response.
type ForecastAccuracyData struct {
	LookbackMonths int                        `json:"lookback_months"`
	Commodity      string                     `json:"commodity"`
	Statistics     ForecastAccuracyStatistics `json:"statistics"`
}

// ForecastAccuracyResponse represents the response from /v1/forecasts/monthly/accuracy.
type ForecastAccuracyResponse struct {
	Data ForecastAccuracyData `json:"data"`
}

// ForecastsOptions contains options for forecast methods.
type ForecastsOptions struct {
	Commodity      string
	LookbackMonths int
}

// ForecastsOption is a functional option for forecast methods.
type ForecastsOption func(*ForecastsOptions)

// WithForecastCommodity filters forecasts by commodity code.
func WithForecastCommodity(code string) ForecastsOption {
	return func(o *ForecastsOptions) {
		o.Commodity = code
	}
}

// WithLookbackMonths sets the accuracy lookback window in months (3-36).
func WithLookbackMonths(months int) ForecastsOption {
	return func(o *ForecastsOptions) {
		o.LookbackMonths = months
	}
}

// StorageLevel represents a single storage location's latest reading.
type StorageLevel struct {
	Code                string  `json:"code"`
	Location            string  `json:"location"`
	Value               float64 `json:"value"`
	Units               string  `json:"units"`
	CapacityUtilization float64 `json:"capacity_utilization"`
	MarketSignal        string  `json:"market_signal,omitempty"`
	DataDate            string  `json:"data_date"`
	CreatedAt           string  `json:"created_at,omitempty"`
}

// StorageData contains the data from a storage list response.
type StorageData struct {
	Storage []StorageLevel `json:"storage"`
}

// StorageResponse represents the response from /v1/storage.
type StorageResponse struct {
	Status string      `json:"status"`
	Data   StorageData `json:"data"`
}

// StorageHubCurrent contains the latest reading for a single storage hub.
type StorageHubCurrent struct {
	Value               float64 `json:"value"`
	Units               string  `json:"units"`
	CapacityUtilization float64 `json:"capacity_utilization"`
	WeekOverWeekChange  float64 `json:"week_over_week_change"`
	DataDate            string  `json:"data_date"`
	CreatedAt           string  `json:"created_at,omitempty"`
	UpdatedAt           string  `json:"updated_at,omitempty"`
}

// StorageHubAnalytics contains analytics for a single storage hub.
type StorageHubAnalytics struct {
	MarketSignal       string  `json:"market_signal,omitempty"`
	TradingImplication string  `json:"trading_implication,omitempty"`
	HistoricalAverage  float64 `json:"historical_average"`
	Week52High         float64 `json:"week_52_high"`
	Week52Low          float64 `json:"week_52_low"`
	Trend              string  `json:"trend,omitempty"`
}

// StorageHubMetadata contains metadata for a single storage hub.
type StorageHubMetadata struct {
	Source              string  `json:"source,omitempty"`
	UpdateFrequency     string  `json:"update_frequency,omitempty"`
	OperationalCapacity float64 `json:"operational_capacity,omitempty"`
	TotalCapacity       float64 `json:"total_capacity,omitempty"`
}

// StorageHub represents a detailed view of a single storage hub (Cushing, SPR).
type StorageHub struct {
	Current   StorageHubCurrent   `json:"current"`
	Analytics StorageHubAnalytics `json:"analytics"`
	Metadata  StorageHubMetadata  `json:"metadata"`
}

// StorageHubData contains the data from a single-hub storage response.
type StorageHubData struct {
	Storage StorageHub `json:"storage"`
}

// StorageHubResponse represents the response from /v1/storage/cushing and /v1/storage/spr.
type StorageHubResponse struct {
	Status string         `json:"status"`
	Data   StorageHubData `json:"data"`
}
