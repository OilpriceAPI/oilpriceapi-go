package oilpriceapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// The path contract had drifted in two directions at once.
//
// Over-rejection: the character scan refused the space character outright, so
// "/v1/alerts/my alert id" — a path that previously worked, escaped to %20 —
// came back as *InvalidPathError. A space in a path segment cannot introduce
// an authority; it only needs escaping.
//
// Under-rejection: the ".." segment check ran on the raw string only, so
// "/v1/%2e%2e/%2e%2e/admin" and "/v1/..%2fadmin" passed it and went out
// verbatim. Both stay on the configured origin, so neither is a credential
// leak — but a path the SDK refuses in one spelling and forwards in another
// is not a contract.

// --- over-rejection: spaces must be escaped, not refused ---------------------

func TestResolveEndpointEscapesSpacesInPath(t *testing.T) {
	client := NewClient(fixtureKey)

	got, err := client.resolveEndpoint("/v1/alerts/my alert id")
	if err != nil {
		t.Fatalf("space in a path segment was rejected: %v", err)
	}
	if want := DefaultBaseURL + "/v1/alerts/my%20alert%20id"; got != want {
		t.Fatalf("resolveEndpoint = %q, want %q", got, want)
	}
}

func TestResolveEndpointEscapesSpacesInQuery(t *testing.T) {
	client := NewClient(fixtureKey)

	got, err := client.resolveEndpoint("/v1/prices/latest?by_code=BRENT CRUDE")
	if err != nil {
		t.Fatalf("space in a query value was rejected: %v", err)
	}
	if want := DefaultBaseURL + "/v1/prices/latest?by_code=BRENT%20CRUDE"; got != want {
		t.Fatalf("resolveEndpoint = %q, want %q", got, want)
	}
}

// On the wire: the request must be built, addressed at the base origin, and
// carry the space as %20 in the escaped path.
func TestRawSendsSpacedPathEscapedToBaseOrigin(t *testing.T) {
	rt := &originProbe{}
	client := NewClient(fixtureKey, WithHTTPClient(&http.Client{Transport: rt}))

	if err := client.Raw(context.Background(), http.MethodGet, "/v1/alerts/my alert id", nil, nil); err != nil {
		t.Fatalf("Raw with a spaced path returned %v", err)
	}
	if len(rt.calls) != 1 {
		t.Fatalf("got %d requests, want 1", len(rt.calls))
	}

	call := rt.calls[0]
	if call.URL.Hostname() != "api.oilpriceapi.com" {
		t.Fatalf("request addressed %q, want the base origin", call.URL.Host)
	}
	if got := call.URL.EscapedPath(); got != "/v1/alerts/my%20alert%20id" {
		t.Fatalf("escaped path = %q, want %q", got, "/v1/alerts/my%20alert%20id")
	}
	if got := call.URL.Path; got != "/v1/alerts/my alert id" {
		t.Fatalf("decoded path = %q, want %q", got, "/v1/alerts/my alert id")
	}
}

// --- under-rejection: percent-encoded traversal must be refused --------------

func TestResolveEndpointRejectsPercentEncodedTraversal(t *testing.T) {
	client := NewClient(fixtureKey)

	for _, path := range []string{
		"/v1/%2e%2e/%2e%2e/admin",
		"/v1/%2E%2E/admin",
		"/v1/..%2fadmin",
		"/v1/..%2Fadmin",
		"/v1/%2e%2e%2fadmin",
	} {
		t.Run(path, func(t *testing.T) {
			got, err := client.resolveEndpoint(path)
			if err == nil {
				t.Fatalf("percent-encoded traversal accepted: resolveEndpoint(%q) = %q", path, got)
			}
			var invalid *InvalidPathError
			if !errors.As(err, &invalid) {
				t.Fatalf("rejected with %T (%v), want *InvalidPathError", err, err)
			}
			if strings.Contains(err.Error(), fixtureKey) {
				t.Fatalf("error message leaked the API key")
			}
		})
	}
}

func TestResolveEndpointRejectsMalformedPercentEscape(t *testing.T) {
	client := NewClient(fixtureKey)

	if got, err := client.resolveEndpoint("/v1/%zz/prices"); err == nil {
		t.Fatalf("malformed percent-escape accepted: %q", got)
	}
}

// --- the guards that must not move ------------------------------------------

// Control characters stay rejected: CR, LF and NUL in a request line are a
// header-injection vector, not a path that needs escaping.
func TestResolveEndpointStillRejectsControlCharacters(t *testing.T) {
	client := NewClient(fixtureKey)

	for _, path := range []string{
		"/v1/prices\r\nX-Injected: 1",
		"/v1/prices\nlatest",
		"/v1/prices\tlatest",
		"/v1/prices\x00",
		"/v1/prices\x7f",
	} {
		t.Run(strings.ReplaceAll(path, "\x00", "NUL"), func(t *testing.T) {
			if got, err := client.resolveEndpoint(path); err == nil {
				t.Fatalf("control character accepted: %q", got)
			}
		})
	}
}

// A leading space cannot smuggle an authority past the "starts with /" check.
func TestResolveEndpointStillRejectsSpacePaddedAuthority(t *testing.T) {
	client := NewClient(fixtureKey)

	for _, path := range []string{
		"  //fixture.invalid/v1/prices",
		" http://fixture.invalid/v1/prices",
		"/v1/prices //fixture.invalid",
	} {
		t.Run(path, func(t *testing.T) {
			got, err := client.resolveEndpoint(path)
			if err != nil {
				return
			}
			if !strings.HasPrefix(got, DefaultBaseURL+"/") {
				t.Fatalf("path %q resolved off-origin: %q", path, got)
			}
		})
	}
}

// An encoded slash inside a segment is legitimate data (a commodity code that
// contains "/") and must keep working — it is not traversal.
func TestResolveEndpointStillAllowsEncodedSlashInSegment(t *testing.T) {
	client := NewClient(fixtureKey)

	got, err := client.resolveEndpoint("/v1/prices/BRENT%2FWTI")
	if err != nil {
		t.Fatalf("encoded slash in a segment was rejected: %v", err)
	}
	if want := DefaultBaseURL + "/v1/prices/BRENT%2FWTI"; got != want {
		t.Fatalf("resolveEndpoint = %q, want %q", got, want)
	}
}

// A dot that is not a traversal segment stays allowed.
func TestResolveEndpointStillAllowsEncodedDotsInSegment(t *testing.T) {
	client := NewClient(fixtureKey)

	for _, path := range []string{"/v1/prices/BRENT%2eWTI", "/v1/prices/..wti", "/v1/prices/wti.."} {
		t.Run(path, func(t *testing.T) {
			if _, err := client.resolveEndpoint(path); err != nil {
				t.Fatalf("legitimate dotted segment rejected: %v", err)
			}
		})
	}
}
