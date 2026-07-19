package oilpriceapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// energyPricesChannel is the ActionCable channel exposed by the OilPriceAPI
// server. Confirmed from app/channels/energy_prices_channel.rb
// (class EnergyPricesChannel).
const energyPricesChannel = "EnergyPricesChannel"

// Default reconnect tuning for the streaming client.
const (
	defaultReconnectDelay    = 1 * time.Second
	defaultMaxReconnectDelay = 30 * time.Second
	defaultMaxReconnectTries = 10
)

// commodityCodeToSlug maps upstream price codes to the streamed slug used in
// the broadcast prices map, so WithStreamCommodities accepts either form.
var commodityCodeToSlug = map[string]string{
	"BRENT_CRUDE_USD": "brent",
	"WTI_USD":         "wti",
	"NATURAL_GAS_GBP": "uk",
	"NATURAL_GAS_USD": "us",
	"DUTCH_TTF_EUR":   "eu",
}

// StreamOptions configures a price stream opened with Client.StreamPrices.
type StreamOptions struct {
	// Commodities is an optional client-side filter. When non-empty, only
	// price_update messages whose normalized map contains at least one of these
	// commodities are delivered on Updates(). Accepts streamed slugs ("brent",
	// "wti", "uk", "us", "eu") or upstream codes ("BRENT_CRUDE_USD", "WTI_USD",
	// "NATURAL_GAS_GBP", "NATURAL_GAS_USD", "DUTCH_TTF_EUR"). The server always
	// broadcasts the full map; filtering is applied locally. welcome and
	// rig_count_update messages are never filtered.
	Commodities []string

	// AutoReconnect enables automatic reconnection with exponential backoff
	// after a transient disconnect. Default: true.
	AutoReconnect bool

	// ReconnectDelay is the base backoff delay. Default: 1s.
	ReconnectDelay time.Duration

	// MaxReconnectDelay caps the exponential backoff. Default: 30s.
	MaxReconnectDelay time.Duration

	// MaxReconnectAttempts is the number of consecutive reconnect attempts
	// before the stream terminates with an error. Default: 10. A negative value
	// retries forever.
	MaxReconnectAttempts int
}

// StreamOption is a functional option for StreamPrices.
type StreamOption func(*StreamOptions)

// WithStreamCommodities filters delivered price_update messages to those that
// include at least one of the given commodities. Accepts streamed slugs or
// upstream codes (see StreamOptions.Commodities).
func WithStreamCommodities(commodities ...string) StreamOption {
	return func(o *StreamOptions) {
		o.Commodities = append(o.Commodities, commodities...)
	}
}

// WithStreamAutoReconnect enables or disables auto-reconnect (default enabled).
func WithStreamAutoReconnect(enabled bool) StreamOption {
	return func(o *StreamOptions) {
		o.AutoReconnect = enabled
	}
}

// WithStreamReconnectDelay sets the base reconnect backoff delay.
func WithStreamReconnectDelay(d time.Duration) StreamOption {
	return func(o *StreamOptions) {
		o.ReconnectDelay = d
	}
}

// WithStreamMaxReconnectDelay caps the exponential reconnect backoff.
func WithStreamMaxReconnectDelay(d time.Duration) StreamOption {
	return func(o *StreamOptions) {
		o.MaxReconnectDelay = d
	}
}

// WithStreamMaxReconnectAttempts sets the maximum number of consecutive
// reconnect attempts before the stream gives up. A negative value retries
// forever.
func WithStreamMaxReconnectAttempts(n int) StreamOption {
	return func(o *StreamOptions) {
		o.MaxReconnectAttempts = n
	}
}

// dialer abstracts the WebSocket dial so tests can inject a server without
// touching production. It returns a wsConn the stream reads from and writes to.
type wsDialer func(ctx context.Context, url string, header http.Header) (wsConn, error)

// wsConn is the minimal connection surface the stream needs. *websocket.Conn
// (via a small adapter) and test fakes both satisfy it.
type wsConn interface {
	// Read returns the next text/binary message payload.
	Read(ctx context.Context) ([]byte, error)
	// Write sends a text message payload.
	Write(ctx context.Context, data []byte) error
	// Close closes the connection with a normal closure status.
	Close() error
}

// gorillaConn adapts *websocket.Conn (gorilla) to the wsConn interface.
//
// gorilla's API is deadline-based rather than context-native, so Read and Write
// bridge the context by spawning a watcher that closes the underlying
// connection when ctx is cancelled. A cancelled context therefore unblocks an
// in-flight Read/Write with a connection error, which the run loop interprets
// (via s.ctx.Err()) as a clean shutdown.
type gorillaConn struct {
	c        *websocket.Conn
	writeMu  sync.Mutex
	closeErr error
}

func (a *gorillaConn) Read(ctx context.Context) ([]byte, error) {
	stop := a.watch(ctx)
	defer stop()
	_, data, err := a.c.ReadMessage()
	return data, err
}

func (a *gorillaConn) Write(ctx context.Context, data []byte) error {
	stop := a.watch(ctx)
	defer stop()
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	return a.c.WriteMessage(websocket.TextMessage, data)
}

func (a *gorillaConn) Close() error {
	return a.c.Close()
}

// watch closes the connection if ctx is cancelled before the returned stop
// function is called, bridging context cancellation onto gorilla's blocking I/O.
func (a *gorillaConn) watch(ctx context.Context) func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = a.c.Close()
		case <-done:
		}
	}()
	return func() { close(done) }
}

// defaultDialer dials with github.com/gorilla/websocket.
func defaultDialer(ctx context.Context, urlStr string, header http.Header) (wsConn, error) {
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 30 * time.Second
	c, resp, err := dialer.DialContext(ctx, urlStr, header)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return nil, &AuthenticationError{Message: "websocket handshake rejected (401): verify the API key and account entitlement"}
		}
		return nil, err
	}
	// Allow large welcome snapshots (drilling intelligence payloads can be big).
	c.SetReadLimit(1 << 20)
	return &gorillaConn{c: c}, nil
}

// PriceStream is an active WebSocket price subscription. Consume typed updates
// from Updates(); after the channel is closed, inspect Err() for the
// terminating error (nil on a clean Close or context cancellation). Call
// Close() to tear the stream down. PriceStream is safe for concurrent use.
type PriceStream struct {
	url        string
	apiKey     string
	identifier string
	opts       StreamOptions
	filter     map[string]struct{}
	dial       wsDialer

	updates chan Update

	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.Mutex
	err      error
	conn     wsConn
	closed   bool
	doneOnce sync.Once
	done     chan struct{}
}

// cableURL derives the ActionCable wss endpoint from a REST base URL:
//
//	https://api.oilpriceapi.com -> wss://api.oilpriceapi.com/cable
//	http://localhost:5000       -> ws://localhost:5000/cable
func cableURL(baseURL string) string {
	u := strings.TrimSuffix(baseURL, "/")
	switch {
	case strings.HasPrefix(u, "https://"):
		u = "wss://" + strings.TrimPrefix(u, "https://")
	case strings.HasPrefix(u, "http://"):
		u = "ws://" + strings.TrimPrefix(u, "http://")
	}
	return u + "/cable"
}

// StreamPrices opens a price stream over the EnergyPricesChannel.
//
// Availability varies by plan and account entitlement. A rejected subscription
// surfaces as a *StreamRejectedError on Err(). The supplied context controls the
// lifetime of the stream: cancelling it (or calling Close) tears the connection
// down cleanly.
//
// Example:
//
//	stream, err := client.StreamPrices(ctx,
//	    oilpriceapi.WithStreamCommodities("BRENT_CRUDE_USD", "WTI_USD"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer stream.Close()
//
//	for update := range stream.Updates() {
//	    switch update.Type {
//	    case "price_update":
//	        wti := update.Price.Prices.Oil.WTI
//	        if wti != nil && wti.OriginalPrice != nil {
//	            fmt.Printf("WTI %.2f @ %s\n", *wti.OriginalPrice, update.Price.Timestamp)
//	        }
//	    case "rig_count_update":
//	        fmt.Printf("%s rigs: %.0f\n", update.RigCount.RigCount.Region, update.RigCount.RigCount.Count)
//	    }
//	}
//	if err := stream.Err(); err != nil {
//	    log.Printf("stream ended: %v", err)
//	}
func (c *Client) StreamPrices(ctx context.Context, opts ...StreamOption) (*PriceStream, error) {
	if c.apiKey == "" {
		return nil, &AuthenticationError{Message: "an API key is required for streaming"}
	}

	options := StreamOptions{
		AutoReconnect:        true,
		ReconnectDelay:       defaultReconnectDelay,
		MaxReconnectDelay:    defaultMaxReconnectDelay,
		MaxReconnectAttempts: defaultMaxReconnectTries,
	}
	for _, opt := range opts {
		opt(&options)
	}

	return c.newStream(ctx, options, defaultDialer)
}

// newStream builds and starts a PriceStream with an injectable dialer (tests
// pass a dialer backed by an httptest WebSocket server).
func (c *Client) newStream(ctx context.Context, options StreamOptions, dial wsDialer) (*PriceStream, error) {
	identifier, err := json.Marshal(map[string]string{
		"channel": energyPricesChannel,
		"api_key": c.apiKey,
	})
	if err != nil {
		return nil, err
	}

	var filter map[string]struct{}
	if len(options.Commodities) > 0 {
		filter = make(map[string]struct{}, len(options.Commodities))
		for _, comm := range options.Commodities {
			slug, ok := commodityCodeToSlug[comm]
			if !ok {
				slug = strings.ToLower(comm)
			}
			filter[slug] = struct{}{}
		}
	}

	streamCtx, cancel := context.WithCancel(ctx)
	s := &PriceStream{
		url:        cableURL(c.baseURL),
		apiKey:     c.apiKey,
		identifier: string(identifier),
		opts:       options,
		filter:     filter,
		dial:       dial,
		updates:    make(chan Update, 64),
		ctx:        streamCtx,
		cancel:     cancel,
		done:       make(chan struct{}),
	}

	go s.run()
	return s, nil
}

// Updates returns the channel of typed updates. It is closed when the stream
// terminates (clean close, context cancellation, or a fatal error). After it is
// closed, call Err() to distinguish a clean shutdown from an error.
func (s *PriceStream) Updates() <-chan Update {
	return s.updates
}

// Err returns the error that terminated the stream, or nil if it ended cleanly
// (Close or context cancellation). Call after Updates() is drained.
func (s *PriceStream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Close tears down the stream: it cancels the context, closes the active
// connection, and (once the run loop exits) closes Updates(). Safe to call
// multiple times and concurrently.
func (s *PriceStream) Close() error {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	conn := s.conn
	s.mu.Unlock()

	if conn != nil {
		_ = conn.Close()
	}
	<-s.done
	return nil
}

// run is the connect/read/reconnect loop. It owns the updates channel and
// closes it exactly once on exit.
func (s *PriceStream) run() {
	defer close(s.updates)
	defer s.doneOnce.Do(func() { close(s.done) })

	attempt := 0
	for {
		err := s.connectAndRead()

		// Context cancelled / Close() called: clean shutdown, no error.
		if s.ctx.Err() != nil {
			return
		}

		// Fatal errors (e.g. subscription rejected) are not retried.
		var rejected *StreamRejectedError
		if errors.As(err, &rejected) {
			s.setErr(err)
			return
		}

		if !s.opts.AutoReconnect {
			s.setErr(err)
			return
		}

		if s.opts.MaxReconnectAttempts >= 0 && attempt >= s.opts.MaxReconnectAttempts {
			s.setErr(fmt.Errorf("stream reconnect failed after %d attempt(s): %w", attempt, err))
			return
		}

		delay := s.backoff(attempt)
		attempt++

		select {
		case <-s.ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

// connectAndRead dials, performs the ActionCable handshake, and reads frames
// until the connection drops or the context is cancelled. A successful
// subscription resets the caller's backoff counter via the returned nil-reset
// behaviour: callers treat any return as a disconnect and reconnect.
func (s *PriceStream) connectAndRead() error {
	header := http.Header{}
	header.Set("Authorization", "Token "+s.apiKey)
	header.Set("User-Agent", fmt.Sprintf("oilpriceapi-go/%s", Version))

	conn, err := s.dial(s.ctx, s.url, header)
	if err != nil {
		return err
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = conn.Close()
		return s.ctx.Err()
	}
	s.conn = conn
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		if s.conn == conn {
			s.conn = nil
		}
		s.mu.Unlock()
		_ = conn.Close()
	}()

	// Subscribe to the channel. ActionCable tolerates an early subscribe and
	// replies with confirm_subscription once the connection is established.
	subscribe, _ := json.Marshal(map[string]string{
		"command":    "subscribe",
		"identifier": s.identifier,
	})
	if err := conn.Write(s.ctx, subscribe); err != nil {
		return err
	}

	for {
		data, err := conn.Read(s.ctx)
		if err != nil {
			return err
		}
		if fatal := s.handleFrame(data); fatal != nil {
			return fatal
		}
	}
}

// cableFrame is the ActionCable transport envelope.
type cableFrame struct {
	Type       string          `json:"type"`
	Identifier string          `json:"identifier"`
	Message    json.RawMessage `json:"message"`
}

// handleFrame decodes one ActionCable transport frame and dispatches channel
// messages. It returns a non-nil error only for fatal conditions (subscription
// rejected) that must terminate the stream without reconnecting.
func (s *PriceStream) handleFrame(data []byte) error {
	var frame cableFrame
	if err := json.Unmarshal(data, &frame); err != nil {
		// Ignore malformed frames rather than tear down the stream.
		return nil
	}

	switch frame.Type {
	case "ping", "welcome":
		// Heartbeat / transport handshake — nothing to do.
		return nil
	case "confirm_subscription":
		return nil
	case "reject_subscription":
		return &StreamRejectedError{}
	case "disconnect":
		// Server-initiated disconnect; let the read loop's next Read fail and
		// drive reconnect.
		return nil
	}

	// Channel message: payload lives under "message".
	if len(frame.Message) == 0 {
		return nil
	}
	s.dispatch(frame.Message)
	return nil
}

// messageType peeks at the channel message "type" field.
type messageType struct {
	Type string `json:"type"`
}

// dispatch decodes a channel message payload into a typed Update and delivers
// it (respecting the commodity filter and context cancellation).
func (s *PriceStream) dispatch(raw json.RawMessage) {
	var mt messageType
	_ = json.Unmarshal(raw, &mt)

	update := Update{Type: mt.Type, Raw: raw}

	switch mt.Type {
	case "price_update":
		var pu PriceUpdate
		if err := json.Unmarshal(raw, &pu); err != nil {
			return
		}
		if !s.matchesFilter(&pu) {
			return
		}
		update.Price = &pu
	case "rig_count_update":
		var rc RigCountUpdate
		if err := json.Unmarshal(raw, &rc); err != nil {
			return
		}
		update.RigCount = &rc
	case "welcome":
		var w WelcomeSnapshot
		if err := json.Unmarshal(raw, &w); err != nil {
			return
		}
		update.Welcome = &w
	}

	select {
	case s.updates <- update:
	case <-s.ctx.Done():
	}
}

// matchesFilter reports whether a price_update should be delivered given the
// configured commodity filter.
func (s *PriceStream) matchesFilter(pu *PriceUpdate) bool {
	if s.filter == nil {
		return true
	}
	present := map[string]*StreamedPrice{
		"brent": pu.Prices.Oil.Brent,
		"wti":   pu.Prices.Oil.WTI,
		"uk":    pu.Prices.NaturalGas.UK,
		"us":    pu.Prices.NaturalGas.US,
		"eu":    pu.Prices.NaturalGas.EU,
	}
	for slug := range s.filter {
		if present[slug] != nil {
			return true
		}
	}
	return false
}

// backoff returns the exponential backoff delay for the given attempt, capped
// at MaxReconnectDelay.
func (s *PriceStream) backoff(attempt int) time.Duration {
	delay := s.opts.ReconnectDelay
	for i := 0; i < attempt; i++ {
		delay *= 2
		if delay >= s.opts.MaxReconnectDelay {
			return s.opts.MaxReconnectDelay
		}
	}
	if delay > s.opts.MaxReconnectDelay {
		return s.opts.MaxReconnectDelay
	}
	return delay
}

// setErr records the terminating error if one isn't already set.
func (s *PriceStream) setErr(err error) {
	s.mu.Lock()
	if s.err == nil {
		s.err = err
	}
	s.mu.Unlock()
}
