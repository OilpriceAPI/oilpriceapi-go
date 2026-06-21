//go:build live

// Package oilpriceapi live tests hit the REAL production API.
//
// They are excluded from the default `go test` run via the `live` build tag.
// Run them explicitly with:
//
//	OILPRICEAPI_TEST_KEY=<key> go test -tags live ./...
//
// When OILPRICEAPI_TEST_KEY is not set the tests skip, so CI on forks (which
// have no secret) does not fail. The production API is rate limited to roughly
// 1 request/second, so these tests space their calls and keep the total number
// of requests small.
package oilpriceapi

import (
	"context"
	"os"
	"testing"
	"time"
)

// liveClient builds a client against the real API or skips the test when no
// key is configured.
func liveClient(t *testing.T) *Client {
	t.Helper()
	key := os.Getenv("OILPRICEAPI_TEST_KEY")
	if key == "" {
		t.Skip("OILPRICEAPI_TEST_KEY not set; skipping live API test")
	}
	return NewClient(key, WithTimeout(30*time.Second))
}

// liveRateLimit spaces calls to respect the ~1 req/sec production rate limit.
func liveRateLimit() {
	time.Sleep(1100 * time.Millisecond)
}

// TestLiveGetFuturesLatest verifies that the corrected /v1/futures/{slug} path
// returns a 200 with a sane Brent price. This is the regression guard for the
// v1.1.0 bug where the SDK hit /v1/futures/latest?contract=BZ (404).
func TestLiveGetFuturesLatest(t *testing.T) {
	client := liveClient(t)
	ctx := context.Background()

	resp, err := client.GetFuturesLatest(ctx, WithContract("ice-brent"))
	if err != nil {
		t.Fatalf("GetFuturesLatest(ice-brent) returned error (path regression?): %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	// The latest settlement is exposed at front_month.last_price. Accept either
	// the front_month object or the first listed contract as the price source.
	price := frontMonthPrice(resp)
	if price <= 0 || price > 1000 {
		t.Errorf("front Brent price %.2f is outside a sane range", price)
	}
}

// frontMonthPrice extracts the front-month settlement price from a futures
// response, preferring the front_month object and falling back to the first
// listed contract.
func frontMonthPrice(resp *FuturesResponse) float64 {
	if resp == nil {
		return 0
	}
	if resp.FrontMonth != nil && resp.FrontMonth.LastPrice != 0 {
		return resp.FrontMonth.LastPrice
	}
	if len(resp.Contracts) > 0 {
		return resp.Contracts[0].LastPrice
	}
	return 0
}

// TestLiveGetFuturesLatestByCode verifies the contract-code -> slug mapping
// works end to end against the real API (CL -> ice-wti).
func TestLiveGetFuturesLatestByCode(t *testing.T) {
	client := liveClient(t)
	ctx := context.Background()

	liveRateLimit()

	resp, err := client.GetFuturesLatest(ctx, WithContract("CL"))
	if err != nil {
		t.Fatalf("GetFuturesLatest(CL) returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response for CL (ice-wti)")
	}
	if p := frontMonthPrice(resp); p <= 0 || p > 1000 {
		t.Errorf("front WTI price %.2f is outside a sane range", p)
	}
}

// TestLiveGetFuturesCurve verifies the corrected /v1/futures/{slug}/curve path.
func TestLiveGetFuturesCurve(t *testing.T) {
	client := liveClient(t)
	ctx := context.Background()

	liveRateLimit()

	resp, err := client.GetFuturesCurve(ctx, WithContract("ice-brent"))
	if err != nil {
		t.Fatalf("GetFuturesCurve(ice-brent) returned error (path regression?): %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil curve response")
	}

	// The curve endpoint legitimately returns either curve data (contracts)
	// or a documented no-data response, e.g.
	// {"error":"No futures data available for curve analysis","date":...}.
	// Both are valid: accept either and only fail on an unexpected empty shape.
	if resp.Error != "" {
		t.Logf("curve returned documented no-data response: %q (date=%s)", resp.Error, resp.Date)
		return
	}
	if len(resp.Contracts) == 0 && frontMonthPrice(resp) == 0 {
		t.Error("curve returned neither contract data nor a documented no-data error")
	}
}
