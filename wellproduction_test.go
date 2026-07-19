package oilpriceapi

// Tests for the well production client (/v1/well-production*). Response
// fixtures were captured from production on 2026-07-13 (issue #17).

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// wpServer spins up a test server that asserts the request path/query and
// returns the given body.
func wpServer(t *testing.T, wantPath string, wantQuery map[string]string, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != wantPath {
			t.Errorf("expected path %s, got %s", wantPath, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Token test-key" {
			t.Errorf("expected auth header, got %q", r.Header.Get("Authorization"))
		}
		for k, v := range wantQuery {
			if got := r.URL.Query().Get(k); got != v {
				t.Errorf("expected query %s=%s, got %q", k, v, got)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
}

func TestWellProductionGetSummary(t *testing.T) {
	t.Run("decodes national summary envelope", func(t *testing.T) {
		// Trimmed production fixture (2026-07-13): national rollup zeroed while
		// backfill runs, top_states populated.
		const fixture = `{"status":"success","data":{"national":{"period":"2026-07","oil_bbl":0,"gas_mcf":0,"water_bbl":0,"boe":0,"days_producing":null,"source":"market_reporting"},"top_states":[{"state":"TX","period":"2026-04","oil_bbl":174743000,"oil_bpd":5824767,"gas_mcf":1164406000,"boe":368810667},{"state":"NM","period":"2026-04","oil_bbl":70985000,"oil_bpd":2366167,"gas_mcf":359485000,"boe":130899167}],"data_sources":{"state_aggregates":"EIA API v2 (monthly)"},"coverage":{"states_with_data":["AK","TX"],"latest_period":"2026-07","total_records":12345}}}`

		server := wpServer(t, "/v1/well-production", nil, http.StatusOK, fixture)
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.WellProduction().GetSummary(context.Background())

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Status != "success" {
			t.Errorf("expected status success, got %q", resp.Status)
		}
		if resp.Data.National == nil {
			t.Fatal("expected national record, got nil")
		}
		if resp.Data.National.Period != "2026-07" {
			t.Errorf("expected national period 2026-07, got %q", resp.Data.National.Period)
		}
		if resp.Data.National.DaysProducing != nil {
			t.Errorf("expected nil days_producing, got %v", *resp.Data.National.DaysProducing)
		}
		if len(resp.Data.TopStates) != 2 {
			t.Fatalf("expected 2 top states, got %d", len(resp.Data.TopStates))
		}
		tx := resp.Data.TopStates[0]
		if tx.State != "TX" || tx.OilBbl != 174743000 || tx.OilBpd != 5824767 {
			t.Errorf("unexpected TX row: %+v", tx)
		}
		if resp.Data.Coverage == nil || resp.Data.Coverage.TotalRecords != 12345 {
			t.Errorf("unexpected coverage: %+v", resp.Data.Coverage)
		}
	})

	t.Run("handles null national (empty successful response)", func(t *testing.T) {
		server := wpServer(t, "/v1/well-production", nil, http.StatusOK,
			`{"status":"success","data":{"national":null,"top_states":[]}}`)
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.WellProduction().GetSummary(context.Background())

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Data.National != nil {
			t.Errorf("expected nil national, got %+v", resp.Data.National)
		}
		if len(resp.Data.TopStates) != 0 {
			t.Errorf("expected 0 top states, got %d", len(resp.Data.TopStates))
		}
	})

	t.Run("surfaces entitlement error (Scale tier gate)", func(t *testing.T) {
		server := wpServer(t, "/v1/well-production", nil, http.StatusForbidden,
			`{"error":{"code":"ENTERPRISE_REQUIRED","message":"Well production data requires the Scale plan. Contact sales@oilpriceapi.com for access.","status":403}}`)
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.WellProduction().GetSummary(context.Background())

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected *APIError, got %T: %v", err, err)
		}
		if apiErr.StatusCode != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", apiErr.StatusCode)
		}
	})
}

func TestWellProductionGetStates(t *testing.T) {
	const fixture = `{"status":"success","data":{"period":"2026-04","count":2,"states":[{"state":"TX","period":"2026-04","oil_bbl":174743000,"oil_bpd":5824767,"gas_mcf":1164406000,"boe":368810667},{"state":"NM","period":"2026-04","oil_bbl":70985000,"oil_bpd":2366167,"gas_mcf":359485000,"boe":130899167}]}}`

	t.Run("decodes state list", func(t *testing.T) {
		server := wpServer(t, "/v1/well-production/states", nil, http.StatusOK, fixture)
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.WellProduction().GetStates(context.Background())

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Data.Period != "2026-04" || resp.Data.Count != 2 || len(resp.Data.States) != 2 {
			t.Errorf("unexpected data: %+v", resp.Data)
		}
	})

	t.Run("sends period param", func(t *testing.T) {
		server := wpServer(t, "/v1/well-production/states", map[string]string{"period": "2026-03"}, http.StatusOK, fixture)
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		_, err := client.WellProduction().GetStates(context.Background(), WithProductionPeriod("2026-03"))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("empty successful response decodes to zero states", func(t *testing.T) {
		server := wpServer(t, "/v1/well-production/states", nil, http.StatusOK,
			`{"status":"success","data":{"period":"2026-04","count":0,"states":[]}}`)
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.WellProduction().GetStates(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Data.Count != 0 || len(resp.Data.States) != 0 {
			t.Errorf("expected empty states, got %+v", resp.Data)
		}
	})
}

func TestWellProductionGetStateDetail(t *testing.T) {
	const fixture = `{"status":"success","data":{"state":"TX","period":{"start":"2024-07-13","end":"2026-07-13"},"count":1,"data":[{"period":"2025-06","oil_bbl":172690000,"gas_mcf":1110000000,"water_bbl":null,"boe":357690000,"days_producing":null,"source":"eia_api"}]}}`

	t.Run("builds path, uppercases state, sends date range", func(t *testing.T) {
		server := wpServer(t, "/v1/well-production/states/TX",
			map[string]string{"start_date": "2025-01-01", "end_date": "2025-12-31"},
			http.StatusOK, fixture)
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.WellProduction().GetStateDetail(context.Background(), "tx",
			WithProductionStartDate("2025-01-01"),
			WithProductionEndDate("2025-12-31"),
		)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Data.State != "TX" || resp.Data.Period.Start != "2024-07-13" {
			t.Errorf("unexpected data: %+v", resp.Data)
		}
		rec := resp.Data.Data[0]
		if rec.WaterBbl != nil {
			t.Errorf("expected nil water_bbl, got %v", *rec.WaterBbl)
		}
		if rec.Source != "eia_api" {
			t.Errorf("expected source eia_api, got %q", rec.Source)
		}
	})

	t.Run("unsupported state returns NotFoundError", func(t *testing.T) {
		server := wpServer(t, "/v1/well-production/states/ZZ", nil, http.StatusNotFound,
			`{"error":{"code":"DATA_NOT_AVAILABLE","message":"No production data for state: ZZ","status":404}}`)
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.WellProduction().GetStateDetail(context.Background(), "ZZ")

		var nfErr *NotFoundError
		if !errors.As(err, &nfErr) {
			t.Fatalf("expected *NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("empty state code fails client-side", func(t *testing.T) {
		client := NewClient("test-key")
		_, err := client.WellProduction().GetStateDetail(context.Background(), " ")
		if err == nil {
			t.Fatal("expected error for empty state code")
		}
	})
}

func TestWellProductionGetWellDetail(t *testing.T) {
	const fixture = `{"status":"success","data":{"api_number":"42329447130000","operator":"FIREBIRD ENERGY II LLC","well_name":"MBE","state":"TX","count":1,"data":[{"period":"2023-11","oil_bbl":0,"gas_mcf":0,"water_bbl":null,"boe":0,"days_producing":null,"source":"tx_rrc"}]}}`

	t.Run("decodes well history", func(t *testing.T) {
		server := wpServer(t, "/v1/well-production/wells/42329447130000", nil, http.StatusOK, fixture)
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.WellProduction().GetWellDetail(context.Background(), "42329447130000")

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		d := resp.Data
		if d.APINumber != "42329447130000" || d.Operator != "FIREBIRD ENERGY II LLC" || d.WellName != "MBE" || d.State != "TX" {
			t.Errorf("unexpected data: %+v", d)
		}
	})

	t.Run("invalid API number returns APIError 400", func(t *testing.T) {
		server := wpServer(t, "/v1/well-production/wells/notanapi", nil, http.StatusBadRequest,
			`{"error":{"code":"INVALID_PARAMETER","message":"API number must be 14 digits.","status":400}}`)
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.WellProduction().GetWellDetail(context.Background(), "notanapi")

		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected *APIError, got %T: %v", err, err)
		}
		if apiErr.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", apiErr.StatusCode)
		}
	})

	t.Run("empty API number fails client-side", func(t *testing.T) {
		client := NewClient("test-key")
		_, err := client.WellProduction().GetWellDetail(context.Background(), "")
		if err == nil {
			t.Fatal("expected error for empty API number")
		}
	})
}

func TestWellProductionGetTopProducers(t *testing.T) {
	const fixture = `{"status":"success","data":{"state":"TX","period":{"start":"2025-07-01","end":"2026-07-13"},"count":1,"producers":[{"api_number":"42329447130000","operator":"FIREBIRD ENERGY II LLC","well_name":"MBE","total_oil_bbl":1004658,"total_gas_mcf":1605878,"months_producing":7}]}}`

	t.Run("decodes producers and sends params", func(t *testing.T) {
		server := wpServer(t, "/v1/well-production/top-producers",
			map[string]string{"state_code": "NM", "limit": "5", "months": "6"},
			http.StatusOK, fixture)
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.WellProduction().GetTopProducers(context.Background(),
			WithProductionState("NM"),
			WithProductionLimit(5),
			WithProductionMonths(6),
		)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		p := resp.Data.Producers[0]
		if p.APINumber != "42329447130000" || p.TotalOilBbl != 1004658 || p.MonthsProducing != 7 {
			t.Errorf("unexpected producer: %+v", p)
		}
	})
}

func TestWellProductionGetCycleTime(t *testing.T) {
	const fixture = `{"status":"success","data":{"filters":{"state":"TX"},"well_count":10000,"wells_with_cycle_data":1900,"cycle_time_stats":{"count":1900,"median_days":400,"p25_days":396,"p75_days":457,"p90_days":516,"min_days":1,"max_days":593,"avg_days":422},"stage_breakdown":{"permit_to_spud":{"count":2,"median":149,"p25":122,"p75":149,"avg":136}},"quarterly_cohorts":{"2024-Q3":{"well_count":406,"median_days":512,"avg_days":488}},"top_fastest":[{"api_number":"42285343290000","operator":"FW EAGLE FORD I, LLC","total_days":46,"permit_date":"2025-07-17","first_production":"2025-09-01"}],"top_slowest":[{"api_number":"42289322100000","operator":"COMSTOCK OIL & GAS, LLC","total_days":132,"permit_date":"2025-05-22","first_production":"2025-10-01"}]}}`

	t.Run("decodes analysis and sends filters", func(t *testing.T) {
		server := wpServer(t, "/v1/well-production/cycle-time",
			map[string]string{
				"state": "TX", "start_date": "2024-01-01", "end_date": "2025-12-31",
				"operator": "ACME", "formation": "EAGLE FORD",
				"lat": "31.9", "lng": "-102.1", "radius_miles": "50",
			},
			http.StatusOK, fixture)
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.WellProduction().GetCycleTime(context.Background(),
			WithProductionState("TX"),
			WithProductionStartDate("2024-01-01"),
			WithProductionEndDate("2025-12-31"),
			WithProductionOperator("ACME"),
			WithProductionFormation("EAGLE FORD"),
			WithProductionLocation(31.9, -102.1, 50),
		)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		d := resp.Data
		if d.WellCount != 10000 || d.WellsWithCycleData != 1900 {
			t.Errorf("unexpected counts: %+v", d)
		}
		if d.CycleTimeStats.MedianDays != 400 || d.CycleTimeStats.P90Days != 516 {
			t.Errorf("unexpected stats: %+v", d.CycleTimeStats)
		}
		if d.StageBreakdown["permit_to_spud"].Median != 149 {
			t.Errorf("unexpected stage breakdown: %+v", d.StageBreakdown)
		}
		if d.QuarterlyCohorts["2024-Q3"].WellCount != 406 {
			t.Errorf("unexpected quarterly cohorts: %+v", d.QuarterlyCohorts)
		}
		if len(d.TopFastest) != 1 || d.TopFastest[0].TotalDays != 46 {
			t.Errorf("unexpected top_fastest: %+v", d.TopFastest)
		}
	})

	t.Run("no matching wells returns NotFoundError", func(t *testing.T) {
		server := wpServer(t, "/v1/well-production/cycle-time", nil, http.StatusNotFound,
			`{"error":{"code":"DATA_NOT_AVAILABLE","message":"No wells match the requested filters","status":404}}`)
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.WellProduction().GetCycleTime(context.Background())

		var nfErr *NotFoundError
		if !errors.As(err, &nfErr) {
			t.Fatalf("expected *NotFoundError, got %T: %v", err, err)
		}
	})
}

func TestWellProductionGetCycleTimeCohorts(t *testing.T) {
	const fixture = `{"status":"success","data":{"group_by":"quarter","cohorts":{"2024-Q3":{"well_count":406,"wells_with_data":406,"stats":{"count":406,"median_days":512,"p25_days":485,"p75_days":525,"p90_days":549,"min_days":1,"max_days":593,"avg_days":488}}}}}`

	t.Run("decodes cohorts and sends group_by", func(t *testing.T) {
		server := wpServer(t, "/v1/well-production/cycle-time/cohorts",
			map[string]string{"group_by": "quarter", "state": "TX"},
			http.StatusOK, fixture)
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.WellProduction().GetCycleTimeCohorts(context.Background(),
			WithProductionGroupBy("quarter"),
			WithProductionState("TX"),
		)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Data.GroupBy != "quarter" {
			t.Errorf("expected group_by quarter, got %q", resp.Data.GroupBy)
		}
		cohort, ok := resp.Data.Cohorts["2024-Q3"]
		if !ok {
			t.Fatalf("expected 2024-Q3 cohort, got %+v", resp.Data.Cohorts)
		}
		if cohort.WellsWithData != 406 || cohort.Stats.MedianDays != 512 {
			t.Errorf("unexpected cohort: %+v", cohort)
		}
	})
}
