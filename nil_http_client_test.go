package oilpriceapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// WithHTTPClient(nil) is what a caller writes when the client comes out of a
// config struct or a helper that returned nil on an error path. The option
// stored the nil verbatim and the first request dereferenced it, so a
// misconfiguration that should have surfaced as a typed error took the
// process down instead.
//
// The contract: a nil *http.Client is ignored and the client keeps a usable
// default, exactly as if the option had not been supplied.

func TestWithHTTPClientNilDoesNotPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"prices":[{"code":"BRENT_CRUDE_USD","name":"Brent Crude Oil","price":75.42,"currency":"USD","unit":"barrel","updated_at":"2024-01-10T12:00:00Z"}]}}`))
	}))
	defer srv.Close()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("WithHTTPClient(nil) panicked on the first request: %v", r)
		}
	}()

	client := NewClient("test-key", WithBaseURL(srv.URL), WithHTTPClient(nil))

	if client.httpClient == nil {
		t.Fatal("WithHTTPClient(nil) left the client with a nil *http.Client")
	}

	if _, err := client.GetLatestPrices(context.Background()); err != nil {
		t.Fatalf("request with WithHTTPClient(nil) failed: %v", err)
	}
}

// The fallback is the SDK default, not a zero-value client: a nil option must
// not silently remove the default request timeout.
func TestWithHTTPClientNilKeepsDefaultTimeout(t *testing.T) {
	client := NewClient("test-key", WithHTTPClient(nil))

	if client.httpClient == nil {
		t.Fatal("WithHTTPClient(nil) left the client with a nil *http.Client")
	}
	if got := client.httpClient.Timeout; got != DefaultTimeout {
		t.Fatalf("timeout after WithHTTPClient(nil) = %v, want DefaultTimeout (%v)", got, DefaultTimeout)
	}
}

// A nil client must not discard an explicitly configured timeout either,
// whichever order the two options are supplied in.
func TestWithHTTPClientNilPreservesExplicitTimeout(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []ClientOption
	}{
		{"nil then timeout", []ClientOption{WithHTTPClient(nil), WithTimeout(7 * time.Second)}},
		{"timeout then nil", []ClientOption{WithTimeout(7 * time.Second), WithHTTPClient(nil)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClient("test-key", tc.opts...)
			if client.httpClient == nil {
				t.Fatal("WithHTTPClient(nil) left the client with a nil *http.Client")
			}
			if got := client.httpClient.Timeout; got != 7*time.Second {
				t.Fatalf("timeout = %v, want 7s", got)
			}
		})
	}
}
