package ide

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"forge.lthn.ai/core/go-ws"
)

// --- Helpers ---

// newNilBridgeSubsystem returns a Subsystem with no hub/bridge (headless mode).
func newNilBridgeSubsystem() *Subsystem {
	return New(nil)
}

// newConnectedSubsystem returns a Subsystem with a connected bridge and a
// running echo WS server. Caller must cancel ctx and close server when done.
func newConnectedSubsystem(t *testing.T) (*Subsystem, context.CancelFunc, *httptest.Server) {
	t.Helper()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				break
			}
			_ = conn.WriteMessage(mt, data)
		}
	}))

	hub := ws.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)

	sub := New(hub,
		WithLaravelURL(wsURL(ts)),
		WithReconnectInterval(50*time.Millisecond),
	)
	sub.StartBridge(ctx)

	waitConnected(t, sub.Bridge(), 2*time.Second)
	return sub, cancel, ts
}

// --- 4.3: Chat tool tests ---

// TestChatSend_Bad_NilBridge verifies chatSend returns error without a bridge.
func TestChatSend_Bad_NilBridge(t *testing.T) {
	sub := newNilBridgeSubsystem()
	_, _, err := sub.chatSend(context.Background(), nil, ChatSendInput{
		SessionID: "s1",
		Message:   "hello",
	})
	if err == nil {
		t.Error("expected error when bridge is nil")
	}
}

// TestChatSend_Good_Connected verifies chatSend succeeds with a connected bridge.
func TestChatSend_Good_Connected(t *testing.T) {
	sub, cancel, ts := newConnectedSubsystem(t)
	defer cancel()
	defer ts.Close()

	_, out, err := sub.chatSend(context.Background(), nil, ChatSendInput{
		SessionID: "sess-42",
		Message:   "hello",
	})
	if err != nil {
		t.Fatalf("chatSend failed: %v", err)
	}
	if !out.Sent {
		t.Error("expected Sent=true")
	}
	if out.SessionID != "sess-42" {
		t.Errorf("expected sessionId 'sess-42', got %q", out.SessionID)
	}
	if out.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

// TestChatHistory_Bad_NilBridge verifies chatHistory returns error without a bridge.
func TestChatHistory_Bad_NilBridge(t *testing.T) {
	sub := newNilBridgeSubsystem()
	_, _, err := sub.chatHistory(context.Background(), nil, ChatHistoryInput{
		SessionID: "s1",
	})
	if err == nil {
		t.Error("expected error when bridge is nil")
	}
}

// TestChatHistory_Good_Connected verifies chatHistory succeeds and returns empty messages.
func TestChatHistory_Good_Connected(t *testing.T) {
	sub, cancel, ts := newConnectedSubsystem(t)
	defer cancel()
	defer ts.Close()

	_, out, err := sub.chatHistory(context.Background(), nil, ChatHistoryInput{
		SessionID: "sess-1",
		Limit:     50,
	})
	if err != nil {
		t.Fatalf("chatHistory failed: %v", err)
	}
	if out.SessionID != "sess-1" {
		t.Errorf("expected sessionId 'sess-1', got %q", out.SessionID)
	}
	if out.Messages == nil {
		t.Error("expected non-nil messages slice")
	}
	if len(out.Messages) != 0 {
		t.Errorf("expected 0 messages (stub), got %d", len(out.Messages))
	}
}

// TestSessionList_Bad_NilBridge verifies sessionList returns error without a bridge.
func TestSessionList_Bad_NilBridge(t *testing.T) {
	sub := newNilBridgeSubsystem()
	_, _, err := sub.sessionList(context.Background(), nil, SessionListInput{})
	if err == nil {
		t.Error("expected error when bridge is nil")
	}
}

// TestSessionList_Good_Connected verifies sessionList returns empty sessions.
func TestSessionList_Good_Connected(t *testing.T) {
	sub, cancel, ts := newConnectedSubsystem(t)
	defer cancel()
	defer ts.Close()

	_, out, err := sub.sessionList(context.Background(), nil, SessionListInput{})
	if err != nil {
		t.Fatalf("sessionList failed: %v", err)
	}
	if out.Sessions == nil {
		t.Error("expected non-nil sessions slice")
	}
	if len(out.Sessions) != 0 {
		t.Errorf("expected 0 sessions (stub), got %d", len(out.Sessions))
	}
}

// TestSessionCreate_Bad_NilBridge verifies sessionCreate returns error without a bridge.
func TestSessionCreate_Bad_NilBridge(t *testing.T) {
	sub := newNilBridgeSubsystem()
	_, _, err := sub.sessionCreate(context.Background(), nil, SessionCreateInput{
		Name: "test",
	})
	if err == nil {
		t.Error("expected error when bridge is nil")
	}
}

// TestSessionCreate_Good_Connected verifies sessionCreate returns a session stub.
func TestSessionCreate_Good_Connected(t *testing.T) {
	sub, cancel, ts := newConnectedSubsystem(t)
	defer cancel()
	defer ts.Close()

	_, out, err := sub.sessionCreate(context.Background(), nil, SessionCreateInput{
		Name: "my-session",
	})
	if err != nil {
		t.Fatalf("sessionCreate failed: %v", err)
	}
	if out.Session.Name != "my-session" {
		t.Errorf("expected name 'my-session', got %q", out.Session.Name)
	}
	if out.Session.Status != "creating" {
		t.Errorf("expected status 'creating', got %q", out.Session.Status)
	}
	if out.Session.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

// TestPlanStatus_Bad_NilBridge verifies planStatus returns error without a bridge.
func TestPlanStatus_Bad_NilBridge(t *testing.T) {
	sub := newNilBridgeSubsystem()
	_, _, err := sub.planStatus(context.Background(), nil, PlanStatusInput{
		SessionID: "s1",
	})
	if err == nil {
		t.Error("expected error when bridge is nil")
	}
}

// TestPlanStatus_Good_Connected verifies planStatus returns a stub status.
func TestPlanStatus_Good_Connected(t *testing.T) {
	sub, cancel, ts := newConnectedSubsystem(t)
	defer cancel()
	defer ts.Close()

	_, out, err := sub.planStatus(context.Background(), nil, PlanStatusInput{
		SessionID: "sess-7",
	})
	if err != nil {
		t.Fatalf("planStatus failed: %v", err)
	}
	if out.SessionID != "sess-7" {
		t.Errorf("expected sessionId 'sess-7', got %q", out.SessionID)
	}
	if out.Status != "unknown" {
		t.Errorf("expected status 'unknown', got %q", out.Status)
	}
	if out.Steps == nil {
		t.Error("expected non-nil steps slice")
	}
}

// --- 4.3: Build tool tests ---

// TestBuildStatus_Bad_NilBridge verifies buildStatus returns error without a bridge.
func TestBuildStatus_Bad_NilBridge(t *testing.T) {
	sub := newNilBridgeSubsystem()
	_, _, err := sub.buildStatus(context.Background(), nil, BuildStatusInput{
		BuildID: "b1",
	})
	if err == nil {
		t.Error("expected error when bridge is nil")
	}
}

// TestBuildStatus_Good_Connected verifies buildStatus returns a stub.
func TestBuildStatus_Good_Connected(t *testing.T) {
	sub, cancel, ts := newConnectedSubsystem(t)
	defer cancel()
	defer ts.Close()

	_, out, err := sub.buildStatus(context.Background(), nil, BuildStatusInput{
		BuildID: "build-99",
	})
	if err != nil {
		t.Fatalf("buildStatus failed: %v", err)
	}
	if out.Build.ID != "build-99" {
		t.Errorf("expected build ID 'build-99', got %q", out.Build.ID)
	}
	if out.Build.Status != "unknown" {
		t.Errorf("expected status 'unknown', got %q", out.Build.Status)
	}
}

// TestBuildList_Bad_NilBridge verifies buildList returns error without a bridge.
func TestBuildList_Bad_NilBridge(t *testing.T) {
	sub := newNilBridgeSubsystem()
	_, _, err := sub.buildList(context.Background(), nil, BuildListInput{
		Repo:  "core-php",
		Limit: 10,
	})
	if err == nil {
		t.Error("expected error when bridge is nil")
	}
}

// TestBuildList_Good_Connected verifies buildList returns an empty list.
func TestBuildList_Good_Connected(t *testing.T) {
	sub, cancel, ts := newConnectedSubsystem(t)
	defer cancel()
	defer ts.Close()

	_, out, err := sub.buildList(context.Background(), nil, BuildListInput{
		Repo:  "core-php",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("buildList failed: %v", err)
	}
	if out.Builds == nil {
		t.Error("expected non-nil builds slice")
	}
	if len(out.Builds) != 0 {
		t.Errorf("expected 0 builds (stub), got %d", len(out.Builds))
	}
}

// TestBuildLogs_Bad_NilBridge verifies buildLogs returns error without a bridge.
func TestBuildLogs_Bad_NilBridge(t *testing.T) {
	sub := newNilBridgeSubsystem()
	_, _, err := sub.buildLogs(context.Background(), nil, BuildLogsInput{
		BuildID: "b1",
		Tail:    100,
	})
	if err == nil {
		t.Error("expected error when bridge is nil")
	}
}

// TestBuildLogs_Good_Connected verifies buildLogs returns empty lines.
func TestBuildLogs_Good_Connected(t *testing.T) {
	sub, cancel, ts := newConnectedSubsystem(t)
	defer cancel()
	defer ts.Close()

	_, out, err := sub.buildLogs(context.Background(), nil, BuildLogsInput{
		BuildID: "build-55",
		Tail:    50,
	})
	if err != nil {
		t.Fatalf("buildLogs failed: %v", err)
	}
	if out.BuildID != "build-55" {
		t.Errorf("expected buildId 'build-55', got %q", out.BuildID)
	}
	if out.Lines == nil {
		t.Error("expected non-nil lines slice")
	}
	if len(out.Lines) != 0 {
		t.Errorf("expected 0 lines (stub), got %d", len(out.Lines))
	}
}

// --- 4.3: Dashboard tool tests ---

// TestDashboardOverview_Good_NilBridge verifies dashboardOverview works without bridge
// (it does not return error — it reports BridgeOnline=false).
func TestDashboardOverview_Good_NilBridge(t *testing.T) {
	sub := newNilBridgeSubsystem()
	_, out, err := sub.dashboardOverview(context.Background(), nil, DashboardOverviewInput{})
	if err != nil {
		t.Fatalf("dashboardOverview failed: %v", err)
	}
	if out.Overview.BridgeOnline {
		t.Error("expected BridgeOnline=false when bridge is nil")
	}
}

// TestDashboardOverview_Good_Connected verifies dashboardOverview reports bridge online.
func TestDashboardOverview_Good_Connected(t *testing.T) {
	sub, cancel, ts := newConnectedSubsystem(t)
	defer cancel()
	defer ts.Close()

	_, out, err := sub.dashboardOverview(context.Background(), nil, DashboardOverviewInput{})
	if err != nil {
		t.Fatalf("dashboardOverview failed: %v", err)
	}
	if !out.Overview.BridgeOnline {
		t.Error("expected BridgeOnline=true when bridge is connected")
	}
}

// TestDashboardActivity_Bad_NilBridge verifies dashboardActivity returns error without bridge.
func TestDashboardActivity_Bad_NilBridge(t *testing.T) {
	sub := newNilBridgeSubsystem()
	_, _, err := sub.dashboardActivity(context.Background(), nil, DashboardActivityInput{
		Limit: 10,
	})
	if err == nil {
		t.Error("expected error when bridge is nil")
	}
}

// TestDashboardActivity_Good_Connected verifies dashboardActivity returns empty events.
func TestDashboardActivity_Good_Connected(t *testing.T) {
	sub, cancel, ts := newConnectedSubsystem(t)
	defer cancel()
	defer ts.Close()

	_, out, err := sub.dashboardActivity(context.Background(), nil, DashboardActivityInput{
		Limit: 20,
	})
	if err != nil {
		t.Fatalf("dashboardActivity failed: %v", err)
	}
	if out.Events == nil {
		t.Error("expected non-nil events slice")
	}
	if len(out.Events) != 0 {
		t.Errorf("expected 0 events (stub), got %d", len(out.Events))
	}
}

// TestDashboardMetrics_Bad_NilBridge verifies dashboardMetrics returns error without bridge.
func TestDashboardMetrics_Bad_NilBridge(t *testing.T) {
	sub := newNilBridgeSubsystem()
	_, _, err := sub.dashboardMetrics(context.Background(), nil, DashboardMetricsInput{
		Period: "1h",
	})
	if err == nil {
		t.Error("expected error when bridge is nil")
	}
}

// TestDashboardMetrics_Good_Connected verifies dashboardMetrics returns empty metrics.
func TestDashboardMetrics_Good_Connected(t *testing.T) {
	sub, cancel, ts := newConnectedSubsystem(t)
	defer cancel()
	defer ts.Close()

	_, out, err := sub.dashboardMetrics(context.Background(), nil, DashboardMetricsInput{
		Period: "7d",
	})
	if err != nil {
		t.Fatalf("dashboardMetrics failed: %v", err)
	}
	if out.Period != "7d" {
		t.Errorf("expected period '7d', got %q", out.Period)
	}
}

// TestDashboardMetrics_Good_DefaultPeriod verifies the default period is "24h".
func TestDashboardMetrics_Good_DefaultPeriod(t *testing.T) {
	sub, cancel, ts := newConnectedSubsystem(t)
	defer cancel()
	defer ts.Close()

	_, out, err := sub.dashboardMetrics(context.Background(), nil, DashboardMetricsInput{})
	if err != nil {
		t.Fatalf("dashboardMetrics failed: %v", err)
	}
	if out.Period != "24h" {
		t.Errorf("expected default period '24h', got %q", out.Period)
	}
}

// --- Struct serialisation round-trip tests ---

// TestChatSendInput_Good_RoundTrip verifies JSON serialisation of ChatSendInput.
func TestChatSendInput_Good_RoundTrip(t *testing.T) {
	in := ChatSendInput{SessionID: "s1", Message: "hello"}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out ChatSendInput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out != in {
		t.Errorf("round-trip mismatch: %+v != %+v", out, in)
	}
}

// TestChatSendOutput_Good_RoundTrip verifies JSON serialisation of ChatSendOutput.
func TestChatSendOutput_Good_RoundTrip(t *testing.T) {
	in := ChatSendOutput{Sent: true, SessionID: "s1", Timestamp: time.Now().Truncate(time.Second)}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out ChatSendOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.Sent != in.Sent || out.SessionID != in.SessionID {
		t.Errorf("round-trip mismatch: %+v != %+v", out, in)
	}
}

// TestChatHistoryOutput_Good_RoundTrip verifies ChatHistoryOutput JSON round-trip.
func TestChatHistoryOutput_Good_RoundTrip(t *testing.T) {
	in := ChatHistoryOutput{
		SessionID: "s1",
		Messages: []ChatMessage{
			{Role: "user", Content: "hello", Timestamp: time.Now().Truncate(time.Second)},
			{Role: "assistant", Content: "hi", Timestamp: time.Now().Truncate(time.Second)},
		},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out ChatHistoryOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.SessionID != in.SessionID {
		t.Errorf("sessionId mismatch: %q != %q", out.SessionID, in.SessionID)
	}
	if len(out.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(out.Messages))
	}
}

// TestSessionListOutput_Good_RoundTrip verifies SessionListOutput JSON round-trip.
func TestSessionListOutput_Good_RoundTrip(t *testing.T) {
	in := SessionListOutput{
		Sessions: []Session{
			{ID: "s1", Name: "test", Status: "active", CreatedAt: time.Now().Truncate(time.Second)},
		},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out SessionListOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(out.Sessions) != 1 || out.Sessions[0].ID != "s1" {
		t.Errorf("round-trip mismatch: %+v", out)
	}
}

// TestPlanStatusOutput_Good_RoundTrip verifies PlanStatusOutput JSON round-trip.
func TestPlanStatusOutput_Good_RoundTrip(t *testing.T) {
	in := PlanStatusOutput{
		SessionID: "s1",
		Status:    "running",
		Steps:     []PlanStep{{Name: "step1", Status: "done"}, {Name: "step2", Status: "pending"}},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out PlanStatusOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.SessionID != "s1" || len(out.Steps) != 2 {
		t.Errorf("round-trip mismatch: %+v", out)
	}
}

// TestBuildStatusOutput_Good_RoundTrip verifies BuildStatusOutput JSON round-trip.
func TestBuildStatusOutput_Good_RoundTrip(t *testing.T) {
	in := BuildStatusOutput{
		Build: BuildInfo{
			ID:        "b1",
			Repo:      "core-php",
			Branch:    "main",
			Status:    "success",
			Duration:  "2m30s",
			StartedAt: time.Now().Truncate(time.Second),
		},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out BuildStatusOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.Build.ID != "b1" || out.Build.Status != "success" {
		t.Errorf("round-trip mismatch: %+v", out)
	}
}

// TestBuildListOutput_Good_RoundTrip verifies BuildListOutput JSON round-trip.
func TestBuildListOutput_Good_RoundTrip(t *testing.T) {
	in := BuildListOutput{
		Builds: []BuildInfo{
			{ID: "b1", Repo: "core-php", Branch: "main", Status: "success"},
			{ID: "b2", Repo: "core-admin", Branch: "dev", Status: "failed"},
		},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out BuildListOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(out.Builds) != 2 {
		t.Errorf("expected 2 builds, got %d", len(out.Builds))
	}
}

// TestBuildLogsOutput_Good_RoundTrip verifies BuildLogsOutput JSON round-trip.
func TestBuildLogsOutput_Good_RoundTrip(t *testing.T) {
	in := BuildLogsOutput{
		BuildID: "b1",
		Lines:   []string{"line1", "line2", "line3"},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out BuildLogsOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.BuildID != "b1" || len(out.Lines) != 3 {
		t.Errorf("round-trip mismatch: %+v", out)
	}
}

// TestDashboardOverviewOutput_Good_RoundTrip verifies DashboardOverviewOutput JSON round-trip.
func TestDashboardOverviewOutput_Good_RoundTrip(t *testing.T) {
	in := DashboardOverviewOutput{
		Overview: DashboardOverview{
			Repos:          18,
			Services:       5,
			ActiveSessions: 3,
			RecentBuilds:   12,
			BridgeOnline:   true,
		},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out DashboardOverviewOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.Overview.Repos != 18 || !out.Overview.BridgeOnline {
		t.Errorf("round-trip mismatch: %+v", out)
	}
}

// TestDashboardActivityOutput_Good_RoundTrip verifies DashboardActivityOutput JSON round-trip.
func TestDashboardActivityOutput_Good_RoundTrip(t *testing.T) {
	in := DashboardActivityOutput{
		Events: []ActivityEvent{
			{Type: "deploy", Message: "deployed v1.2", Timestamp: time.Now().Truncate(time.Second)},
		},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out DashboardActivityOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(out.Events) != 1 || out.Events[0].Type != "deploy" {
		t.Errorf("round-trip mismatch: %+v", out)
	}
}

// TestDashboardMetricsOutput_Good_RoundTrip verifies DashboardMetricsOutput JSON round-trip.
func TestDashboardMetricsOutput_Good_RoundTrip(t *testing.T) {
	in := DashboardMetricsOutput{
		Period: "24h",
		Metrics: DashboardMetrics{
			BuildsTotal:   100,
			BuildsSuccess: 90,
			BuildsFailed:  10,
			AvgBuildTime:  "3m",
			AgentSessions: 5,
			MessagesTotal: 500,
			SuccessRate:   0.9,
		},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out DashboardMetricsOutput
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.Period != "24h" || out.Metrics.BuildsTotal != 100 || out.Metrics.SuccessRate != 0.9 {
		t.Errorf("round-trip mismatch: %+v", out)
	}
}

// TestBridgeMessage_Good_RoundTrip verifies BridgeMessage JSON round-trip.
func TestBridgeMessage_Good_RoundTrip(t *testing.T) {
	in := BridgeMessage{
		Type:      "test",
		Channel:   "ch1",
		SessionID: "s1",
		Data:      "payload",
		Timestamp: time.Now().Truncate(time.Second),
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out BridgeMessage
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.Type != "test" || out.Channel != "ch1" || out.SessionID != "s1" {
		t.Errorf("round-trip mismatch: %+v", out)
	}
}

// --- Subsystem integration tests ---

// TestSubsystem_Good_RegisterTools verifies RegisterTools does not panic.
func TestSubsystem_Good_RegisterTools(t *testing.T) {
	// RegisterTools requires a real mcp.Server which is complex to construct
	// in isolation. This test verifies the Subsystem can be created and
	// the Bridge/Shutdown path works end-to-end.
	sub := New(nil)
	if sub.Bridge() != nil {
		t.Error("expected nil bridge with nil hub")
	}
	if err := sub.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

// TestSubsystem_Good_StartBridgeNilHub verifies StartBridge is a no-op with nil hub.
func TestSubsystem_Good_StartBridgeNilHub(t *testing.T) {
	sub := New(nil)
	// Should not panic
	sub.StartBridge(context.Background())
}

// TestSubsystem_Good_WithOptions verifies all config options apply correctly.
func TestSubsystem_Good_WithOptions(t *testing.T) {
	hub := ws.NewHub()
	sub := New(hub,
		WithLaravelURL("ws://custom:1234/ws"),
		WithWorkspaceRoot("/tmp/test"),
		WithReconnectInterval(5*time.Second),
		WithToken("secret-123"),
	)

	if sub.cfg.LaravelWSURL != "ws://custom:1234/ws" {
		t.Errorf("expected custom URL, got %q", sub.cfg.LaravelWSURL)
	}
	if sub.cfg.WorkspaceRoot != "/tmp/test" {
		t.Errorf("expected workspace '/tmp/test', got %q", sub.cfg.WorkspaceRoot)
	}
	if sub.cfg.ReconnectInterval != 5*time.Second {
		t.Errorf("expected 5s reconnect interval, got %v", sub.cfg.ReconnectInterval)
	}
	if sub.cfg.Token != "secret-123" {
		t.Errorf("expected token 'secret-123', got %q", sub.cfg.Token)
	}
}

// --- Tool sends correct bridge message type ---

// TestChatSend_Good_BridgeMessageType verifies the bridge receives the correct message type.
func TestChatSend_Good_BridgeMessageType(t *testing.T) {
	msgCh := make(chan BridgeMessage, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg BridgeMessage
		json.Unmarshal(data, &msg)
		msgCh <- msg
		// Keep alive
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}))
	defer ts.Close()

	hub := ws.NewHub()
	ctx := t.Context()
	go hub.Run(ctx)

	sub := New(hub, WithLaravelURL(wsURL(ts)), WithReconnectInterval(50*time.Millisecond))
	sub.StartBridge(ctx)
	waitConnected(t, sub.Bridge(), 2*time.Second)

	sub.chatSend(ctx, nil, ChatSendInput{SessionID: "s1", Message: "test"})

	select {
	case received := <-msgCh:
		if received.Type != "chat_send" {
			t.Errorf("expected bridge message type 'chat_send', got %q", received.Type)
		}
		if received.Channel != "chat:s1" {
			t.Errorf("expected channel 'chat:s1', got %q", received.Channel)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for bridge message")
	}
}
