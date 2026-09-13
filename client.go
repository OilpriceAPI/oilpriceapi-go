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
	// DefaultMaxRetryWait bounds the total time one call will spend waiting
	// between automatic retries before giving up and returning the typed error
	// to the caller. It exists because Retry-After is server-controlled: the
	// keyless demo endpoint answers 429 with a Retry-After counting down to
	// the next daily reset, which is hours.
	DefaultMaxRetryWait = 60 * time.Second
	// Version is the SDK version.
	Version = "1.6.0"
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
	apiKey       string
	baseURL      string
	retries      int
	maxRetryWait time.Duration
	httpClient   *http.Client
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
		apiKey:       apiKey,
		baseURL:      DefaultBaseURL,
		retries:      DefaultRetries,
		maxRetryWait: DefaultMaxRetryWait,
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

// WithMaxRetryWait bounds the total time one call will spend waiting between
// automatic retries.
//
// The budget covers every wait in the call added together, not each wait on
// its own: with the default three retries, a per-wait bound let a server
// answering Retry-After just under the budget hold the call for three times
// the number the caller configured.
//
// Retry-After is chosen by the server, and a rate limit that resets at the end
// of the day produces a Retry-After of hours. Rather than sleep for that (a
// caller using context.Background() has no deadline to rescue it), the client
// stops retrying and returns the typed *RateLimitError, whose RetryAfter field
// carries the server's requested wait so the caller can decide.
//
// A value of zero or less restores DefaultMaxRetryWait.
func WithMaxRetryWait(d time.Duration) ClientOption {
	return func(c *Client) {
		if d <= 0 {
			d = DefaultMaxRetryWait
		}
		c.maxRetryWait = d
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
		apiKey:       "",
		baseURL:      c.baseURL,
		retries:      c.retries,
		maxRetryWait: c.maxRetryWait,
		httpClient:   c.httpClient,
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

// resolveEndpoint turns a caller-supplied API path into an absolute URL and
// proves the result still addresses the configured base origin.
//
// This runs before the request is built and before credentials are attached,
// because setHeaders adds "Authorization: Token <key>" unconditionally: a path
// able to move the request to another host would hand that key to that host.
//
// The contract is the one the SDK already documents — an origin-relative path
// beginning with a single "/" — now enforced rather than assumed. Enforcement
// is structural first (a path that cannot introduce an authority component)
// and then confirmed by resolving and comparing (scheme, host, port) against
// the configured base URL.
func (c *Client) resolveEndpoint(endpoint string) (string, error) {
	// Anything that is not a printable, space-free ASCII path is rejected
	// outright rather than handed to a parser whose normalisation we would
	// then have to reason about.
	for i := 0; i < len(endpoint); i++ {
		if ch := endpoint[i]; ch <= ' ' || ch == 0x7f {
			return "", &InvalidPathError{Path: endpoint, Reason: "contains a space or control character"}
		}
	}

	// The authority, if any, can only appear before the query or fragment.
	pathPart := endpoint
	if i := strings.IndexAny(pathPart, "?#"); i >= 0 {
		pathPart = pathPart[:i]
	}

	switch {
	case pathPart == "":
		return "", &InvalidPathError{Path: endpoint, Reason: "path is empty"}
	case pathPart[0] != '/':
		// Covers "v1/prices", "@evil.invalid/x", ".evil.invalid/x",
		// "http://evil.invalid/x" and every other form that would append to
		// the base host instead of to its path.
		return "", &InvalidPathError{Path: endpoint, Reason: "path does not start with \"/\""}
	case strings.HasPrefix(pathPart, "//"):
		// Scheme-relative reference: harmless under plain concatenation, but
		// it becomes an authority the moment anything resolves it as a URL
		// reference, so it is refused at the boundary.
		return "", &InvalidPathError{Path: endpoint, Reason: "scheme-relative path would name another host"}
	case strings.Contains(pathPart, "\\"):
		// Backslashes are treated as slashes by several parsers.
		return "", &InvalidPathError{Path: endpoint, Reason: "path contains a backslash"}
	}

	for _, segment := range strings.Split(pathPart, "/") {
		if segment == ".." {
			return "", &InvalidPathError{Path: endpoint, Reason: "path contains a \"..\" segment"}
		}
	}

	base, err := url.Parse(strings.TrimRight(c.baseURL, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", &InvalidPathError{Path: endpoint, Reason: "client base URL is not an absolute http(s) URL"}
	}

	full := base.String() + endpoint

	// Structurally this cannot move the origin. Confirm it anyway: this is the
	// assertion that survives a future refactor of the joining above.
	resolved, err := url.Parse(full)
	if err != nil {
		return "", &InvalidPathError{Path: endpoint, Reason: "path does not form a valid URL"}
	}
	if !sameOrigin(base, resolved) {
		return "", &InvalidPathError{Path: endpoint, Reason: "path changes the API origin"}
	}

	return full, nil
}

// sameOrigin compares scheme, host and port, folding the default port for the
// scheme so "https://host" and "https://host:443" are one origin.
func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && originHost(a) == originHost(b)
}

func originHost(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "https":
			port = "443"
		case "http":
			port = "80"
		}
	}
	return host + ":" + port
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
		retryAfter, _ := retryAfterSeconds(resp)
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
	if c.retries < 0 {
		return nil, &ConfigurationError{Option: "WithRetries", Reason: "must not be negative"}
	}

	maxWait := c.maxRetryWait
	if maxWait <= 0 {
		maxWait = DefaultMaxRetryWait
	}

	// Validate the path before any credential is attached to a request.
	requestURL, err := c.resolveEndpoint(endpoint)
	if err != nil {
		return nil, err
	}

	// Only safe requests are replayed automatically. See retry.go.
	replayable := isRetryableMethod(method)

	var lastErr error

	// spent is the automatic wait already consumed by this call. maxWait is a
	// budget for the sum of every wait, so each decision below is made against
	// what is left rather than against the whole number again.
	var spent time.Duration

	for attempt := 0; attempt <= c.retries; attempt++ {
		// If body is a *bytes.Reader we can rewind it between retries.
		// For nil bodies this is a no-op.
		if br, ok := body.(*bytes.Reader); ok && attempt > 0 {
			br.Seek(0, io.SeekStart)
		}

		req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
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
			// A transport failure is ambiguous: the server may have received
			// and committed the request and only the response was lost. Replay
			// that and a non-idempotent write happens twice.
			remaining := maxWait - spent
			if replayable && attempt < c.retries && remaining > 0 {
				wait := exponentialBackoff(attempt)
				if wait > remaining {
					wait = remaining
				}
				if waitErr := c.waitBeforeRetry(ctx, wait, remaining); waitErr != nil {
					return nil, waitErr
				}
				spent += wait
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

		if !replayable || attempt >= c.retries || !isRetryableStatus(resp) {
			// Hand the response back; the caller turns it into a typed error.
			// A durable quota lands here, so the caller sees *RateLimitError
			// immediately rather than after a wait that cannot help.
			return resp, nil
		}

		delay := backoffFor(resp, attempt)

		// Retry-After is server-controlled and can be hours. Waiting it out
		// under context.Background() blocks with no way to cancel, so a wait
		// past what is left of the budget (or past the caller's own deadline)
		// stops the retry loop and returns the typed error carrying the
		// server's request. Waiting a shorter time than the server asked for
		// would not clear the limit, so the remaining budget is a stop
		// condition here rather than a clamp.
		remaining := maxWait - spent
		if delay > remaining || exceedsDeadline(ctx, delay) {
			return resp, nil
		}

		resp.Body.Close()

		if waitErr := c.waitBeforeRetry(ctx, delay, remaining); waitErr != nil {
			return nil, waitErr
		}
		spent += delay
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d retries: %w", c.retries, lastErr)
	}
	return nil, fmt.Errorf("request failed after %d retries", c.retries)
}

// waitBeforeRetry sleeps for delay, bounded by maxWait, and returns early if
// the caller cancels.
func (c *Client) waitBeforeRetry(ctx context.Context, delay, maxWait time.Duration) error {
	if delay > maxWait {
		delay = maxWait
	}
	if delay < 0 {
		delay = 0
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// exceedsDeadline reports whether waiting delay would consume the caller's own
// deadline. Sleeping past it guarantees a context error in place of the real
// reason the request failed.
func exceedsDeadline(ctx context.Context, delay time.Duration) bool {
	deadline, ok := ctx.Deadline()
	if !ok {
		return false
	}
	return delay >= time.Until(deadline)
}
