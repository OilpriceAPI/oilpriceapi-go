package oilpriceapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ===================
// MARKET BRIEF
// ===================

func TestGetMarketBrief(t *testing.T) {
	tests := []struct {
		name          string
		codes         []string
		opts          []MarketBriefOption
		wantCodesArg  string
		wantNarrative string // "" means narrative param must be absent
	}{
		{
			name:          "single code, no narrative",
			codes:         []string{"BRENT_CRUDE_USD"},
			wantCodesArg:  "BRENT_CRUDE_USD",
			wantNarrative: "",
		},
		{
			name:          "multiple codes joined with comma",
			codes:         []string{"BRENT_CRUDE_USD", "WTI_USD"},
			wantCodesArg:  "BRENT_CRUDE_USD,WTI_USD",
			wantNarrative: "",
		},
		{
			name:          "narrative requested",
			codes:         []string{"WTI_USD"},
			opts:          []MarketBriefOption{WithNarrative(true)},
			wantCodesArg:  "WTI_USD",
			wantNarrative: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/market-brief" {
					t.Errorf("expected path /v1/market-brief, got %s", r.URL.Path)
				}
				if got := r.URL.Query().Get("codes"); got != tt.wantCodesArg {
					t.Errorf("expected codes=%q, got %q", tt.wantCodesArg, got)
				}
				if got := r.URL.Query().Get("narrative"); got != tt.wantNarrative {
					t.Errorf("expected narrative=%q, got %q", tt.wantNarrative, got)
				}
				if r.Header.Get("Authorization") != "Token test-key" {
					t.Errorf("expected auth header, got %q", r.Header.Get("Authorization"))
				}

				resp := MarketBriefResponse{
					Status: "success",
					Data: MarketBrief{
						AsOf:  "2026-06-21T00:00:00Z",
						Codes: tt.codes,
						Commodities: []MarketBriefCommodity{
							{
								Code:         "BRENT_CRUDE_USD",
								Name:         "Brent Crude Oil",
								Price:        78.42,
								Currency:     "USD",
								Unit:         "barrel",
								Change24hPct: 1.2,
								Stale:        false,
								Forecast1M:   &MarketBriefForecast{Point: 80.1, Low: 76.0, High: 84.2, Confidence: 0.7},
							},
						},
						Spreads: []MarketBriefSpread{
							{Pair: "BRENT_CRUDE_USD-WTI_USD", Label: "Brent-WTI", Value: 6.08, Currency: "USD"},
						},
					},
				}
				if tt.wantNarrative == "true" {
					resp.Data.Summary = "Brent is up 1.2% at $78.42."
					resp.Data.Context = &MarketBriefContext{
						TopIndicators: []MarketBriefIndicator{{Code: "DXY", Value: 104.2, ChangePct: -0.3}},
					}
				}
				_ = json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			got, err := client.GetMarketBrief(context.Background(), tt.codes, tt.opts...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Status != "success" {
				t.Errorf("expected status success, got %s", got.Status)
			}
			if len(got.Data.Commodities) != 1 {
				t.Fatalf("expected 1 commodity, got %d", len(got.Data.Commodities))
			}
			c := got.Data.Commodities[0]
			if c.Price != 78.42 {
				t.Errorf("expected price 78.42, got %v", c.Price)
			}
			if c.Forecast1M == nil || c.Forecast1M.Point != 80.1 {
				t.Errorf("expected forecast point 80.1, got %+v", c.Forecast1M)
			}
			if tt.wantNarrative == "true" {
				if got.Data.Summary == "" {
					t.Error("expected non-empty summary when narrative requested")
				}
				if got.Data.Context == nil || len(got.Data.Context.TopIndicators) == 0 {
					t.Error("expected narrative context with indicators")
				}
			} else if got.Data.Context != nil {
				t.Error("expected no context when narrative not requested")
			}
		})
	}
}

func TestGetMarketBriefValidation(t *testing.T) {
	client := NewClient("test-key")
	if _, err := client.GetMarketBrief(context.Background(), nil); err == nil {
		t.Error("expected error for empty codes")
	}
}

func TestGetMarketBriefServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	client := NewClient("bad-key", WithBaseURL(server.URL))
	_, err := client.GetMarketBrief(context.Background(), []string{"WTI_USD"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := err.(*AuthenticationError); !ok {
		t.Errorf("expected *AuthenticationError, got %T", err)
	}
}

// ===================
// SUBSCRIPTIONS LIST
// ===================

func TestGetSubscriptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/subscriptions" {
			t.Errorf("expected /v1/subscriptions, got %s", r.URL.Path)
		}
		resp := SubscriptionsResponse{
			Status: "success",
			Data: SubscriptionsData{
				Subscriptions: []Subscription{
					{
						ID:              "11111111-1111-1111-1111-111111111111",
						Name:            "Crude watch",
						Codes:           []string{"BRENT_CRUDE_USD", "WTI_USD"},
						IntervalSeconds: 300,
						Status:          "active",
						Source:          "sdk-go",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	got, err := client.GetSubscriptions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Data.Subscriptions) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(got.Data.Subscriptions))
	}
	s := got.Data.Subscriptions[0]
	if s.Name != "Crude watch" || s.IntervalSeconds != 300 {
		t.Errorf("unexpected subscription: %+v", s)
	}
	if len(s.Codes) != 2 {
		t.Errorf("expected 2 codes, got %d", len(s.Codes))
	}
}

// ===================
// SUBSCRIPTION CREATE
// ===================

func TestCreateSubscription(t *testing.T) {
	tests := []struct {
		name           string
		input          SubscriptionInput
		wantSource     string
		wantTool       string // "" means header must be absent
		wantStatusCode int
	}{
		{
			name: "defaults source to sdk-go",
			input: SubscriptionInput{
				Name:            "Crude watch",
				Codes:           []string{"BRENT_CRUDE_USD"},
				IntervalSeconds: 300,
			},
			wantSource:     "sdk-go",
			wantTool:       "",
			wantStatusCode: http.StatusOK,
		},
		{
			name: "explicit source and tool sent as headers",
			input: SubscriptionInput{
				Name:            "MCP watch",
				Codes:           []string{"WTI_USD"},
				IntervalSeconds: 600,
				Source:          "mcp",
				ToolName:        "claude-desktop",
			},
			wantSource:     "mcp",
			wantTool:       "claude-desktop",
			wantStatusCode: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if got := r.Header.Get("X-OPA-Source"); got != tt.wantSource {
					t.Errorf("expected X-OPA-Source=%q, got %q", tt.wantSource, got)
				}
				if got := r.Header.Get("X-OPA-Tool"); got != tt.wantTool {
					t.Errorf("expected X-OPA-Tool=%q, got %q", tt.wantTool, got)
				}

				// Verify body has codes + interval and does NOT leak source/tool.
				body, _ := io.ReadAll(r.Body)
				var sent map[string]interface{}
				if err := json.Unmarshal(body, &sent); err != nil {
					t.Fatalf("bad request body: %v", err)
				}
				if _, leaked := sent["source"]; leaked {
					t.Error("source should not appear in JSON body")
				}
				if _, leaked := sent["tool_name"]; leaked {
					t.Error("tool_name should not appear in JSON body")
				}
				if _, ok := sent["codes"]; !ok {
					t.Error("expected codes in body")
				}

				w.WriteHeader(tt.wantStatusCode)
				_ = json.NewEncoder(w).Encode(SubscriptionResponse{
					Status: "success",
					Data: SubscriptionData{
						Subscription: Subscription{
							ID:              "22222222-2222-2222-2222-222222222222",
							Name:            tt.input.Name,
							Codes:           tt.input.Codes,
							IntervalSeconds: tt.input.IntervalSeconds,
							Status:          "active",
							Source:          tt.wantSource,
							ToolName:        tt.wantTool,
						},
					},
				})
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			got, err := client.CreateSubscription(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Data.Subscription.ID == "" {
				t.Error("expected subscription ID in response")
			}
			if got.Data.Subscription.Name != tt.input.Name {
				t.Errorf("expected name %q, got %q", tt.input.Name, got.Data.Subscription.Name)
			}
		})
	}
}

// ===================
// SUBSCRIPTION DELETE
// ===================

func TestDeleteSubscription(t *testing.T) {
	t.Run("204 no content success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("expected DELETE, got %s", r.Method)
			}
			if r.URL.Path != "/v1/subscriptions/abc-123" {
				t.Errorf("unexpected path %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		if err := client.DeleteSubscription(context.Background(), "abc-123"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty id errors without request", func(t *testing.T) {
		client := NewClient("test-key")
		if err := client.DeleteSubscription(context.Background(), ""); err == nil {
			t.Error("expected error for empty id")
		}
	})

	t.Run("not found returns typed error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))
		err := client.DeleteSubscription(context.Background(), "missing")
		if _, ok := err.(*NotFoundError); !ok {
			t.Errorf("expected *NotFoundError, got %T", err)
		}
	})
}

// ===================
// SUBSCRIPTION EVENTS
// ===================

func TestGetSubscriptionEvents(t *testing.T) {
	tests := []struct {
		name      string
		opts      []EventOption
		wantQuery string
	}{
		{
			name:      "no options sends no query",
			opts:      nil,
			wantQuery: "",
		},
		{
			name:      "since cursor",
			opts:      []EventOption{WithSince(42)},
			wantQuery: "since=42",
		},
		{
			name:      "since zero is still sent",
			opts:      []EventOption{WithSince(0)},
			wantQuery: "since=0",
		},
		{
			name:      "limit and watch id",
			opts:      []EventOption{WithEventLimit(10), WithEventWatchID("w-1")},
			wantQuery: "limit=10&watch_id=w-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/subscriptions/events" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				if got := r.URL.RawQuery; got != tt.wantQuery {
					t.Errorf("expected query %q, got %q", tt.wantQuery, got)
				}
				_ = json.NewEncoder(w).Encode(SubscriptionEventsResponse{
					Status: "success",
					Data: SubscriptionEventsData{
						Cursor:  99,
						HasMore: true,
						Events: []SubscriptionEvent{
							{
								ID:       1,
								Seq:      99,
								WatchID:  "w-1",
								Snapshot: map[string]interface{}{"BRENT_CRUDE_USD": 78.4},
								Deltas:   map[string]interface{}{"BRENT_CRUDE_USD": 1.2},
								Source:   "sdk-go",
							},
						},
					},
				})
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			got, err := client.GetSubscriptionEvents(context.Background(), tt.opts...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Data.Cursor != 99 {
				t.Errorf("expected cursor 99, got %d", got.Data.Cursor)
			}
			if !got.Data.HasMore {
				t.Error("expected has_more true")
			}
			if len(got.Data.Events) != 1 || got.Data.Events[0].Seq != 99 {
				t.Errorf("unexpected events: %+v", got.Data.Events)
			}
		})
	}
}

// ===================
// INTERVAL HELPER
// ===================

func TestParseInterval(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    int
		wantErr bool
	}{
		{name: "minutes", in: "5m", want: 300},
		{name: "hour", in: "1h", want: 3600},
		{name: "seconds suffix", in: "30s", want: 30},
		{name: "compound", in: "1h30m", want: 5400},
		{name: "plain integer seconds", in: "300", want: 300},
		{name: "whitespace trimmed", in: "  10m  ", want: 600},
		{name: "empty errors", in: "", wantErr: true},
		{name: "garbage errors", in: "abc", wantErr: true},
		{name: "zero errors", in: "0", wantErr: true},
		{name: "negative errors", in: "-5", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseInterval(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got %d", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseInterval(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// TestCreateSubscriptionWithParsedInterval exercises the documented
// ParseInterval -> IntervalSeconds flow end to end.
func TestCreateSubscriptionWithParsedInterval(t *testing.T) {
	secs, err := ParseInterval("5m")
	if err != nil {
		t.Fatalf("ParseInterval failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"interval_seconds":300`) {
			t.Errorf("expected interval_seconds:300 in body, got %s", string(body))
		}
		_ = json.NewEncoder(w).Encode(SubscriptionResponse{Status: "success"})
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	_, err = client.CreateSubscription(context.Background(), SubscriptionInput{
		Name:            "x",
		Codes:           []string{"WTI_USD"},
		IntervalSeconds: secs,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
