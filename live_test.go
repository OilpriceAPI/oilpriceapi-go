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
	if len(resp.Data.Contracts) == 0 {
		t.Fatal("expected at least one futures contract for ice-brent")
	}

	// Sanity-check the front contract price is in a plausible Brent range.
	price := resp.Data.Contracts[0].Price
	if price <= 0 || price > 1000 {
		t.Errorf("front Brent contract price %.2f is outside a sane range", price)
	}
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
	if resp == nil || len(resp.Data.Contracts) == 0 {
		t.Fatal("expected at least one futures contract for CL (ice-wti)")
	}
	if p := resp.Data.Contracts[0].Price; p <= 0 || p > 1000 {
		t.Errorf("front WTI contract price %.2f is outside a sane range", p)
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
}
