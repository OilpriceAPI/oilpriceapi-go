package oilpriceapi

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// This file holds the single retry classifier used by every request the SDK
// makes. Three rules, in order:
//
//  1. Only safe requests are retried automatically. A retried POST or PATCH is
//     a duplicate write whenever the server committed and the response was
//     lost, and the API offers no idempotency key to make that safe.
//  2. Only recoverable rate limits are retried. A spent daily or monthly
//     allowance does not refill inside any retry budget.
//  3. Every wait is bounded — by the configured budget and by the caller's own
//     deadline. Retry-After is chosen by the server and can be hours.

// isRetryableMethod reports whether a request may be replayed automatically.
//
// Safe methods (RFC 9110 §9.2.1) have no intended effect on server state, so a
// duplicate is harmless. Everything else — POST, PUT, PATCH, DELETE — is
// returned to the caller, who knows whether a replay is acceptable.
//
// PUT and DELETE are idempotent by definition but are deliberately excluded:
// idempotent means a repeat has the same effect on the resource, not that the
// caller sees the same outcome, and a replayed DELETE can remove a resource
// recreated in between.
func isRetryableMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace, "":
		return true
	default:
		return false
	}
}

// isRetryableStatus reports whether a response status is worth another attempt
// at all, before any budget is considered.
func isRetryableStatus(resp *http.Response) bool {
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return !isDurableQuotaExhaustion(resp)
	case resp.StatusCode >= 500:
		return true
	default:
		return false
	}
}

// isDurableQuotaExhaustion distinguishes a burst limit (a short window that
// refills on its own) from a spent plan allowance.
//
// OilPriceAPI reports both on a 429: X-RateLimit-Remaining is the per-minute
// burst window, while X-RateLimit-Remaining-Day and -Month track the plan
// allowance. A zero on a day or month counter means no amount of waiting
// inside a request will help, so the typed *RateLimitError is returned
// immediately and the caller can surface an upgrade path instead of stalling.
func isDurableQuotaExhaustion(resp *http.Response) bool {
	for _, header := range []string{"X-RateLimit-Remaining-Day", "X-RateLimit-Remaining-Month"} {
		value := strings.TrimSpace(resp.Header.Get(header))
		if value == "" {
			continue
		}
		if remaining, err := strconv.Atoi(value); err == nil && remaining <= 0 {
			return true
		}
	}
	return false
}

// retryAfterSeconds parses a Retry-After header into whole seconds.
//
// Both RFC 9110 forms are accepted: delta-seconds and HTTP-date. It returns
// ok=false for an absent, unparseable, negative or past value, in which case
// the caller falls back to exponential backoff rather than to a zero or
// negative delay — strconv.Atoi("-3600") previously succeeded and produced a
// negative duration, which time.After fires on immediately, turning backoff
// into a hot loop.
//
// The result is clamped to maxRetryAfterSeconds so that converting it to a
// time.Duration cannot overflow int64 nanoseconds and wrap negative.
func retryAfterSeconds(resp *http.Response) (int, bool) {
	raw := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if raw == "" {
		return 0, false
	}

	if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if seconds <= 0 {
			return 0, false
		}
		return clampSeconds(seconds), true
	}

	if when, err := http.ParseTime(raw); err == nil {
		seconds := int64(math.Ceil(time.Until(when).Seconds()))
		if seconds <= 0 {
			return 0, false
		}
		return clampSeconds(seconds), true
	}

	return 0, false
}

// maxRetryAfterSeconds is one year: far beyond any budget, and small enough
// that seconds * time.Second stays well inside int64 nanoseconds.
const maxRetryAfterSeconds = 365 * 24 * 60 * 60

func clampSeconds(seconds int64) int {
	if seconds > maxRetryAfterSeconds {
		return maxRetryAfterSeconds
	}
	return int(seconds)
}

// backoffFor returns the delay before the next attempt: the server's
// Retry-After when it gave a usable one, otherwise exponential backoff from
// one second.
func backoffFor(resp *http.Response, attempt int) time.Duration {
	if seconds, ok := retryAfterSeconds(resp); ok {
		return time.Duration(seconds) * time.Second
	}
	return exponentialBackoff(attempt)
}

func exponentialBackoff(attempt int) time.Duration {
	if attempt > 20 {
		attempt = 20
	}
	return time.Duration(1<<uint(attempt)) * time.Second
}
