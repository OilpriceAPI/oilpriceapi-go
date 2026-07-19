package oilpriceapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// WellProductionClient is an accessor for the well production
// (/v1/well-production*) endpoints.
//
// It is obtained from a Client via Client.WellProduction and shares the same
// HTTP client, authentication, and retry behaviour:
//
//	summary, err := client.WellProduction().GetSummary(ctx)
//
// Well production data requires the drilling_intelligence entitlement.
// Coverage note: state-level aggregates come from the EIA API
// (monthly); well-level data comes from state regulatory agencies and is beta
// — it is NOT complete US well-level production.
type WellProductionClient struct {
	client *Client
}

// WellProduction returns the well production accessor for this client.
func (c *Client) WellProduction() *WellProductionClient {
	return &WellProductionClient{client: c}
}

// ===================
// Response types
// ===================

// WellProductionRecord is a single monthly production record for a state or
// an individual well. WaterBbl and DaysProducing are nullable server-side and
// are therefore pointers.
type WellProductionRecord struct {
	Period        string   `json:"period"`
	OilBbl        float64  `json:"oil_bbl"`
	GasMcf        float64  `json:"gas_mcf"`
	WaterBbl      *float64 `json:"water_bbl"`
	Boe           float64  `json:"boe"`
	DaysProducing *int     `json:"days_producing"`
	Source        string   `json:"source,omitempty"`
}

// WellProductionStateSummary is a state-level production summary row.
type WellProductionStateSummary struct {
	State  string  `json:"state"`
	Period string  `json:"period"`
	OilBbl float64 `json:"oil_bbl"`
	OilBpd float64 `json:"oil_bpd"`
	GasMcf float64 `json:"gas_mcf"`
	Boe    float64 `json:"boe"`
}

// WellProductionCoverage describes which states and periods have data.
type WellProductionCoverage struct {
	StatesWithData          []string `json:"states_with_data,omitempty"`
	WellLevelStatesWithData []string `json:"well_level_states_with_data,omitempty"`
	LatestPeriod            string   `json:"latest_period,omitempty"`
	TotalRecords            int      `json:"total_records,omitempty"`
}

// WellProductionSummaryData is the data payload of the national summary
// endpoint (GET /v1/well-production).
type WellProductionSummaryData struct {
	National    *WellProductionRecord        `json:"national"`
	TopStates   []WellProductionStateSummary `json:"top_states"`
	DataSources map[string]interface{}       `json:"data_sources,omitempty"`
	Coverage    *WellProductionCoverage      `json:"coverage,omitempty"`
}

// WellProductionSummaryResponse is the envelope returned by
// GET /v1/well-production.
type WellProductionSummaryResponse struct {
	Status string                    `json:"status"`
	Data   WellProductionSummaryData `json:"data"`
}

// WellProductionStatesData is the data payload of the state list endpoint.
type WellProductionStatesData struct {
	Period string                       `json:"period"`
	Count  int                          `json:"count"`
	States []WellProductionStateSummary `json:"states"`
}

// WellProductionStatesResponse is the envelope returned by
// GET /v1/well-production/states.
type WellProductionStatesResponse struct {
	Status string                   `json:"status"`
	Data   WellProductionStatesData `json:"data"`
}

// WellProductionDateRange is a start/end date pair (YYYY-MM-DD).
type WellProductionDateRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// WellProductionStateDetailData is the data payload of the state history
// endpoint.
type WellProductionStateDetailData struct {
	State  string                  `json:"state"`
	Period WellProductionDateRange `json:"period"`
	Count  int                     `json:"count"`
	Data   []WellProductionRecord  `json:"data"`
}

// WellProductionStateDetailResponse is the envelope returned by
// GET /v1/well-production/states/{code}.
type WellProductionStateDetailResponse struct {
	Status string                        `json:"status"`
	Data   WellProductionStateDetailData `json:"data"`
}

// WellProductionWellDetailData is the data payload of the single-well history
// endpoint.
type WellProductionWellDetailData struct {
	APINumber string                 `json:"api_number"`
	Operator  string                 `json:"operator"`
	WellName  string                 `json:"well_name"`
	State     string                 `json:"state"`
	Count     int                    `json:"count"`
	Data      []WellProductionRecord `json:"data"`
}

// WellProductionWellDetailResponse is the envelope returned by
// GET /v1/well-production/wells/{api_number}.
type WellProductionWellDetailResponse struct {
	Status string                       `json:"status"`
	Data   WellProductionWellDetailData `json:"data"`
}

// WellProducer is a single row of the top-producers ranking.
type WellProducer struct {
	APINumber       string  `json:"api_number"`
	Operator        string  `json:"operator"`
	WellName        string  `json:"well_name"`
	TotalOilBbl     float64 `json:"total_oil_bbl"`
	TotalGasMcf     float64 `json:"total_gas_mcf"`
	MonthsProducing int     `json:"months_producing"`
}

// WellProductionTopProducersData is the data payload of the top-producers
// endpoint.
type WellProductionTopProducersData struct {
	State     string                  `json:"state"`
	Period    WellProductionDateRange `json:"period"`
	Count     int                     `json:"count"`
	Producers []WellProducer          `json:"producers"`
}

// WellProductionTopProducersResponse is the envelope returned by
// GET /v1/well-production/top-producers.
type WellProductionTopProducersResponse struct {
	Status string                         `json:"status"`
	Data   WellProductionTopProducersData `json:"data"`
}

// WellCycleTimeStats is the percentile summary used by the cycle-time
// endpoints (days from permit to first production).
type WellCycleTimeStats struct {
	Count      int     `json:"count"`
	MedianDays float64 `json:"median_days"`
	P25Days    float64 `json:"p25_days"`
	P75Days    float64 `json:"p75_days"`
	P90Days    float64 `json:"p90_days"`
	MinDays    float64 `json:"min_days"`
	MaxDays    float64 `json:"max_days"`
	AvgDays    float64 `json:"avg_days"`
}

// WellCycleStageStats is the per-stage breakdown (permit_to_spud,
// spud_to_completion, completion_to_production).
type WellCycleStageStats struct {
	Count  int     `json:"count"`
	Median float64 `json:"median"`
	P25    float64 `json:"p25"`
	P75    float64 `json:"p75"`
	Avg    float64 `json:"avg"`
}

// WellCycleQuarterSummary is a condensed quarterly cohort row embedded in the
// cycle-time analysis response.
type WellCycleQuarterSummary struct {
	WellCount  int     `json:"well_count"`
	MedianDays float64 `json:"median_days"`
	AvgDays    float64 `json:"avg_days"`
}

// WellCycleExtreme is one of the fastest/slowest wells listed in the
// cycle-time analysis.
type WellCycleExtreme struct {
	APINumber       string `json:"api_number"`
	Operator        string `json:"operator"`
	TotalDays       int    `json:"total_days"`
	PermitDate      string `json:"permit_date"`
	FirstProduction string `json:"first_production"`
}

// WellCycleTimeData is the data payload of the cycle-time analysis endpoint.
// Filters echoes the applied query filters and is preserved raw because its
// keys vary with the request.
type WellCycleTimeData struct {
	Filters            json.RawMessage                    `json:"filters,omitempty"`
	WellCount          int                                `json:"well_count"`
	WellsWithCycleData int                                `json:"wells_with_cycle_data"`
	CycleTimeStats     WellCycleTimeStats                 `json:"cycle_time_stats"`
	StageBreakdown     map[string]WellCycleStageStats     `json:"stage_breakdown,omitempty"`
	QuarterlyCohorts   map[string]WellCycleQuarterSummary `json:"quarterly_cohorts,omitempty"`
	TopFastest         []WellCycleExtreme                 `json:"top_fastest,omitempty"`
	TopSlowest         []WellCycleExtreme                 `json:"top_slowest,omitempty"`
}

// WellCycleTimeResponse is the envelope returned by
// GET /v1/well-production/cycle-time.
type WellCycleTimeResponse struct {
	Status string            `json:"status"`
	Data   WellCycleTimeData `json:"data"`
}

// WellCycleCohort is a single cohort row of the cohort-comparison endpoint.
type WellCycleCohort struct {
	WellCount     int                `json:"well_count"`
	WellsWithData int                `json:"wells_with_data"`
	Stats         WellCycleTimeStats `json:"stats"`
}

// WellCycleCohortsData is the data payload of the cohort-comparison endpoint.
type WellCycleCohortsData struct {
	GroupBy string                     `json:"group_by"`
	Cohorts map[string]WellCycleCohort `json:"cohorts"`
}

// WellCycleCohortsResponse is the envelope returned by
// GET /v1/well-production/cycle-time/cohorts.
type WellCycleCohortsResponse struct {
	Status string               `json:"status"`
	Data   WellCycleCohortsData `json:"data"`
}

// ===================
// Options
// ===================

// WellProductionOptions contains the query options shared across well
// production endpoints. Not every option applies to every endpoint; see the
// individual With* helpers for which endpoints honour them.
type WellProductionOptions struct {
	Period      string
	StartDate   string
	EndDate     string
	State       string
	Limit       int
	Months      int
	Operator    string
	Formation   string
	Lat         float64
	Lng         float64
	RadiusMiles float64
	hasLocation bool
	GroupBy     string
}

// WellProductionOption is a functional option for well production methods.
type WellProductionOption func(*WellProductionOptions)

// WithProductionPeriod selects a specific month (YYYY-MM). Used by GetStates;
// defaults to the latest available month.
func WithProductionPeriod(period string) WellProductionOption {
	return func(o *WellProductionOptions) {
		o.Period = period
	}
}

// WithProductionStartDate sets the range start (YYYY-MM-DD). Used by
// GetStateDetail, GetCycleTime, and GetCycleTimeCohorts.
func WithProductionStartDate(date string) WellProductionOption {
	return func(o *WellProductionOptions) {
		o.StartDate = date
	}
}

// WithProductionEndDate sets the range end (YYYY-MM-DD). Used by
// GetStateDetail, GetCycleTime, and GetCycleTimeCohorts.
func WithProductionEndDate(date string) WellProductionOption {
	return func(o *WellProductionOptions) {
		o.EndDate = date
	}
}

// WithProductionState filters by state code (e.g. "TX"). Used by
// GetTopProducers (as state_code, default "TX"), GetCycleTime, and
// GetCycleTimeCohorts (as state).
func WithProductionState(state string) WellProductionOption {
	return func(o *WellProductionOptions) {
		o.State = state
	}
}

// WithProductionLimit caps the number of rows returned by GetTopProducers
// (server clamps to 1-100, default 20).
func WithProductionLimit(limit int) WellProductionOption {
	return func(o *WellProductionOptions) {
		o.Limit = limit
	}
}

// WithProductionMonths sets the trailing window in months for GetTopProducers
// (default 12).
func WithProductionMonths(months int) WellProductionOption {
	return func(o *WellProductionOptions) {
		o.Months = months
	}
}

// WithProductionOperator filters GetCycleTime by operator name.
func WithProductionOperator(operator string) WellProductionOption {
	return func(o *WellProductionOptions) {
		o.Operator = operator
	}
}

// WithProductionFormation filters GetCycleTime by formation.
func WithProductionFormation(formation string) WellProductionOption {
	return func(o *WellProductionOptions) {
		o.Formation = formation
	}
}

// WithProductionLocation filters GetCycleTime and GetCycleTimeCohorts to a
// geographic radius (miles) around a lat/lng point.
func WithProductionLocation(lat, lng, radiusMiles float64) WellProductionOption {
	return func(o *WellProductionOptions) {
		o.Lat = lat
		o.Lng = lng
		o.RadiusMiles = radiusMiles
		o.hasLocation = true
	}
}

// WithProductionGroupBy sets the cohort grouping for GetCycleTimeCohorts
// (default "quarter").
func WithProductionGroupBy(groupBy string) WellProductionOption {
	return func(o *WellProductionOptions) {
		o.GroupBy = groupBy
	}
}

func applyWellProductionOptions(opts []WellProductionOption) *WellProductionOptions {
	options := &WellProductionOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

// formatFloat renders a float query param without scientific notation.
func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// ===================
// Methods
// ===================

// GetSummary fetches the national production overview.
//
// Endpoint: GET /v1/well-production. Data.National may be nil when no
// national rollup exists for the current period, and its fields can be zero
// while backfills are running — check Data.TopStates for state-level numbers.
func (w *WellProductionClient) GetSummary(ctx context.Context) (*WellProductionSummaryResponse, error) {
	var result WellProductionSummaryResponse
	if err := w.get(ctx, "/v1/well-production", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetStates fetches the latest state-level production summary, ordered by oil
// volume. Use WithProductionPeriod to select a specific month (YYYY-MM).
//
// Endpoint: GET /v1/well-production/states.
func (w *WellProductionClient) GetStates(ctx context.Context, opts ...WellProductionOption) (*WellProductionStatesResponse, error) {
	options := applyWellProductionOptions(opts)

	query := url.Values{}
	if options.Period != "" {
		query.Set("period", options.Period)
	}

	var result WellProductionStatesResponse
	if err := w.get(ctx, "/v1/well-production/states", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetStateDetail fetches monthly production history for a state (default
// window: the last two years). Use WithProductionStartDate and
// WithProductionEndDate to change the range.
//
// Endpoint: GET /v1/well-production/states/{code}. Returns *NotFoundError
// when the state has no production data.
func (w *WellProductionClient) GetStateDetail(ctx context.Context, stateCode string, opts ...WellProductionOption) (*WellProductionStateDetailResponse, error) {
	if strings.TrimSpace(stateCode) == "" {
		return nil, fmt.Errorf("oilpriceapi: state code is required")
	}
	options := applyWellProductionOptions(opts)

	query := url.Values{}
	if options.StartDate != "" {
		query.Set("start_date", options.StartDate)
	}
	if options.EndDate != "" {
		query.Set("end_date", options.EndDate)
	}

	var result WellProductionStateDetailResponse
	if err := w.get(ctx, "/v1/well-production/states/"+url.PathEscape(strings.ToUpper(strings.TrimSpace(stateCode))), query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetWellDetail fetches monthly production history for a single well by its
// 14-digit API number.
//
// Endpoint: GET /v1/well-production/wells/{api_number}. The server rejects
// non-14-digit API numbers with an INVALID_PARAMETER error and returns
// *NotFoundError when the well has no production data.
func (w *WellProductionClient) GetWellDetail(ctx context.Context, apiNumber string) (*WellProductionWellDetailResponse, error) {
	if strings.TrimSpace(apiNumber) == "" {
		return nil, fmt.Errorf("oilpriceapi: well API number is required")
	}

	var result WellProductionWellDetailResponse
	if err := w.get(ctx, "/v1/well-production/wells/"+url.PathEscape(strings.TrimSpace(apiNumber)), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTopProducers fetches the top producing wells for a state (default TX,
// last 12 months, 20 rows). Use WithProductionState, WithProductionMonths,
// and WithProductionLimit to adjust.
//
// Endpoint: GET /v1/well-production/top-producers.
func (w *WellProductionClient) GetTopProducers(ctx context.Context, opts ...WellProductionOption) (*WellProductionTopProducersResponse, error) {
	options := applyWellProductionOptions(opts)

	query := url.Values{}
	if options.State != "" {
		query.Set("state_code", options.State)
	}
	if options.Limit > 0 {
		query.Set("limit", strconv.Itoa(options.Limit))
	}
	if options.Months > 0 {
		query.Set("months", strconv.Itoa(options.Months))
	}

	var result WellProductionTopProducersResponse
	if err := w.get(ctx, "/v1/well-production/top-producers", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetCycleTime fetches permit-to-production cycle-time analysis. Filter with
// WithProductionState, WithProductionStartDate/EndDate, WithProductionOperator,
// WithProductionFormation, and WithProductionLocation.
//
// Endpoint: GET /v1/well-production/cycle-time. Returns *NotFoundError when
// no wells match the filters.
func (w *WellProductionClient) GetCycleTime(ctx context.Context, opts ...WellProductionOption) (*WellCycleTimeResponse, error) {
	options := applyWellProductionOptions(opts)

	var result WellCycleTimeResponse
	if err := w.get(ctx, "/v1/well-production/cycle-time", cycleTimeQuery(options), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetCycleTimeCohorts compares cycle times across cohorts (default grouping:
// quarter). Accepts the same geographic and date filters as GetCycleTime plus
// WithProductionGroupBy.
//
// Endpoint: GET /v1/well-production/cycle-time/cohorts.
func (w *WellProductionClient) GetCycleTimeCohorts(ctx context.Context, opts ...WellProductionOption) (*WellCycleCohortsResponse, error) {
	options := applyWellProductionOptions(opts)

	query := cycleTimeQuery(options)
	if options.GroupBy != "" {
		query.Set("group_by", options.GroupBy)
	}

	var result WellCycleCohortsResponse
	if err := w.get(ctx, "/v1/well-production/cycle-time/cohorts", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// cycleTimeQuery builds the filter params shared by the cycle-time endpoints.
func cycleTimeQuery(options *WellProductionOptions) url.Values {
	query := url.Values{}
	if options.State != "" {
		query.Set("state", options.State)
	}
	if options.StartDate != "" {
		query.Set("start_date", options.StartDate)
	}
	if options.EndDate != "" {
		query.Set("end_date", options.EndDate)
	}
	if options.Operator != "" {
		query.Set("operator", options.Operator)
	}
	if options.Formation != "" {
		query.Set("formation", options.Formation)
	}
	if options.hasLocation {
		query.Set("lat", formatFloat(options.Lat))
		query.Set("lng", formatFloat(options.Lng))
		query.Set("radius_miles", formatFloat(options.RadiusMiles))
	}
	return query
}

// get is the shared GET-and-decode implementation for well production
// endpoints.
func (w *WellProductionClient) get(ctx context.Context, path string, query url.Values, out interface{}) error {
	endpoint := path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	resp, err := w.client.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return w.client.handleError(resp)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
