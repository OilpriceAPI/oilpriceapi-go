package oilpriceapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ===================
// PRICE ALERTS
// ===================

func TestGetAlerts(t *testing.T) {
	t.Run("lists alerts from a bare array", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/alerts" {
				t.Errorf("expected path /v1/alerts, got %s", r.URL.Path)
			}
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			if r.Header.Get("Authorization") != "Token test-key" {
				t.Errorf("expected auth header, got %q", r.Header.Get("Authorization"))
			}
			json.NewEncoder(w).Encode([]Alert{
				{ID: 1, Name: "Brent High", CommodityCode: "BRENT_CRUDE_USD", ConditionOperator: "greater_than", ConditionValue: 85, Enabled: true},
				{ID: 2, Name: "WTI Low", CommodityCode: "WTI_USD", ConditionOperator: "less_than", ConditionValue: 60, Enabled: false},
			})
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		alerts, err := client.GetAlerts(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(alerts) != 2 {
			t.Fatalf("expected 2 alerts, got %d", len(alerts))
		}
		if alerts[0].Name != "Brent High" {
			t.Errorf("expected Brent High, got %s", alerts[0].Name)
		}
	})

	t.Run("lists alerts from an enveloped array", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"alerts":[{"id":7,"name":"Env","commodity_code":"WTI_USD"}]}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		alerts, err := client.GetAlerts(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(alerts) != 1 || alerts[0].ID != 7 {
			t.Errorf("expected 1 alert with id 7, got %+v", alerts)
		}
	})

	t.Run("returns error on non-200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		client := NewClient("bad-key", WithBaseURL(server.URL), WithRetries(0))
		if _, err := client.GetAlerts(context.Background()); !isAuthError(err) {
			t.Errorf("expected AuthenticationError, got %T: %v", err, err)
		}
	})
}

func TestCreateAlert(t *testing.T) {
	t.Run("wraps input in price_alert and POSTs", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			var body alertRequestBody
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.PriceAlert.Name != "Brent $85" {
				t.Errorf("expected name Brent $85, got %s", body.PriceAlert.Name)
			}
			if body.PriceAlert.ConditionValue != 85 {
				t.Errorf("expected condition_value 85, got %v", body.PriceAlert.ConditionValue)
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(Alert{ID: 99, Name: body.PriceAlert.Name, CommodityCode: "BRENT_CRUDE_USD"})
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		enabled := true
		alert, err := client.CreateAlert(context.Background(), AlertInput{
			Name:              "Brent $85",
			CommodityCode:     "BRENT_CRUDE_USD",
			ConditionOperator: "greater_than",
			ConditionValue:    85,
			Enabled:           &enabled,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if alert.ID != 99 {
			t.Errorf("expected ID 99, got %d", alert.ID)
		}
	})

	t.Run("returns error on unprocessable entity", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(`{"errors":{"name":["is required"]}}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(0))
		if _, err := client.CreateAlert(context.Background(), AlertInput{}); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestGetAlert(t *testing.T) {
	t.Run("fetches a single alert", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/alerts/42" {
				t.Errorf("expected /v1/alerts/42, got %s", r.URL.Path)
			}
			json.NewEncoder(w).Encode(Alert{ID: 42, Name: "Single", CommodityCode: "WTI_USD"})
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		alert, err := client.GetAlert(context.Background(), "42")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if alert.ID != 42 {
			t.Errorf("expected ID 42, got %d", alert.ID)
		}
	})

	t.Run("returns NotFoundError on 404", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(0))
		if _, err := client.GetAlert(context.Background(), "999"); !isNotFoundError(err) {
			t.Errorf("expected NotFoundError, got %T: %v", err, err)
		}
	})
}

func TestUpdateAlert(t *testing.T) {
	t.Run("PATCHes a price_alert envelope", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPatch {
				t.Errorf("expected PATCH, got %s", r.Method)
			}
			if r.URL.Path != "/v1/alerts/5" {
				t.Errorf("expected /v1/alerts/5, got %s", r.URL.Path)
			}
			var body alertRequestBody
			json.NewDecoder(r.Body).Decode(&body)
			if body.PriceAlert.ConditionValue != 90 {
				t.Errorf("expected condition_value 90, got %v", body.PriceAlert.ConditionValue)
			}
			json.NewEncoder(w).Encode(Alert{ID: 5, ConditionValue: 90})
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		alert, err := client.UpdateAlert(context.Background(), "5", AlertInput{ConditionValue: 90})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if alert.ConditionValue != 90 {
			t.Errorf("expected 90, got %v", alert.ConditionValue)
		}
	})
}

func TestDeleteAlert(t *testing.T) {
	t.Run("DELETEs and accepts 204", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("expected DELETE, got %s", r.Method)
			}
			if r.URL.Path != "/v1/alerts/3" {
				t.Errorf("expected /v1/alerts/3, got %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		if err := client.DeleteAlert(context.Background(), "3"); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("returns error on 404", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(0))
		if err := client.DeleteAlert(context.Background(), "x"); !isNotFoundError(err) {
			t.Errorf("expected NotFoundError, got %T: %v", err, err)
		}
	})
}

func TestGetAlertTriggers(t *testing.T) {
	t.Run("returns the deprecated message on 410", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/alerts/triggers" {
				t.Errorf("expected /v1/alerts/triggers, got %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusGone)
			w.Write([]byte(`{"message":"Trigger history endpoint deprecated."}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.GetAlertTriggers(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(resp.Message, "deprecated") {
			t.Errorf("expected deprecation message, got %q", resp.Message)
		}
	})
}

// ===================
// WEBHOOKS (extended CRUD)
// ===================

func TestGetWebhook(t *testing.T) {
	t.Run("fetches webhook detail", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/webhooks/12" {
				t.Errorf("expected /v1/webhooks/12, got %s", r.URL.Path)
			}
			w.Write([]byte(`{"id":12,"url":"https://example.com/hook","status":"active","events":["price.updated"],"secret":"whsec_x","event_stats":{"total":3,"delivered":2,"failed":1,"pending":0},"available_events":["price.updated"]}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		wh, err := client.GetWebhook(context.Background(), "12")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if wh.ID != 12 || wh.Secret != "whsec_x" {
			t.Errorf("unexpected webhook: %+v", wh)
		}
		if wh.EventStats.Delivered != 2 {
			t.Errorf("expected 2 delivered, got %d", wh.EventStats.Delivered)
		}
	})

	t.Run("returns NotFoundError on 404", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(0))
		if _, err := client.GetWebhook(context.Background(), "0"); !isNotFoundError(err) {
			t.Errorf("expected NotFoundError, got %T: %v", err, err)
		}
	})
}

func TestUpdateWebhook(t *testing.T) {
	t.Run("PATCHes webhook fields", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPatch {
				t.Errorf("expected PATCH, got %s", r.Method)
			}
			var body WebhookUpdateInput
			json.NewDecoder(r.Body).Decode(&body)
			if body.Status != "paused" {
				t.Errorf("expected status paused, got %s", body.Status)
			}
			w.Write([]byte(`{"id":12,"url":"https://example.com/hook","status":"paused","events":[],"event_stats":{}}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		wh, err := client.UpdateWebhook(context.Background(), "12", WebhookUpdateInput{Status: "paused"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if wh.Status != "paused" {
			t.Errorf("expected paused, got %s", wh.Status)
		}
	})
}

func TestTestWebhook(t *testing.T) {
	t.Run("POSTs to /test and returns result", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/v1/webhooks/12/test" {
				t.Errorf("expected /v1/webhooks/12/test, got %s", r.URL.Path)
			}
			w.Write([]byte(`{"message":"Test webhook sent successfully","response_code":200,"response_body":"ok"}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		res, err := client.TestWebhook(context.Background(), "12")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if res.ResponseCode != 200 {
			t.Errorf("expected 200, got %d", res.ResponseCode)
		}
	})
}

func TestGetWebhookEvents(t *testing.T) {
	t.Run("paginates events", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/webhooks/12/events" {
				t.Errorf("expected /v1/webhooks/12/events, got %s", r.URL.Path)
			}
			if r.URL.Query().Get("page") != "2" {
				t.Errorf("expected page=2, got %s", r.URL.Query().Get("page"))
			}
			if r.URL.Query().Get("per_page") != "10" {
				t.Errorf("expected per_page=10, got %s", r.URL.Query().Get("per_page"))
			}
			w.Write([]byte(`{"events":[{"id":1,"event_type":"price.updated","status":"delivered","attempts":1,"response_code":200}],"pagination":{"page":2,"per_page":10,"total":15,"total_pages":2}}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.GetWebhookEvents(context.Background(), "12", WithWebhookPage(2), WithWebhookPerPage(10))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(resp.Events) != 1 || resp.Events[0].EventType != "price.updated" {
			t.Errorf("unexpected events: %+v", resp.Events)
		}
		if resp.Pagination.Total != 15 {
			t.Errorf("expected total 15, got %d", resp.Pagination.Total)
		}
	})
}

// ===================
// ANALYTICS
// ===================

func TestGetAnalyticsPerformance(t *testing.T) {
	t.Run("sends range and decodes overview", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/analytics/performance" {
				t.Errorf("expected /v1/analytics/performance, got %s", r.URL.Path)
			}
			if r.URL.Query().Get("range") != "7d" {
				t.Errorf("expected range=7d, got %s", r.URL.Query().Get("range"))
			}
			w.Write([]byte(`{"overview":{"totalRequests":120,"errorRate":1.5,"peakHour":14},"dailyUsage":[{"date":"2026-06-20","requests":12,"errors":0}],"endpointBreakdown":[{"endpoint":"/v1/prices/latest","requests":80,"percentage":66.7}]}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		perf, err := client.GetAnalyticsPerformance(context.Background(), WithAnalyticsRange("7d"))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if perf.Overview.TotalRequests != 120 {
			t.Errorf("expected 120, got %d", perf.Overview.TotalRequests)
		}
		if len(perf.EndpointBreakdown) != 1 {
			t.Errorf("expected 1 endpoint, got %d", len(perf.EndpointBreakdown))
		}
	})
}

func TestGetAnalyticsCorrelation(t *testing.T) {
	t.Run("sends code1/code2/period and preserves raw", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("code1") != "WTI_USD" || q.Get("code2") != "BRENT_CRUDE_USD" {
				t.Errorf("unexpected codes: %s %s", q.Get("code1"), q.Get("code2"))
			}
			if q.Get("period") != "90" {
				t.Errorf("expected period=90, got %s", q.Get("period"))
			}
			w.Write([]byte(`{"type":"analysis","correlation":0.94,"strength":"very_strong","tier":"reservoir_mastery"}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		res, err := client.GetAnalyticsCorrelation(context.Background(),
			WithAnalyticsCode1("WTI_USD"), WithAnalyticsCode2("BRENT_CRUDE_USD"), WithAnalyticsPeriod(90))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if res.Type != "analysis" || res.Tier != "reservoir_mastery" {
			t.Errorf("unexpected typed fields: %+v", res)
		}
		var raw map[string]interface{}
		json.Unmarshal(res.Raw, &raw)
		if raw["correlation"].(float64) != 0.94 {
			t.Errorf("expected correlation 0.94 in raw, got %v", raw["correlation"])
		}
	})
}

func TestGetAnalyticsTrend(t *testing.T) {
	t.Run("sends code and type", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/analytics/trend" {
				t.Errorf("expected /v1/analytics/trend, got %s", r.URL.Path)
			}
			if r.URL.Query().Get("code") != "WTI_USD" {
				t.Errorf("expected code=WTI_USD, got %s", r.URL.Query().Get("code"))
			}
			if r.URL.Query().Get("type") != "rsi" {
				t.Errorf("expected type=rsi, got %s", r.URL.Query().Get("type"))
			}
			w.Write([]byte(`{"type":"rsi","code":"WTI_USD","rsi":58.5}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		res, err := client.GetAnalyticsTrend(context.Background(), "WTI_USD", WithAnalyticsType("rsi"))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if res.Code != "WTI_USD" {
			t.Errorf("expected WTI_USD, got %s", res.Code)
		}
	})
}

func TestGetAnalyticsForecast(t *testing.T) {
	t.Run("sends code and method", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/analytics/forecast" {
				t.Errorf("expected /v1/analytics/forecast, got %s", r.URL.Path)
			}
			if r.URL.Query().Get("method") != "ema" {
				t.Errorf("expected method=ema, got %s", r.URL.Query().Get("method"))
			}
			w.Write([]byte(`{"code":"WTI_USD","method":"ema","current_price":72.5}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		res, err := client.GetAnalyticsForecast(context.Background(), "WTI_USD", WithAnalyticsMethod("ema"))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if res.Code != "WTI_USD" {
			t.Errorf("expected WTI_USD, got %s", res.Code)
		}
	})

	t.Run("returns NotFoundError on 404", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(0))
		if _, err := client.GetAnalyticsForecast(context.Background(), "WTI_USD"); !isNotFoundError(err) {
			t.Errorf("expected NotFoundError, got %T: %v", err, err)
		}
	})
}

// ===================
// ENERGY INTELLIGENCE (EI)
// ===================

func TestEIGetOilInventories(t *testing.T) {
	t.Run("fetches latest oil inventories", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/ei/oil_inventories/latest" {
				t.Errorf("expected /v1/ei/oil_inventories/latest, got %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Token test-key" {
				t.Errorf("expected auth header, got %q", r.Header.Get("Authorization"))
			}
			w.Write([]byte(`{"data":{"week_ending":"2026-06-13","crude_commercial":420.5},"meta":{"source":"EIA WPSR"}}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.EI().GetOilInventories(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Meta.Source != "EIA WPSR" {
			t.Errorf("expected EIA WPSR, got %s", resp.Meta.Source)
		}
		var data map[string]interface{}
		json.Unmarshal(resp.Data, &data)
		if data["week_ending"] != "2026-06-13" {
			t.Errorf("expected week_ending in data, got %v", data["week_ending"])
		}
	})

	t.Run("returns NotFoundError on 404", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(0))
		if _, err := client.EI().GetOilInventories(context.Background()); !isNotFoundError(err) {
			t.Errorf("expected NotFoundError, got %T: %v", err, err)
		}
	})
}

func TestEIGetOpecProduction(t *testing.T) {
	t.Run("fetches latest OPEC production with months option", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/ei/opec_productions/latest" {
				t.Errorf("expected /v1/ei/opec_productions/latest, got %s", r.URL.Path)
			}
			if r.URL.Query().Get("months") != "12" {
				t.Errorf("expected months=12, got %s", r.URL.Query().Get("months"))
			}
			w.Write([]byte(`{"data":{"report_month":"2026-05","opec_total":27.1},"meta":{"source":"OPEC MOMR"}}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.EI().GetOpecProduction(context.Background(), WithEIMonths(12))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Meta.Source != "OPEC MOMR" {
			t.Errorf("expected OPEC MOMR, got %s", resp.Meta.Source)
		}
	})
}

func TestEIGetWellPermits(t *testing.T) {
	t.Run("fetches latest well permits with days and states", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/ei/well-permits/latest" {
				t.Errorf("expected /v1/ei/well-permits/latest, got %s", r.URL.Path)
			}
			q := r.URL.Query()
			if q.Get("days") != "30" {
				t.Errorf("expected days=30, got %s", q.Get("days"))
			}
			if q.Get("states") != "TX,OK" {
				t.Errorf("expected states=TX,OK, got %s", q.Get("states"))
			}
			w.Write([]byte(`{"well_permits":[{"api_number":"42329000000001","state_code":"TX","operator":{"name":"Acme"},"well":{"name":"Eagle 1"}}],"meta":{"days":30,"count":1}}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.EI().GetWellPermits(context.Background(), WithEIDays(30), WithEIStates("TX,OK"))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		// well-permits returns its payload outside of "data"; Data should be empty.
		if len(resp.Data) != 0 {
			t.Errorf("expected empty data for well-permits envelope, got %s", string(resp.Data))
		}
		if len(resp.WellPermits) != 1 || resp.WellPermits[0].APINumber != "42329000000001" {
			t.Errorf("expected typed top-level well permits, got %#v", resp.WellPermits)
		}
		if resp.Meta.Source != "" {
			t.Errorf("expected no source meta, got %s", resp.Meta.Source)
		}
	})
}

func TestEISearchWellPermits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/ei/well-permits/search" {
			t.Errorf("expected /v1/ei/well-permits/search, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("states") != "TX" || r.URL.Query().Get("well_name") != "Eagle" {
			t.Errorf("unexpected query %s", r.URL.RawQuery)
		}
		w.Write([]byte(`{"well_permits":[{"api_number":"42329000000001","state_code":"TX","operator":{"name":"Operator"},"well":{"name":"Eagle 1"}}],"meta":{"count":1}}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	resp, err := client.EI().SearchWellPermits(context.Background(), WellPermitSearchQuery{
		States:   "TX",
		WellName: "Eagle",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resp.WellPermits) != 1 || resp.WellPermits[0].APINumber != "42329000000001" {
		t.Fatalf("expected typed permit result, got %#v", resp.WellPermits)
	}
	if resp.WellPermits[0].Operator.Name != "Operator" || resp.WellPermits[0].Well.Name != "Eagle 1" {
		t.Fatalf("expected typed operator and well, got %#v", resp.WellPermits[0])
	}

	latitude := 31.9
	if _, err := client.EI().SearchWellPermits(context.Background(), WellPermitSearchQuery{
		Latitude: &latitude,
	}); err == nil {
		t.Fatal("expected paired-coordinate validation error")
	}
	if _, err := client.EI().SearchWellPermits(context.Background(), WellPermitSearchQuery{
		RadiusMiles: 101,
	}); err == nil {
		t.Fatal("expected radius validation error")
	}
}
