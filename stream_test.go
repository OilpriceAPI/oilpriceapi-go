package oilpriceapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// --- test ActionCable server -------------------------------------------------

// cableServer is a minimal ActionCable-speaking WebSocket server for tests. It
// performs the welcome -> subscribe -> confirm_subscription handshake and lets
// each test script the messages broadcast to the client.
type cableServer struct {
	upgrader websocket.Upgrader

	mu              sync.Mutex
	gotAuthHeader   string
	gotSubscribe    json.RawMessage
	subscribeCount  int32
	rejectSubscribe bool

	// onSubscribed is invoked (in a goroutine) after confirm_subscription is
	// sent, with the live connection, so a test can push channel messages.
	onSubscribed func(conn *websocket.Conn)
}

func (s *cableServer) handler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.gotAuthHeader = r.Header.Get("Authorization")
	s.mu.Unlock()

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// ActionCable transport welcome frame.
	_ = conn.WriteJSON(map[string]string{"type": "welcome"})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var cmd struct {
			Command    string `json:"command"`
			Identifier string `json:"identifier"`
		}
		if err := json.Unmarshal(data, &cmd); err != nil {
			continue
		}
		switch cmd.Command {
		case "subscribe":
			atomic.AddInt32(&s.subscribeCount, 1)
			s.mu.Lock()
			s.gotSubscribe = json.RawMessage(cmd.Identifier)
			reject := s.rejectSubscribe
			cb := s.onSubscribed
			s.mu.Unlock()

			if reject {
				_ = conn.WriteJSON(map[string]string{"type": "reject_subscription", "identifier": cmd.Identifier})
				continue
			}
			_ = conn.WriteJSON(map[string]string{"type": "confirm_subscription", "identifier": cmd.Identifier})
			if cb != nil {
				go cb(conn)
			}
		case "unsubscribe":
			return
		}
	}
}

// writeChannelMessage sends an ActionCable channel message envelope.
func writeChannelMessage(t *testing.T, conn *websocket.Conn, identifier string, message interface{}) {
	t.Helper()
	raw, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}
	env := map[string]interface{}{
		"identifier": identifier,
		"message":    json.RawMessage(raw),
	}
	if err := conn.WriteJSON(env); err != nil {
		t.Logf("write channel message: %v", err)
	}
}

// newTestStream spins up a cable server and returns a started PriceStream
// pointed at it, plus the server for assertions/teardown.
func newTestStream(t *testing.T, srv *cableServer, apiKey string, opts ...StreamOption) (*PriceStream, *httptest.Server) {
	t.Helper()
	if srv.upgrader.CheckOrigin == nil {
		srv.upgrader.CheckOrigin = func(*http.Request) bool { return true }
	}
	httpSrv := httptest.NewServer(http.HandlerFunc(srv.handler))

	client := NewClient(apiKey, WithBaseURL(httpSrv.URL))

	options := StreamOptions{
		AutoReconnect:        true,
		ReconnectDelay:       10 * time.Millisecond,
		MaxReconnectDelay:    50 * time.Millisecond,
		MaxReconnectAttempts: defaultMaxReconnectTries,
	}
	for _, opt := range opts {
		opt(&options)
	}

	stream, err := client.newStream(context.Background(), options, defaultDialer)
	if err != nil {
		t.Fatalf("newStream: %v", err)
	}
	return stream, httpSrv
}

func floatPtr(f float64) *float64 { return &f }

// --- tests -------------------------------------------------------------------

func TestCableURL(t *testing.T) {
	cases := map[string]string{
		"https://api.oilpriceapi.com":  "wss://api.oilpriceapi.com/cable",
		"https://api.oilpriceapi.com/": "wss://api.oilpriceapi.com/cable",
		"http://localhost:5000":        "ws://localhost:5000/cable",
		"http://127.0.0.1:8080/":       "ws://127.0.0.1:8080/cable",
	}
	for in, want := range cases {
		if got := cableURL(in); got != want {
			t.Errorf("cableURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStreamPricesRequiresAPIKey(t *testing.T) {
	client := NewClient("")
	_, err := client.StreamPrices(context.Background())
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if _, ok := err.(*AuthenticationError); !ok {
		t.Fatalf("expected *AuthenticationError, got %T", err)
	}
}

func TestStreamHandshakeAndAuthHeader(t *testing.T) {
	srv := &cableServer{
		onSubscribed: func(conn *websocket.Conn) {
			// keep connection open briefly so test can read welcome path
			time.Sleep(50 * time.Millisecond)
		},
	}
	stream, httpSrv := newTestStream(t, srv, "secret-key")
	defer httpSrv.Close()
	defer stream.Close()

	// Wait for subscribe to land.
	waitFor(t, func() bool { return atomic.LoadInt32(&srv.subscribeCount) >= 1 }, time.Second)

	srv.mu.Lock()
	auth := srv.gotAuthHeader
	sub := srv.gotSubscribe
	srv.mu.Unlock()

	if auth != "Token secret-key" {
		t.Errorf("Authorization header = %q, want %q", auth, "Token secret-key")
	}

	var ident struct {
		Channel string `json:"channel"`
		APIKey  string `json:"api_key"`
	}
	if err := json.Unmarshal(sub, &ident); err != nil {
		t.Fatalf("unmarshal subscribe identifier: %v", err)
	}
	if ident.Channel != energyPricesChannel {
		t.Errorf("identifier channel = %q, want %q", ident.Channel, energyPricesChannel)
	}
	if ident.APIKey != "secret-key" {
		t.Errorf("identifier api_key = %q, want %q", ident.APIKey, "secret-key")
	}
}

func TestStreamDispatchPriceUpdate(t *testing.T) {
	srv := &cableServer{
		onSubscribed: func(conn *websocket.Conn) {
			writeChannelMessage(t, conn, "id", PriceUpdate{
				Type:         "price_update",
				Timestamp:    "2026-06-20T00:00:00Z",
				BaseCurrency: "USD",
				BaseUnit:     "MMBtu",
				Prices: StreamedPriceMap{
					Oil: StreamedOilPrices{
						WTI: &StreamedPrice{OriginalPrice: floatPtr(72.5), OriginalUnit: "barrel_oil", OriginalCurrency: "USD"},
					},
				},
			})
		},
	}
	stream, httpSrv := newTestStream(t, srv, "k")
	defer httpSrv.Close()
	defer stream.Close()

	update := readUpdate(t, stream, 2*time.Second)
	if update.Type != "price_update" {
		t.Fatalf("update.Type = %q, want price_update", update.Type)
	}
	if update.Price == nil {
		t.Fatal("update.Price is nil")
	}
	wti := update.Price.Prices.Oil.WTI
	if wti == nil || wti.OriginalPrice == nil || *wti.OriginalPrice != 72.5 {
		t.Fatalf("WTI original price mismatch: %+v", wti)
	}
}

func TestStreamDispatchWelcomeAndRigCount(t *testing.T) {
	srv := &cableServer{
		onSubscribed: func(conn *websocket.Conn) {
			writeChannelMessage(t, conn, "id", WelcomeSnapshot{
				Type: "welcome",
				Data: WelcomeSnapshotData{Timestamp: "2026-06-20T00:00:00Z", BaseCurrency: "USD"},
			})
			writeChannelMessage(t, conn, "id", RigCountUpdate{
				Type:      "rig_count_update",
				Timestamp: "2026-06-20T00:00:00Z",
				RigCount:  RigCount{Code: "US_RIG_COUNT", Region: "United States", Count: 588, Source: "baker_hughes"},
			})
		},
	}
	stream, httpSrv := newTestStream(t, srv, "k")
	defer httpSrv.Close()
	defer stream.Close()

	first := readUpdate(t, stream, 2*time.Second)
	if first.Type != "welcome" || first.Welcome == nil {
		t.Fatalf("first update not welcome: %+v", first)
	}
	if first.Welcome.Data.BaseCurrency != "USD" {
		t.Errorf("welcome base currency = %q", first.Welcome.Data.BaseCurrency)
	}

	second := readUpdate(t, stream, 2*time.Second)
	if second.Type != "rig_count_update" || second.RigCount == nil {
		t.Fatalf("second update not rig_count_update: %+v", second)
	}
	if second.RigCount.RigCount.Count != 588 {
		t.Errorf("rig count = %v, want 588", second.RigCount.RigCount.Count)
	}
}

func TestStreamCommodityFilter(t *testing.T) {
	srv := &cableServer{
		onSubscribed: func(conn *websocket.Conn) {
			// This one should be filtered out (only natural gas present, filter wants brent).
			writeChannelMessage(t, conn, "id", PriceUpdate{
				Type:   "price_update",
				Prices: StreamedPriceMap{NaturalGas: StreamedNaturalGasPrices{US: &StreamedPrice{OriginalPrice: floatPtr(2.1)}}},
			})
			// This one should pass (brent present).
			writeChannelMessage(t, conn, "id", PriceUpdate{
				Type:   "price_update",
				Prices: StreamedPriceMap{Oil: StreamedOilPrices{Brent: &StreamedPrice{OriginalPrice: floatPtr(78.0)}}},
			})
		},
	}
	// Filter using the upstream code form to exercise the code->slug mapping.
	stream, httpSrv := newTestStream(t, srv, "k", WithStreamCommodities("BRENT_CRUDE_USD"))
	defer httpSrv.Close()
	defer stream.Close()

	update := readUpdate(t, stream, 2*time.Second)
	if update.Price == nil || update.Price.Prices.Oil.Brent == nil {
		t.Fatalf("expected brent update to pass filter, got %+v", update)
	}
	if *update.Price.Prices.Oil.Brent.OriginalPrice != 78.0 {
		t.Errorf("brent price = %v", *update.Price.Prices.Oil.Brent.OriginalPrice)
	}
}

func TestStreamSubscriptionRejected(t *testing.T) {
	srv := &cableServer{rejectSubscribe: true}
	stream, httpSrv := newTestStream(t, srv, "free-tier-key", WithStreamAutoReconnect(false))
	defer httpSrv.Close()
	defer stream.Close()

	// Updates channel should close without delivering anything.
	select {
	case u, ok := <-stream.Updates():
		if ok {
			t.Fatalf("expected no updates on rejected subscription, got %+v", u)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for stream to terminate")
	}

	err := stream.Err()
	if _, ok := err.(*StreamRejectedError); !ok {
		t.Fatalf("expected *StreamRejectedError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "account entitlement") {
		t.Errorf("rejection error should include recovery guidance, got %q", err.Error())
	}
}

func TestStreamReconnect(t *testing.T) {
	var connections int32
	srv := &cableServer{
		onSubscribed: func(conn *websocket.Conn) {
			n := atomic.AddInt32(&connections, 1)
			if n == 1 {
				// First connection: drop immediately to force a reconnect.
				conn.Close()
				return
			}
			// Second connection: deliver a message so the test can confirm recovery.
			writeChannelMessage(t, conn, "id", PriceUpdate{
				Type:   "price_update",
				Prices: StreamedPriceMap{Oil: StreamedOilPrices{WTI: &StreamedPrice{OriginalPrice: floatPtr(70.0)}}},
			})
			time.Sleep(50 * time.Millisecond)
		},
	}
	stream, httpSrv := newTestStream(t, srv, "k")
	defer httpSrv.Close()
	defer stream.Close()

	update := readUpdate(t, stream, 3*time.Second)
	if update.Price == nil || update.Price.Prices.Oil.WTI == nil {
		t.Fatalf("expected price update after reconnect, got %+v", update)
	}
	if atomic.LoadInt32(&connections) < 2 {
		t.Errorf("expected at least 2 connection attempts (reconnect), got %d", connections)
	}
}

func TestStreamContextCancelTeardown(t *testing.T) {
	srv := &cableServer{
		onSubscribed: func(conn *websocket.Conn) {
			time.Sleep(2 * time.Second) // hold the connection open
		},
	}
	srv.upgrader.CheckOrigin = func(*http.Request) bool { return true }
	httpSrv := httptest.NewServer(http.HandlerFunc(srv.handler))
	defer httpSrv.Close()

	client := NewClient("k", WithBaseURL(httpSrv.URL))
	ctx, cancel := context.WithCancel(context.Background())

	stream, err := client.newStream(ctx, StreamOptions{
		AutoReconnect:        true,
		ReconnectDelay:       10 * time.Millisecond,
		MaxReconnectDelay:    50 * time.Millisecond,
		MaxReconnectAttempts: defaultMaxReconnectTries,
	}, defaultDialer)
	if err != nil {
		t.Fatalf("newStream: %v", err)
	}

	waitFor(t, func() bool { return atomic.LoadInt32(&srv.subscribeCount) >= 1 }, time.Second)

	cancel()

	// Updates channel must close promptly, with no terminating error (clean).
	select {
	case _, ok := <-stream.Updates():
		if ok {
			// drain any in-flight update, then expect close next
			select {
			case _, ok2 := <-stream.Updates():
				if ok2 {
					t.Fatal("expected channel to close after context cancel")
				}
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for channel close after cancel")
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: Updates channel not closed after context cancel")
	}

	if err := stream.Err(); err != nil {
		t.Errorf("expected nil Err() on context cancel, got %v", err)
	}
}

func TestStreamCloseIsIdempotent(t *testing.T) {
	srv := &cableServer{
		onSubscribed: func(conn *websocket.Conn) { time.Sleep(time.Second) },
	}
	stream, httpSrv := newTestStream(t, srv, "k")
	defer httpSrv.Close()

	waitFor(t, func() bool { return atomic.LoadInt32(&srv.subscribeCount) >= 1 }, time.Second)

	if err := stream.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}

	// Updates must be closed.
	select {
	case _, ok := <-stream.Updates():
		if ok {
			// drain then confirm closed
			<-stream.Updates()
		}
	case <-time.After(time.Second):
		t.Fatal("Updates not closed after Close")
	}
}

func TestStreamMaxReconnectAttemptsExhausted(t *testing.T) {
	// Point at a closed server so every dial fails, then assert the stream
	// gives up after MaxReconnectAttempts with an error.
	httpSrv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	addr := httpSrv.URL
	httpSrv.Close() // nothing is listening now

	client := NewClient("k", WithBaseURL(addr))
	stream, err := client.newStream(context.Background(), StreamOptions{
		AutoReconnect:        true,
		ReconnectDelay:       1 * time.Millisecond,
		MaxReconnectDelay:    5 * time.Millisecond,
		MaxReconnectAttempts: 2,
	}, defaultDialer)
	if err != nil {
		t.Fatalf("newStream: %v", err)
	}
	defer stream.Close()

	select {
	case _, ok := <-stream.Updates():
		if ok {
			t.Fatal("did not expect updates from an unreachable server")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for reconnect exhaustion")
	}

	if stream.Err() == nil {
		t.Error("expected an error after reconnect attempts exhausted")
	}
	if !strings.Contains(stream.Err().Error(), "reconnect failed") {
		t.Errorf("unexpected error: %v", stream.Err())
	}
}

func TestStreamBackoff(t *testing.T) {
	s := &PriceStream{opts: StreamOptions{ReconnectDelay: time.Second, MaxReconnectDelay: 30 * time.Second}}
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{5, 30 * time.Second}, // capped
		{10, 30 * time.Second},
	}
	for _, c := range cases {
		if got := s.backoff(c.attempt); got != c.want {
			t.Errorf("backoff(%d) = %v, want %v", c.attempt, got, c.want)
		}
	}
}

func TestStreamPricesPublicEntryAndOptions(t *testing.T) {
	srv := &cableServer{
		onSubscribed: func(conn *websocket.Conn) {
			writeChannelMessage(t, conn, "id", PriceUpdate{
				Type:   "price_update",
				Prices: StreamedPriceMap{Oil: StreamedOilPrices{WTI: &StreamedPrice{OriginalPrice: floatPtr(71.0)}}},
			})
			time.Sleep(50 * time.Millisecond)
		},
	}
	srv.upgrader.CheckOrigin = func(*http.Request) bool { return true }
	httpSrv := httptest.NewServer(http.HandlerFunc(srv.handler))
	defer httpSrv.Close()

	client := NewClient("k", WithBaseURL(httpSrv.URL))

	// Exercise the public StreamPrices entry point and every option setter.
	stream, err := client.StreamPrices(context.Background(),
		WithStreamCommodities("WTI_USD"),
		WithStreamAutoReconnect(true),
		WithStreamReconnectDelay(5*time.Millisecond),
		WithStreamMaxReconnectDelay(20*time.Millisecond),
		WithStreamMaxReconnectAttempts(3),
	)
	if err != nil {
		t.Fatalf("StreamPrices: %v", err)
	}
	defer stream.Close()

	update := readUpdate(t, stream, 2*time.Second)
	if update.Price == nil || update.Price.Prices.Oil.WTI == nil {
		t.Fatalf("expected WTI update, got %+v", update)
	}
}

func TestStreamIgnoresMalformedFrame(t *testing.T) {
	srv := &cableServer{
		onSubscribed: func(conn *websocket.Conn) {
			_ = conn.WriteMessage(websocket.TextMessage, []byte("{not json"))
			writeChannelMessage(t, conn, "id", PriceUpdate{
				Type:   "price_update",
				Prices: StreamedPriceMap{Oil: StreamedOilPrices{WTI: &StreamedPrice{OriginalPrice: floatPtr(65.0)}}},
			})
		},
	}
	stream, httpSrv := newTestStream(t, srv, "k")
	defer httpSrv.Close()
	defer stream.Close()

	update := readUpdate(t, stream, 2*time.Second)
	if update.Price == nil {
		t.Fatalf("expected price update after malformed frame, got %+v", update)
	}
}

// --- helpers -----------------------------------------------------------------

func readUpdate(t *testing.T, stream *PriceStream, timeout time.Duration) Update {
	t.Helper()
	select {
	case u, ok := <-stream.Updates():
		if !ok {
			t.Fatalf("stream closed unexpectedly: %v", stream.Err())
		}
		return u
	case <-time.After(timeout):
		t.Fatalf("timeout waiting for update (err: %v)", stream.Err())
		return Update{}
	}
}

func waitFor(t *testing.T, cond func() bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}
