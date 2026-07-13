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

import "encoding/json"

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
//
// Name is not returned by the production /v1/prices/latest endpoint; prefer
// Code for display. The Formatted, Type, Source, and CreatedAt fields mirror
// the production response and may be empty on other endpoints.
type Price struct {
	Code      string  `json:"code"`
	Name      string  `json:"name,omitempty"`
	Price     float64 `json:"price"`
	Formatted string  `json:"formatted,omitempty"`
	Currency  string  `json:"currency"`
	Unit      string  `json:"unit"`
	Type      string  `json:"type,omitempty"`
	Source    string  `json:"source,omitempty"`
	CreatedAt string  `json:"created_at,omitempty"`
	UpdatedAt string  `json:"updated_at"`
}

// PriceData contains the data from a prices response.
//
// The production /v1/prices/latest endpoint returns a single price object in
// "data" (not a {"prices": [...]} list), so PriceData accepts both shapes:
// a single object decodes into a one-element Prices slice. This keeps the
// documented prices.Data.Prices[0].Price access pattern working.
type PriceData struct {
	Prices []Price `json:"prices"`
}

// UnmarshalJSON decodes both response envelopes used by price endpoints:
//
//   - {"prices": [...]} — a list of prices
//   - {"code": "...", "price": ...} — a single price object, as returned by
//     the production /v1/prices/latest endpoint
func (d *PriceData) UnmarshalJSON(data []byte) error {
	// List shape first: {"prices": [...]}.
	var list struct {
		Prices []Price `json:"prices"`
	}
	if err := json.Unmarshal(data, &list); err == nil && len(list.Prices) > 0 {
		d.Prices = list.Prices
		return nil
	}

	// Single-object shape: the price fields live directly on "data".
	var single Price
	if err := json.Unmarshal(data, &single); err != nil {
		return err
	}
	if single.Code == "" && single.Price == 0 {
		// Neither shape matched — leave Prices empty so callers can surface
		// a loud error instead of silently returning zero prices.
		d.Prices = nil
		return nil
	}
	d.Prices = []Price{single}
	return nil
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

// FuturesContract represents a single futures contract as returned by the API.
//
// The production API returns contracts at the top level of the futures
// response (there is no {"data": ...} envelope). Each contract carries a
// contract code, its contract month (e.g. "2026-08"), and OHLC/price fields.
type FuturesContract struct {
	Code          string  `json:"code"`
	ContractMonth string  `json:"contract_month"`
	LastPrice     float64 `json:"last_price"`
	Currency      string  `json:"currency,omitempty"`
	Open          string  `json:"open,omitempty"`
	Close         string  `json:"close,omitempty"`
	High          string  `json:"high,omitempty"`
	Low           string  `json:"low,omitempty"`
	Volume        int     `json:"volume,omitempty"`
}

// FuturesResponse represents the response from /v1/futures/{slug} and
// /v1/futures/{slug}/curve.
//
// The API returns a top-level object (no {"data": ...} envelope). The latest
// settlement price is available at FrontMonth.LastPrice, and the full set of
// listed contracts is in Contracts.
type FuturesResponse struct {
	Commodity      string                 `json:"commodity,omitempty"`
	Source         string                 `json:"source,omitempty"`
	UpdatedAt      string                 `json:"updated_at,omitempty"`
	SettlementDate string                 `json:"settlement_date,omitempty"`
	FrontMonth     *FuturesContract       `json:"front_month,omitempty"`
	Contracts      []FuturesContract      `json:"contracts,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	DataAgeWarning interface{}            `json:"data_age_warning,omitempty"`

	// Error and Date are populated when the API returns a documented
	// no-data response, e.g. the curve endpoint returns
	// {"error":"No futures data available for curve analysis","date":...}.
	Error string `json:"error,omitempty"`
	Date  string `json:"date,omitempty"`
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

// WithContract sets the futures contract to fetch.
//
// It accepts either an API slug directly (e.g. "ice-brent", "natural-gas",
// "ttf-gas", "continuous/brent") or a short contract code that is mapped to
// the corresponding slug:
//
//	BZ          -> ice-brent
//	CL          -> ice-wti
//	G, QS       -> ice-gasoil
//	NG          -> natural-gas
//	TTF         -> ttf-gas
//	JKM         -> lng-jkm
//	EUA         -> eua-carbon
//	UKA         -> uk-carbon
//
// Slugs and codes are case-insensitive. Unknown values are passed through
// unchanged so future slugs work without an SDK update.
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

// ===================
// PRICE ALERTS
// ===================

// Alert represents a price alert configuration as returned by /v1/alerts.
//
// The API serializes both the legacy (alert_type/threshold) and the current
// (condition_operator/condition_value) alert schemas, so fields from both are
// present; only the ones relevant to a given alert are populated.
type Alert struct {
	ID            int    `json:"id"`
	CommodityCode string `json:"commodity_code"`
	Name          string `json:"name,omitempty"`

	// Current schema (condition-based).
	ConditionOperator string                 `json:"condition_operator,omitempty"`
	ConditionValue    float64                `json:"condition_value,omitempty"`
	WebhookURL        string                 `json:"webhook_url,omitempty"`
	Enabled           bool                   `json:"enabled,omitempty"`
	CooldownMinutes   int                    `json:"cooldown_minutes,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	HasWebhook        bool                   `json:"has_webhook,omitempty"`
	Condition         string                 `json:"condition,omitempty"`

	// Legacy schema (threshold/volatility-based).
	AlertType            string  `json:"alert_type,omitempty"`
	ThresholdValue       float64 `json:"threshold_value,omitempty"`
	ThresholdDirection   string  `json:"threshold_direction,omitempty"`
	VolatilityPeriod     string  `json:"volatility_period,omitempty"`
	VolatilityPercentage float64 `json:"volatility_percentage,omitempty"`
	Active               bool    `json:"active,omitempty"`

	// Analytics alert fields.
	AnalyticsType    string                 `json:"analytics_type,omitempty"`
	AnalyticsPeriod  string                 `json:"analytics_period,omitempty"`
	AnalyticsConfig  map[string]interface{} `json:"analytics_config,omitempty"`
	IsAnalyticsAlert bool                   `json:"is_analytics_alert,omitempty"`

	Summary       string `json:"summary,omitempty"`
	TriggerCount  int    `json:"trigger_count,omitempty"`
	LastTriggered string `json:"last_triggered_at,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

// AlertInput contains the parameters for creating or updating a price alert.
//
// It maps to the controller's price_alert strong-parameters. Fields with the
// omitempty tag are dropped when zero so partial updates are possible.
type AlertInput struct {
	Name              string                 `json:"name,omitempty"`
	CommodityCode     string                 `json:"commodity_code,omitempty"`
	ConditionOperator string                 `json:"condition_operator,omitempty"`
	ConditionValue    float64                `json:"condition_value,omitempty"`
	WebhookURL        string                 `json:"webhook_url,omitempty"`
	Enabled           *bool                  `json:"enabled,omitempty"`
	CooldownMinutes   int                    `json:"cooldown_minutes,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`

	// Analytics alert fields.
	AnalyticsType   string                 `json:"analytics_type,omitempty"`
	AnalyticsPeriod string                 `json:"analytics_period,omitempty"`
	AnalyticsConfig map[string]interface{} `json:"analytics_config,omitempty"`
}

// alertRequestBody wraps an AlertInput in the "price_alert" key the API expects.
type alertRequestBody struct {
	PriceAlert AlertInput `json:"price_alert"`
}

// AlertTriggersResponse represents the response from /v1/alerts/triggers.
//
// The endpoint is deprecated server-side and typically returns a message
// payload; the raw fields are captured here for forward compatibility.
type AlertTriggersResponse struct {
	Message  string          `json:"message,omitempty"`
	Triggers json.RawMessage `json:"triggers,omitempty"`
}

// ===================
// WEBHOOKS (extended CRUD)
// ===================

// WebhookEventStats summarizes delivery outcomes for a webhook.
type WebhookEventStats struct {
	Total     int `json:"total"`
	Delivered int `json:"delivered"`
	Failed    int `json:"failed"`
	Pending   int `json:"pending"`
}

// WebhookDetail is the full webhook representation returned by the show, update,
// and (extended) create endpoints. It differs from the basic Webhook type used
// by ListWebhooks/CreateWebhook in that it carries the rich configuration and
// the available-options metadata the management API exposes.
type WebhookDetail struct {
	ID                 int                    `json:"id"`
	URL                string                 `json:"url"`
	Status             string                 `json:"status"`
	Description        string                 `json:"description,omitempty"`
	Events             []string               `json:"events"`
	CommodityFilters   []string               `json:"commodity_filters,omitempty"`
	StateFilters       []string               `json:"state_filters,omitempty"`
	Conditions         map[string]interface{} `json:"conditions,omitempty"`
	ConditionLogic     string                 `json:"condition_logic,omitempty"`
	CustomHeaders      map[string]interface{} `json:"custom_headers,omitempty"`
	TimeoutSeconds     int                    `json:"timeout_seconds,omitempty"`
	MaxRetries         int                    `json:"max_retries,omitempty"`
	RateLimitPerSecond int                    `json:"rate_limit_per_second,omitempty"`
	FailureCount       int                    `json:"failure_count,omitempty"`
	LastTriggeredAt    string                 `json:"last_triggered_at,omitempty"`
	CreatedAt          string                 `json:"created_at,omitempty"`
	UpdatedAt          string                 `json:"updated_at,omitempty"`
	EventStats         WebhookEventStats      `json:"event_stats"`

	// Present on show/create/update detail responses.
	Secret               string   `json:"secret,omitempty"`
	AvailableEvents      []string `json:"available_events,omitempty"`
	AvailableCommodities []string `json:"available_commodities,omitempty"`
	AvailableStates      []string `json:"available_states,omitempty"`
	AvailableOperators   []string `json:"available_operators,omitempty"`
}

// WebhookUpdateInput contains the fields that can be changed on a webhook.
//
// Zero-valued fields are omitted, so callers can perform partial updates. The
// signing secret cannot be changed via update.
type WebhookUpdateInput struct {
	URL                string                 `json:"url,omitempty"`
	Status             string                 `json:"status,omitempty"`
	Description        string                 `json:"description,omitempty"`
	TimeoutSeconds     int                    `json:"timeout_seconds,omitempty"`
	MaxRetries         int                    `json:"max_retries,omitempty"`
	RateLimitPerSecond int                    `json:"rate_limit_per_second,omitempty"`
	ConditionLogic     string                 `json:"condition_logic,omitempty"`
	Events             []string               `json:"events,omitempty"`
	CommodityFilters   []string               `json:"commodity_filters,omitempty"`
	StateFilters       []string               `json:"state_filters,omitempty"`
	Conditions         map[string]interface{} `json:"conditions,omitempty"`
	CustomHeaders      map[string]interface{} `json:"custom_headers,omitempty"`
}

// WebhookTestResult is the response from POST /v1/webhooks/:id/test.
type WebhookTestResult struct {
	Message      string `json:"message"`
	ResponseCode int    `json:"response_code,omitempty"`
	ResponseBody string `json:"response_body,omitempty"`
	Error        string `json:"error,omitempty"`
}

// WebhookEvent represents a single webhook delivery attempt.
type WebhookEvent struct {
	ID           int    `json:"id"`
	EventType    string `json:"event_type"`
	Status       string `json:"status"`
	Attempts     int    `json:"attempts"`
	ResponseCode int    `json:"response_code,omitempty"`
	DeliveredAt  string `json:"delivered_at,omitempty"`
	NextRetryAt  string `json:"next_retry_at,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	PayloadSize  int    `json:"payload_size,omitempty"`
}

// WebhookEventsPagination contains the pagination metadata for an events page.
type WebhookEventsPagination struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// WebhookEventsResponse is the response from GET /v1/webhooks/:id/events.
type WebhookEventsResponse struct {
	Events     []WebhookEvent          `json:"events"`
	Pagination WebhookEventsPagination `json:"pagination"`
}

// WebhookEventsOptions contains options for GetWebhookEvents.
type WebhookEventsOptions struct {
	Page    int
	PerPage int
}

// WebhookEventsOption is a functional option for GetWebhookEvents.
type WebhookEventsOption func(*WebhookEventsOptions)

// WithWebhookPage sets the page number for webhook event pagination.
func WithWebhookPage(page int) WebhookEventsOption {
	return func(o *WebhookEventsOptions) {
		o.Page = page
	}
}

// WithWebhookPerPage sets the page size for webhook event pagination (max 100).
func WithWebhookPerPage(perPage int) WebhookEventsOption {
	return func(o *WebhookEventsOptions) {
		o.PerPage = perPage
	}
}

// ===================
// ANALYTICS
// ===================

// AnalyticsOverview contains the summary metrics from the analytics performance
// endpoint.
type AnalyticsOverview struct {
	TotalRequests   int     `json:"totalRequests"`
	MonthlyRequests int     `json:"monthlyRequests"`
	DailyAverage    int     `json:"dailyAverage"`
	PeakDay         string  `json:"peakDay"`
	PeakHour        int     `json:"peakHour"`
	UniqueEndpoints int     `json:"uniqueEndpoints"`
	ErrorRate       float64 `json:"errorRate"`
	AvgResponseTime float64 `json:"avgResponseTime"`
}

// AnalyticsDailyUsage is a single day's request/error counts.
type AnalyticsDailyUsage struct {
	Date     string `json:"date"`
	Requests int    `json:"requests"`
	Errors   int    `json:"errors"`
}

// AnalyticsHourly is a single hour's request count.
type AnalyticsHourly struct {
	Hour     int `json:"hour"`
	Requests int `json:"requests"`
}

// AnalyticsEndpointBreakdown is one row of the per-endpoint usage breakdown.
type AnalyticsEndpointBreakdown struct {
	Endpoint   string  `json:"endpoint"`
	Requests   int     `json:"requests"`
	Percentage float64 `json:"percentage"`
}

// AnalyticsGeographic is one row of the geographic usage breakdown.
type AnalyticsGeographic struct {
	Country  string `json:"country"`
	Requests int    `json:"requests"`
}

// AnalyticsPerformanceResponse is the response from /v1/analytics/performance.
type AnalyticsPerformanceResponse struct {
	Overview           AnalyticsOverview            `json:"overview"`
	DailyUsage         []AnalyticsDailyUsage        `json:"dailyUsage"`
	HourlyDistribution []AnalyticsHourly            `json:"hourlyDistribution"`
	EndpointBreakdown  []AnalyticsEndpointBreakdown `json:"endpointBreakdown"`
	GeographicData     []AnalyticsGeographic        `json:"geographicData"`
}

// AnalyticsResult is a flexible container for the correlation, trend, and
// forecast analytics endpoints. These endpoints return a varying set of
// top-level fields depending on the requested type, so the common fields are
// captured directly and the complete payload is preserved in Raw for callers
// that need fields beyond the typed set.
type AnalyticsResult struct {
	Type string          `json:"type,omitempty"`
	Code string          `json:"code,omitempty"`
	Tier string          `json:"tier,omitempty"`
	Raw  json.RawMessage `json:"-"`
}

// UnmarshalJSON captures the full analytics payload in Raw while also decoding
// the common typed fields.
func (a *AnalyticsResult) UnmarshalJSON(data []byte) error {
	a.Raw = append(a.Raw[:0], data...)

	type alias AnalyticsResult
	var tmp alias
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	tmp.Raw = a.Raw
	*a = AnalyticsResult(tmp)
	return nil
}

// AnalyticsOptions contains options for the analytics methods.
type AnalyticsOptions struct {
	Range  string
	Code1  string
	Code2  string
	Period int
	Type   string
	Method string
}

// AnalyticsOption is a functional option for the analytics methods.
type AnalyticsOption func(*AnalyticsOptions)

func applyAnalyticsOptions(opts []AnalyticsOption) *AnalyticsOptions {
	options := &AnalyticsOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

// WithAnalyticsRange sets the window for GetAnalyticsPerformance ("7d", "30d",
// "90d").
func WithAnalyticsRange(r string) AnalyticsOption {
	return func(o *AnalyticsOptions) {
		o.Range = r
	}
}

// WithAnalyticsCode1 sets the first commodity code for correlation analysis.
func WithAnalyticsCode1(code string) AnalyticsOption {
	return func(o *AnalyticsOptions) {
		o.Code1 = code
	}
}

// WithAnalyticsCode2 sets the second commodity code for correlation analysis.
func WithAnalyticsCode2(code string) AnalyticsOption {
	return func(o *AnalyticsOptions) {
		o.Code2 = code
	}
}

// WithAnalyticsPeriod sets the lookback window in days for analytics queries.
func WithAnalyticsPeriod(period int) AnalyticsOption {
	return func(o *AnalyticsOptions) {
		o.Period = period
	}
}

// WithAnalyticsType selects an analytics sub-type (e.g. "sma", "ema", "rsi",
// "levels", "matrix", "rolling", "historical").
func WithAnalyticsType(t string) AnalyticsOption {
	return func(o *AnalyticsOptions) {
		o.Type = t
	}
}

// WithAnalyticsMethod selects the forecast method for GetAnalyticsForecast.
func WithAnalyticsMethod(method string) AnalyticsOption {
	return func(o *AnalyticsOptions) {
		o.Method = method
	}
}

// ===================
// ENERGY INTELLIGENCE (EI)
// ===================

// EIMeta contains the response metadata common to EI endpoints.
type EIMeta struct {
	Source          string `json:"source,omitempty"`
	UpdateFrequency string `json:"update_frequency,omitempty"`
	Description     string `json:"description,omitempty"`
	GeneratedAt     string `json:"generated_at,omitempty"`
}

// EIResponse is the envelope returned by the Energy Intelligence endpoints.
//
// The Data field is the raw, endpoint-specific payload (oil-inventory report,
// OPEC production report, or well-permit list). Decode it into your own type
// when you need typed access:
//
//	var report MyReport
//	json.Unmarshal(resp.Data, &report)
type EIResponse struct {
	Status string          `json:"status,omitempty"`
	Data   json.RawMessage `json:"data"`
	Meta   EIMeta          `json:"meta"`
}

// EIOptions contains the query options shared across EI endpoints.
type EIOptions struct {
	Days   int
	Weeks  int
	Months int
	States string
}

// EIOption is a functional option for EI methods.
type EIOption func(*EIOptions)

// WithEIDays sets the trailing-window size in days (used by well permits).
func WithEIDays(days int) EIOption {
	return func(o *EIOptions) {
		o.Days = days
	}
}

// WithEIWeeks sets the trailing-window size in weeks (used by inventory series).
func WithEIWeeks(weeks int) EIOption {
	return func(o *EIOptions) {
		o.Weeks = weeks
	}
}

// WithEIMonths sets the trailing-window size in months (used by OPEC series).
func WithEIMonths(months int) EIOption {
	return func(o *EIOptions) {
		o.Months = months
	}
}

// WithEIStates filters by one or more comma-separated state codes (e.g. "TX,OK").
func WithEIStates(states string) EIOption {
	return func(o *EIOptions) {
		o.States = states
	}
}

// =============================================================================
// Streaming (WebSocket) types
//
// These mirror the payloads broadcast by the OilPriceAPI ActionCable server.
// Confirmed against app/channels/energy_prices_channel.rb (welcome snapshot)
// and app/jobs/broadcast_energy_prices_job.rb (price_update / rig_count_update).
// =============================================================================

// StreamedPrice is a single normalized price point as broadcast by the server.
//
// It mirrors the shape produced by BroadcastEnergyPricesJob#normalized_price_for_cached
// and EnergyPricesChannel#normalize_price. The 24h change fields are only
// present on the initial welcome snapshot.
type StreamedPrice struct {
	// NormalizedPrice is the price converted to the common base unit (USD / MMBtu).
	NormalizedPrice *float64 `json:"normalized_price"`
	// OriginalPrice is the price in its native unit/currency.
	OriginalPrice *float64 `json:"original_price"`
	// OriginalUnit is the native unit (e.g. "barrel_oil", "mmbtu").
	OriginalUnit string `json:"original_unit"`
	// OriginalCurrency is the native currency (e.g. "USD", "GBP", "EUR").
	OriginalCurrency string `json:"original_currency"`
	// Timestamp is the ISO-8601 timestamp of the underlying price record.
	Timestamp string `json:"timestamp"`
	// Change24h is the 24h absolute change (present on the welcome snapshot only).
	Change24h *float64 `json:"change_24h,omitempty"`
	// Change24hPercent is the 24h percent change (present on the welcome snapshot only).
	Change24hPercent *float64 `json:"change_24h_percent,omitempty"`
}

// StreamedOilPrices groups the oil commodities carried by streamed messages.
type StreamedOilPrices struct {
	Brent *StreamedPrice `json:"brent"`
	WTI   *StreamedPrice `json:"wti"`
}

// StreamedNaturalGasPrices groups the natural gas commodities carried by
// streamed messages.
type StreamedNaturalGasPrices struct {
	UK *StreamedPrice `json:"uk"`
	US *StreamedPrice `json:"us"`
	EU *StreamedPrice `json:"eu"`
}

// StreamedPriceMap is the prices map carried by welcome and price_update messages.
type StreamedPriceMap struct {
	Oil        StreamedOilPrices        `json:"oil"`
	NaturalGas StreamedNaturalGasPrices `json:"natural_gas"`
}

// PriceUpdate is a live price_update broadcast — the most common streamed message.
type PriceUpdate struct {
	Type         string           `json:"type"`
	Timestamp    string           `json:"timestamp"`
	BaseCurrency string           `json:"base_currency"`
	BaseUnit     string           `json:"base_unit"`
	Prices       StreamedPriceMap `json:"prices"`
}

// WelcomeSnapshot is the initial snapshot transmitted immediately on
// subscription confirmation.
//
// Note: the server uses type "welcome" for this *channel* message. It is
// distinct from the ActionCable transport-level "welcome" frame, which the
// client consumes internally and never surfaces.
type WelcomeSnapshot struct {
	Type string              `json:"type"`
	Data WelcomeSnapshotData `json:"data"`
}

// WelcomeSnapshotData is the payload of a WelcomeSnapshot.
type WelcomeSnapshotData struct {
	Timestamp    string            `json:"timestamp,omitempty"`
	BaseCurrency string            `json:"base_currency,omitempty"`
	BaseUnit     string            `json:"base_unit,omitempty"`
	Prices       *StreamedPriceMap `json:"prices,omitempty"`
	// DrillingIntelligence is present only for drilling-tier accounts. Its shape
	// varies, so it is exposed as raw JSON for forward compatibility.
	DrillingIntelligence json.RawMessage `json:"drilling_intelligence,omitempty"`
	// Error is present when the initial snapshot could not be built.
	Error string `json:"error,omitempty"`
}

// RigCount is the rig-count payload nested in a RigCountUpdate.
type RigCount struct {
	Code      string  `json:"code"`
	Region    string  `json:"region"`
	Count     float64 `json:"count"`
	Source    string  `json:"source"`
	UpdatedAt string  `json:"updated_at"`
}

// RigCountUpdate is a rig-count update broadcast (drilling / Professional+ accounts).
type RigCountUpdate struct {
	Type      string   `json:"type"`
	Timestamp string   `json:"timestamp"`
	RigCount  RigCount `json:"rig_count"`
}

// Update is a typed wrapper delivered on the stream's Updates channel. Exactly
// one of PriceUpdate, RigCountUpdate, or Welcome is non-nil, indicated by Type.
// Raw carries the undecoded channel message for forward compatibility with
// message types this SDK version does not model.
type Update struct {
	// Type is the channel message type: "price_update", "rig_count_update",
	// "welcome", or any future server-defined value.
	Type string
	// Price is set when Type == "price_update".
	Price *PriceUpdate
	// RigCount is set when Type == "rig_count_update".
	RigCount *RigCountUpdate
	// Welcome is set when Type == "welcome".
	Welcome *WelcomeSnapshot
	// Raw is the undecoded channel message payload (always set).
	Raw json.RawMessage
}

// =============================================================================
// MARKET BRIEF (#3245 Phase 1a) — GET /v1/market-brief
// =============================================================================

// MarketBriefForecast is the optional 1-month forecast attached to a commodity
// in a market brief. It is only present for commodities the forecast model
// covers (WTI, Brent, Natural Gas) and only when the data is available; any
// individual bound may be zero when the server omits it.
type MarketBriefForecast struct {
	Point      float64 `json:"point,omitempty"`
	Low        float64 `json:"low,omitempty"`
	High       float64 `json:"high,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

// MarketBriefCommodity is a single commodity entry in a market brief. It
// composes the latest spot price, the 24h change, freshness, and (when
// available) the 1-month forecast.
type MarketBriefCommodity struct {
	Code         string               `json:"code"`
	Name         string               `json:"name"`
	Price        float64              `json:"price"`
	Currency     string               `json:"currency"`
	Unit         string               `json:"unit,omitempty"`
	Change24hPct float64              `json:"change_24h_pct,omitempty"`
	Change24hAbs float64              `json:"change_24h_abs,omitempty"`
	AsOf         string               `json:"as_of,omitempty"`
	Source       string               `json:"source,omitempty"`
	Stale        bool                 `json:"stale"`
	Forecast1M   *MarketBriefForecast `json:"forecast_1m,omitempty"`
}

// MarketBriefSpread is a commodity spread (e.g. Brent-WTI) surfaced when both
// legs are present in the requested codes.
type MarketBriefSpread struct {
	Pair     string  `json:"pair"`
	Label    string  `json:"label"`
	Value    float64 `json:"value"`
	Currency string  `json:"currency"`
}

// MarketBriefDisruption is a single active supply disruption included in the
// narrative context (only present when narrative is requested).
type MarketBriefDisruption struct {
	Title    string `json:"title"`
	Severity string `json:"severity"`
	Region   string `json:"region,omitempty"`
}

// MarketBriefIndicator is a single economic indicator included in the narrative
// context (only present when narrative is requested).
type MarketBriefIndicator struct {
	Code      string  `json:"code"`
	Value     float64 `json:"value"`
	ChangePct float64 `json:"change_pct,omitempty"`
}

// MarketBriefContext is the optional narrative context block, present only when
// the brief was requested with WithNarrative(true).
type MarketBriefContext struct {
	Disruptions   []MarketBriefDisruption `json:"disruptions,omitempty"`
	TopIndicators []MarketBriefIndicator  `json:"top_indicators,omitempty"`
}

// MarketBrief is the data payload of a /v1/market-brief response. Context and
// Summary are only populated when the brief is requested with narrative
// enabled.
type MarketBrief struct {
	AsOf        string                 `json:"as_of"`
	Codes       []string               `json:"codes"`
	Commodities []MarketBriefCommodity `json:"commodities"`
	Spreads     []MarketBriefSpread    `json:"spreads,omitempty"`
	Context     *MarketBriefContext    `json:"context,omitempty"`
	Summary     string                 `json:"summary,omitempty"`
}

// MarketBriefResponse is the full envelope returned by GET /v1/market-brief.
type MarketBriefResponse struct {
	Status string      `json:"status"`
	Data   MarketBrief `json:"data"`
}

// MarketBriefOptions contains options for GetMarketBrief.
type MarketBriefOptions struct {
	Narrative bool
}

// MarketBriefOption is a functional option for GetMarketBrief.
type MarketBriefOption func(*MarketBriefOptions)

// WithNarrative requests the optional narrative context and summary fields in
// the market brief. When omitted, only the structured commodity data is
// returned.
func WithNarrative(narrative bool) MarketBriefOption {
	return func(o *MarketBriefOptions) {
		o.Narrative = narrative
	}
}

// =============================================================================
// AGENT SUBSCRIPTIONS (#3245 Phase 2) — /v1/subscriptions
// =============================================================================

// Subscription is a persistent agent "watch" — a saved set of commodity codes
// the platform re-evaluates on a fixed interval, emitting events when prices
// move. It is returned by the subscription CRUD endpoints.
type Subscription struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Codes           []string `json:"codes"`
	IntervalSeconds int      `json:"interval_seconds"`
	Status          string   `json:"status"`
	DeliverWebhook  bool     `json:"deliver_webhook"`
	Source          string   `json:"source,omitempty"`
	ToolName        string   `json:"tool_name,omitempty"`
	LastEvaluatedAt string   `json:"last_evaluated_at,omitempty"`
	NextRunAt       string   `json:"next_run_at,omitempty"`
	CreatedAt       string   `json:"created_at,omitempty"`
}

// SubscriptionsData is the data payload of a list-subscriptions response.
type SubscriptionsData struct {
	Subscriptions []Subscription `json:"subscriptions"`
}

// SubscriptionsResponse is the envelope returned by GET /v1/subscriptions.
type SubscriptionsResponse struct {
	Status string            `json:"status"`
	Data   SubscriptionsData `json:"data"`
}

// SubscriptionData is the data payload of a single-subscription response
// (create/show/update), which nests the watch under "subscription".
type SubscriptionData struct {
	Subscription Subscription `json:"subscription"`
}

// SubscriptionResponse is the envelope returned by POST /v1/subscriptions.
type SubscriptionResponse struct {
	Status string           `json:"status"`
	Data   SubscriptionData `json:"data"`
}

// SubscriptionInput contains the parameters for creating a subscription.
//
// Provide IntervalSeconds directly, or use WithInterval on the input via the
// IntervalString helper to convert a friendly duration ("5m", "1h") to seconds.
// Source and ToolName are sent as the X-OPA-Source / X-OPA-Tool attribution
// headers (Source defaults to "sdk-go" when left blank).
type SubscriptionInput struct {
	Name            string   `json:"name"`
	Codes           []string `json:"codes"`
	IntervalSeconds int      `json:"interval_seconds,omitempty"`
	DeliverWebhook  bool     `json:"deliver_webhook,omitempty"`

	// Source and ToolName are attribution metadata sent as request headers, not
	// in the JSON body. They are excluded from the marshaled request body.
	Source   string `json:"-"`
	ToolName string `json:"-"`
}

// SubscriptionEvent is a single watch event returned by the poll endpoint. It
// carries a monotonically increasing per-user Seq cursor, the observed price
// Snapshot, and the Deltas that triggered the event.
type SubscriptionEvent struct {
	ID         int64                  `json:"id"`
	Seq        int64                  `json:"seq"`
	WatchID    string                 `json:"watch_id"`
	ObservedAt string                 `json:"observed_at,omitempty"`
	Snapshot   map[string]interface{} `json:"snapshot,omitempty"`
	Deltas     map[string]interface{} `json:"deltas,omitempty"`
	Source     string                 `json:"source,omitempty"`
	ToolName   string                 `json:"tool_name,omitempty"`
}

// SubscriptionEventsData is the data payload of a poll-events response.
type SubscriptionEventsData struct {
	Cursor  int64               `json:"cursor"`
	HasMore bool                `json:"has_more"`
	Events  []SubscriptionEvent `json:"events"`
}

// SubscriptionEventsResponse is the envelope returned by
// GET /v1/subscriptions/events.
type SubscriptionEventsResponse struct {
	Status string                 `json:"status"`
	Data   SubscriptionEventsData `json:"data"`
}

// EventsOptions contains options for GetSubscriptionEvents.
type EventsOptions struct {
	Since    int64
	sinceSet bool
	Limit    int
	WatchID  string
}

// EventOption is a functional option for GetSubscriptionEvents.
type EventOption func(*EventsOptions)

// WithSince sets the cursor: only events with seq greater than this value are
// returned. Pass the cursor from the previous response to page forward.
func WithSince(seq int64) EventOption {
	return func(o *EventsOptions) {
		o.Since = seq
		o.sinceSet = true
	}
}

// WithEventLimit caps the number of events returned (server clamps to 1..500).
func WithEventLimit(limit int) EventOption {
	return func(o *EventsOptions) {
		o.Limit = limit
	}
}

// WithEventWatchID restricts the poll to events from a single subscription.
func WithEventWatchID(watchID string) EventOption {
	return func(o *EventsOptions) {
		o.WatchID = watchID
	}
}
