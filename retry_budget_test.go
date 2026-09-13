package oilpriceapi

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// WithMaxRetryWait bounded each individual wait, not their sum. With the
// default 3 retries, a server answering Retry-After just under the budget made
// the client block for roughly three times the number the caller configured —
// and under context.Background() there is no deadline to rescue it. The
// option's whole reason for existing (#37) is that a caller must be able to
// put a ceiling on how long a call can sit inside the SDK.
//
// The contract these tests fix: WithMaxRetryWait is the total automatic wait
// budget for one call, across every retry.

// budgetSlack absorbs scheduler jitter and the (sub-millisecond) request time
// against a local httptest server.
const budgetSlack = 400 * time.Millisecond

func TestMaxRetryWaitBoundsTheTotalWaitNotEachOne(t *testing.T) {
	var hits int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	const budget = 1200 * time.Millisecond
	client := NewClient("test-key",
		WithBaseURL(server.URL),
		WithRetries(5),
		WithMaxRetryWait(budget),
	)

	start := time.Now()
	_, err := client.GetLatestPrices(context.Background())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a rate limit error")
	}
	if elapsed > budget+budgetSlack {
		t.Fatalf("total automatic wait was %v against a %v budget (server hit %d times)",
			elapsed, budget, atomic.LoadInt64(&hits))
	}
}

// The same budget must bound the transport-failure retry path, which backs off
// exponentially rather than from Retry-After.
func TestMaxRetryWaitBoundsTheTotalWaitOnTransportFailures(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Accept and hang up without answering: a transport failure the
			// SDK treats as ambiguous and retries.
			_ = conn.Close()
		}
	}()

	const budget = 1500 * time.Millisecond
	client := NewClient("test-key",
		WithBaseURL("http://"+listener.Addr().String()),
		WithRetries(3),
		WithMaxRetryWait(budget),
	)

	start := time.Now()
	_, err = client.GetLatestPrices(context.Background())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a transport error")
	}
	if elapsed > budget+budgetSlack {
		t.Fatalf("total automatic wait on transport failures was %v against a %v budget", elapsed, budget)
	}
}

// The #37 behaviour must survive: a single Retry-After larger than the whole
// budget returns the typed error immediately rather than waiting at all.
func TestSingleWaitLongerThanBudgetStillReturnsImmediately(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClient("test-key",
		WithBaseURL(server.URL),
		WithRetries(3),
		WithMaxRetryWait(2*time.Second),
	)

	start := time.Now()
	_, err := client.GetLatestPrices(context.Background())
	elapsed := time.Since(start)

	var rateLimit *RateLimitError
	if !errors.As(err, &rateLimit) {
		t.Fatalf("got %T (%v), want *RateLimitError", err, err)
	}
	if rateLimit.RetryAfter != 3600 {
		t.Fatalf("RetryAfter = %d, want the server's 3600", rateLimit.RetryAfter)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("waited %v for a Retry-After past the budget; want an immediate return", elapsed)
	}
}

// Budget left over is still spent: a retry that fits must still happen, so the
// fix cannot be "never retry".
func TestRetryStillHappensWithinTheBudget(t *testing.T) {
	var hits int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt64(&hits, 1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"prices":[{"code":"BRENT_CRUDE_USD","name":"Brent Crude Oil","price":75.42,"currency":"USD","unit":"barrel","updated_at":"2024-01-10T12:00:00Z"}]}}`))
	}))
	defer server.Close()

	client := NewClient("test-key",
		WithBaseURL(server.URL),
		WithRetries(3),
		WithMaxRetryWait(5*time.Second),
	)

	if _, err := client.GetLatestPrices(context.Background()); err != nil {
		t.Fatalf("retry within budget failed: %v", err)
	}
	if got := atomic.LoadInt64(&hits); got != 2 {
		t.Fatalf("server hit %d times, want 2 (one 429 then one success)", got)
	}
}
