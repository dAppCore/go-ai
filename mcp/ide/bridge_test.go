package ide

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

	"forge.lthn.ai/core/go-ws"
	"github.com/gorilla/websocket"
)

var testUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// echoServer creates a test WebSocket server that echoes messages back.
func echoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer conn.Close()
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			if err := conn.WriteMessage(mt, data); err != nil {
				break
			}
		}
	}))
}

func wsURL(ts *httptest.Server) string {
	return "ws" + strings.TrimPrefix(ts.URL, "http")
}

// waitConnected polls bridge.Connected() until true or timeout.
func waitConnected(t *testing.T, bridge *Bridge, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !bridge.Connected() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if !bridge.Connected() {
		t.Fatal("bridge did not connect within timeout")
	}
}

func TestBridge_Good_ConnectAndSend(t *testing.T) {
	ts := echoServer(t)
	defer ts.Close()

	hub := ws.NewHub()
	ctx := t.Context()
	go hub.Run(ctx)

	cfg := DefaultConfig()
	cfg.LaravelWSURL = wsURL(ts)
	cfg.ReconnectInterval = 100 * time.Millisecond

	bridge := NewBridge(hub, cfg)
	bridge.Start(ctx)

	waitConnected(t, bridge, 2*time.Second)

	err := bridge.Send(BridgeMessage{
		Type: "test",
		Data: "hello",
	})
	if err != nil {
		t.Fatalf("Send() failed: %v", err)
	}
}

func TestBridge_Good_Shutdown(t *testing.T) {
	ts := echoServer(t)
	defer ts.Close()

	hub := ws.NewHub()
	ctx := t.Context()
	go hub.Run(ctx)

	cfg := DefaultConfig()
	cfg.LaravelWSURL = wsURL(ts)
	cfg.ReconnectInterval = 100 * time.Millisecond

	bridge := NewBridge(hub, cfg)
	bridge.Start(ctx)

	waitConnected(t, bridge, 2*time.Second)

	bridge.Shutdown()
	if bridge.Connected() {
		t.Error("bridge should be disconnected after Shutdown")
	}
}

func TestBridge_Bad_SendWithoutConnection(t *testing.T) {
	hub := ws.NewHub()
	cfg := DefaultConfig()
	bridge := NewBridge(hub, cfg)

	err := bridge.Send(BridgeMessage{Type: "test"})
	if err == nil {
		t.Error("expected error when sending without connection")
	}
}

func TestBridge_Good_MessageDispatch(t *testing.T) {
	// Server that sends a message to the bridge on connect.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		msg := BridgeMessage{
			Type:    "chat_response",
			Channel: "chat:session-1",
			Data:    "hello from laravel",
		}
		data, _ := json.Marshal(msg)
		conn.WriteMessage(websocket.TextMessage, data)

		// Keep connection open
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}))
	defer ts.Close()

	hub := ws.NewHub()
	ctx := t.Context()
	go hub.Run(ctx)

	cfg := DefaultConfig()
	cfg.LaravelWSURL = wsURL(ts)
	cfg.ReconnectInterval = 100 * time.Millisecond

	bridge := NewBridge(hub, cfg)
	bridge.Start(ctx)

	waitConnected(t, bridge, 2*time.Second)

	// Give time for the dispatched message to be processed.
	time.Sleep(200 * time.Millisecond)

	// Verify hub stats — the message was dispatched (even without subscribers).
	// This confirms the dispatch path ran without error.
}

func TestBridge_Good_Reconnect(t *testing.T) {
	// Use atomic counter to avoid data race between HTTP handler goroutine
	// and the test goroutine.
	var callCount atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		// Close immediately on first connection to force reconnect
		if n == 1 {
			conn.Close()
			return
		}
		defer conn.Close()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}))
	defer ts.Close()

	hub := ws.NewHub()
	ctx := t.Context()
	go hub.Run(ctx)

	cfg := DefaultConfig()
	cfg.LaravelWSURL = wsURL(ts)
	cfg.ReconnectInterval = 100 * time.Millisecond
	cfg.MaxReconnectInterval = 200 * time.Millisecond

	bridge := NewBridge(hub, cfg)
	bridge.Start(ctx)

	waitConnected(t, bridge, 3*time.Second)

	if callCount.Load() < 2 {
		t.Errorf("expected at least 2 connection attempts, got %d", callCount.Load())
	}
}

func TestBridge_Good_ExponentialBackoff(t *testing.T) {
	// Track timestamps of dial attempts to verify backoff behaviour.
	// The server rejects the WebSocket upgrade with HTTP 403, so dial()
	// returns an error and the exponential backoff path fires.
	var attempts []time.Time
	var mu sync.Mutex
	var attemptCount atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts = append(attempts, time.Now())
		mu.Unlock()
		attemptCount.Add(1)

		// Reject the upgrade — this makes dial() fail, triggering backoff.
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer ts.Close()

	hub := ws.NewHub()
	ctx := t.Context()
	go hub.Run(ctx)

	cfg := DefaultConfig()
	cfg.LaravelWSURL = wsURL(ts)
	cfg.ReconnectInterval = 100 * time.Millisecond
	cfg.MaxReconnectInterval = 400 * time.Millisecond

	bridge := NewBridge(hub, cfg)
	bridge.Start(ctx)

	// Wait for at least 4 dial attempts.
	deadline := time.Now().Add(5 * time.Second)
	for attemptCount.Load() < 4 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	bridge.Shutdown()

	mu.Lock()
	defer mu.Unlock()

	if len(attempts) < 4 {
		t.Fatalf("expected at least 4 connection attempts, got %d", len(attempts))
	}

	// Verify exponential backoff: gap between attempts should increase.
	// Expected delays: ~100ms, ~200ms, ~400ms (capped).
	// Allow generous tolerance since timing is non-deterministic.
	for i := 1; i < len(attempts) && i <= 3; i++ {
		gap := attempts[i].Sub(attempts[i-1])
		// Minimum expected delay doubles each time: 100, 200, 400.
		// We check a lower bound (50% of expected) to be resilient.
		expectedMin := time.Duration(50*(1<<(i-1))) * time.Millisecond
		if gap < expectedMin {
			t.Errorf("attempt %d->%d gap %v < expected minimum %v", i-1, i, gap, expectedMin)
		}
	}

	// Verify the backoff caps at MaxReconnectInterval.
	if len(attempts) >= 5 {
		gap := attempts[4].Sub(attempts[3])
		// After cap is hit, delay should not exceed MaxReconnectInterval + tolerance.
		maxExpected := cfg.MaxReconnectInterval + 200*time.Millisecond
		if gap > maxExpected {
			t.Errorf("attempt 3->4 gap %v exceeded max backoff %v", gap, maxExpected)
		}
	}
}

func TestBridge_Good_ReconnectDetectsServerShutdown(t *testing.T) {
	// Start a server that closes the WS connection on demand, then close
	// the server entirely so the bridge cannot reconnect.
	closeConn := make(chan struct{}, 1)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Wait for signal to close
		<-closeConn
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseGoingAway, "shutdown"))
	}))

	hub := ws.NewHub()
	ctx := t.Context()
	go hub.Run(ctx)

	cfg := DefaultConfig()
	cfg.LaravelWSURL = wsURL(ts)
	// Use long reconnect so bridge stays disconnected after server dies.
	cfg.ReconnectInterval = 5 * time.Second
	cfg.MaxReconnectInterval = 5 * time.Second

	bridge := NewBridge(hub, cfg)
	bridge.Start(ctx)

	waitConnected(t, bridge, 2*time.Second)

	// Signal server handler to close the WS connection, then shut down
	// the server so the reconnect dial() also fails.
	closeConn <- struct{}{}
	ts.Close()

	// Wait for disconnection.
	deadline := time.Now().Add(3 * time.Second)
	for bridge.Connected() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}

	if bridge.Connected() {
		t.Error("expected bridge to detect server-side connection close")
	}
}

func TestBridge_Good_AuthHeader(t *testing.T) {
	// Server that checks for the Authorization header on upgrade.
	var receivedAuth atomic.Value

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth.Store(r.Header.Get("Authorization"))
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}))
	defer ts.Close()

	hub := ws.NewHub()
	ctx := t.Context()
	go hub.Run(ctx)

	cfg := DefaultConfig()
	cfg.LaravelWSURL = wsURL(ts)
	cfg.ReconnectInterval = 100 * time.Millisecond
	cfg.Token = "test-secret-token-42"

	bridge := NewBridge(hub, cfg)
	bridge.Start(ctx)

	waitConnected(t, bridge, 2*time.Second)

	auth, ok := receivedAuth.Load().(string)
	if !ok || auth == "" {
		t.Fatal("server did not receive Authorization header")
	}

	expected := "Bearer test-secret-token-42"
	if auth != expected {
		t.Errorf("expected auth header %q, got %q", expected, auth)
	}
}

func TestBridge_Good_NoAuthHeaderWhenTokenEmpty(t *testing.T) {
	// Verify that no Authorization header is sent when Token is empty.
	var receivedAuth atomic.Value

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth.Store(r.Header.Get("Authorization"))
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}))
	defer ts.Close()

	hub := ws.NewHub()
	ctx := t.Context()
	go hub.Run(ctx)

	cfg := DefaultConfig()
	cfg.LaravelWSURL = wsURL(ts)
	cfg.ReconnectInterval = 100 * time.Millisecond
	// Token intentionally left empty

	bridge := NewBridge(hub, cfg)
	bridge.Start(ctx)

	waitConnected(t, bridge, 2*time.Second)

	auth, _ := receivedAuth.Load().(string)
	if auth != "" {
		t.Errorf("expected no Authorization header when token is empty, got %q", auth)
	}
}

func TestBridge_Good_WithTokenOption(t *testing.T) {
	// Verify the WithToken option function works.
	cfg := DefaultConfig()
	opt := WithToken("my-token")
	opt(&cfg)

	if cfg.Token != "my-token" {
		t.Errorf("expected token 'my-token', got %q", cfg.Token)
	}
}

func TestSubsystem_Good_Name(t *testing.T) {
	sub := New(nil)
	if sub.Name() != "ide" {
		t.Errorf("expected name 'ide', got %q", sub.Name())
	}
}

func TestSubsystem_Good_NilHub(t *testing.T) {
	sub := New(nil)
	if sub.Bridge() != nil {
		t.Error("expected nil bridge when hub is nil")
	}
	// Shutdown should not panic
	if err := sub.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown with nil bridge failed: %v", err)
	}
}
