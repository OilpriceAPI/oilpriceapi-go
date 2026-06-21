package oilpriceapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// GetAlerts fetches all price alerts for the authenticated user.
//
// The API returns alerts ordered by commodity code and creation time.
//
// Example:
//
//	alerts, err := client.GetAlerts(ctx)
//	for _, a := range alerts {
//	    fmt.Printf("%s: %s %v %v\n", a.Name, a.CommodityCode, a.ConditionOperator, a.ConditionValue)
//	}
func (c *Client) GetAlerts(ctx context.Context) ([]Alert, error) {
	resp, err := c.doRequest(ctx, "GET", "/v1/alerts", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return decodeAlertList(resp.Body)
}

// CreateAlert creates a new price alert.
//
// Example:
//
//	alert, err := client.CreateAlert(ctx, oilpriceapi.AlertInput{
//	    Name:              "Brent $85 Alert",
//	    CommodityCode:     "BRENT_CRUDE_USD",
//	    ConditionOperator: "greater_than",
//	    ConditionValue:    85.0,
//	    WebhookURL:        "https://example.com/webhook",
//	})
func (c *Client) CreateAlert(ctx context.Context, input AlertInput) (*Alert, error) {
	body, err := json.Marshal(alertRequestBody{PriceAlert: input})
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(ctx, "POST", "/v1/alerts", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, c.handleError(resp)
	}

	return decodeAlert(resp.Body)
}

// GetAlert fetches a single price alert by ID.
func (c *Client) GetAlert(ctx context.Context, id string) (*Alert, error) {
	resp, err := c.doRequest(ctx, "GET", "/v1/alerts/"+id, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return decodeAlert(resp.Body)
}

// UpdateAlert updates an existing price alert. Only the fields set on input are
// sent; the zero value of a field is still marshaled, so callers should supply
// the full desired state.
func (c *Client) UpdateAlert(ctx context.Context, id string, input AlertInput) (*Alert, error) {
	body, err := json.Marshal(alertRequestBody{PriceAlert: input})
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(ctx, "PATCH", "/v1/alerts/"+id, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	return decodeAlert(resp.Body)
}

// DeleteAlert deletes a price alert by ID.
func (c *Client) DeleteAlert(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, "DELETE", "/v1/alerts/"+id, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return c.handleError(resp)
	}
	return nil
}

// GetAlertTriggers fetches the trigger history endpoint for price alerts.
//
// The trigger-history endpoint is deprecated server-side (trigger data now lives
// on the TriggerCount and LastTriggeredAt fields of each alert), so this method
// returns the raw response payload for forward compatibility.
func (c *Client) GetAlertTriggers(ctx context.Context) (*AlertTriggersResponse, error) {
	resp, err := c.doRequest(ctx, "GET", "/v1/alerts/triggers", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// The API may return 410 Gone for the deprecated endpoint with a JSON
	// message body; surface that as a normal response rather than an error so
	// callers can read the message.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusGone {
		return nil, c.handleError(resp)
	}

	var result AlertTriggersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// decodeAlert decodes a single-alert response. The API returns the alert object
// directly, but a {"alert": {...}} or {"data": {...}} envelope is also accepted
// for forward compatibility.
func decodeAlert(r io.Reader) (*Alert, error) {
	var raw json.RawMessage
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, err
	}

	var env struct {
		Alert *Alert `json:"alert"`
		Data  *Alert `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err == nil {
		if env.Alert != nil {
			return env.Alert, nil
		}
		if env.Data != nil {
			return env.Data, nil
		}
	}

	var alert Alert
	if err := json.Unmarshal(raw, &alert); err != nil {
		return nil, err
	}
	return &alert, nil
}

// decodeAlertList decodes a list-alerts response. The API returns a bare array,
// but an {"alerts": [...]} or {"data": [...]} envelope is also accepted.
func decodeAlertList(r io.Reader) ([]Alert, error) {
	var raw json.RawMessage
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, err
	}

	var arr []Alert
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}

	var env struct {
		Alerts []Alert `json:"alerts"`
		Data   []Alert `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("unexpected alerts response shape: %w", err)
	}
	if env.Alerts != nil {
		return env.Alerts, nil
	}
	return env.Data, nil
}
