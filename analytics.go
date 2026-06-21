package oilpriceapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// GetAnalyticsPerformance fetches the authenticated user's API usage analytics
// for the dashboard (request volume, error rate, endpoint and geographic
// breakdowns).
//
// Use WithAnalyticsRange to set the window ("7d", "30d", or "90d"; default
// "30d").
//
// Example:
//
//	perf, err := client.GetAnalyticsPerformance(ctx)
//	fmt.Printf("Total requests: %d (error rate %.2f%%)\n",
//	    perf.Overview.TotalRequests, perf.Overview.ErrorRate)
func (c *Client) GetAnalyticsPerformance(ctx context.Context, opts ...AnalyticsOption) (*AnalyticsPerformanceResponse, error) {
	options := applyAnalyticsOptions(opts)

	query := url.Values{}
	if options.Range != "" {
		query.Set("range", options.Range)
	}

	var result AnalyticsPerformanceResponse
	if err := c.getAnalytics(ctx, "/v1/analytics/performance", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetAnalyticsCorrelation fetches correlation analysis between two commodities.
//
// Code1 and Code2 are required (the API returns 400 otherwise). Use
// WithAnalyticsCode1/WithAnalyticsCode2 to set them and WithAnalyticsPeriod to
// set the lookback window in days (default 30).
//
// Example:
//
//	corr, err := client.GetAnalyticsCorrelation(ctx,
//	    oilpriceapi.WithAnalyticsCode1("WTI_USD"),
//	    oilpriceapi.WithAnalyticsCode2("BRENT_CRUDE_USD"),
//	    oilpriceapi.WithAnalyticsPeriod(90),
//	)
func (c *Client) GetAnalyticsCorrelation(ctx context.Context, opts ...AnalyticsOption) (*AnalyticsResult, error) {
	options := applyAnalyticsOptions(opts)

	query := url.Values{}
	if options.Code1 != "" {
		query.Set("code1", options.Code1)
	}
	if options.Code2 != "" {
		query.Set("code2", options.Code2)
	}
	if options.Period > 0 {
		query.Set("period", strconv.Itoa(options.Period))
	}
	if options.Type != "" {
		query.Set("type", options.Type)
	}

	var result AnalyticsResult
	if err := c.getAnalytics(ctx, "/v1/analytics/correlation", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetAnalyticsTrend fetches trend and momentum analysis for a commodity.
//
// Use WithAnalyticsPeriod to set the lookback window in days (default 30) and
// WithAnalyticsType to select a sub-analysis ("sma", "ema", "rsi", "levels", or
// the default full "analysis").
//
// Example:
//
//	trend, err := client.GetAnalyticsTrend(ctx, "WTI_USD",
//	    oilpriceapi.WithAnalyticsPeriod(60))
func (c *Client) GetAnalyticsTrend(ctx context.Context, commodity string, opts ...AnalyticsOption) (*AnalyticsResult, error) {
	options := applyAnalyticsOptions(opts)

	query := url.Values{}
	query.Set("code", commodity)
	if options.Period > 0 {
		query.Set("period", strconv.Itoa(options.Period))
	}
	if options.Type != "" {
		query.Set("type", options.Type)
	}

	var result AnalyticsResult
	if err := c.getAnalytics(ctx, "/v1/analytics/trend", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetAnalyticsForecast fetches a statistical price forecast for a commodity.
//
// Use WithAnalyticsMethod to choose the forecast method (default "ema") and
// WithAnalyticsPeriod to set the lookback window in days (default 90). This is a
// Reservoir Mastery feature.
//
// Example:
//
//	fc, err := client.GetAnalyticsForecast(ctx, "WTI_USD",
//	    oilpriceapi.WithAnalyticsMethod("ema"))
func (c *Client) GetAnalyticsForecast(ctx context.Context, commodity string, opts ...AnalyticsOption) (*AnalyticsResult, error) {
	options := applyAnalyticsOptions(opts)

	query := url.Values{}
	query.Set("code", commodity)
	if options.Method != "" {
		query.Set("method", options.Method)
	}
	if options.Period > 0 {
		query.Set("period", strconv.Itoa(options.Period))
	}

	var result AnalyticsResult
	if err := c.getAnalytics(ctx, "/v1/analytics/forecast", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// getAnalytics is the shared GET-and-decode implementation for analytics
// endpoints.
func (c *Client) getAnalytics(ctx context.Context, path string, query url.Values, out interface{}) error {
	endpoint := path
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.handleError(resp)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
