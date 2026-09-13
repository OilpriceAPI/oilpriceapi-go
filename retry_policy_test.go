package oilpriceapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// okPricesBody is a minimal valid /v1/prices/latest success payload.
const okPricesBody = `{"status":"success","data":{"prices":[{"code":"BRENT_CRUDE_USD","price":82.14,"created_at":"2026-09-13T12:00:00Z"}]}}`

// countingServer returns an httptest server plus a counter of requests seen.
func countingServer(t *testing.T, h func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, *int64) {
	t.Helper()
	var n int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&n, 1)
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

// TestHugeRetryAfterDoesNotBlock is the headline defect: the keyless demo
// endpoint answers 429 with a Retry-After counting down to the next daily
// reset (28,197s = 7.8h when measured on 2026-09-13). A caller using
// context.Background() has no deadline, so the client slept for the whole of
// it.
//
// The assertion is on observable behaviour through the real client — the call
// returns promptly with the typed limit error — not on an internal variable.
func TestHugeRetryAfterDoesNotBlock(t *testing.T) {
	const retryAfter = 28197

	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", strconvItoa(retryAfter))
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limit exceeded"}`))
	})

	client := NewClient("k", WithBaseURL(srv.URL))

	done := make(chan error, 1)
	start := time.Now()
	go func() {
		_, err := client.GetLatestPrices(context.Background())
		done <- err
	}()

	select {
	case err := <-done:
		elapsed := time.Since(start)
		if elapsed > 2*time.Second {
			t.Fatalf("blocked %v on a Retry-After of %ds; want a prompt return", elapsed, retryAfter)
		}
		var limit *RateLimitError
		if !errors.As(err, &limit) {
			t.Fatalf("got %T (%v), want *RateLimitError", err, err)
		}
		if limit.RetryAfter != retryAfter {
			t.Errorf("RateLimitError.RetryAfter = %d, want %d", limit.RetryAfter, retryAfter)
		}
		if got := atomic.LoadInt64(calls); got != 1 {
			t.Errorf("sent %d requests, want 1", got)
		}
		t.Logf("returned in %v with %v", elapsed, err)
	case <-time.After(10 * time.Second):
		t.Fatalf("still blocked after 10s: Retry-After %ds (%.2f hours) was honoured uncapped; requests sent=%d",
			retryAfter, float64(retryAfter)/3600, atomic.LoadInt64(calls))
	}
}

// TestRetryAfterWithinBudgetIsHonoured guards against over-correcting: a wait
// the caller's budget can absorb is still waited out and the retry succeeds.
func TestRetryAfterWithinBudgetIsHonoured(t *testing.T) {
	var served int64
	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt64(&served, 1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(okPricesBody))
	})

	client := NewClient("k", WithBaseURL(srv.URL), WithMaxRetryWait(5*time.Second))

	start := time.Now()
	if _, err := client.GetLatestPrices(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		t.Errorf("returned after %v; the 1s Retry-After was not honoured", elapsed)
	}
	if got := atomic.LoadInt64(calls); got != 2 {
		t.Errorf("sent %d requests, want 2", got)
	}
}

// TestNegativeRetryAfterDoesNotCollapseBackoff: strconv.Atoi("-3600")
// succeeded and produced a negative delay, so time.After fired immediately and
// the client hammered the server as fast as it could answer.
func TestNegativeRetryAfterDoesNotCollapseBackoff(t *testing.T) {
	for _, header := range []string{"-3600", "0", "not-a-number", ""} {
		t.Run("Retry-After="+header, func(t *testing.T) {
			srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
				if header != "" {
					w.Header().Set("Retry-After", header)
				}
				w.WriteHeader(http.StatusTooManyRequests)
			})

			client := NewClient("k", WithBaseURL(srv.URL), WithRetries(1))

			start := time.Now()
			_, _ = client.GetLatestPrices(context.Background())
			elapsed := time.Since(start)

			if got := atomic.LoadInt64(calls); got != 2 {
				t.Fatalf("sent %d requests, want 2", got)
			}
			if elapsed < 900*time.Millisecond {
				t.Fatalf("retried after %v; backoff collapsed to zero (hot retry loop)", elapsed)
			}
		})
	}
}

// TestRetryAfterHTTPDate: RFC 7231 permits an HTTP-date. It must be read, and
// bounded by the same budget.
func TestRetryAfterHTTPDate(t *testing.T) {
	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", time.Now().Add(6*time.Hour).UTC().Format(http.TimeFormat))
		w.WriteHeader(http.StatusTooManyRequests)
	})

	client := NewClient("k", WithBaseURL(srv.URL))

	start := time.Now()
	_, err := client.GetLatestPrices(context.Background())
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("blocked %v on a 6-hour HTTP-date Retry-After", elapsed)
	}
	var limit *RateLimitError
	if !errors.As(err, &limit) {
		t.Fatalf("got %T (%v), want *RateLimitError", err, err)
	}
	if limit.RetryAfter < 6*3600-60 || limit.RetryAfter > 6*3600+60 {
		t.Errorf("RetryAfter = %ds, want ~%ds from the HTTP-date", limit.RetryAfter, 6*3600)
	}
	if got := atomic.LoadInt64(calls); got != 1 {
		t.Errorf("sent %d requests, want 1", got)
	}
}

// TestOverflowRetryAfterDoesNotWrap: seconds * time.Second overflows int64
// well below math.MaxInt, which could produce a negative duration.
func TestOverflowRetryAfterDoesNotWrap(t *testing.T) {
	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "9223372036854775807")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	client := NewClient("k", WithBaseURL(srv.URL))

	start := time.Now()
	_, err := client.GetLatestPrices(context.Background())
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("blocked %v on an overflowing Retry-After", elapsed)
	}
	var limit *RateLimitError
	if !errors.As(err, &limit) {
		t.Fatalf("got %T (%v), want *RateLimitError", err, err)
	}
	if got := atomic.LoadInt64(calls); got != 1 {
		t.Errorf("sent %d requests, want 1", got)
	}
}

// TestDurableQuotaIsNotRetried: a monthly/daily allowance that is spent will
// not refill inside any retry budget. Retrying burns the caller's own time and
// adds load for a guaranteed 429.
func TestDurableQuotaIsNotRetried(t *testing.T) {
	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Remaining-Day", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"Daily quota exceeded"}`))
	})

	client := NewClient("k", WithBaseURL(srv.URL))

	start := time.Now()
	_, err := client.GetLatestPrices(context.Background())
	if got := atomic.LoadInt64(calls); got != 1 {
		t.Fatalf("durable quota 429 sent %d requests, want 1", got)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("slept %v before giving up on a durable quota", elapsed)
	}
	var limit *RateLimitError
	if !errors.As(err, &limit) {
		t.Fatalf("got %T (%v), want *RateLimitError", err, err)
	}
}

// TestBurstRateLimitIsStillRetried: the recoverable case must keep working.
func TestBurstRateLimitIsStillRetried(t *testing.T) {
	var served int64
	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt64(&served, 1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Remaining-Day", "4000")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(okPricesBody))
	})

	client := NewClient("k", WithBaseURL(srv.URL))
	if _, err := client.GetLatestPrices(context.Background()); err != nil {
		t.Fatalf("burst 429 was not retried: %v", err)
	}
	if got := atomic.LoadInt64(calls); got != 2 {
		t.Errorf("sent %d requests, want 2", got)
	}
}

// TestNonIdempotentWriteNotReplayedOn503: the server may have committed before
// returning 503. Replaying the POST creates a duplicate subscription.
func TestNonIdempotentWriteNotReplayedOn503(t *testing.T) {
	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":"upstream unavailable"}`))
	})

	client := NewClient("k", WithBaseURL(srv.URL))
	_, err := client.CreateSubscription(context.Background(), SubscriptionInput{
		Name: "Crude watch", Codes: []string{"BRENT_CRUDE_USD"}, IntervalSeconds: 300,
	})
	if got := atomic.LoadInt64(calls); got != 1 {
		t.Fatalf("POST /v1/subscriptions sent %d times on 503, want 1", got)
	}
	var serverErr *ServerError
	if !errors.As(err, &serverErr) {
		t.Fatalf("got %T (%v), want *ServerError", err, err)
	}
}

// TestNonIdempotentWriteNotReplayedOnTransportError: the ambiguous case. The
// server accepted and committed the request, then the connection died before
// the response arrived. A replay duplicates the write.
func TestNonIdempotentWriteNotReplayedOnTransportError(t *testing.T) {
	var n int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&n, 1)
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("ResponseWriter is not a Hijacker")
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		conn.Close() // committed server-side, response lost in transit
	}))
	defer srv.Close()

	client := NewClient("k", WithBaseURL(srv.URL), WithRetries(2))
	_, err := client.CreateWebhook(context.Background(), WebhookCreateInput{URL: "https://x.example/hook"})
	if got := atomic.LoadInt64(&n); got != 1 {
		t.Fatalf("POST /v1/webhooks sent %d times after an ambiguous transport failure, want 1", got)
	}
	if err == nil {
		t.Fatal("expected a transport error")
	}
}

// TestSafeRequestStillRetriesOnTransportError: GET is safe; the retry that
// makes the SDK resilient must survive.
func TestSafeRequestStillRetriesOnTransportError(t *testing.T) {
	var n int64
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt64(&n, 1) == 1 {
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
		w.Write([]byte(okPricesBody))
	}))
	defer srv.Close()

	client := NewClient("k", WithBaseURL(srv.URL), WithRetries(2))
	if _, err := client.GetLatestPrices(context.Background()); err != nil {
		t.Fatalf("safe GET was not retried through a transport error: %v", err)
	}
	if got := atomic.LoadInt64(&n); got != 2 {
		t.Errorf("sent %d requests, want 2", got)
	}
}

// TestContextCancellationInterruptsRetryWait: existing behaviour that must not
// regress.
func TestContextCancellationInterruptsRetryWait(t *testing.T) {
	srv, _ := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	client := NewClient("k", WithBaseURL(srv.URL), WithMaxRetryWait(time.Minute))

	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(150 * time.Millisecond); cancel() }()

	start := time.Now()
	_, err := client.GetLatestPrices(ctx)
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("cancellation did not interrupt the retry wait: %v", elapsed)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

// TestRetryWaitBoundedByContextDeadline: sleeping past the caller's own
// deadline guarantees a wasted wait and a context error instead of the real
// reason.
func TestRetryWaitBoundedByContextDeadline(t *testing.T) {
	srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	client := NewClient("k", WithBaseURL(srv.URL), WithMaxRetryWait(time.Minute))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	_, err := client.GetLatestPrices(ctx)
	elapsed := time.Since(start)
	if elapsed > time.Second {
		t.Fatalf("waited %v against a 2s deadline for a 30s Retry-After", elapsed)
	}
	var limit *RateLimitError
	if !errors.As(err, &limit) {
		t.Fatalf("got %T (%v), want *RateLimitError reporting the real reason", err, err)
	}
	if got := atomic.LoadInt64(calls); got != 1 {
		t.Errorf("sent %d requests, want 1", got)
	}
}

// TestRetryConfiguration covers the settings being honoured, and an invalid
// setting being rejected rather than silently disabling the client.
func TestRetryConfiguration(t *testing.T) {
	t.Run("zero retries sends exactly one request", func(t *testing.T) {
		srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		})
		client := NewClient("k", WithBaseURL(srv.URL), WithRetries(0))
		_, _ = client.GetLatestPrices(context.Background())
		if got := atomic.LoadInt64(calls); got != 1 {
			t.Errorf("sent %d requests, want 1", got)
		}
	})

	t.Run("negative retries is rejected, not silently fatal", func(t *testing.T) {
		srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(okPricesBody))
		})
		client := NewClient("k", WithBaseURL(srv.URL), WithRetries(-1))
		_, err := client.GetLatestPrices(context.Background())

		var cfg *ConfigurationError
		if !errors.As(err, &cfg) {
			t.Fatalf("got %T (%v), want *ConfigurationError", err, err)
		}
		if got := atomic.LoadInt64(calls); got != 0 {
			t.Errorf("sent %d requests on an invalid configuration, want 0", got)
		}
	})

	t.Run("explicit retry count is honoured", func(t *testing.T) {
		srv, calls := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("X-RateLimit-Remaining-Day", "100")
			w.WriteHeader(http.StatusTooManyRequests)
		})
		client := NewClient("k", WithBaseURL(srv.URL), WithRetries(2))
		_, _ = client.GetLatestPrices(context.Background())
		if got := atomic.LoadInt64(calls); got != 3 {
			t.Errorf("sent %d requests with WithRetries(2), want 3", got)
		}
	})
}

func strconvItoa(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{digits[i%10]}, b...)
		i /= 10
	}
	return string(b)
}
