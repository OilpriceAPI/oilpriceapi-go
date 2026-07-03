package oilpriceapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// Raw performs an authenticated request against an arbitrary API path and
// JSON-decodes the response into v.
//
// It is the escape hatch for endpoints the SDK does not (yet) wrap with a
// typed method: it uses the same authentication, User-Agent, and retry/backoff
// behaviour as every other client method, and surfaces API errors via the same
// error types (AuthenticationError, RateLimitError, NotFoundError,
// ServerError, APIError).
//
// path must start with "/" (e.g. "/v1/prices/latest"). query is optional and
// may be nil; when set it is encoded and appended to the path. v may be nil to
// discard the response body (useful for DELETE-style calls); otherwise it must
// be a pointer suitable for json.Unmarshal.
//
// Example:
//
//	var out map[string]any
//	err := client.Raw(ctx, http.MethodGet, "/v1/some/new/endpoint",
//	    url.Values{"days": {"30"}}, &out)
func (c *Client) Raw(ctx context.Context, method, path string, query url.Values, v any) error {
	endpoint := path
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	resp, err := c.doRequest(ctx, method, endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.handleError(resp)
	}

	if v == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
