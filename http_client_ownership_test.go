package oilpriceapi

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// WithTimeout wrote through to Client.httpClient.Timeout. When the caller had
// supplied their own *http.Client, that pointer was theirs, so the option
// reached out of the SDK and reconfigured an object the SDK does not own —
// including every other client sharing it.
//
// The contract these tests fix: the SDK owns its own *http.Client. A supplied
// client is copied, its Transport/Jar/CheckRedirect are kept (that is what the
// caller supplied it for), and nothing the SDK does is visible to the caller
// or to a sibling client.

func TestWithTimeoutDoesNotMutateCallerClient(t *testing.T) {
	caller := &http.Client{Timeout: 30 * time.Second}

	_ = NewClient("test-key", WithHTTPClient(caller), WithTimeout(1*time.Nanosecond))

	if caller.Timeout != 30*time.Second {
		t.Fatalf("caller's own *http.Client was mutated: Timeout = %v, want 30s", caller.Timeout)
	}
}

// The reported symptom: two clients built over one shared *http.Client, and a
// WithTimeout on the first silently became the timeout of the second.
func TestWithTimeoutDoesNotContaminateSiblingClient(t *testing.T) {
	shared := &http.Client{Timeout: 30 * time.Second}

	_ = NewClient("key-one", WithHTTPClient(shared), WithTimeout(1*time.Nanosecond))
	sibling := NewClient("key-two", WithHTTPClient(shared))

	if got := sibling.httpClient.Timeout; got != 30*time.Second {
		t.Fatalf("sibling client's timeout was contaminated: got %v, want 30s", got)
	}
}

// The same write is a genuine data race: one goroutine issuing requests
// through the shared client reads Client.Timeout inside net/http while another
// goroutine constructs an SDK client with WithTimeout and writes it.
func TestWithTimeoutRaceOnSharedHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"prices":[{"code":"BRENT_CRUDE_USD","name":"Brent Crude Oil","price":75.42,"currency":"USD","unit":"barrel","updated_at":"2024-01-10T12:00:00Z"}]}}`))
	}))
	defer server.Close()

	shared := &http.Client{Timeout: 30 * time.Second}
	inFlight := NewClient("key-one", WithHTTPClient(shared), WithBaseURL(server.URL))

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			if _, err := inFlight.GetLatestPrices(context.Background()); err != nil {
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = NewClient("key-two", WithHTTPClient(shared), WithTimeout(time.Duration(i+1)*time.Second))
		}
	}()

	wg.Wait()
}

// Copying must be shallow: the Transport is the reason a caller supplies a
// client at all (connection pooling, proxies, TLS config), and so are the
// cookie jar and redirect policy.
func TestSuppliedHTTPClientRetainsTransportJarAndRedirectPolicy(t *testing.T) {
	transport := &http.Transport{MaxIdleConns: 17}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	redirect := func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }

	caller := &http.Client{Transport: transport, Jar: jar, CheckRedirect: redirect, Timeout: 12 * time.Second}
	client := NewClient("test-key", WithHTTPClient(caller))

	if client.httpClient == caller {
		t.Fatal("SDK kept the caller's *http.Client pointer instead of its own copy")
	}
	if client.httpClient.Transport != transport {
		t.Error("supplied Transport was not carried over to the SDK's copy")
	}
	if client.httpClient.Jar != jar {
		t.Error("supplied cookie jar was not carried over to the SDK's copy")
	}
	if client.httpClient.CheckRedirect == nil {
		t.Error("supplied CheckRedirect was not carried over to the SDK's copy")
	}
}

// Without an explicit WithTimeout, a supplied client keeps its own timeout —
// the SDK must not impose DefaultTimeout on a client the caller configured.
func TestSuppliedHTTPClientKeepsItsOwnTimeout(t *testing.T) {
	caller := &http.Client{Timeout: 12 * time.Second}
	client := NewClient("test-key", WithHTTPClient(caller))

	if got := client.httpClient.Timeout; got != 12*time.Second {
		t.Fatalf("timeout = %v, want the supplied client's 12s", got)
	}
}

// An explicit WithTimeout wins over the supplied client's timeout, in either
// option order. Before this change the result depended on which option ran
// last.
func TestExplicitTimeoutWinsInEitherOptionOrder(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts func(*http.Client) []ClientOption
	}{
		{"client then timeout", func(hc *http.Client) []ClientOption {
			return []ClientOption{WithHTTPClient(hc), WithTimeout(5 * time.Second)}
		}},
		{"timeout then client", func(hc *http.Client) []ClientOption {
			return []ClientOption{WithTimeout(5 * time.Second), WithHTTPClient(hc)}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caller := &http.Client{Timeout: 30 * time.Second}
			client := NewClient("test-key", tc.opts(caller)...)

			if got := client.httpClient.Timeout; got != 5*time.Second {
				t.Fatalf("timeout = %v, want the explicit 5s", got)
			}
			if caller.Timeout != 30*time.Second {
				t.Fatalf("caller's client was mutated: %v", caller.Timeout)
			}
		})
	}
}

// WithTimeout(0) means "no timeout" in net/http and must keep meaning that,
// rather than being read as "unset" and silently restored to DefaultTimeout.
func TestWithTimeoutZeroMeansNoTimeout(t *testing.T) {
	client := NewClient("test-key", WithTimeout(0))

	if got := client.httpClient.Timeout; got != 0 {
		t.Fatalf("timeout = %v, want 0 (no timeout)", got)
	}
}
