package oilpriceapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
	Version = "1.0.0"
)

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
// This endpoint is rate-limited to 20 requests per hour per IP.
// It returns prices for free-tier commodities only.
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
//	// Get all prices
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
		endpoint += "?by_code=" + options.Commodity
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
func (c *Client) GetHistoricalPrices(ctx context.Context, commodity string, opts ...HistoricalOption) (*HistoricalResponse, error) {
	options := &HistoricalOptions{
		Commodity: commodity,
		Period:    "month",
	}
	for _, opt := range opts {
		opt(options)
	}

	if !validPeriods[options.Period] {
		return nil, fmt.Errorf("invalid period %q: must be one of \"day\", \"week\", \"month\", \"year\"", options.Period)
	}

	endpoint := fmt.Sprintf("/v1/prices/past_%s?by_code=%s", options.Period, options.Commodity)
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

// GetFuturesLatest fetches the latest futures contract data.
func (c *Client) GetFuturesLatest(ctx context.Context, opts ...FuturesOption) (*FuturesResponse, error) {
	options := &FuturesOptions{Contract: "BZ"}
	for _, opt := range opts {
		opt(options)
	}

	endpoint := fmt.Sprintf("/v1/futures/latest?contract=%s", options.Contract)

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

// GetFuturesCurve fetches the futures forward curve.
func (c *Client) GetFuturesCurve(ctx context.Context, opts ...FuturesOption) (*FuturesResponse, error) {
	options := &FuturesOptions{Contract: "BZ"}
	for _, opt := range opts {
		opt(options)
	}

	endpoint := fmt.Sprintf("/v1/futures/curve?contract=%s", options.Contract)

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

// GetDrillingIntelligence fetches the latest drilling intelligence data.
func (c *Client) GetDrillingIntelligence(ctx context.Context) (*DrillingResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/v1/drilling/latest", nil)
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
