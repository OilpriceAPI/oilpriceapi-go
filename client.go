package oilpriceapi

import (
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
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/v1/demo/prices", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("oilpriceapi-go/%s", Version))

	resp, err := c.httpClient.Do(req)
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

	url := c.baseURL + "/v1/prices/latest"
	if options.Commodity != "" {
		url += "?by_code=" + options.Commodity
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
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
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/v1/commodities", nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
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
