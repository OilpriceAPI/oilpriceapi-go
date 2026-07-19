package oilpriceapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ===================
// CLIENT CREATION
// ===================

func TestNewClient(t *testing.T) {
	t.Run("creates client with API key", func(t *testing.T) {
		client := NewClient("test-api-key")
		if client == nil {
			t.Fatal("expected client, got nil")
		}
		if client.apiKey != "test-api-key" {
			t.Errorf("expected apiKey 'test-api-key', got '%s'", client.apiKey)
		}
	})

	t.Run("uses default base URL", func(t *testing.T) {
		client := NewClient("test-api-key")
		if client.baseURL != "https://api.oilpriceapi.com" {
			t.Errorf("expected default baseURL, got '%s'", client.baseURL)
		}
	})

	t.Run("uses default timeout", func(t *testing.T) {
		client := NewClient("test-api-key")
		if client.httpClient.Timeout != 30*time.Second {
			t.Errorf("expected 30s timeout, got %v", client.httpClient.Timeout)
		}
	})
}

func TestClientWithOptions(t *testing.T) {
	t.Run("sets custom base URL", func(t *testing.T) {
		client := NewClient("test-api-key", WithBaseURL("https://custom.api.com"))
		if client.baseURL != "https://custom.api.com" {
			t.Errorf("expected custom baseURL, got '%s'", client.baseURL)
		}
	})

	t.Run("sets custom timeout", func(t *testing.T) {
		client := NewClient("test-api-key", WithTimeout(10*time.Second))
		if client.httpClient.Timeout != 10*time.Second {
			t.Errorf("expected 10s timeout, got %v", client.httpClient.Timeout)
		}
	})

	t.Run("sets retry count", func(t *testing.T) {
		client := NewClient("test-api-key", WithRetries(5))
		if client.retries != 5 {
			t.Errorf("expected 5 retries, got %d", client.retries)
		}
	})
}

// ===================
// DEMO ENDPOINTS (NO AUTH)
// ===================

func TestDemoPrices(t *testing.T) {
	t.Run("fetches demo prices without auth", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify no Authorization header
			if r.Header.Get("Authorization") != "" {
				t.Error("expected no Authorization header for demo endpoint")
			}
			if r.URL.Path != "/v1/demo/prices" {
				t.Errorf("expected path /v1/demo/prices, got %s", r.URL.Path)
			}

			response := DemoPricesResponse{
				Status: "success",
				Data: DemoPricesData{
					Prices: []DemoPrice{
						{Code: "BRENT_CRUDE_USD", Name: "Brent Crude Oil", Price: 75.42, Currency: "USD", Unit: "barrel"},
						{Code: "WTI_USD", Name: "WTI Crude Oil", Price: 72.34, Currency: "USD", Unit: "barrel"},
					},
					Meta: DemoMeta{DemoMode: true, RateLimit: "20 requests per hour"},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("", WithBaseURL(server.URL))
		resp, err := client.GetDemoPrices(context.Background())

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Status != "success" {
			t.Errorf("expected status 'success', got '%s'", resp.Status)
		}
		if len(resp.Data.Prices) != 2 {
			t.Errorf("expected 2 prices, got %d", len(resp.Data.Prices))
		}
		if resp.Data.Prices[0].Code != "BRENT_CRUDE_USD" {
			t.Errorf("expected BRENT_CRUDE_USD, got %s", resp.Data.Prices[0].Code)
		}
	})
}

// ===================
// LATEST PRICES
// ===================

func TestGetLatestPrices(t *testing.T) {
	t.Run("sends authorization header", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth != "Token test-api-key" {
				t.Errorf("expected 'Token test-api-key', got '%s'", auth)
			}

			response := PricesResponse{
				Status: "success",
				Data: PriceData{
					Prices: []Price{
						{Code: "BRENT_CRUDE_USD", Name: "Brent Crude Oil", Price: 75.42, Currency: "USD", Unit: "barrel", UpdatedAt: "2024-01-10T12:00:00Z"},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-api-key", WithBaseURL(server.URL))
		_, err := client.GetLatestPrices(context.Background())

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("returns latest prices", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := PricesResponse{
				Status: "success",
				Data: PriceData{
					Prices: []Price{
						{Code: "BRENT_CRUDE_USD", Name: "Brent Crude Oil", Price: 75.42, Currency: "USD", Unit: "barrel", UpdatedAt: "2024-01-10T12:00:00Z"},
						{Code: "WTI_USD", Name: "WTI Crude Oil", Price: 72.34, Currency: "USD", Unit: "barrel", UpdatedAt: "2024-01-10T12:00:00Z"},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-api-key", WithBaseURL(server.URL))
		resp, err := client.GetLatestPrices(context.Background())

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(resp.Data.Prices) != 2 {
			t.Errorf("expected 2 prices, got %d", len(resp.Data.Prices))
		}
	})

	t.Run("filters by commodity code", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			byCode := r.URL.Query().Get("by_code")
			if byCode != "BRENT_CRUDE_USD" {
				t.Errorf("expected by_code=BRENT_CRUDE_USD, got %s", byCode)
			}

			response := PricesResponse{
				Status: "success",
				Data: PriceData{
					Prices: []Price{
						{Code: "BRENT_CRUDE_USD", Name: "Brent Crude Oil", Price: 75.42, Currency: "USD", Unit: "barrel", UpdatedAt: "2024-01-10T12:00:00Z"},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-api-key", WithBaseURL(server.URL))
		resp, err := client.GetLatestPrices(context.Background(), WithCommodity("BRENT_CRUDE_USD"))

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(resp.Data.Prices) != 1 {
			t.Errorf("expected 1 price, got %d", len(resp.Data.Prices))
		}
	})

	t.Run("decodes the production singleton payload", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"code":"BRENT_CRUDE_USD",
					"name":"Brent Crude Oil",
					"price":71.80,
					"currency":"USD",
					"unit":"barrel",
					"updated_at":"2026-07-19T12:00:00Z",
					"source":"market_reporting"
				}
			}`))
		}))
		defer server.Close()

		client := NewClient("test-api-key", WithBaseURL(server.URL))
		resp, err := client.GetLatestPrices(context.Background(), WithCommodity("BRENT_CRUDE_USD"))

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(resp.Data.Prices) != 1 {
			t.Fatalf("expected one production price, got %d", len(resp.Data.Prices))
		}
		price := resp.Data.Prices[0]
		if price.Code != "BRENT_CRUDE_USD" || price.Price != 71.80 || price.UpdatedAt != "2026-07-19T12:00:00Z" {
			t.Fatalf("unexpected production price: %+v", price)
		}
	})

	t.Run("rejects a success payload without a price", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{}}`))
		}))
		defer server.Close()

		client := NewClient("test-api-key", WithBaseURL(server.URL))
		resp, err := client.GetLatestPrices(context.Background(), WithCommodity("BRENT_CRUDE_USD"))

		if err == nil {
			t.Fatalf("expected malformed success payload to fail, got %+v", resp)
		}
	})
}

// ===================
// ERROR HANDLING
// ===================

func TestErrorHandling(t *testing.T) {
	t.Run("handles authentication error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid API key"})
		}))
		defer server.Close()

		client := NewClient("invalid-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.GetLatestPrices(context.Background())

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var authErr *AuthenticationError
		if !isAuthError(err) {
			t.Errorf("expected AuthenticationError, got %T: %v", err, err)
		}
		_ = authErr
	})

	t.Run("handles rate limit error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "Rate limit exceeded"})
		}))
		defer server.Close()

		client := NewClient("test-api-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.GetLatestPrices(context.Background())

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !isRateLimitError(err) {
			t.Errorf("expected RateLimitError, got %T: %v", err, err)
		}
	})

	t.Run("handles not found error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Resource not found"})
		}))
		defer server.Close()

		client := NewClient("test-api-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.GetLatestPrices(context.Background())

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !isNotFoundError(err) {
			t.Errorf("expected NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("handles server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		}))
		defer server.Close()

		client := NewClient("test-api-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.GetLatestPrices(context.Background())

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !isServerError(err) {
			t.Errorf("expected ServerError, got %T: %v", err, err)
		}
	})
}

// ===================
// CONTEXT SUPPORT
// ===================

func TestContextSupport(t *testing.T) {
	t.Run("respects context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			json.NewEncoder(w).Encode(PricesResponse{Status: "success"})
		}))
		defer server.Close()

		client := NewClient("test-api-key", WithBaseURL(server.URL))

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := client.GetLatestPrices(ctx)

		if err == nil {
			t.Fatal("expected error due to cancelled context")
		}
	})

	t.Run("respects context timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			json.NewEncoder(w).Encode(PricesResponse{Status: "success"})
		}))
		defer server.Close()

		client := NewClient("test-api-key", WithBaseURL(server.URL))

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := client.GetLatestPrices(ctx)

		if err == nil {
			t.Fatal("expected error due to context timeout")
		}
	})
}

// ===================
// COMMODITIES
// ===================

func TestGetCommodities(t *testing.T) {
	t.Run("returns list of commodities", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/commodities" {
				t.Errorf("expected path /v1/commodities, got %s", r.URL.Path)
			}

			response := CommoditiesResponse{
				Status: "success",
				Data: CommoditiesData{
					Commodities: []Commodity{
						{Code: "BRENT_CRUDE_USD", Name: "Brent Crude Oil", Category: "Crude Oil"},
						{Code: "WTI_USD", Name: "WTI Crude Oil", Category: "Crude Oil"},
						{Code: "NATURAL_GAS_USD", Name: "Natural Gas", Category: "Natural Gas"},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-api-key", WithBaseURL(server.URL))
		resp, err := client.GetCommodities(context.Background())

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(resp.Data.Commodities) != 3 {
			t.Errorf("expected 3 commodities, got %d", len(resp.Data.Commodities))
		}
	})
}

// Helper functions for error type checking
func isAuthError(err error) bool {
	_, ok := err.(*AuthenticationError)
	return ok
}

func isRateLimitError(err error) bool {
	_, ok := err.(*RateLimitError)
	return ok
}

func isNotFoundError(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}

func isServerError(err error) bool {
	_, ok := err.(*ServerError)
	return ok
}

// ===================
// HISTORICAL PRICES
// ===================

func TestGetHistoricalPrices(t *testing.T) {
	t.Run("fetches historical prices with default period", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/prices/past_month" {
				t.Errorf("expected path /v1/prices/past_month, got %s", r.URL.Path)
			}
			if r.URL.Query().Get("by_code") != "BRENT_CRUDE_USD" {
				t.Errorf("expected by_code=BRENT_CRUDE_USD, got %s", r.URL.Query().Get("by_code"))
			}
			if r.Header.Get("Authorization") != "Token test-key" {
				t.Errorf("expected auth header 'Token test-key', got '%s'", r.Header.Get("Authorization"))
			}

			response := HistoricalResponse{
				Status: "success",
				Data: HistoricalData{
					Prices: []HistoricalPrice{
						{Price: 75.42, CreatedAt: "2024-01-10T00:00:00Z"},
						{Price: 74.10, CreatedAt: "2024-01-09T00:00:00Z"},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.GetHistoricalPrices(context.Background(), "BRENT_CRUDE_USD")

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(resp.Data.Prices) != 2 {
			t.Errorf("expected 2 prices, got %d", len(resp.Data.Prices))
		}
		if resp.Data.Prices[0].Price != 75.42 {
			t.Errorf("expected price 75.42, got %f", resp.Data.Prices[0].Price)
		}
	})

	t.Run("uses custom period", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/prices/past_year" {
				t.Errorf("expected path /v1/prices/past_year, got %s", r.URL.Path)
			}
			json.NewEncoder(w).Encode(HistoricalResponse{Status: "success", Data: HistoricalData{Prices: []HistoricalPrice{}}})
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		_, err := client.GetHistoricalPrices(context.Background(), "WTI_USD", WithPeriod("year"))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("sends pagination params", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("page") != "2" {
				t.Errorf("expected page=2, got %s", r.URL.Query().Get("page"))
			}
			if r.URL.Query().Get("per_page") != "50" {
				t.Errorf("expected per_page=50, got %s", r.URL.Query().Get("per_page"))
			}
			json.NewEncoder(w).Encode(HistoricalResponse{Status: "success", Data: HistoricalData{Prices: []HistoricalPrice{}}})
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		_, err := client.GetHistoricalPrices(context.Background(), "BRENT_CRUDE_USD", WithPage(2), WithPerPage(50))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("returns error on non-200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "not found"}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.GetHistoricalPrices(context.Background(), "UNKNOWN_CODE")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !isNotFoundError(err) {
			t.Errorf("expected NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("returns error for invalid period", func(t *testing.T) {
		client := NewClient("test-key")
		_, err := client.GetHistoricalPrices(context.Background(), "BRENT_CRUDE_USD", WithPeriod("quarter"))
		if err == nil {
			t.Fatal("expected error for invalid period, got nil")
		}
	})
}

// ===================
// FUTURES
// ===================

func TestGetFuturesLatest(t *testing.T) {
	t.Run("fetches futures latest with default contract", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/futures/ice-brent" {
				t.Errorf("expected path /v1/futures/ice-brent, got %s", r.URL.Path)
			}
			if r.URL.RawQuery != "" {
				t.Errorf("expected no query params, got %q", r.URL.RawQuery)
			}
			if r.Header.Get("Authorization") != "Token test-key" {
				t.Errorf("expected auth header, got '%s'", r.Header.Get("Authorization"))
			}

			response := FuturesResponse{
				Commodity: "BRENT_FUTURES",
				Source:    "market_reporting",
				FrontMonth: &FuturesContract{
					Code: "BZF25", ContractMonth: "2026-08", LastPrice: 78.50, Currency: "USD",
				},
				Contracts: []FuturesContract{
					{Code: "BZF25", ContractMonth: "2026-08", LastPrice: 78.50},
					{Code: "BZG25", ContractMonth: "2026-09", LastPrice: 77.90},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.GetFuturesLatest(context.Background())

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.FrontMonth == nil {
			t.Fatal("expected non-nil front_month")
		}
		if resp.FrontMonth.LastPrice != 78.50 {
			t.Errorf("expected front_month price 78.50, got %f", resp.FrontMonth.LastPrice)
		}
		if len(resp.Contracts) != 2 {
			t.Errorf("expected 2 contracts, got %d", len(resp.Contracts))
		}
		if resp.Contracts[0].LastPrice != 78.50 {
			t.Errorf("expected price 78.50, got %f", resp.Contracts[0].LastPrice)
		}
	})

	t.Run("maps contract code CL to ice-wti slug", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/futures/ice-wti" {
				t.Errorf("expected path /v1/futures/ice-wti, got %s", r.URL.Path)
			}
			json.NewEncoder(w).Encode(FuturesResponse{Commodity: "WTI_FUTURES", Contracts: []FuturesContract{}})
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		_, err := client.GetFuturesLatest(context.Background(), WithContract("CL"))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("accepts slug directly", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/futures/natural-gas" {
				t.Errorf("expected path /v1/futures/natural-gas, got %s", r.URL.Path)
			}
			json.NewEncoder(w).Encode(FuturesResponse{Commodity: "NATURAL_GAS_FUTURES", Contracts: []FuturesContract{}})
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		_, err := client.GetFuturesLatest(context.Background(), WithContract("natural-gas"))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("returns error on non-200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "Unauthorized"}`))
		}))
		defer server.Close()

		client := NewClient("bad-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.GetFuturesLatest(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !isAuthError(err) {
			t.Errorf("expected AuthenticationError, got %T: %v", err, err)
		}
	})
}

func TestGetFuturesCurve(t *testing.T) {
	t.Run("fetches futures curve with default contract", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/futures/ice-brent/curve" {
				t.Errorf("expected path /v1/futures/ice-brent/curve, got %s", r.URL.Path)
			}
			if r.URL.RawQuery != "" {
				t.Errorf("expected no query params, got %q", r.URL.RawQuery)
			}
			if r.Header.Get("Authorization") != "Token test-key" {
				t.Errorf("expected auth header, got '%s'", r.Header.Get("Authorization"))
			}

			response := FuturesResponse{
				Commodity: "BRENT_FUTURES",
				Source:    "market_reporting",
				Contracts: []FuturesContract{
					{Code: "BZF25", ContractMonth: "2026-08", LastPrice: 78.50},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.GetFuturesCurve(context.Background())

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(resp.Contracts) != 1 {
			t.Errorf("expected 1 contract, got %d", len(resp.Contracts))
		}
	})

	t.Run("decodes documented no-data response without error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"error":"No futures data available for curve analysis","date":"2026-06-21"}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.GetFuturesCurve(context.Background())
		if err != nil {
			t.Fatalf("expected no error for documented no-data response, got %v", err)
		}
		if resp.Error == "" {
			t.Error("expected Error field to be populated for no-data response")
		}
		if len(resp.Contracts) != 0 {
			t.Errorf("expected 0 contracts for no-data response, got %d", len(resp.Contracts))
		}
	})

	t.Run("maps contract code CL to ice-wti curve slug", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/futures/ice-wti/curve" {
				t.Errorf("expected path /v1/futures/ice-wti/curve, got %s", r.URL.Path)
			}
			json.NewEncoder(w).Encode(FuturesResponse{Commodity: "WTI_FUTURES", Contracts: []FuturesContract{}})
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		_, err := client.GetFuturesCurve(context.Background(), WithContract("CL"))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("returns error on non-200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "server error"}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.GetFuturesCurve(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !isServerError(err) {
			t.Errorf("expected ServerError, got %T: %v", err, err)
		}
	})
}

func TestFuturesSlug(t *testing.T) {
	cases := map[string]string{
		"":                 "ice-brent",
		"BZ":               "ice-brent",
		"bz":               "ice-brent",
		"CL":               "ice-wti",
		"G":                "ice-gasoil",
		"QS":               "ice-gasoil",
		"NG":               "natural-gas",
		"TTF":              "ttf-gas",
		"JKM":              "lng-jkm",
		"EUA":              "eua-carbon",
		"UKA":              "uk-carbon",
		"ice-brent":        "ice-brent",
		"ICE-WTI":          "ice-wti",
		"natural-gas":      "natural-gas",
		"continuous/brent": "continuous/brent",
		"  ice-gasoil  ":   "ice-gasoil",
		"some-future-slug": "some-future-slug",
	}
	for in, want := range cases {
		if got := futuresSlug(in); got != want {
			t.Errorf("futuresSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

// ===================
// MARINE FUELS
// ===================

func TestGetMarineFuels(t *testing.T) {
	t.Run("fetches marine fuel prices", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/marine-fuels/latest" {
				t.Errorf("expected path /v1/marine-fuels/latest, got %s", r.URL.Path)
			}
			if r.Method != "GET" {
				t.Errorf("expected GET method, got %s", r.Method)
			}
			if r.Header.Get("Authorization") != "Token test-key" {
				t.Errorf("expected auth header, got '%s'", r.Header.Get("Authorization"))
			}

			response := MarineFuelsResponse{
				Status: "success",
				Data: MarineFuelsData{
					Prices: []MarineFuelPrice{
						{Port: "Rotterdam", FuelType: "VLSFO", Price: 620.50, Currency: "USD", Unit: "MT"},
						{Port: "Singapore", FuelType: "VLSFO", Price: 640.00, Currency: "USD", Unit: "MT"},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.GetMarineFuels(context.Background())

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(resp.Data.Prices) != 2 {
			t.Errorf("expected 2 prices, got %d", len(resp.Data.Prices))
		}
		if resp.Data.Prices[0].Port != "Rotterdam" {
			t.Errorf("expected Rotterdam, got %s", resp.Data.Prices[0].Port)
		}
	})

	t.Run("returns error on non-200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "Unauthorized"}`))
		}))
		defer server.Close()

		client := NewClient("bad-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.GetMarineFuels(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !isAuthError(err) {
			t.Errorf("expected AuthenticationError, got %T: %v", err, err)
		}
	})
}

// ===================
// RIG COUNTS
// ===================

func TestGetRigCounts(t *testing.T) {
	t.Run("fetches rig count data", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/rig-counts/latest" {
				t.Errorf("expected path /v1/rig-counts/latest, got %s", r.URL.Path)
			}
			if r.Method != "GET" {
				t.Errorf("expected GET method, got %s", r.Method)
			}
			if r.Header.Get("Authorization") != "Token test-key" {
				t.Errorf("expected auth header, got '%s'", r.Header.Get("Authorization"))
			}

			response := RigCountResponse{
				Status: "success",
				Data:   RigCountData{Oil: 500, Gas: 100, Total: 600, Date: "2024-01-10"},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.GetRigCounts(context.Background())

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Data.Total != 600 {
			t.Errorf("expected total 600, got %d", resp.Data.Total)
		}
		if resp.Data.Oil != 500 {
			t.Errorf("expected oil 500, got %d", resp.Data.Oil)
		}
	})

	t.Run("returns error on non-200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "Unauthorized"}`))
		}))
		defer server.Close()

		client := NewClient("bad-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.GetRigCounts(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !isAuthError(err) {
			t.Errorf("expected AuthenticationError, got %T: %v", err, err)
		}
	})
}

// ===================
// DRILLING INTELLIGENCE
// ===================

func TestGetDrillingIntelligence(t *testing.T) {
	t.Run("fetches drilling intelligence data", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/drilling/latest" {
				t.Errorf("expected path /v1/drilling/latest, got %s", r.URL.Path)
			}
			if r.Method != "GET" {
				t.Errorf("expected GET method, got %s", r.Method)
			}
			if r.Header.Get("Authorization") != "Token test-key" {
				t.Errorf("expected auth header, got '%s'", r.Header.Get("Authorization"))
			}

			response := DrillingResponse{
				Status: "success",
				Data: DrillingData{
					TotalWells: 1200,
					ActiveRigs: 450,
					Date:       "2024-01-10",
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.GetDrillingIntelligence(context.Background())

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Data.TotalWells != 1200 {
			t.Errorf("expected total_wells 1200, got %d", resp.Data.TotalWells)
		}
		if resp.Data.ActiveRigs != 450 {
			t.Errorf("expected active_rigs 450, got %d", resp.Data.ActiveRigs)
		}
	})

	t.Run("returns error on non-200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "not found"}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.GetDrillingIntelligence(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !isNotFoundError(err) {
			t.Errorf("expected NotFoundError, got %T: %v", err, err)
		}
	})
}

// ===================
// WEBHOOKS
// ===================

func TestListWebhooks(t *testing.T) {
	t.Run("lists all webhooks", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/webhooks" {
				t.Errorf("expected path /v1/webhooks, got %s", r.URL.Path)
			}
			if r.Method != "GET" {
				t.Errorf("expected GET method, got %s", r.Method)
			}
			if r.Header.Get("Authorization") != "Token test-key" {
				t.Errorf("expected auth header, got '%s'", r.Header.Get("Authorization"))
			}

			response := WebhooksResponse{
				Status: "success",
				Data: WebhooksData{
					Webhooks: []Webhook{
						{ID: "wh_001", URL: "https://example.com/hook1", Events: []string{"price.updated"}, Active: true, CreatedAt: "2024-01-01T00:00:00Z"},
						{ID: "wh_002", URL: "https://example.com/hook2", Events: []string{"price.updated", "alert.triggered"}, Active: false, CreatedAt: "2024-01-02T00:00:00Z"},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.ListWebhooks(context.Background())

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(resp.Data.Webhooks) != 2 {
			t.Errorf("expected 2 webhooks, got %d", len(resp.Data.Webhooks))
		}
		if resp.Data.Webhooks[0].ID != "wh_001" {
			t.Errorf("expected wh_001, got %s", resp.Data.Webhooks[0].ID)
		}
	})

	t.Run("returns error on non-200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "Unauthorized"}`))
		}))
		defer server.Close()

		client := NewClient("bad-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.ListWebhooks(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !isAuthError(err) {
			t.Errorf("expected AuthenticationError, got %T: %v", err, err)
		}
	})
}

func TestCreateWebhook(t *testing.T) {
	t.Run("creates a webhook with POST", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/webhooks" {
				t.Errorf("expected path /v1/webhooks, got %s", r.URL.Path)
			}
			if r.Method != "POST" {
				t.Errorf("expected POST method, got %s", r.Method)
			}
			if r.Header.Get("Authorization") != "Token test-key" {
				t.Errorf("expected auth header, got '%s'", r.Header.Get("Authorization"))
			}

			var input WebhookCreateInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Errorf("failed to decode request body: %v", err)
			}
			if input.URL != "https://example.com/webhook" {
				t.Errorf("expected URL https://example.com/webhook, got %s", input.URL)
			}

			w.WriteHeader(http.StatusCreated)
			response := WebhookResponse{
				Status: "success",
				Data: Webhook{
					ID:        "wh_new",
					URL:       input.URL,
					Events:    input.Events,
					Active:    true,
					CreatedAt: "2024-01-10T00:00:00Z",
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.CreateWebhook(context.Background(), WebhookCreateInput{
			URL:    "https://example.com/webhook",
			Events: []string{"price.updated"},
		})

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Data.ID != "wh_new" {
			t.Errorf("expected ID wh_new, got %s", resp.Data.ID)
		}
	})

	t.Run("returns error on non-200/201", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "Unauthorized"}`))
		}))
		defer server.Close()

		client := NewClient("bad-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.CreateWebhook(context.Background(), WebhookCreateInput{
			URL:    "https://example.com/webhook",
			Events: []string{"price.updated"},
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !isAuthError(err) {
			t.Errorf("expected AuthenticationError, got %T: %v", err, err)
		}
	})
}

func TestDeleteWebhook(t *testing.T) {
	t.Run("deletes a webhook with DELETE", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/webhooks/wh_001" {
				t.Errorf("expected path /v1/webhooks/wh_001, got %s", r.URL.Path)
			}
			if r.Method != "DELETE" {
				t.Errorf("expected DELETE method, got %s", r.Method)
			}
			if r.Header.Get("Authorization") != "Token test-key" {
				t.Errorf("expected auth header, got '%s'", r.Header.Get("Authorization"))
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		err := client.DeleteWebhook(context.Background(), "wh_001")

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("also accepts 200 on delete", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "DELETE" {
				t.Errorf("expected DELETE method, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		err := client.DeleteWebhook(context.Background(), "wh_002")

		if err != nil {
			t.Fatalf("expected no error on 200, got %v", err)
		}
	})

	t.Run("returns error on non-200/204", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "Unauthorized"}`))
		}))
		defer server.Close()

		client := NewClient("bad-key", WithBaseURL(server.URL), WithRetries(0))
		err := client.DeleteWebhook(context.Background(), "wh_001")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !isAuthError(err) {
			t.Errorf("expected AuthenticationError, got %T: %v", err, err)
		}
	})
}

// ===================
// RETRY BEHAVIOR
// ===================

func TestRetryBehavior(t *testing.T) {
	t.Run("retries on 429", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 3 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			response := HistoricalResponse{
				Status: "success",
				Data: HistoricalData{
					Prices: []HistoricalPrice{{Price: 75.0, CreatedAt: "2024-01-01T00:00:00Z"}},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(3))
		resp, err := client.GetHistoricalPrices(context.Background(), "BRENT_CRUDE_USD")

		if err != nil {
			t.Fatalf("expected success after retry, got error: %v", err)
		}
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
		if resp.Data.Prices[0].Price != 75.0 {
			t.Errorf("expected price 75.0, got %f", resp.Data.Prices[0].Price)
		}
	})

	t.Run("retries on 500", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			response := RigCountResponse{
				Status: "success",
				Data:   RigCountData{Oil: 500, Gas: 100, Total: 600, Date: "2024-01-10"},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(3))
		resp, err := client.GetRigCounts(context.Background())

		if err != nil {
			t.Fatalf("expected success after retry, got error: %v", err)
		}
		if resp.Data.Total != 600 {
			t.Errorf("expected 600 total rigs, got %d", resp.Data.Total)
		}
	})

	t.Run("does not retry on 401", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "Unauthorized"}`))
		}))
		defer server.Close()

		client := NewClient("bad-key", WithBaseURL(server.URL), WithRetries(3))
		_, err := client.GetRigCounts(context.Background())

		if err == nil {
			t.Fatal("expected error for 401")
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt (no retry on 401), got %d", attempts)
		}
	})
}

// ===================
// DATE-RANGE HISTORICAL
// ===================

func TestGetHistoricalPricesDateRange(t *testing.T) {
	tests := []struct {
		name          string
		opts          []HistoricalOption
		wantPath      string
		wantStart     string
		wantEnd       string
		wantInterval  string
		wantByCode    string
		wantPageQuery string
	}{
		{
			name:       "start date only routes to /historical",
			opts:       []HistoricalOption{WithStartDate("2024-01-01")},
			wantPath:   "/v1/prices/historical",
			wantStart:  "2024-01-01",
			wantByCode: "WTI_USD",
		},
		{
			name:         "full range with interval",
			opts:         []HistoricalOption{WithStartDate("2024-01-01"), WithEndDate("2024-12-31"), WithInterval("daily")},
			wantPath:     "/v1/prices/historical",
			wantStart:    "2024-01-01",
			wantEnd:      "2024-12-31",
			wantInterval: "daily",
			wantByCode:   "WTI_USD",
		},
		{
			name:       "end date only routes to /historical",
			opts:       []HistoricalOption{WithEndDate("2024-06-30")},
			wantPath:   "/v1/prices/historical",
			wantEnd:    "2024-06-30",
			wantByCode: "WTI_USD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.wantPath {
					t.Errorf("expected path %s, got %s", tt.wantPath, r.URL.Path)
				}
				q := r.URL.Query()
				if q.Get("by_code") != tt.wantByCode {
					t.Errorf("expected by_code=%s, got %s", tt.wantByCode, q.Get("by_code"))
				}
				if q.Get("start_date") != tt.wantStart {
					t.Errorf("expected start_date=%q, got %q", tt.wantStart, q.Get("start_date"))
				}
				if q.Get("end_date") != tt.wantEnd {
					t.Errorf("expected end_date=%q, got %q", tt.wantEnd, q.Get("end_date"))
				}
				if q.Get("interval") != tt.wantInterval {
					t.Errorf("expected interval=%q, got %q", tt.wantInterval, q.Get("interval"))
				}

				response := HistoricalResponse{
					Status: "success",
					Data: HistoricalData{
						Prices: []HistoricalPrice{
							{Price: 75.42, CreatedAt: "2024-01-10T00:00:00Z", Code: "WTI_USD", Currency: "USD", Formatted: "$75.42", Type: "daily_average", Unit: "barrel", Source: "aggregated"},
						},
					},
				}
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			resp, err := client.GetHistoricalPrices(context.Background(), "WTI_USD", tt.opts...)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if len(resp.Data.Prices) != 1 {
				t.Fatalf("expected 1 price, got %d", len(resp.Data.Prices))
			}
			if resp.Data.Prices[0].Currency != "USD" {
				t.Errorf("expected currency USD, got %s", resp.Data.Prices[0].Currency)
			}
			if resp.Data.Prices[0].Source != "aggregated" {
				t.Errorf("expected source aggregated, got %s", resp.Data.Prices[0].Source)
			}
		})
	}

	t.Run("date range still passes pagination", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/prices/historical" {
				t.Errorf("expected /v1/prices/historical, got %s", r.URL.Path)
			}
			if r.URL.Query().Get("page") != "3" {
				t.Errorf("expected page=3, got %s", r.URL.Query().Get("page"))
			}
			json.NewEncoder(w).Encode(HistoricalResponse{Status: "success"})
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		_, err := client.GetHistoricalPrices(context.Background(), "WTI_USD", WithStartDate("2024-01-01"), WithPage(3))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("returns error on non-200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "not found"}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.GetHistoricalPrices(context.Background(), "WTI_USD", WithStartDate("2024-01-01"))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !isNotFoundError(err) {
			t.Errorf("expected NotFoundError, got %T: %v", err, err)
		}
	})
}

// ===================
// FORECASTS
// ===================

func TestGetForecasts(t *testing.T) {
	t.Run("fetches all-commodity forecasts", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/forecasts/monthly" {
				t.Errorf("expected path /v1/forecasts/monthly, got %s", r.URL.Path)
			}
			if r.URL.Query().Get("commodity") != "" {
				t.Errorf("expected no commodity filter, got %s", r.URL.Query().Get("commodity"))
			}
			if r.Header.Get("Authorization") != "Token test-key" {
				t.Errorf("expected auth header, got %s", r.Header.Get("Authorization"))
			}

			response := ForecastsResponse{
				Data: ForecastsData{
					Period:      "2026-02",
					GeneratedAt: "2026-01-15T00:00:00Z",
					Commodities: []Forecast{
						{
							Commodity:         "WTI_USD",
							GeneratedForMonth: "2026-01",
							Forecasts: map[string]ForecastHorizon{
								"1_month": {Period: "2026-02", PointEstimate: 78.5, LowBound: 74.0, HighBound: 83.0, Confidence: 0.8},
							},
							ModelInputs: ForecastModelInputs{EMATrendValue: 77.2},
						},
					},
					Disclaimer: "Forecasts are estimates.",
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.GetForecasts(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Data.Period != "2026-02" {
			t.Errorf("expected period 2026-02, got %s", resp.Data.Period)
		}
		if len(resp.Data.Commodities) != 1 {
			t.Fatalf("expected 1 commodity, got %d", len(resp.Data.Commodities))
		}
		fc := resp.Data.Commodities[0]
		if fc.Commodity != "WTI_USD" {
			t.Errorf("expected WTI_USD, got %s", fc.Commodity)
		}
		if fc.Forecasts["1_month"].PointEstimate != 78.5 {
			t.Errorf("expected 1m point 78.5, got %f", fc.Forecasts["1_month"].PointEstimate)
		}
	})

	t.Run("fetches single-commodity forecast", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("commodity") != "BRENT_CRUDE_USD" {
				t.Errorf("expected commodity=BRENT_CRUDE_USD, got %s", r.URL.Query().Get("commodity"))
			}
			response := ForecastsResponse{
				Data: ForecastsData{
					Commodity:         "BRENT_CRUDE_USD",
					GeneratedForMonth: "2026-01",
					Forecasts: map[string]ForecastHorizon{
						"3_month": {Period: "2026-04", PointEstimate: 82.0, Confidence: 0.6},
					},
					Accuracy: &ForecastAccuracy{MAPE1M: 3.2, DirectionCorrect1M: true},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.GetForecasts(context.Background(), WithForecastCommodity("BRENT_CRUDE_USD"))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Data.Commodity != "BRENT_CRUDE_USD" {
			t.Errorf("expected commodity BRENT_CRUDE_USD, got %s", resp.Data.Commodity)
		}
		if resp.Data.Forecasts["3_month"].PointEstimate != 82.0 {
			t.Errorf("expected 3m point 82.0, got %f", resp.Data.Forecasts["3_month"].PointEstimate)
		}
		if resp.Data.Accuracy == nil || !resp.Data.Accuracy.DirectionCorrect1M {
			t.Errorf("expected accuracy with DirectionCorrect1M=true, got %+v", resp.Data.Accuracy)
		}
	})

	t.Run("returns error on non-200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "No forecasts available"}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.GetForecasts(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !isNotFoundError(err) {
			t.Errorf("expected NotFoundError, got %T: %v", err, err)
		}
	})
}

func TestGetForecastsAccuracy(t *testing.T) {
	t.Run("fetches accuracy stats with options", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/forecasts/monthly/accuracy" {
				t.Errorf("expected path /v1/forecasts/monthly/accuracy, got %s", r.URL.Path)
			}
			if r.URL.Query().Get("commodity") != "WTI_USD" {
				t.Errorf("expected commodity=WTI_USD, got %s", r.URL.Query().Get("commodity"))
			}
			if r.URL.Query().Get("lookback_months") != "24" {
				t.Errorf("expected lookback_months=24, got %s", r.URL.Query().Get("lookback_months"))
			}

			response := ForecastAccuracyResponse{
				Data: ForecastAccuracyData{
					LookbackMonths: 24,
					Commodity:      "WTI_USD",
					Statistics: ForecastAccuracyStatistics{
						SampleSize:            18,
						AvgMAPE1M:             3.5,
						DirectionalAccuracy1M: 72.2,
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.GetForecastsAccuracy(context.Background(), WithForecastCommodity("WTI_USD"), WithLookbackMonths(24))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if resp.Data.Statistics.SampleSize != 18 {
			t.Errorf("expected sample_size 18, got %d", resp.Data.Statistics.SampleSize)
		}
		if resp.Data.Statistics.DirectionalAccuracy1M != 72.2 {
			t.Errorf("expected directional accuracy 72.2, got %f", resp.Data.Statistics.DirectionalAccuracy1M)
		}
	})

	t.Run("omits query params when unset", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.RawQuery != "" {
				t.Errorf("expected empty query, got %q", r.URL.RawQuery)
			}
			json.NewEncoder(w).Encode(ForecastAccuracyResponse{Data: ForecastAccuracyData{LookbackMonths: 12}})
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		_, err := client.GetForecastsAccuracy(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}

// ===================
// STORAGE
// ===================

func TestGetStorage(t *testing.T) {
	t.Run("fetches all storage levels", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/storage" {
				t.Errorf("expected path /v1/storage, got %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Token test-key" {
				t.Errorf("expected auth header, got %s", r.Header.Get("Authorization"))
			}

			response := StorageResponse{
				Status: "success",
				Data: StorageData{
					Storage: []StorageLevel{
						{Code: "CUSHING_STORAGE", Location: "Cushing, OK", Value: 28.4, Units: "million_barrels", CapacityUtilization: 37.4, MarketSignal: "bearish", DataDate: "2024-01-10"},
						{Code: "US_SPR", Location: "Strategic Petroleum Reserve", Value: 351.3, Units: "million_barrels", DataDate: "2024-01-10"},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		resp, err := client.GetStorage(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(resp.Data.Storage) != 2 {
			t.Fatalf("expected 2 storage levels, got %d", len(resp.Data.Storage))
		}
		if resp.Data.Storage[0].Code != "CUSHING_STORAGE" {
			t.Errorf("expected CUSHING_STORAGE, got %s", resp.Data.Storage[0].Code)
		}
		if resp.Data.Storage[0].Value != 28.4 {
			t.Errorf("expected value 28.4, got %f", resp.Data.Storage[0].Value)
		}
	})

	t.Run("returns error on non-200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "Unauthorized"}`))
		}))
		defer server.Close()

		client := NewClient("bad-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.GetStorage(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !isAuthError(err) {
			t.Errorf("expected AuthenticationError, got %T: %v", err, err)
		}
	})
}

func TestGetStorageHubs(t *testing.T) {
	tests := []struct {
		name     string
		call     func(*Client) (*StorageHubResponse, error)
		wantPath string
	}{
		{
			name:     "cushing",
			call:     func(c *Client) (*StorageHubResponse, error) { return c.GetStorageCushing(context.Background()) },
			wantPath: "/v1/storage/cushing",
		},
		{
			name:     "spr",
			call:     func(c *Client) (*StorageHubResponse, error) { return c.GetStorageSPR(context.Background()) },
			wantPath: "/v1/storage/spr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.wantPath {
					t.Errorf("expected path %s, got %s", tt.wantPath, r.URL.Path)
				}
				response := StorageHubResponse{
					Status: "success",
					Data: StorageHubData{
						Storage: StorageHub{
							Current: StorageHubCurrent{
								Value:               28.4,
								Units:               "million_barrels",
								CapacityUtilization: 37.4,
								WeekOverWeekChange:  -0.5,
								DataDate:            "2024-01-10",
							},
							Analytics: StorageHubAnalytics{
								MarketSignal: "bearish",
								Week52High:   34.1,
								Week52Low:    22.0,
							},
							Metadata: StorageHubMetadata{
								Source:        "EIA",
								TotalCapacity: 76,
							},
						},
					},
				}
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			resp, err := tt.call(client)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if resp.Data.Storage.Current.Value != 28.4 {
				t.Errorf("expected value 28.4, got %f", resp.Data.Storage.Current.Value)
			}
			if resp.Data.Storage.Analytics.MarketSignal != "bearish" {
				t.Errorf("expected market_signal bearish, got %s", resp.Data.Storage.Analytics.MarketSignal)
			}
			if resp.Data.Storage.Metadata.TotalCapacity != 76 {
				t.Errorf("expected total_capacity 76, got %f", resp.Data.Storage.Metadata.TotalCapacity)
			}
		})
	}

	t.Run("returns error on non-200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error": "Premium feature"}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(0))
		_, err := client.GetStorageCushing(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
