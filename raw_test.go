package oilpriceapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// ===================
// RAW ESCAPE HATCH
// ===================

func TestRaw(t *testing.T) {
	t.Run("decodes response and sends auth header", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/some/new/endpoint" {
				t.Errorf("expected path /v1/some/new/endpoint, got %s", r.URL.Path)
			}
			if r.Method != "GET" {
				t.Errorf("expected GET method, got %s", r.Method)
			}
			if r.Header.Get("Authorization") != "Token test-key" {
				t.Errorf("expected auth header 'Token test-key', got '%s'", r.Header.Get("Authorization"))
			}
			json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   map[string]any{"value": 42.5},
			})
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))

		var out struct {
			Status string `json:"status"`
			Data   struct {
				Value float64 `json:"value"`
			} `json:"data"`
		}
		err := client.Raw(context.Background(), http.MethodGet, "/v1/some/new/endpoint", nil, &out)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if out.Status != "success" {
			t.Errorf("expected status 'success', got '%s'", out.Status)
		}
		if out.Data.Value != 42.5 {
			t.Errorf("expected value 42.5, got %f", out.Data.Value)
		}
	})

	t.Run("passes query params through", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("by_code") != "BRENT_CRUDE_USD" {
				t.Errorf("expected by_code=BRENT_CRUDE_USD, got %s", q.Get("by_code"))
			}
			if q.Get("days") != "30" {
				t.Errorf("expected days=30, got %s", q.Get("days"))
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))

		var out map[string]any
		err := client.Raw(context.Background(), http.MethodGet, "/v1/prices/latest", url.Values{
			"by_code": {"BRENT_CRUDE_USD"},
			"days":    {"30"},
		}, &out)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("allows nil destination to discard body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "DELETE" {
				t.Errorf("expected DELETE method, got %s", r.Method)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		err := client.Raw(context.Background(), http.MethodDelete, "/v1/webhooks/wh_001", nil, nil)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("surfaces API errors via existing error types", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "not found"}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(0))

		var out map[string]any
		err := client.Raw(context.Background(), http.MethodGet, "/v1/does/not/exist", nil, &out)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !isNotFoundError(err) {
			t.Errorf("expected NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("surfaces authentication error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "Unauthorized"}`))
		}))
		defer server.Close()

		client := NewClient("bad-key", WithBaseURL(server.URL), WithRetries(0))

		var out map[string]any
		err := client.Raw(context.Background(), http.MethodGet, "/v1/prices/latest", nil, &out)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !isAuthError(err) {
			t.Errorf("expected AuthenticationError, got %T: %v", err, err)
		}
	})

	t.Run("retries on 429 like other methods", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 3 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(3))

		var out map[string]any
		err := client.Raw(context.Background(), http.MethodGet, "/v1/some/endpoint", nil, &out)

		if err != nil {
			t.Fatalf("expected success after retry, got error: %v", err)
		}
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
		if out["status"] != "success" {
			t.Errorf("expected status 'success', got %v", out["status"])
		}
	})
}
