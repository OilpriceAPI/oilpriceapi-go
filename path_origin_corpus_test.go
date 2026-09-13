package oilpriceapi

import (
	"net/url"
	"strings"
	"testing"
)

// TestResolveEndpointOriginCorpus regenerates a large hostile-path corpus and
// asserts the one property that matters: every path either is rejected before
// a request is built, or resolves to the configured base origin.
//
// It exists because this PR relaxes the character scan (spaces are now escaped
// rather than refused) and adds a decoded ".." check. Relaxing an input filter
// is exactly the change that reopens an origin hole, so the guard is
// re-measured over generated forms rather than over the hand-written probe
// list alone.
func TestResolveEndpointOriginCorpus(t *testing.T) {
	const base = "https://api.oilpriceapi.com"
	client := NewClient(fixtureKey, WithBaseURL(base))

	baseURL, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse base: %v", err)
	}

	leaders := []string{
		"", "/", "//", "///", "\\", "\\\\", "/\\", "\\/", " ", "  ", " /", "/ ", "\t", ".", "..", "/..", "/../",
		"%2f", "%2F", "%2f%2f", "%5c", "/%2f", "@", ":", ";", "/;", "//;", "/.", "/./", "?", "#",
	}
	authorities := []string{
		"fixture.invalid", "user@fixture.invalid", "fixture.invalid:8443", "[::1]:9",
		"127.0.0.1", "api.oilpriceapi.com.fixture.invalid", "FIXTURE.INVALID",
		"@fixture.invalid", "user:pass@fixture.invalid", "%66ixture.invalid",
		"fixture%2Einvalid", "fixture.invalid#", "fixture.invalid?", "localhost:1",
		"0x7f.0.0.1", "fixture .invalid", "fixture .invalid",
	}
	schemes := []string{
		"", "http:", "https:", "HTTP:", "hTTps:", "javascript:", "file:", "//", "http:/", "https:/",
		"http:\\\\", "%68ttp:", "http%3a//",
	}
	tails := []string{
		"", "/v1/prices", "/v1/prices/latest", "?by_code=BRENT", "#frag", "/../../v1", "/%2e%2e/v1",
		"/my alert id", "/BRENT%2FWTI",
	}

	checked := 0
	for _, lead := range leaders {
		for _, scheme := range schemes {
			for _, authority := range authorities {
				for _, tail := range tails {
					path := lead + scheme + authority + tail
					checked++

					resolved, err := client.resolveEndpoint(path)
					if err != nil {
						continue // rejected at the boundary: safe
					}

					got, parseErr := url.Parse(resolved)
					if parseErr != nil {
						t.Fatalf("accepted path %q produced an unparseable URL %q: %v", path, resolved, parseErr)
					}
					if !sameOrigin(baseURL, got) {
						t.Fatalf("accepted path %q resolved off-origin to %q", path, resolved)
					}
					if !strings.HasPrefix(resolved, base+"/") {
						t.Fatalf("accepted path %q did not stay under the base path: %q", path, resolved)
					}
				}
			}
		}
	}

	t.Logf("checked %d generated hostile path forms; every one was rejected or stayed on the base origin", checked)
}
