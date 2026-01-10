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

		client := NewClient("invalid-key", WithBaseURL(server.URL))
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

		client := NewClient("test-api-key", WithBaseURL(server.URL))
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

		client := NewClient("test-api-key", WithBaseURL(server.URL))
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

		client := NewClient("test-api-key", WithBaseURL(server.URL))
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
