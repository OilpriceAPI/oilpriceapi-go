package oilpriceapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultSubscriptionSource is the attribution source sent as the X-OPA-Source
// header when a SubscriptionInput does not set one explicitly.
const DefaultSubscriptionSource = "sdk-go"

// GetMarketBrief fetches a multi-commodity structured market summary for the
// given commodity codes (#3245 Phase 1a).
//
// Codes accept the same shorthand the rest of the API does (e.g. "WTI",
// "BRENT"); the server alias-resolves them to canonical codes. Pass
// WithNarrative(true) to additionally receive the deterministic narrative
// context and summary.
//
// Example:
//
//	brief, err := client.GetMarketBrief(ctx, []string{"BRENT_CRUDE_USD", "WTI_USD"})
//	for _, c := range brief.Data.Commodities {
//	    fmt.Printf("%s: %.2f %s\n", c.Code, c.Price, c.Currency)
//	}
//
//	// With narrative context + summary
//	brief, err := client.GetMarketBrief(ctx, []string{"WTI_USD"}, oilpriceapi.WithNarrative(true))
func (c *Client) GetMarketBrief(ctx context.Context, codes []string, opts ...MarketBriefOption) (*MarketBriefResponse, error) {
	if len(codes) == 0 {
		return nil, fmt.Errorf("codes is required: pass at least one commodity code")
	}

	options := &MarketBriefOptions{}
	for _, opt := range opts {
		opt(options)
	}

	query := url.Values{}
	query.Set("codes", strings.Join(codes, ","))
	if options.Narrative {
		query.Set("narrative", "true")
	}

	endpoint := "/v1/market-brief?" + query.Encode()

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result MarketBriefResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSubscriptions lists the caller's agent subscriptions (watches), newest
// first (#3245 Phase 2).
//
// Example:
//
//	subs, err := client.GetSubscriptions(ctx)
//	for _, s := range subs.Data.Subscriptions {
//	    fmt.Printf("%s watches %v every %ds\n", s.Name, s.Codes, s.IntervalSeconds)
//	}
func (c *Client) GetSubscriptions(ctx context.Context) (*SubscriptionsResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/v1/subscriptions", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result SubscriptionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateSubscription creates a new agent subscription (watch).
//
// The input's Source and ToolName are sent as the X-OPA-Source and X-OPA-Tool
// attribution headers rather than in the JSON body; Source defaults to
// DefaultSubscriptionSource ("sdk-go") when blank. Set IntervalSeconds directly,
// or convert a friendly duration with ParseInterval ("5m", "1h").
//
// Example:
//
//	secs, _ := oilpriceapi.ParseInterval("5m")
//	sub, err := client.CreateSubscription(ctx, oilpriceapi.SubscriptionInput{
//	    Name:            "Crude watch",
//	    Codes:           []string{"BRENT_CRUDE_USD", "WTI_USD"},
//	    IntervalSeconds: secs,
//	})
func (c *Client) CreateSubscription(ctx context.Context, input SubscriptionInput) (*SubscriptionResponse, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	source := input.Source
	if source == "" {
		source = DefaultSubscriptionSource
	}

	headers := map[string]string{"X-OPA-Source": source}
	if input.ToolName != "" {
		headers["X-OPA-Tool"] = input.ToolName
	}

	resp, err := c.doRequestWithHeaders(ctx, "POST", "/v1/subscriptions", bytes.NewReader(body), headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, c.handleError(resp)
	}

	var result SubscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteSubscription deletes a subscription by ID. The endpoint returns 204 No
// Content on success.
func (c *Client) DeleteSubscription(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}

	resp, err := c.doRequest(ctx, "DELETE", "/v1/subscriptions/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return c.handleError(resp)
	}
	return nil
}

// GetSubscriptionEvents polls the per-user event cursor (#3245 Phase 2). This
// endpoint does NOT consume the monthly request quota, so agents may poll it
// frequently.
//
// Pass WithSince(cursor) to page forward from a previous response, WithEventLimit
// to cap the page size, and WithEventWatchID to scope to a single subscription.
//
// Example:
//
//	events, err := client.GetSubscriptionEvents(ctx, oilpriceapi.WithSince(lastCursor))
//	for _, e := range events.Data.Events {
//	    fmt.Printf("seq=%d watch=%s\n", e.Seq, e.WatchID)
//	}
//	lastCursor = events.Data.Cursor
func (c *Client) GetSubscriptionEvents(ctx context.Context, opts ...EventOption) (*SubscriptionEventsResponse, error) {
	options := &EventsOptions{}
	for _, opt := range opts {
		opt(options)
	}

	query := url.Values{}
	if options.sinceSet {
		query.Set("since", strconv.FormatInt(options.Since, 10))
	}
	if options.Limit > 0 {
		query.Set("limit", strconv.Itoa(options.Limit))
	}
	if options.WatchID != "" {
		query.Set("watch_id", options.WatchID)
	}

	endpoint := "/v1/subscriptions/events"
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result SubscriptionEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ParseInterval converts a friendly interval string to whole seconds for use as
// SubscriptionInput.IntervalSeconds.
//
// It accepts a Go duration string ("5m", "1h", "30s", "90m") as well as a plain
// integer interpreted as seconds ("300"). The result is rounded to whole
// seconds. An empty or unparseable value returns an error.
//
// Example:
//
//	secs, err := oilpriceapi.ParseInterval("5m") // 300
func ParseInterval(interval string) (int, error) {
	s := strings.TrimSpace(interval)
	if s == "" {
		return 0, fmt.Errorf("interval is empty")
	}

	// Plain integer => seconds.
	if n, err := strconv.Atoi(s); err == nil {
		if n <= 0 {
			return 0, fmt.Errorf("interval %q must be positive", interval)
		}
		return n, nil
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid interval %q: use a duration like \"5m\"/\"1h\" or a number of seconds", interval)
	}
	secs := int(d.Round(time.Second) / time.Second)
	if secs <= 0 {
		return 0, fmt.Errorf("interval %q must be positive", interval)
	}
	return secs, nil
}
