package oilpriceapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// EIClient is an accessor for the Energy Intelligence (/v1/ei/*) endpoints.
//
// It is obtained from a Client via Client.EI and shares the same HTTP client,
// authentication, and retry behaviour:
//
//	inv, err := client.EI().GetOilInventories(ctx)
type EIClient struct {
	client *Client
}

// EI returns the Energy Intelligence accessor for this client.
func (c *Client) EI() *EIClient {
	return &EIClient{client: c}
}

// GetOilInventories fetches the latest EIA WPSR oil-inventory report.
//
// Endpoint: GET /v1/ei/oil_inventories/latest. Dataset access varies by plan
// and account entitlement.
//
// The Data field holds the raw report payload (its shape mirrors the API's
// published report JSON); decode it into your own struct if you need typed
// access:
//
//	inv, err := client.EI().GetOilInventories(ctx)
//	var report MyReport
//	json.Unmarshal(inv.Data, &report)
func (e *EIClient) GetOilInventories(ctx context.Context, opts ...EIOption) (*EIResponse, error) {
	return e.get(ctx, "/v1/ei/oil_inventories/latest", opts)
}

// GetOpecProduction fetches the latest OPEC MOMR production report.
//
// Endpoint: GET /v1/ei/opec_productions/latest. Dataset access varies by plan
// and account entitlement.
func (e *EIClient) GetOpecProduction(ctx context.Context, opts ...EIOption) (*EIResponse, error) {
	return e.get(ctx, "/v1/ei/opec_productions/latest", opts)
}

// GetWellPermits fetches the most recent drilling permits across all states.
//
// Endpoint: GET /v1/ei/well-permits/latest. Dataset access varies by plan and
// account entitlement. Use WithEIDays to widen the trailing window (1-90,
// default 7) and WithEIStates to filter by state code(s).
func (e *EIClient) GetWellPermits(ctx context.Context, opts ...EIOption) (*EIResponse, error) {
	return e.get(ctx, "/v1/ei/well-permits/latest", opts)
}

// SearchWellPermits searches permits and returns the typed production
// well_permits envelope. Dataset access varies by plan and account entitlement.
// A permit result does not imply that monthly well production is available;
// check WellProduction().GetSummary().Data.Coverage first.
func (e *EIClient) SearchWellPermits(ctx context.Context, query WellPermitSearchQuery) (*WellPermitsResponse, error) {
	if (query.Latitude == nil) != (query.Longitude == nil) {
		return nil, fmt.Errorf("oilpriceapi: latitude and longitude must be provided together")
	}
	if query.RadiusMiles != 0 && (query.RadiusMiles < 1 || query.RadiusMiles > 100) {
		return nil, fmt.Errorf("oilpriceapi: radius miles must be between 1 and 100")
	}

	params := url.Values{}
	setWellPermitSearchParams(params, query)
	endpoint := "/v1/ei/well-permits/search"
	if encoded := params.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	resp, err := e.client.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, e.client.handleError(resp)
	}

	var result WellPermitsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func setWellPermitSearchParams(params url.Values, query WellPermitSearchQuery) {
	values := map[string]string{
		"states": query.States, "county": query.County, "well_name": query.WellName,
		"permit_type": query.PermitType, "well_type": query.WellType, "address": query.Address,
		"start_date": query.StartDate, "end_date": query.EndDate,
	}
	for key, value := range values {
		if value != "" {
			params.Set(key, value)
		}
	}
	if query.Latitude != nil {
		params.Set("latitude", strconv.FormatFloat(*query.Latitude, 'f', -1, 64))
		params.Set("longitude", strconv.FormatFloat(*query.Longitude, 'f', -1, 64))
	}
	if query.RadiusMiles != 0 {
		params.Set("radius_miles", strconv.Itoa(query.RadiusMiles))
	}
}

// get is the shared GET-and-decode implementation for EI endpoints.
func (e *EIClient) get(ctx context.Context, path string, opts []EIOption) (*EIResponse, error) {
	options := &EIOptions{}
	for _, opt := range opts {
		opt(options)
	}

	query := url.Values{}
	if options.Days > 0 {
		query.Set("days", strconv.Itoa(options.Days))
	}
	if options.Weeks > 0 {
		query.Set("weeks", strconv.Itoa(options.Weeks))
	}
	if options.Months > 0 {
		query.Set("months", strconv.Itoa(options.Months))
	}
	if options.States != "" {
		query.Set("states", options.States)
	}

	endpoint := path
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	resp, err := e.client.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, e.client.handleError(resp)
	}

	var result EIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
