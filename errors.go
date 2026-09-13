package oilpriceapi

import "fmt"

// AuthenticationError is returned when API authentication fails.
type AuthenticationError struct {
	Message string
}

func (e *AuthenticationError) Error() string {
	return fmt.Sprintf("authentication error: %s", e.Message)
}

// RateLimitError is returned when the rate limit is exceeded.
type RateLimitError struct {
	Message    string
	RetryAfter int
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded: %s (retry after %d seconds)", e.Message, e.RetryAfter)
}

// NotFoundError is returned when a resource is not found.
type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("not found: %s", e.Message)
}

// ServerError is returned when the API returns a server error.
type ServerError struct {
	Message    string
	StatusCode int
}

func (e *ServerError) Error() string {
	return fmt.Sprintf("server error (%d): %s", e.StatusCode, e.Message)
}

// APIError is a generic API error.
type APIError struct {
	Message    string
	StatusCode int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.Message)
}

// StreamRejectedError is returned when the server rejects the WebSocket
// subscription. Dataset access varies by plan and account entitlement.
type StreamRejectedError struct {
	Message string
}

func (e *StreamRejectedError) Error() string {
	if e.Message == "" {
		return "stream subscription rejected: verify the API key and account entitlement at https://www.oilpriceapi.com/pricing"
	}
	return fmt.Sprintf("stream subscription rejected: %s", e.Message)
}

// InvalidPathError is returned when a request path would address something
// other than the configured API origin, or is otherwise not a valid API path.
//
// The SDK attaches the caller's API key to every request, so a path that can
// move the request to another host is a credential-transport bug rather than a
// routing convenience. Paths must be origin-relative and begin with a single
// "/" (e.g. "/v1/prices/latest"). The offending path is echoed back; the API
// key never is.
type InvalidPathError struct {
	Path   string
	Reason string
}

func (e *InvalidPathError) Error() string {
	return fmt.Sprintf("invalid API path %q: %s (paths must be origin-relative and start with a single \"/\")", e.Path, e.Reason)
}

// ConfigurationError is returned when the client is configured with a value it
// cannot act on, such as a negative retry count. It is returned from the call
// rather than panicking at construction so an option supplied from config does
// not take a process down.
type ConfigurationError struct {
	Option string
	Reason string
}

func (e *ConfigurationError) Error() string {
	return fmt.Sprintf("invalid client configuration: %s %s", e.Option, e.Reason)
}
