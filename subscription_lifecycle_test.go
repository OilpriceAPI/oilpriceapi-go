package oilpriceapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// These tests drive the subscription member routes (#31) against an httptest
// server that answers with the bodies production returned on 2026-09-13 for
// the test account: GET/PATCH /v1/subscriptions/:id and
// POST /v1/subscriptions/:id/{pause,resume}.

const lifecycleID = "f72ceac2-8b9a-406a-a57e-90c625785444"

// prodSubscriptionBody is the exact shape production returned for show, update,
// pause and resume — a nested "subscription" with a null last_evaluated_at on
// a watch the evaluator has not yet run.
func prodSubscriptionBody(id, name, status string) string {
	return `{"status":"success","data":{"subscription":{"id":"` + id + `","name":"` + name +
		`","codes":["WTI_USD","BRENT_CRUDE_USD"],"interval_seconds":86400,"status":"` + status +
		`","deliver_webhook":false,"source":"api","tool_name":"sdk-go-probe-31","last_evaluated_at":null,` +
		`"next_run_at":"2026-09-13T19:57:22Z","created_at":"2026-09-13T19:57:20Z"}}}`
}

// Production 404 body for an unknown or foreign subscription id.
const prodNotFoundBody = `{"error":{"code":"NOT_FOUND","message":"Subscription not found","status":404,"request_id":"67161c5d-43d9-476b-8dc3-cd04dd5fed36","docs":"https://docs.oilpriceapi.com#NOT_FOUND"}}`

// Production 422 bodies from PATCH.
const prodInvalidCodesBody = `{"status":"fail","data":{"error":"VALIDATION_ERROR","message":"Codes contains invalid commodity codes: NOT_A_REAL_CODE","details":{"codes":["contains invalid commodity codes: NOT_A_REAL_CODE"]}}}`
const prodIntervalFloorBody = `{"status":"fail","data":{"error":"VALIDATION_ERROR","message":"Interval seconds is below your plan minimum of 60 seconds","details":{"interval_seconds":["is below your plan minimum of 60 seconds"]}}}`

// recorded captures what the fake server saw.
type recorded struct {
	mu      sync.Mutex
	method  string
	path    string
	rawPath string
	body    string
	hits    int32
}

func (r *recorded) capture(req *http.Request) {
	b, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.method = req.Method
	r.path = req.URL.Path
	r.rawPath = req.URL.EscapedPath()
	r.body = string(b)
	atomic.AddInt32(&r.hits, 1)
}

func newLifecycleServer(t *testing.T, rec *recorded, status int, body string) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.capture(r)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	return NewClient("test-key", WithBaseURL(server.URL), WithRetries(3))
}

// ---------------------------------------------------------------------------
// Happy paths: method, path, body and decoded result
// ---------------------------------------------------------------------------

func TestGetSubscription(t *testing.T) {
	rec := &recorded{}
	client := newLifecycleServer(t, rec, http.StatusOK, prodSubscriptionBody(lifecycleID, "go31-probe", "active"))

	got, err := client.GetSubscription(context.Background(), lifecycleID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.method != http.MethodGet || rec.path != "/v1/subscriptions/"+lifecycleID {
		t.Fatalf("expected GET /v1/subscriptions/%s, got %s %s", lifecycleID, rec.method, rec.path)
	}
	s := got.Data.Subscription
	if s.ID != lifecycleID || s.Name != "go31-probe" || s.Status != "active" || s.IntervalSeconds != 86400 {
		t.Errorf("unexpected subscription: %+v", s)
	}
	if len(s.Codes) != 2 || s.Codes[1] != "BRENT_CRUDE_USD" {
		t.Errorf("unexpected codes: %v", s.Codes)
	}
	if s.LastEvaluatedAt != "" {
		t.Errorf("null last_evaluated_at should decode to empty, got %q", s.LastEvaluatedAt)
	}
}

func TestUpdateSubscriptionSendsOnlySetFields(t *testing.T) {
	name := "go31-probe-renamed"
	interval := 3600
	off := false

	tests := []struct {
		name     string
		input    SubscriptionUpdate
		wantBody map[string]interface{}
	}{
		{
			name:     "name and codes",
			input:    SubscriptionUpdate{Name: &name, Codes: []string{"WTI_USD", "BRENT_CRUDE_USD"}},
			wantBody: map[string]interface{}{"name": name, "codes": []interface{}{"WTI_USD", "BRENT_CRUDE_USD"}},
		},
		{
			name:     "interval only",
			input:    SubscriptionUpdate{IntervalSeconds: &interval},
			wantBody: map[string]interface{}{"interval_seconds": float64(3600)},
		},
		{
			name:     "explicit false webhook is sent, not dropped",
			input:    SubscriptionUpdate{DeliverWebhook: &off},
			wantBody: map[string]interface{}{"deliver_webhook": false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recorded{}
			client := newLifecycleServer(t, rec, http.StatusOK, prodSubscriptionBody(lifecycleID, name, "active"))

			got, err := client.UpdateSubscription(context.Background(), lifecycleID, tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.method != http.MethodPatch || rec.path != "/v1/subscriptions/"+lifecycleID {
				t.Fatalf("expected PATCH /v1/subscriptions/%s, got %s %s", lifecycleID, rec.method, rec.path)
			}
			var sent map[string]interface{}
			if err := json.Unmarshal([]byte(rec.body), &sent); err != nil {
				t.Fatalf("request body is not JSON: %q", rec.body)
			}
			if len(sent) != len(tt.wantBody) {
				t.Errorf("expected body %v, got %v", tt.wantBody, sent)
			}
			for k, want := range tt.wantBody {
				gotJSON, _ := json.Marshal(sent[k])
				wantJSON, _ := json.Marshal(want)
				if string(gotJSON) != string(wantJSON) {
					t.Errorf("body[%q] = %s, want %s", k, gotJSON, wantJSON)
				}
			}
			if got.Data.Subscription.Name != name {
				t.Errorf("expected decoded name %q, got %q", name, got.Data.Subscription.Name)
			}
		})
	}
}

func TestPauseAndResumeSubscription(t *testing.T) {
	tests := []struct {
		name       string
		call       func(*Client, context.Context, string) (*SubscriptionResponse, error)
		wantPath   string
		wantStatus string
	}{
		{"pause", (*Client).PauseSubscription, "/v1/subscriptions/" + lifecycleID + "/pause", "paused"},
		{"resume", (*Client).ResumeSubscription, "/v1/subscriptions/" + lifecycleID + "/resume", "active"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recorded{}
			client := newLifecycleServer(t, rec, http.StatusOK, prodSubscriptionBody(lifecycleID, "w", tt.wantStatus))

			got, err := tt.call(client, context.Background(), lifecycleID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.method != http.MethodPost || rec.path != tt.wantPath {
				t.Fatalf("expected POST %s, got %s %s", tt.wantPath, rec.method, rec.path)
			}
			if got.Data.Subscription.Status != tt.wantStatus {
				t.Errorf("expected status %q, got %q", tt.wantStatus, got.Data.Subscription.Status)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Path contract: caller IDs are one escaped segment
// ---------------------------------------------------------------------------

func TestSubscriptionMemberIDIsEscapedIntoOneSegment(t *testing.T) {
	rec := &recorded{}
	id := "a/b c"
	client := newLifecycleServer(t, rec, http.StatusOK, prodSubscriptionBody(id, "w", "paused"))

	if _, err := client.PauseSubscription(context.Background(), id); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "/v1/subscriptions/a%2Fb%20c/pause"; rec.rawPath != want {
		t.Errorf("expected escaped path %q, got %q", want, rec.rawPath)
	}
}

func TestSubscriptionMemberAcceptsCaseInsensitiveIDMatch(t *testing.T) {
	// The server stores UUIDs lowercase and matches them case-insensitively, so
	// an uppercase caller ID is the same resource, not a malformed response.
	rec := &recorded{}
	client := newLifecycleServer(t, rec, http.StatusOK, prodSubscriptionBody(lifecycleID, "w", "active"))
	if _, err := client.GetSubscription(context.Background(), strings.ToUpper(lifecycleID)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Input validation happens before any network call
// ---------------------------------------------------------------------------

func TestSubscriptionLifecycleValidatesBeforeNetwork(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := NewClient("test-key", WithBaseURL(server.URL))
	ctx := context.Background()

	name := "x"
	zero := 0
	negative := -60
	valid := SubscriptionUpdate{Name: &name}

	cases := []struct {
		name  string
		field string
		call  func() error
	}{
		{"get empty id", "id", func() error { _, err := client.GetSubscription(ctx, ""); return err }},
		{"get blank id", "id", func() error { _, err := client.GetSubscription(ctx, "   "); return err }},
		{"get dot id", "id", func() error { _, err := client.GetSubscription(ctx, "."); return err }},
		{"get dotdot id", "id", func() error { _, err := client.GetSubscription(ctx, ".."); return err }},
		{"get control char id", "id", func() error { _, err := client.GetSubscription(ctx, "abc\n"); return err }},
		{"pause empty id", "id", func() error { _, err := client.PauseSubscription(ctx, ""); return err }},
		{"resume dotdot id", "id", func() error { _, err := client.ResumeSubscription(ctx, ".."); return err }},
		{"delete dot id", "id", func() error { return client.DeleteSubscription(ctx, ".") }},
		{"update empty id", "id", func() error { _, err := client.UpdateSubscription(ctx, "", valid); return err }},
		{"update no fields", "update", func() error {
			_, err := client.UpdateSubscription(ctx, lifecycleID, SubscriptionUpdate{})
			return err
		}},
		{"update empty codes", "codes", func() error {
			_, err := client.UpdateSubscription(ctx, lifecycleID, SubscriptionUpdate{Codes: []string{}})
			return err
		}},
		{"update blank code", "codes", func() error {
			_, err := client.UpdateSubscription(ctx, lifecycleID, SubscriptionUpdate{Codes: []string{"WTI_USD", " "}})
			return err
		}},
		{"update zero interval", "interval_seconds", func() error {
			_, err := client.UpdateSubscription(ctx, lifecycleID, SubscriptionUpdate{IntervalSeconds: &zero})
			return err
		}},
		{"update negative interval", "interval_seconds", func() error {
			_, err := client.UpdateSubscription(ctx, lifecycleID, SubscriptionUpdate{IntervalSeconds: &negative})
			return err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			var inputErr *InvalidInputError
			if !errors.As(err, &inputErr) {
				t.Fatalf("expected *InvalidInputError, got %T: %v", err, err)
			}
			if inputErr.Field != tc.field {
				t.Errorf("expected field %q, got %q", tc.field, inputErr.Field)
			}
		})
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Errorf("validation failures must not reach the network; server saw %d request(s)", n)
	}
}

// ---------------------------------------------------------------------------
// Typed API errors
// ---------------------------------------------------------------------------

func TestSubscriptionLifecycleNotFound(t *testing.T) {
	calls := map[string]func(*Client) error{
		"get":    func(c *Client) error { _, err := c.GetSubscription(context.Background(), lifecycleID); return err },
		"pause":  func(c *Client) error { _, err := c.PauseSubscription(context.Background(), lifecycleID); return err },
		"resume": func(c *Client) error { _, err := c.ResumeSubscription(context.Background(), lifecycleID); return err },
		"update": func(c *Client) error {
			n := "x"
			_, err := c.UpdateSubscription(context.Background(), lifecycleID, SubscriptionUpdate{Name: &n})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			rec := &recorded{}
			client := newLifecycleServer(t, rec, http.StatusNotFound, prodNotFoundBody)
			err := call(client)
			var nf *NotFoundError
			if !errors.As(err, &nf) {
				t.Fatalf("expected *NotFoundError, got %T: %v", err, err)
			}
			if !strings.Contains(nf.Message, "Subscription not found") {
				t.Errorf("expected server message preserved, got %q", nf.Message)
			}
		})
	}
}

func TestUpdateSubscriptionValidationError(t *testing.T) {
	for name, body := range map[string]string{
		"invalid codes":  prodInvalidCodesBody,
		"interval floor": prodIntervalFloorBody,
	} {
		t.Run(name, func(t *testing.T) {
			rec := &recorded{}
			client := newLifecycleServer(t, rec, http.StatusUnprocessableEntity, body)
			codes := []string{"NOT_A_REAL_CODE"}
			_, err := client.UpdateSubscription(context.Background(), lifecycleID, SubscriptionUpdate{Codes: codes})
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusUnprocessableEntity {
				t.Fatalf("expected *APIError 422, got %T: %v", err, err)
			}
			if !strings.Contains(apiErr.Message, "VALIDATION_ERROR") {
				t.Errorf("expected server body preserved, got %q", apiErr.Message)
			}
			if n := atomic.LoadInt32(&rec.hits); n != 1 {
				t.Errorf("a 422 must not be retried; server saw %d request(s)", n)
			}
		})
	}
}

func TestSubscriptionLifecycleEntitlementErrors(t *testing.T) {
	// 402 is the agent upgrade-trigger shape (AgentUpgradeTriggers); 403 is a
	// plan gate. Both must surface as *APIError with the body intact so a
	// caller can read the upgrade path.
	upgradeBody := `{"status":"fail","data":{"error":"INTERVAL_FLOOR","message":"Your plan's minimum snapshot interval is 60s (requested 30s). Upgrade to poll faster.","limit":60,"current":30,"upgrade_trigger":"interval_floor","upgrade_url":"https://www.oilpriceapi.com/pricing"}}`
	forbiddenBody := `{"error":{"code":"FORBIDDEN","message":"This feature is not available on your plan","status":403}}`

	for _, tc := range []struct {
		status int
		body   string
		marker string
	}{
		{http.StatusPaymentRequired, upgradeBody, "upgrade_url"},
		{http.StatusForbidden, forbiddenBody, "FORBIDDEN"},
	} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			rec := &recorded{}
			client := newLifecycleServer(t, rec, tc.status, tc.body)
			interval := 30
			_, err := client.UpdateSubscription(context.Background(), lifecycleID, SubscriptionUpdate{IntervalSeconds: &interval})
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != tc.status {
				t.Fatalf("expected *APIError %d, got %T: %v", tc.status, err, err)
			}
			if !strings.Contains(apiErr.Message, tc.marker) {
				t.Errorf("expected %q preserved in message, got %q", tc.marker, apiErr.Message)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Malformed success
// ---------------------------------------------------------------------------

func TestSubscriptionLifecycleMalformedSuccess(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"not json", `<html>ok</html>`},
		{"empty body", ``},
		{"missing subscription", `{"status":"success","data":{}}`},
		{"list shape instead of member", `{"status":"success","data":{"subscriptions":[]}}`},
		{"different id", prodSubscriptionBody("00000000-0000-0000-0000-000000000000", "w", "paused")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorded{}
			client := newLifecycleServer(t, rec, http.StatusOK, tc.body)
			got, err := client.PauseSubscription(context.Background(), lifecycleID)
			var malformed *MalformedResponseError
			if !errors.As(err, &malformed) {
				t.Fatalf("expected *MalformedResponseError, got %T: %v (result %+v)", err, err, got)
			}
			if malformed.StatusCode != http.StatusOK {
				t.Errorf("expected status 200 recorded, got %d", malformed.StatusCode)
			}
			if got != nil {
				t.Errorf("expected nil result alongside the error, got %+v", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Retries: writes are never replayed, reads are
// ---------------------------------------------------------------------------

func TestSubscriptionLifecycleWritesAreNotRetried(t *testing.T) {
	writes := map[string]func(*Client) error{
		"pause":  func(c *Client) error { _, err := c.PauseSubscription(context.Background(), lifecycleID); return err },
		"resume": func(c *Client) error { _, err := c.ResumeSubscription(context.Background(), lifecycleID); return err },
		"update": func(c *Client) error {
			n := "x"
			_, err := c.UpdateSubscription(context.Background(), lifecycleID, SubscriptionUpdate{Name: &n})
			return err
		},
	}
	for name, call := range writes {
		t.Run(name, func(t *testing.T) {
			rec := &recorded{}
			client := newLifecycleServer(t, rec, http.StatusServiceUnavailable, `{"error":"unavailable"}`)
			err := call(client)
			var serverErr *ServerError
			if !errors.As(err, &serverErr) {
				t.Fatalf("expected *ServerError, got %T: %v", err, err)
			}
			if n := atomic.LoadInt32(&rec.hits); n != 1 {
				t.Errorf("a write must be sent exactly once; server saw %d request(s)", n)
			}
		})
	}
}

func TestGetSubscriptionRetriesTransientFailure(t *testing.T) {
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, prodSubscriptionBody(lifecycleID, "w", "active"))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithRetries(2))
	if _, err := client.GetSubscription(context.Background(), lifecycleID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n := atomic.LoadInt32(&hits); n != 2 {
		t.Errorf("expected one retry of the safe GET (2 requests), got %d", n)
	}
}

// ---------------------------------------------------------------------------
// Cancellation and timeout
// ---------------------------------------------------------------------------

// hangingServer accepts the request and never answers until the test ends.
func hangingServer(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() {
		close(release)
		server.Close()
	})
	return server, &hits
}

func TestSubscriptionLifecycleHonoursCancellation(t *testing.T) {
	t.Run("already cancelled context sends nothing", func(t *testing.T) {
		server, hits := hangingServer(t)
		client := NewClient("test-key", WithBaseURL(server.URL))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := client.ResumeSubscription(ctx, lifecycleID)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %T: %v", err, err)
		}
		if n := atomic.LoadInt32(hits); n != 0 {
			t.Errorf("expected no request on a cancelled context, got %d", n)
		}
	})

	t.Run("cancel while in flight", func(t *testing.T) {
		server, _ := hangingServer(t)
		client := NewClient("test-key", WithBaseURL(server.URL), WithTimeout(30*time.Second))
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(50*time.Millisecond, cancel)

		start := time.Now()
		_, err := client.PauseSubscription(ctx, lifecycleID)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %T: %v", err, err)
		}
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("cancellation took %v; the call should return promptly", elapsed)
		}
	})

	t.Run("context deadline", func(t *testing.T) {
		server, _ := hangingServer(t)
		client := NewClient("test-key", WithBaseURL(server.URL), WithTimeout(30*time.Second))
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := client.GetSubscription(ctx, lifecycleID)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context.DeadlineExceeded, got %T: %v", err, err)
		}
	})
}

func TestSubscriptionLifecycleClientTimeout(t *testing.T) {
	server, hits := hangingServer(t)
	client := NewClient("test-key", WithBaseURL(server.URL), WithTimeout(100*time.Millisecond), WithRetries(3))

	n := "x"
	start := time.Now()
	_, err := client.UpdateSubscription(context.Background(), lifecycleID, SubscriptionUpdate{Name: &n})
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("expected a timeout net.Error, got %T: %v", err, err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("timeout took %v", elapsed)
	}
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Errorf("a timed-out PATCH is ambiguous and must not be replayed; server saw %d request(s)", got)
	}
}
