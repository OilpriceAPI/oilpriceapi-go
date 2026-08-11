package oilpriceapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the default API base URL.
	DefaultBaseURL = "https://api.oilpriceapi.com"
	// DefaultTimeout is the default request timeout.
	DefaultTimeout = 30 * time.Second
	// DefaultRetries is the default number of retries.
	DefaultRetries = 3
	// Version is the SDK version.
	Version = "1.5.1"
)

// futuresContractSlugs maps short futures contract codes to the API path slug
// served under /v1/futures/{slug}. Slugs themselves are accepted directly by
// futuresSlug and pass through unchanged.
var futuresContractSlugs = map[string]string{
	"BZ":  "brent",
	"CL":  "wti",
	"G":   "gasoil",
	"QS":  "gasoil",
	"NG":  "natural-gas",
	"TTF": "ttf-gas",
	"JKM": "lng-jkm",
	"EUA": "eu-carbon",
	"UKA": "uk-carbon",
}

// futuresSlug resolves a user-supplied contract value to the API path slug.
//
// It accepts a short contract code (e.g. "BZ"), an API slug directly (e.g.
// "brent" or "continuous/brent"), and is case-insensitive. Unknown values
// are passed through unchanged so new slugs work without an SDK update. An
// empty value defaults to "brent". Legacy venue slugs remain valid explicit
// inputs and pass through unchanged.
func futuresSlug(contract string) string {
	c := strings.TrimSpace(contract)
	if c == "" {
		return "brent"
	}
	if slug, ok := futuresContractSlugs[strings.ToUpper(c)]; ok {
		return slug
	}
	return strings.ToLower(c)
}

// Client is the Oil Price API client.
type Client struct {
	apiKey     string
	baseURL    string
	retries    int
	httpClient *http.Client
}

// ClientOption is a functional option for configuring the client.
type ClientOption func(*Client)

// NewClient creates a new Oil Price API client.
//
// Example:
//
//	// Basic usage
//	client := oilpriceapi.NewClient("your-api-key")
//
//	// With custom options
//	client := oilpriceapi.NewClient("your-api-key",
//	    oilpriceapi.WithTimeout(10*time.Second),
//	    oilpriceapi.WithRetries(5),
//	)
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		retries: DefaultRetries,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// WithBaseURL sets a custom base URL.
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithTimeout sets a custom request timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithRetries sets the number of retry attempts.
func WithRetries(retries int) ClientOption {
	return func(c *Client) {
		c.retries = retries
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// GetDemoPrices fetches demo prices (no authentication required).
//
// Current limits and available commodities are returned in response metadata.
//
// Example:
//
//	client := oilpriceapi.NewClient("") // No API key needed
//	prices, err := client.GetDemoPrices(context.Background())
func (c *Client) GetDemoPrices(ctx context.Context) (*DemoPricesResponse, error) {
	// Use a temporary client with no API key so doRequest omits the Authorization
	// header. This gives the demo endpoint the same retry logic as all other methods.
	demo := &Client{
		apiKey:     "",
		baseURL:    c.baseURL,
		retries:    c.retries,
		httpClient: c.httpClient,
	}

	resp, err := demo.doRequest(ctx, "GET", "/v1/demo/prices", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result DemoPricesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetLatestPrices fetches the latest commodity prices.
//
// Example:
//
//	// Get the default latest price
//	prices, err := client.GetLatestPrices(ctx)
//
//	// Get specific commodity
//	prices, err := client.GetLatestPrices(ctx, oilpriceapi.WithCommodity("BRENT_CRUDE_USD"))
func (c *Client) GetLatestPrices(ctx context.Context, opts ...LatestPricesOption) (*PricesResponse, error) {
	options := &LatestPricesOptions{}
	for _, opt := range opts {
		opt(options)
	}

	endpoint := "/v1/prices/latest"
	if options.Commodity != "" {
		endpoint += "?by_code=" + url.QueryEscape(options.Commodity)
	}

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result PricesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetCommodities fetches the list of available commodities.
//
// Example:
//
//	commodities, err := client.GetCommodities(ctx)
//	for _, c := range commodities.Data.Commodities {
//	    fmt.Printf("%s: %s\n", c.Code, c.Name)
//	}
func (c *Client) GetCommodities(ctx context.Context) (*CommoditiesResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/v1/commodities", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result CommoditiesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// validPeriods is the set of accepted period values for GetHistoricalPrices.
var validPeriods = map[string]bool{
	"day":   true,
	"week":  true,
	"month": true,
	"year":  true,
}

// GetHistoricalPrices fetches historical price data for a commodity.
//
// By default it queries one of the fixed-period endpoints
// (/v1/prices/past_day, past_week, past_month, past_year) selected via
// WithPeriod. Supplying WithStartDate and/or WithEndDate switches to the
// flexible /v1/prices/historical endpoint, which supports an arbitrary date
// range and an optional aggregation interval set via WithInterval.
//
// Example:
//
//	// Fixed period (last month)
//	prices, err := client.GetHistoricalPrices(ctx, "BRENT_CRUDE_USD")
//
//	// Custom date range with daily aggregation
//	prices, err := client.GetHistoricalPrices(ctx, "WTI_USD",
//	    oilpriceapi.WithStartDate("2024-01-01"),
//	    oilpriceapi.WithEndDate("2024-12-31"),
//	    oilpriceapi.WithInterval("daily"),
//	)
func (c *Client) GetHistoricalPrices(ctx context.Context, commodity string, opts ...HistoricalOption) (*HistoricalResponse, error) {
	options := &HistoricalOptions{
		Commodity: commodity,
		Period:    "month",
	}
	for _, opt := range opts {
		opt(options)
	}

	var endpoint string
	if options.StartDate != "" || options.EndDate != "" {
		// Flexible date-range endpoint.
		endpoint = fmt.Sprintf("/v1/prices/historical?by_code=%s", url.QueryEscape(options.Commodity))
		if options.StartDate != "" {
			endpoint += "&start_date=" + url.QueryEscape(options.StartDate)
		}
		if options.EndDate != "" {
			endpoint += "&end_date=" + url.QueryEscape(options.EndDate)
		}
		if options.Interval != "" {
			endpoint += "&interval=" + url.QueryEscape(options.Interval)
		}
	} else {
		if !validPeriods[options.Period] {
			return nil, fmt.Errorf("invalid period %q: must be one of \"day\", \"week\", \"month\", \"year\"", options.Period)
		}
		endpoint = fmt.Sprintf("/v1/prices/past_%s?by_code=%s", options.Period, url.QueryEscape(options.Commodity))
	}
	if options.Page > 0 {
		endpoint += fmt.Sprintf("&page=%d", options.Page)
	}
	if options.PerPage > 0 {
		endpoint += fmt.Sprintf("&per_page=%d", options.PerPage)
	}

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result HistoricalResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetForecasts fetches monthly price forecasts.
//
// Without options it returns the latest forecasts for all supported
// commodities. Use WithForecastCommodity to fetch the forecast for a single
// commodity.
//
// Example:
//
//	// Default forecast response
//	forecasts, err := client.GetForecasts(ctx)
//	for _, f := range forecasts.Data.Commodities {
//	    fmt.Printf("%s 1m: $%.2f\n", f.Commodity, f.Forecasts["1_month"].PointEstimate)
//	}
//
//	// Single commodity
//	wti, err := client.GetForecasts(ctx, oilpriceapi.WithForecastCommodity("WTI_USD"))
func (c *Client) GetForecasts(ctx context.Context, opts ...ForecastsOption) (*ForecastsResponse, error) {
	options := &ForecastsOptions{}
	for _, opt := range opts {
		opt(options)
	}

	endpoint := "/v1/forecasts/monthly"
	if options.Commodity != "" {
		endpoint += "?commodity=" + url.QueryEscape(options.Commodity)
	}

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result ForecastsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetForecastsAccuracy fetches aggregate accuracy statistics for the monthly
// forecast model.
//
// Use WithForecastCommodity to scope to a single commodity and
// WithLookbackMonths to set the lookback window (3-36 months, default 12).
func (c *Client) GetForecastsAccuracy(ctx context.Context, opts ...ForecastsOption) (*ForecastAccuracyResponse, error) {
	options := &ForecastsOptions{}
	for _, opt := range opts {
		opt(options)
	}

	query := url.Values{}
	if options.Commodity != "" {
		query.Set("commodity", options.Commodity)
	}
	if options.LookbackMonths > 0 {
		query.Set("lookback_months", strconv.Itoa(options.LookbackMonths))
	}

	endpoint := "/v1/forecasts/monthly/accuracy"
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result ForecastAccuracyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetStorage fetches the latest storage levels across all tracked hubs
// (Cushing, US SPR, and regional storage).
//
// Example:
//
//	storage, err := client.GetStorage(ctx)
//	for _, s := range storage.Data.Storage {
//	    fmt.Printf("%s: %.1f %s\n", s.Location, s.Value, s.Units)
//	}
func (c *Client) GetStorage(ctx context.Context) (*StorageResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/v1/storage", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result StorageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetStorageCushing fetches detailed storage levels and analytics for the
// Cushing, Oklahoma hub.
func (c *Client) GetStorageCushing(ctx context.Context) (*StorageHubResponse, error) {
	return c.getStorageHub(ctx, "/v1/storage/cushing")
}

// GetStorageSPR fetches detailed storage levels and analytics for the US
// Strategic Petroleum Reserve.
func (c *Client) GetStorageSPR(ctx context.Context) (*StorageHubResponse, error) {
	return c.getStorageHub(ctx, "/v1/storage/spr")
}

// getStorageHub is the shared implementation for single-hub storage endpoints.
func (c *Client) getStorageHub(ctx context.Context, endpoint string) (*StorageHubResponse, error) {
	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result StorageHubResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetFuturesLatest fetches the latest futures curve for a contract.
//
// The contract is selected with WithContract, which accepts either a short
// code (e.g. "BZ", "CL", "NG") or an API slug (e.g. "brent",
// "natural-gas"). It defaults to Brent.
//
// Example:
//
//	// Default (Brent)
//	resp, err := client.GetFuturesLatest(ctx)
//
//	// WTI by code, or equivalently by slug
//	resp, err := client.GetFuturesLatest(ctx, oilpriceapi.WithContract("CL"))
//	resp, err := client.GetFuturesLatest(ctx, oilpriceapi.WithContract("wti"))
func (c *Client) GetFuturesLatest(ctx context.Context, opts ...FuturesOption) (*FuturesResponse, error) {
	options := &FuturesOptions{}
	for _, opt := range opts {
		opt(options)
	}

	endpoint := "/v1/futures/" + futuresSlug(options.Contract)

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result FuturesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetFuturesCurve fetches the futures forward curve (contango/backwardation
// analysis) for a contract.
//
// The contract is selected with WithContract, which accepts either a short
// code (e.g. "BZ", "CL", "NG") or an API slug (e.g. "brent",
// "natural-gas"). It defaults to Brent.
//
// Example:
//
//	resp, err := client.GetFuturesCurve(ctx, oilpriceapi.WithContract("wti"))
func (c *Client) GetFuturesCurve(ctx context.Context, opts ...FuturesOption) (*FuturesResponse, error) {
	options := &FuturesOptions{}
	for _, opt := range opts {
		opt(options)
	}

	endpoint := "/v1/futures/" + futuresSlug(options.Contract) + "/curve"

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result FuturesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetMarineFuels fetches the latest marine fuel prices.
func (c *Client) GetMarineFuels(ctx context.Context) (*MarineFuelsResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/v1/marine-fuels/latest", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result MarineFuelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetRigCounts fetches the latest rig count data.
func (c *Client) GetRigCounts(ctx context.Context) (*RigCountResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/v1/rig-counts/latest", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result RigCountResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetDrillingSummary fetches the canonical drilling intelligence summary
// (rig counts, frac spread count, 30-day well permits, DUC wells).
//
// Endpoint: GET /v1/drilling-intelligence/summary. Requires the
// drilling_intelligence entitlement. For
// well-level production data see Client.WellProduction.
func (c *Client) GetDrillingSummary(ctx context.Context) (*DrillingResponse, error) {
	return c.getDrillingSummary(ctx, "/v1/drilling-intelligence/summary")
}

// GetDrillingIntelligence retains the legacy /v1/drilling/latest route for
// backward compatibility. New code should call GetDrillingSummary.
func (c *Client) GetDrillingIntelligence(ctx context.Context) (*DrillingResponse, error) {
	return c.getDrillingSummary(ctx, "/v1/drilling/latest")
}

func (c *Client) getDrillingSummary(ctx context.Context, endpoint string) (*DrillingResponse, error) {
	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result DrillingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListWebhooks fetches all configured webhooks.
func (c *Client) ListWebhooks(ctx context.Context) (*WebhooksResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/v1/webhooks", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result WebhooksResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateWebhook creates a new webhook.
func (c *Client) CreateWebhook(ctx context.Context, input WebhookCreateInput) (*WebhookResponse, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(ctx, "POST", "/v1/webhooks", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, c.handleError(resp)
	}

	var result WebhookResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteWebhook deletes a webhook by ID.
func (c *Client) DeleteWebhook(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, "DELETE", "/v1/webhooks/"+id, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return c.handleError(resp)
	}
	return nil
}

// setHeaders sets the common request headers.
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("oilpriceapi-go/%s", Version))
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Token "+c.apiKey)
	}
}

// handleError processes HTTP error responses.
func (c *Client) handleError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	message := string(body)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &AuthenticationError{Message: message}
	case http.StatusTooManyRequests:
		retryAfter := 0
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			retryAfter, _ = strconv.Atoi(ra)
		}
		return &RateLimitError{Message: message, RetryAfter: retryAfter}
	case http.StatusNotFound:
		return &NotFoundError{Message: message}
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return &ServerError{Message: message, StatusCode: resp.StatusCode}
	default:
		return &APIError{Message: message, StatusCode: resp.StatusCode}
	}
}

// doRequest makes an authenticated request with retry logic.
func (c *Client) doRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Response, error) {
	return c.doRequestWithHeaders(ctx, method, endpoint, body, nil)
}

// doRequestWithHeaders is doRequest with additional per-request headers (e.g.
// the X-OPA-Source / X-OPA-Tool attribution headers used by subscriptions).
// The extra headers are applied after the common headers, so they take
// precedence.
func (c *Client) doRequestWithHeaders(ctx context.Context, method, endpoint string, body io.Reader, headers map[string]string) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.retries; attempt++ {
		// If body is a *bytes.Reader we can rewind it between retries.
		// For nil bodies this is a no-op.
		if br, ok := body.(*bytes.Reader); ok && attempt > 0 {
			br.Seek(0, io.SeekStart)
		}

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, body)
		if err != nil {
			return nil, err
		}

		c.setHeaders(req)
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < c.retries {
				delay := time.Duration(1<<uint(attempt)) * time.Second
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
				continue
			}
			return nil, err
		}

		// Success
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		// Don't retry 401
		if resp.StatusCode == 401 {
			return resp, nil
		}

		// Retry on 429 and 5xx
		if (resp.StatusCode == 429 || resp.StatusCode >= 500) && attempt < c.retries {
			resp.Body.Close()

			delay := time.Duration(1<<uint(attempt)) * time.Second
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if seconds, err := strconv.Atoi(ra); err == nil {
					delay = time.Duration(seconds) * time.Second
				}
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			continue
		}

		// Non-retryable error
		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", c.retries, lastErr)
}
