package oilpriceapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

// GetWebhook fetches a single webhook by ID.
//
// The detail response includes the signing secret and the lists of available
// events, commodities, states, and operators, in addition to the webhook's
// configuration and event statistics.
//
// Example:
//
//	wh, err := client.GetWebhook(ctx, "42")
//	fmt.Printf("%s -> %s (status %s)\n", wh.ID, wh.URL, wh.Status)
func (c *Client) GetWebhook(ctx context.Context, id string) (*WebhookDetail, error) {
	resp, err := c.doRequest(ctx, "GET", "/v1/webhooks/"+id, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result WebhookDetail
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateWebhook updates an existing webhook by ID.
//
// Only the fields set on input are sent (zero values are omitted), so callers
// can perform partial updates. The signing secret cannot be changed via update.
func (c *Client) UpdateWebhook(ctx context.Context, id string, input WebhookUpdateInput) (*WebhookDetail, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(ctx, "PATCH", "/v1/webhooks/"+id, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result WebhookDetail
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TestWebhook sends a test delivery to the webhook's configured URL and returns
// the result of the attempt.
func (c *Client) TestWebhook(ctx context.Context, id string) (*WebhookTestResult, error) {
	resp, err := c.doRequest(ctx, "POST", "/v1/webhooks/"+id+"/test", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result WebhookTestResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetWebhookEvents fetches the recent delivery events for a webhook, newest
// first. Use WithWebhookPage and WithWebhookPerPage to paginate.
func (c *Client) GetWebhookEvents(ctx context.Context, id string, opts ...WebhookEventsOption) (*WebhookEventsResponse, error) {
	options := &WebhookEventsOptions{}
	for _, opt := range opts {
		opt(options)
	}

	endpoint := "/v1/webhooks/" + id + "/events"
	sep := "?"
	if options.Page > 0 {
		endpoint += sep + "page=" + strconv.Itoa(options.Page)
		sep = "&"
	}
	if options.PerPage > 0 {
		endpoint += sep + "per_page=" + strconv.Itoa(options.PerPage)
	}

	resp, err := c.doRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var result WebhookEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
