package oilpriceapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// MaxReconnectAttempts is documented as "the number of consecutive reconnect
// attempts before the stream terminates". The run loop incremented one counter
// and never reset it, so it was really a lifetime disconnect count: a stream
// that recovered fully, ran healthily, and then dropped again hours later
// spent budget it had already earned back, and eventually terminated a working
// subscription.
//
// The boundary chosen for "recovered" is deliberately not the TCP dial — that
// resets on a server that accepts and hangs up, which is an unbounded rapid
// flap. A session counts as healthy only once the ActionCable subscription has
// been confirmed AND at least one further server frame (a ping, or a channel
// message) has arrived on it. That proves a live session beyond the handshake,
// and a server that confirms and immediately drops never reaches it.

// scriptedCable is a cable server whose per-connection behaviour is scripted
// by connection index, so a test can stage healthy sessions followed by
// failures.
type scriptedCable struct {
	upgrader websocket.Upgrader

	connections int32
	subscribes  int32

	// healthySessions is the number of leading connections that complete the
	// handshake and deliver a post-confirmation ping before hanging up.
	// Connections past that are accepted and immediately closed.
	healthySessions int32

	// postConfirmPing controls whether a confirmed session also sends the
	// frame that makes it count as healthy. False models a flapping server
	// that confirms and drops.
	postConfirmPing bool
}

func (s *scriptedCable) handler(w http.ResponseWriter, r *http.Request) {
	n := atomic.AddInt32(&s.connections, 1)

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	if s.healthySessions >= 0 && n > s.healthySessions {
		// Past the scripted healthy sessions: accept and hang up.
		return
	}

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
		if cmd.Command != "subscribe" {
			continue
		}
		atomic.AddInt32(&s.subscribes, 1)
		_ = conn.WriteJSON(map[string]string{"type": "confirm_subscription", "identifier": cmd.Identifier})

		if s.postConfirmPing {
			// The frame that makes this a healthy session rather than a bare
			// handshake.
			_ = conn.WriteJSON(map[string]string{"type": "ping"})
		}
		// Drop the session.
		return
	}
}

func newScriptedStream(t *testing.T, srv *scriptedCable, opts StreamOptions) (*PriceStream, *httptest.Server) {
	t.Helper()
	srv.upgrader.CheckOrigin = func(*http.Request) bool { return true }
	httpSrv := httptest.NewServer(http.HandlerFunc(srv.handler))

	client := NewClient("k", WithBaseURL(httpSrv.URL))
	stream, err := client.newStream(context.Background(), opts, defaultDialer)
	if err != nil {
		httpSrv.Close()
		t.Fatalf("newStream: %v", err)
	}
	return stream, httpSrv
}

// A healthy session must restore the budget. With MaxReconnectAttempts=1 and
// three healthy sessions staged, all three must be reached: each recovery
// earns the budget back. Before the fix the stream terminated after the
// second, having spent a budget that intervening healthy sessions never
// refunded.
func TestStreamHealthySessionResetsReconnectBudget(t *testing.T) {
	srv := &scriptedCable{healthySessions: 3, postConfirmPing: true}
	stream, httpSrv := newScriptedStream(t, srv, StreamOptions{
		AutoReconnect:        true,
		ReconnectDelay:       time.Millisecond,
		MaxReconnectDelay:    5 * time.Millisecond,
		MaxReconnectAttempts: 1,
	})
	defer httpSrv.Close()
	defer stream.Close()

	// Drain until the stream terminates.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-stream.Updates():
			if !ok {
				goto done
			}
		case <-deadline:
			t.Fatal("timeout waiting for the stream to terminate")
		}
	}
done:

	if got := atomic.LoadInt32(&srv.subscribes); got < 3 {
		t.Fatalf("confirmed subscriptions = %d, want the 3 staged healthy sessions; "+
			"the reconnect budget was not restored by a healthy session (terminating error: %v)",
			got, stream.Err())
	}
	if stream.Err() == nil {
		t.Fatal("expected a terminating error once the server stopped recovering")
	}
	if !strings.Contains(stream.Err().Error(), "reconnect failed") {
		t.Fatalf("unexpected terminating error: %v", stream.Err())
	}
}

// The reset must not create an unbounded rapid-flap loop. A server that
// confirms the subscription and immediately drops has not delivered a healthy
// session, so the budget is not restored and the cap still terminates the
// stream.
func TestStreamConfirmThenImmediateDropDoesNotResetBudget(t *testing.T) {
	srv := &scriptedCable{healthySessions: -1, postConfirmPing: false}
	stream, httpSrv := newScriptedStream(t, srv, StreamOptions{
		AutoReconnect:        true,
		ReconnectDelay:       time.Millisecond,
		MaxReconnectDelay:    5 * time.Millisecond,
		MaxReconnectAttempts: 2,
	})
	defer httpSrv.Close()
	defer stream.Close()

	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-stream.Updates():
			if !ok {
				goto done
			}
		case <-deadline:
			t.Fatal("a confirm-then-drop server flapped without ever exhausting the reconnect budget")
		}
	}
done:

	if stream.Err() == nil {
		t.Fatal("expected the reconnect budget to be exhausted by a flapping server")
	}
	// 1 initial session + at most MaxReconnectAttempts recoveries.
	if got := atomic.LoadInt32(&srv.connections); got > 3 {
		t.Fatalf("server saw %d connections, want at most 3 (1 initial + 2 attempts)", got)
	}
}

// A rejected subscription is fatal and must never be retried, healthy-session
// accounting or not.
func TestStreamRejectedSubscriptionStillTerminatesWithoutRetrying(t *testing.T) {
	srv := &cableServer{rejectSubscribe: true}
	stream, httpSrv := newTestStream(t, srv, "k",
		WithStreamMaxReconnectAttempts(5),
		WithStreamReconnectDelay(time.Millisecond),
	)
	defer httpSrv.Close()
	defer stream.Close()

	select {
	case _, ok := <-stream.Updates():
		if ok {
			t.Fatal("did not expect updates from a rejected subscription")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for a rejected subscription to terminate the stream")
	}

	var rejected *StreamRejectedError
	if err := stream.Err(); err == nil || !asRejected(err, &rejected) {
		t.Fatalf("got %v, want *StreamRejectedError", stream.Err())
	}
	if got := atomic.LoadInt32(&srv.subscribeCount); got != 1 {
		t.Fatalf("subscribe attempts = %d, want exactly 1 (a rejection must not be retried)", got)
	}
}

// Close() during the reconnect backoff must stop the loop and leave no
// goroutine behind.
func TestStreamCloseDuringBackoffLeavesNoGoroutine(t *testing.T) {
	before := runtime.NumGoroutine()

	srv := &scriptedCable{healthySessions: 1, postConfirmPing: true}
	stream, httpSrv := newScriptedStream(t, srv, StreamOptions{
		AutoReconnect:        true,
		ReconnectDelay:       500 * time.Millisecond,
		MaxReconnectDelay:    500 * time.Millisecond,
		MaxReconnectAttempts: 10,
	})
	defer httpSrv.Close()

	// Wait until the first session has dropped and the loop is in backoff.
	waitFor(t, func() bool { return atomic.LoadInt32(&srv.connections) >= 2 }, 2*time.Second)

	if err := stream.Close(); err != nil {
		t.Fatalf("Close during backoff returned %v", err)
	}

	if _, ok := <-stream.Updates(); ok {
		t.Fatal("updates channel should be closed after Close")
	}

	// Give the runtime a moment to reap.
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		if runtime.NumGoroutine() <= before+2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if after := runtime.NumGoroutine(); after > before+2 {
		t.Fatalf("goroutines: before=%d after=%d", before, after)
	}
}

func asRejected(err error, target **StreamRejectedError) bool {
	r, ok := err.(*StreamRejectedError)
	if ok {
		*target = r
	}
	return ok
}
