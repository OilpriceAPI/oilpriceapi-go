package oilpriceapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// fixtureKey is a non-secret placeholder. It must never reach a host other
// than the configured base origin.
const fixtureKey = "FIXTURE-KEY-NOT-A-REAL-CREDENTIAL"

// originProbe records where a request was actually addressed and which
// credentials rode along, so a test can assert on the wire rather than on an
// internal variable.
type originProbe struct {
	calls []*http.Request
}

func (p *originProbe) RoundTrip(r *http.Request) (*http.Response, error) {
	p.calls = append(p.calls, r)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
		Header:     http.Header{},
		Request:    r,
	}, nil
}

// TestRawRejectsOffOriginPaths is the adversarial probe set. Every entry must
// either be rejected before a request is built, or resolve to the configured
// base origin. A probe that reaches a foreign host is a credential leak,
// because setHeaders attaches "Authorization: Token <key>" unconditionally.
func TestRawRejectsOffOriginPaths(t *testing.T) {
	const baseHost = "api.oilpriceapi.com"

	probes := []struct {
		name string
		path string
	}{
		{"scheme-relative", "//fixture.invalid/v1/prices"},
		{"userinfo-embedded", "@fixture.invalid/v1/prices"},
		{"userinfo-with-port", ":8080@fixture.invalid/v1/prices"},
		{"userinfo-host-port", "@fixture.invalid:443/v1/prices"},
		{"userinfo-ipv6", "@[::1]:9/v1/prices"},
		{"userinfo-in-authority", "//user@fixture.invalid/v1/prices"},
		{"host-suffix", ".evil.invalid/v1/prices"},
		{"no-leading-slash", "v1/prices"},
		{"backslash-authority", `\\fixture.invalid/v1/prices`},
		{"backslash-userinfo", `\\@fixture.invalid/v1/prices`},
		{"percent-encoded-slashes", "%2F%2Ffixture.invalid/v1/prices"},
		{"uppercase-scheme", "HTTP://fixture.invalid/v1/prices"},
		{"absolute-url", "http://fixture.invalid/v1/prices"},
		{"scheme-downgrade-own-host", "http://" + baseHost + "/v1/prices"},
		{"whitespace-padded", "  //fixture.invalid/v1/prices"},
		{"traversal-then-authority", "/..//fixture.invalid/v1/prices"},
		{"traversal-above-root", "/../../v1/prices"},
		{"empty", ""},
	}

	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			rt := &originProbe{}
			client := NewClient(fixtureKey, WithHTTPClient(&http.Client{Transport: rt}))

			err := client.Raw(context.Background(), http.MethodGet, probe.path, nil, nil)

			for _, call := range rt.calls {
				if call.URL.Hostname() != baseHost {
					t.Fatalf("off-origin request: path %q reached host %q carrying %q",
						probe.path, call.URL.Host, call.Header.Get("Authorization"))
				}
				if call.URL.Scheme != "https" {
					t.Fatalf("scheme changed: path %q produced scheme %q", probe.path, call.URL.Scheme)
				}
			}

			if len(rt.calls) == 0 && err == nil {
				t.Fatalf("path %q sent nothing but returned no error", probe.path)
			}
			if err != nil {
				var invalid *InvalidPathError
				if !errors.As(err, &invalid) {
					t.Fatalf("path %q rejected with %T (%v), want *InvalidPathError", probe.path, err, err)
				}
				if strings.Contains(err.Error(), fixtureKey) {
					t.Fatalf("path %q leaked the API key into the error message", probe.path)
				}
			}
		})
	}
}

// TestRawAllowsLegitimatePaths guards against over-rejection: ordinary API
// paths, encoded segments, queries and explicit custom base URLs must all
// still work.
func TestRawAllowsLegitimatePaths(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		query    url.Values
		wantPath string
		wantRaw  string
	}{
		{"plain", "/v1/prices/latest", nil, "/v1/prices/latest", "/v1/prices/latest"},
		{"query", "/v1/prices/latest", url.Values{"by_code": {"BRENT_CRUDE_USD"}}, "/v1/prices/latest", "/v1/prices/latest"},
		{"encoded-space", "/v1/pri%20ces/latest", nil, "/v1/pri ces/latest", "/v1/pri%20ces/latest"},
		{"encoded-slash-in-segment", "/v1/prices/BRENT%2FWTI", nil, "/v1/prices/BRENT/WTI", "/v1/prices/BRENT%2FWTI"},
		{"at-sign-inside-path", "/@fixture.invalid/v1/prices", nil, "/@fixture.invalid/v1/prices", "/@fixture.invalid/v1/prices"},
		{"deep-path", "/v1/well-production/wells/42/monthly", nil, "/v1/well-production/wells/42/monthly", "/v1/well-production/wells/42/monthly"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rt := &originProbe{}
			client := NewClient(fixtureKey, WithHTTPClient(&http.Client{Transport: rt}))

			if err := client.Raw(context.Background(), http.MethodGet, tc.path, tc.query, nil); err != nil {
				t.Fatalf("legitimate path %q rejected: %v", tc.path, err)
			}
			if len(rt.calls) != 1 {
				t.Fatalf("expected 1 request for %q, got %d", tc.path, len(rt.calls))
			}
			got := rt.calls[0].URL
			if got.Hostname() != "api.oilpriceapi.com" {
				t.Fatalf("path %q addressed host %q", tc.path, got.Host)
			}
			if got.Path != tc.wantPath {
				t.Errorf("path %q decoded to %q, want %q", tc.path, got.Path, tc.wantPath)
			}
			if got.EscapedPath() != tc.wantRaw {
				t.Errorf("path %q escaped to %q, want %q", tc.path, got.EscapedPath(), tc.wantRaw)
			}
			if rt.calls[0].Header.Get("Authorization") != "Token "+fixtureKey {
				t.Errorf("legitimate path %q lost its Authorization header", tc.path)
			}
		})
	}
}

// TestCustomBaseURLStillWorks: an explicitly configured base URL (proxy, test
// server, on-prem) defines the allowed origin. Its own host is fine; a
// different host is not.
func TestCustomBaseURLStillWorks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := NewClient(fixtureKey, WithBaseURL(server.URL))
	var out map[string]any
	if err := client.Raw(context.Background(), http.MethodGet, "/v1/prices/latest", nil, &out); err != nil {
		t.Fatalf("custom base URL rejected legitimate path: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("unexpected body: %v", out)
	}

	// Trailing slash on the base URL must not turn the path into an authority.
	trailing := NewClient(fixtureKey, WithBaseURL(server.URL+"/"))
	if err := trailing.Raw(context.Background(), http.MethodGet, "/v1/prices/latest", nil, &out); err != nil {
		t.Fatalf("trailing-slash base URL rejected legitimate path: %v", err)
	}

	if err := client.Raw(context.Background(), http.MethodGet, "@fixture.invalid/v1/prices", nil, nil); err == nil {
		t.Fatal("custom base URL still allowed an off-origin path")
	}
}

// TestTypedEndpointsRejectOffOriginIDs: Raw is not the only entry point. Path
// segments interpolated from caller-supplied IDs go through the same gate.
func TestTypedEndpointsRejectOffOriginIDs(t *testing.T) {
	rt := &originProbe{}
	client := NewClient(fixtureKey, WithHTTPClient(&http.Client{Transport: rt}))

	if err := client.DeleteWebhook(context.Background(), "../../../@fixture.invalid/x"); err != nil {
		t.Logf("DeleteWebhook rejected: %v", err)
	}
	for _, call := range rt.calls {
		if call.URL.Hostname() != "api.oilpriceapi.com" {
			t.Fatalf("DeleteWebhook reached host %q with %q", call.URL.Host, call.Header.Get("Authorization"))
		}
	}
}
