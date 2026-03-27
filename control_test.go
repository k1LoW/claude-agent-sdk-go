package agent

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
)

// mockTransport is a test Transport that reads from a queue and captures writes.
type mockTransport struct {
	mu       sync.Mutex
	messages []map[string]any
	pos      int
	writes   [][]byte
	closed   bool
}

func newMockTransport(messages ...map[string]any) *mockTransport {
	return &mockTransport{messages: messages}
}

func (m *mockTransport) Connect(_ context.Context) error { return nil }

func (m *mockTransport) Write(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writes = append(m.writes, data)
	return nil
}

func (m *mockTransport) ReadMessage() (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pos >= len(m.messages) {
		return nil, io.EOF
	}
	msg := m.messages[m.pos]
	m.pos++
	return msg, nil
}

func (m *mockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockTransport) EndInput() error { return nil }

// mockTransportWithControlResponse injects a control_response after seeing
// a control_request write. It uses a channel to synchronize.
type mockTransportWithControlResponse struct {
	mu       sync.Mutex
	messages []map[string]any
	pos      int
	writes   [][]byte
	pending  chan map[string]any
}

func newMockTransportWithControlResponse() *mockTransportWithControlResponse {
	return &mockTransportWithControlResponse{
		pending: make(chan map[string]any, 10),
	}
}

func (m *mockTransportWithControlResponse) Connect(_ context.Context) error { return nil }

func (m *mockTransportWithControlResponse) Write(data []byte) error {
	m.mu.Lock()
	m.writes = append(m.writes, data)
	m.mu.Unlock()

	// Parse the written data and respond to control requests
	var msg map[string]any
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil
	}

	if msg["type"] == "control_request" {
		requestID, _ := msg["request_id"].(string)
		// Queue a success response
		m.pending <- map[string]any{
			"type": "control_response",
			"response": map[string]any{
				"subtype":    "success",
				"request_id": requestID,
				"response":   map[string]any{"initialized": true},
			},
		}
	}
	return nil
}

func (m *mockTransportWithControlResponse) ReadMessage() (map[string]any, error) {
	// First return queued messages
	m.mu.Lock()
	if m.pos < len(m.messages) {
		msg := m.messages[m.pos]
		m.pos++
		m.mu.Unlock()
		return msg, nil
	}
	m.mu.Unlock()

	// Then wait for pending responses
	msg, ok := <-m.pending
	if !ok {
		return nil, io.EOF
	}
	return msg, nil
}

func (m *mockTransportWithControlResponse) Close() error {
	close(m.pending)
	return nil
}

func (m *mockTransportWithControlResponse) EndInput() error { return nil }

func TestControlSession_RoutesMessages(t *testing.T) {
	transport := newMockTransport(
		map[string]any{"type": "assistant", "message": map[string]any{
			"model": "claude-opus-4-20250514",
			"content": []any{
				map[string]any{"type": "text", "text": "Hello!"},
			},
		}},
		map[string]any{"type": "result", "subtype": "success",
			"duration_ms": float64(100), "duration_api_ms": float64(80),
			"is_error": false, "num_turns": float64(1), "session_id": "s1"},
	)

	cs := newControlSession(t.Context(), transport, &Options{})
	cs.start()
	t.Cleanup(func() { cs.close() })

	var messages []Message
	for msg := range cs.msgCh {
		messages = append(messages, msg)
	}

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	if _, ok := messages[0].(*AssistantMessage); !ok {
		t.Errorf("messages[0]: expected *AssistantMessage, got %T", messages[0])
	}
	if _, ok := messages[1].(*ResultMessage); !ok {
		t.Errorf("messages[1]: expected *ResultMessage, got %T", messages[1])
	}
}

func TestControlSession_SkipsUnknownMessages(t *testing.T) {
	transport := newMockTransport(
		map[string]any{"type": "future_message_type", "data": "something"},
		map[string]any{"type": "assistant", "message": map[string]any{
			"model":   "test",
			"content": []any{map[string]any{"type": "text", "text": "hi"}},
		}},
	)

	cs := newControlSession(t.Context(), transport, &Options{})
	cs.start()
	t.Cleanup(func() { cs.close() })

	var messages []Message
	for msg := range cs.msgCh {
		messages = append(messages, msg)
	}

	// Should only get the assistant message, not the unknown type
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
}

func TestControlSession_Initialize(t *testing.T) {
	transport := newMockTransportWithControlResponse()

	cs := newControlSession(t.Context(), transport, &Options{})
	cs.start()
	t.Cleanup(func() { cs.close() })

	resp, err := cs.initialize(t.Context())
	if err != nil {
		t.Fatalf("initialize error: %v", err)
	}

	if resp == nil {
		t.Fatal("response should not be nil")
	}
	if resp["initialized"] != true {
		t.Errorf("response[initialized] = %v", resp["initialized"])
	}
}

func TestControlSession_InitializeWithHooks(t *testing.T) {
	transport := newMockTransportWithControlResponse()

	hookCalled := false
	hook := func(_ context.Context, input HookInput, _ string) (HookOutput, error) {
		hookCalled = true
		return HookOutput{}, nil
	}

	options := &Options{
		Hooks: map[HookEvent][]HookMatcher{
			HookPreToolUse: {
				{Matcher: "Bash", Hooks: []HookCallback{hook}},
			},
		},
	}

	cs := newControlSession(t.Context(), transport, options)
	cs.start()
	t.Cleanup(func() { cs.close() })

	_, err := cs.initialize(t.Context())
	if err != nil {
		t.Fatalf("initialize error: %v", err)
	}

	// Verify the hook callback was registered
	cs.mu.Lock()
	callbackCount := len(cs.hookCallbacks)
	cs.mu.Unlock()

	if callbackCount != 1 {
		t.Errorf("expected 1 hook callback, got %d", callbackCount)
	}

	// Verify the initialize request was sent with hooks config
	transport.mu.Lock()
	writes := transport.writes
	transport.mu.Unlock()

	if len(writes) == 0 {
		t.Fatal("no writes captured")
	}

	var initReq map[string]any
	if err := json.Unmarshal(writes[0], &initReq); err != nil {
		t.Fatalf("failed to parse init request: %v", err)
	}

	req, _ := initReq["request"].(map[string]any)
	hooks, _ := req["hooks"].(map[string]any)
	if hooks == nil {
		t.Fatal("hooks should be present in initialize request")
	}

	preToolUse, ok := hooks["PreToolUse"].([]any)
	if !ok || len(preToolUse) == 0 {
		t.Fatal("PreToolUse hooks should be present")
	}

	matcher, _ := preToolUse[0].(map[string]any)
	if matcher["matcher"] != "Bash" {
		t.Errorf("matcher = %v, want Bash", matcher["matcher"])
	}

	_ = hookCalled // Hook isn't called during init, just registered
}

func TestControlSession_InitializeWithAgents(t *testing.T) {
	transport := newMockTransportWithControlResponse()

	options := &Options{
		Agents: map[string]*Definition{
			"reviewer": {
				Description: "Code reviewer",
				Prompt:      "Review this code",
				Model:       "opus",
			},
		},
	}

	cs := newControlSession(t.Context(), transport, options)
	cs.start()
	t.Cleanup(func() { cs.close() })

	_, err := cs.initialize(t.Context())
	if err != nil {
		t.Fatalf("initialize error: %v", err)
	}

	transport.mu.Lock()
	writes := transport.writes
	transport.mu.Unlock()

	var initReq map[string]any
	if err := json.Unmarshal(writes[0], &initReq); err != nil {
		t.Fatalf("failed to parse init request: %v", err)
	}

	req, _ := initReq["request"].(map[string]any)
	agents, _ := req["agents"].(map[string]any)
	if agents == nil {
		t.Fatal("agents should be present in initialize request")
	}

	reviewer, _ := agents["reviewer"].(map[string]any)
	if reviewer["description"] != "Code reviewer" {
		t.Errorf("reviewer.description = %v", reviewer["description"])
	}
	if reviewer["model"] != "opus" {
		t.Errorf("reviewer.model = %v", reviewer["model"])
	}
}

func TestControlSession_HandleOnToolUse(t *testing.T) {
	canUseTool := func(_ context.Context, toolName string, input map[string]any, _ ToolPermissionContext) (PermissionResult, error) {
		if toolName == "Bash" {
			return &PermissionDeny{Message: "bash not allowed"}, nil
		}
		return &PermissionAllow{}, nil
	}

	cs := &controlSession{
		options:         &Options{OnToolUse: canUseTool},
		pendingRequests: make(map[string]chan controlResult),
		hookCallbacks:   make(map[string]HookCallback),
	}
	cs.ctx, cs.cancel = context.WithCancel(t.Context())
	t.Cleanup(cs.cancel)

	// Test deny
	resp, err := cs.handleCanUseTool(map[string]any{
		"tool_name": "Bash",
		"input":     map[string]any{"command": "rm -rf /"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp["behavior"] != "deny" {
		t.Errorf("behavior = %v, want deny", resp["behavior"])
	}
	if resp["message"] != "bash not allowed" {
		t.Errorf("message = %v", resp["message"])
	}

	// Test allow
	resp, err = cs.handleCanUseTool(map[string]any{
		"tool_name": "Read",
		"input":     map[string]any{"path": "/tmp/file"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp["behavior"] != "allow" {
		t.Errorf("behavior = %v, want allow", resp["behavior"])
	}
}

func TestControlSession_HandleHookCallback(t *testing.T) {
	called := false
	hook := func(_ context.Context, input HookInput, toolUseID string) (HookOutput, error) {
		called = true
		if input.ToolName != "Bash" {
			t.Errorf("ToolName = %q, want Bash", input.ToolName)
		}
		if toolUseID != "tu1" {
			t.Errorf("toolUseID = %q, want tu1", toolUseID)
		}
		deny := false
		return HookOutput{
			Continue: &deny,
			HookSpecificOutput: map[string]any{
				"hookEventName":      "PreToolUse",
				"permissionDecision": "deny",
			},
		}, nil
	}

	cs := &controlSession{
		options:         &Options{},
		pendingRequests: make(map[string]chan controlResult),
		hookCallbacks:   map[string]HookCallback{"hook_0": hook},
	}
	cs.ctx, cs.cancel = context.WithCancel(t.Context())
	t.Cleanup(cs.cancel)

	resp, err := cs.handleHookCallback(map[string]any{
		"callback_id": "hook_0",
		"input": map[string]any{
			"hook_event_name": "PreToolUse",
			"tool_name":       "Bash",
			"tool_input":      map[string]any{"command": "ls"},
		},
		"tool_use_id": "tu1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("hook was not called")
	}
	if resp["continue"] != false {
		t.Errorf("continue = %v, want false", resp["continue"])
	}

	hookSpec, ok := resp["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("hookSpecificOutput missing")
	}
	if hookSpec["permissionDecision"] != "deny" {
		t.Errorf("permissionDecision = %v", hookSpec["permissionDecision"])
	}
}

func TestControlSession_HandleHookCallback_NotFound(t *testing.T) {
	cs := &controlSession{
		options:         &Options{},
		pendingRequests: make(map[string]chan controlResult),
		hookCallbacks:   make(map[string]HookCallback),
	}
	cs.ctx, cs.cancel = context.WithCancel(t.Context())
	t.Cleanup(cs.cancel)

	_, err := cs.handleHookCallback(map[string]any{
		"callback_id": "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for missing callback")
	}
}

func TestControlSession_OnToolUseWithUpdatedPermissions(t *testing.T) {
	canUseTool := func(_ context.Context, toolName string, input map[string]any, _ ToolPermissionContext) (PermissionResult, error) {
		return &PermissionAllow{
			UpdatedInput: map[string]any{"command": "safe command"},
			UpdatedPermissions: []PermissionUpdate{
				{
					Type:        "addRules",
					Behavior:    "allow",
					Rules:       []PermissionRuleValue{{ToolName: "Bash", RuleContent: "echo *"}},
					Destination: "session",
				},
			},
		}, nil
	}

	cs := &controlSession{
		options:         &Options{OnToolUse: canUseTool},
		pendingRequests: make(map[string]chan controlResult),
		hookCallbacks:   make(map[string]HookCallback),
	}
	cs.ctx, cs.cancel = context.WithCancel(t.Context())
	t.Cleanup(cs.cancel)

	resp, err := cs.handleCanUseTool(map[string]any{
		"tool_name": "Bash",
		"input":     map[string]any{"command": "ls"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp["behavior"] != "allow" {
		t.Errorf("behavior = %v", resp["behavior"])
	}

	updatedInput, ok := resp["updatedInput"].(map[string]any)
	if !ok {
		t.Fatal("updatedInput missing")
	}
	if updatedInput["command"] != "safe command" {
		t.Errorf("updatedInput.command = %v", updatedInput["command"])
	}

	perms, ok := resp["updatedPermissions"].([]map[string]any)
	if !ok || len(perms) != 1 {
		t.Fatalf("updatedPermissions = %v", resp["updatedPermissions"])
	}
	if perms[0]["type"] != "addRules" {
		t.Errorf("permission type = %v", perms[0]["type"])
	}
}

func TestControlSession_HandleAskUserQuestion(t *testing.T) {
	answerFn := func(_ context.Context, _ Question) (string, error) {
		return "blue", nil
	}

	cs := &controlSession{
		options:         &Options{OnAskUserQuestion: answerFn},
		pendingRequests: make(map[string]chan controlResult),
		hookCallbacks:   make(map[string]HookCallback),
	}
	cs.ctx, cs.cancel = context.WithCancel(t.Context())
	t.Cleanup(cs.cancel)

	resp, err := cs.handleCanUseTool(map[string]any{
		"tool_name": "AskUserQuestion",
		"input": map[string]any{
			"questions": []any{
				map[string]any{
					"question": "What is your favorite color?",
					"header":   "Color",
					"options": []any{
						map[string]any{"label": "red", "description": "Red color"},
						map[string]any{"label": "blue", "description": "Blue color"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp["behavior"] != "allow" {
		t.Errorf("behavior = %v, want allow", resp["behavior"])
	}

	updatedInput, ok := resp["updatedInput"].(map[string]any)
	if !ok {
		t.Fatal("updatedInput missing")
	}

	answers, ok := updatedInput["answers"].(map[string]string)
	if !ok {
		t.Fatal("answers missing")
	}
	if answers["What is your favorite color?"] != "blue" {
		t.Errorf("answer = %v, want blue", answers["What is your favorite color?"])
	}

	// Verify original input fields are preserved.
	if updatedInput["questions"] == nil {
		t.Error("questions should be preserved in updatedInput")
	}
}

func TestControlSession_HandleAskUserQuestion_DefaultAllow(t *testing.T) {
	// When only OnAskUserQuestion is set (no OnToolUse), non-AskUserQuestion
	// tools should be allowed by default.
	cs := &controlSession{
		options:         &Options{OnAskUserQuestion: func(_ context.Context, _ Question) (string, error) { return "", nil }},
		pendingRequests: make(map[string]chan controlResult),
		hookCallbacks:   make(map[string]HookCallback),
	}
	cs.ctx, cs.cancel = context.WithCancel(t.Context())
	t.Cleanup(cs.cancel)

	resp, err := cs.handleCanUseTool(map[string]any{
		"tool_name": "Bash",
		"input":     map[string]any{"command": "echo hello"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp["behavior"] != "allow" {
		t.Errorf("behavior = %v, want allow", resp["behavior"])
	}
}

func TestControlSession_HandleControlCancelRequest(t *testing.T) {
	// Directly test handleControlCancelRequest by manually registering a pending request.
	cs := &controlSession{
		options:         &Options{},
		pendingRequests: make(map[string]chan controlResult),
		hookCallbacks:   make(map[string]HookCallback),
	}
	cs.ctx, cs.cancel = context.WithCancel(t.Context())
	t.Cleanup(cs.cancel)

	// Register a pending request.
	ch := make(chan controlResult, 1)
	cs.mu.Lock()
	cs.pendingRequests["req_1"] = ch
	cs.mu.Unlock()

	// Cancel it.
	cs.handleControlCancelRequest(map[string]any{
		"type":       "control_cancel_request",
		"request_id": "req_1",
	})

	// The pending channel should receive an error.
	result := <-ch
	if result.err == nil {
		t.Fatal("expected error from canceled request")
	}
	if !strings.Contains(result.err.Error(), "canceled") {
		t.Errorf("expected cancellation error, got: %v", result.err)
	}

	// Verify the request was removed from pendingRequests.
	cs.mu.Lock()
	_, exists := cs.pendingRequests["req_1"]
	cs.mu.Unlock()
	if exists {
		t.Error("canceled request should be removed from pendingRequests")
	}
}

func TestControlSession_HandleControlCancelRequest_NotFound(t *testing.T) {
	cs := &controlSession{
		options:         &Options{},
		pendingRequests: make(map[string]chan controlResult),
		hookCallbacks:   make(map[string]HookCallback),
	}
	cs.ctx, cs.cancel = context.WithCancel(t.Context())
	t.Cleanup(cs.cancel)

	// Canceling a non-existent request should not panic.
	cs.handleControlCancelRequest(map[string]any{
		"type":       "control_cancel_request",
		"request_id": "nonexistent",
	})
}
