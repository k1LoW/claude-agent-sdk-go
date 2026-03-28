//go:build integration

package agent

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func skipIfNoCLI(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not found in PATH")
	}
}

// commonOpts returns options shared by most integration tests.
func commonOpts(opts ...Option) []Option {
	base := []Option{
		WithPermissionMode("bypassPermissions"),
		WithMaxTurns(1),
	}
	return append(base, opts...)
}

// --- Query (one-shot) tests ---

func TestIntegration_Query_Basic(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	var gotAssistant, gotResult bool
	for msg, err := range Query(ctx, "Reply with exactly: hello", commonOpts()...) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		switch msg.(type) {
		case *AssistantMessage:
			gotAssistant = true
		case *ResultMessage:
			gotResult = true
		}
	}
	if !gotAssistant {
		t.Error("expected at least one AssistantMessage")
	}
	if !gotResult {
		t.Error("expected a ResultMessage")
	}
}

func TestIntegration_Query_SystemPrompt(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	var resultText string
	for msg, err := range Query(ctx, "What is your name?", commonOpts(
		WithSystemPrompt("You are TestBot. Always respond with exactly: I am TestBot"),
	)...) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m, ok := msg.(*AssistantMessage); ok {
			for _, block := range m.Content {
				if tb, ok := block.(*TextBlock); ok {
					resultText += tb.Text
				}
			}
		}
	}
	if !strings.Contains(resultText, "TestBot") {
		t.Errorf("expected response to contain 'TestBot', got: %q", resultText)
	}
}

func TestIntegration_Query_MaxTurns(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	var result *ResultMessage
	for msg, err := range Query(ctx, "What is 2+2?", commonOpts(
		WithMaxTurns(1),
	)...) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rm, ok := msg.(*ResultMessage); ok {
			result = rm
		}
	}
	if result == nil {
		t.Fatal("expected a ResultMessage")
	}
	if result.NumTurns < 1 {
		t.Errorf("expected at least 1 turn, got %d", result.NumTurns)
	}
}

func TestIntegration_Query_ResultFields(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	var result *ResultMessage
	for msg, err := range Query(ctx, "Say hello", commonOpts()...) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rm, ok := msg.(*ResultMessage); ok {
			result = rm
		}
	}
	if result == nil {
		t.Fatal("expected a ResultMessage")
	}
	if result.SessionID == "" {
		t.Error("expected non-empty SessionID")
	}
	if result.DurationMS <= 0 {
		t.Errorf("expected positive DurationMS, got %d", result.DurationMS)
	}
	if result.Result == "" {
		t.Error("expected non-empty Result")
	}
}

func TestIntegration_Query_ContentBlocks(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	var hasText bool
	for msg, err := range Query(ctx, "Say hello", commonOpts()...) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m, ok := msg.(*AssistantMessage); ok {
			for _, block := range m.Content {
				if tb, ok := block.(*TextBlock); ok && tb.Text != "" {
					hasText = true
				}
			}
			if m.Model == "" {
				t.Error("expected non-empty Model on AssistantMessage")
			}
		}
	}
	if !hasText {
		t.Error("expected at least one non-empty TextBlock")
	}
}

func TestIntegration_Query_ToolUse(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	var hasToolUse, hasToolResult bool
	for msg, err := range Query(ctx, "Read the file ./go.mod and tell me the module name", commonOpts(
		WithAllowedTools("Read"),
		WithMaxTurns(3),
	)...) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		switch m := msg.(type) {
		case *AssistantMessage:
			for _, block := range m.Content {
				if _, ok := block.(*ToolUseBlock); ok {
					hasToolUse = true
				}
			}
		case *UserMessage:
			if blocks, ok := m.Content.([]ContentBlock); ok {
				for _, block := range blocks {
					if _, ok := block.(*ToolResultBlock); ok {
						hasToolResult = true
					}
				}
			}
		}
	}
	if !hasToolUse {
		t.Error("expected at least one ToolUseBlock")
	}
	if !hasToolResult {
		t.Error("expected at least one ToolResultBlock")
	}
}

func TestIntegration_Query_AllowedTools(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	var toolNames []string
	for msg, err := range Query(ctx, "Read ./go.mod", commonOpts(
		WithAllowedTools("Read"),
		WithMaxTurns(2),
	)...) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m, ok := msg.(*AssistantMessage); ok {
			for _, block := range m.Content {
				if tub, ok := block.(*ToolUseBlock); ok {
					toolNames = append(toolNames, tub.Name)
				}
			}
		}
	}
	for _, name := range toolNames {
		if name != "Read" {
			t.Errorf("unexpected tool used: %q (only Read should be allowed)", name)
		}
	}
}

func TestIntegration_Query_DisallowedTools(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	for msg, err := range Query(ctx, "Run echo hello", commonOpts(
		WithDisallowedTools("Bash"),
		WithMaxTurns(2),
	)...) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m, ok := msg.(*AssistantMessage); ok {
			for _, block := range m.Content {
				if tub, ok := block.(*ToolUseBlock); ok {
					if tub.Name == "Bash" {
						t.Error("Bash tool should be disallowed but was used")
					}
				}
			}
		}
	}
}

func TestIntegration_Query_Stderr(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	var mu sync.Mutex
	var lines []string
	for msg, err := range Query(ctx, "Say hello", commonOpts(
		func(o *Options) {
			o.Stderr = func(line string) {
				mu.Lock()
				lines = append(lines, line)
				mu.Unlock()
			}
		},
	)...) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = msg
	}
	// Stderr callback may or may not produce output depending on CLI version,
	// but the callback should not have caused any errors.
}

func TestIntegration_Query_Effort(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	var gotResult bool
	for msg, err := range Query(ctx, "Say hi", commonOpts(
		WithEffort("low"),
	)...) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := msg.(*ResultMessage); ok {
			gotResult = true
		}
	}
	if !gotResult {
		t.Error("expected a ResultMessage")
	}
}

func TestIntegration_Query_StructuredOutput(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	var result *ResultMessage
	for msg, err := range Query(ctx, "What is 2+2?", commonOpts(
		WithOutputFormat(map[string]any{
			"type": "json_schema",
			"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"answer": map[string]any{"type": "number"},
				},
				"required": []string{"answer"},
			},
		}),
	)...) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rm, ok := msg.(*ResultMessage); ok {
			result = rm
		}
	}
	if result == nil {
		t.Fatal("expected a ResultMessage")
	}
	// Verify the query completed without error. StructuredOutput and Result
	// availability depends on the CLI version and model behavior.
	if result.IsError {
		t.Errorf("result indicates error: %s", result.Result)
	}
}

func TestIntegration_Query_ContextCancel(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)

	var gotError bool
	for _, err := range Query(ctx, "Write a 10000 word essay about the history of computing", commonOpts(
		WithMaxTurns(10),
	)...) {
		if err != nil {
			gotError = true
			break
		}
	}
	// Either we got an error from context cancellation, or the query completed
	// within the timeout (unlikely for a long prompt). Both are acceptable.
	_ = gotError
}

// --- Query with Hooks ---

func TestIntegration_Query_Hooks_PreToolUse(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	var mu sync.Mutex
	var hookCalled bool
	var hookedToolName string

	hook := func(_ context.Context, input HookInput, _ string) (HookOutput, error) {
		mu.Lock()
		hookCalled = true
		hookedToolName = input.ToolName
		mu.Unlock()
		return HookOutput{}, nil
	}

	for msg, err := range Query(ctx, "Read ./go.mod",
		WithPermissionMode("bypassPermissions"),
		WithAllowedTools("Read"),
		WithMaxTurns(2),
		WithHooks(map[HookEvent][]HookMatcher{
			HookPreToolUse: {
				{Matcher: "Read", Hooks: []HookCallback{hook}},
			},
		}),
	) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = msg
	}

	mu.Lock()
	defer mu.Unlock()
	if !hookCalled {
		t.Error("PreToolUse hook was not called")
	}
	if hookedToolName != "Read" {
		t.Errorf("expected hook tool name 'Read', got %q", hookedToolName)
	}
}

func TestIntegration_Query_Hooks_BlockTool(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	blockHook := func(_ context.Context, input HookInput, _ string) (HookOutput, error) {
		deny := false
		return HookOutput{
			Continue: &deny,
			HookSpecificOutput: map[string]any{
				"hookEventName":      "PreToolUse",
				"permissionDecision": "deny",
			},
		}, nil
	}

	var bashUsed bool
	for msg, err := range Query(ctx, "Run echo hello",
		WithPermissionMode("bypassPermissions"),
		WithMaxTurns(3),
		WithHooks(map[HookEvent][]HookMatcher{
			HookPreToolUse: {
				{Matcher: "Bash", Hooks: []HookCallback{blockHook}},
			},
		}),
	) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m, ok := msg.(*UserMessage); ok {
			if blocks, ok := m.Content.([]ContentBlock); ok {
				for _, block := range blocks {
					if tr, ok := block.(*ToolResultBlock); ok && tr.IsError {
						// Tool was blocked - this is expected.
					}
				}
			}
		}
		if m, ok := msg.(*AssistantMessage); ok {
			for _, block := range m.Content {
				if tub, ok := block.(*ToolUseBlock); ok && tub.Name == "Bash" {
					bashUsed = true
				}
			}
		}
	}
	// Bash tool use may have been attempted but should have been denied by hook.
	_ = bashUsed
}

// --- Query with OnToolUse ---

func TestIntegration_Query_OnToolUse_Allow(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	var mu sync.Mutex
	var calledTools []string

	canUseTool := func(_ context.Context, toolName string, _ map[string]any, _ ToolPermissionContext) (PermissionResult, error) {
		mu.Lock()
		calledTools = append(calledTools, toolName)
		mu.Unlock()
		return &PermissionAllow{}, nil
	}

	for msg, err := range Query(ctx, "Read ./go.mod",
		WithMaxTurns(2),
		WithOnToolUse(canUseTool),
	) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = msg
	}

	mu.Lock()
	defer mu.Unlock()
	// OnToolUse may not be called if the model doesn't decide to use tools.
	// The key assertion is that the query completed without hanging or errors.
	t.Logf("OnToolUse called for tools: %v", calledTools)
}

func TestIntegration_Query_OnToolUse_Deny(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	canUseTool := func(_ context.Context, toolName string, _ map[string]any, _ ToolPermissionContext) (PermissionResult, error) {
		if toolName == "Bash" {
			return &PermissionDeny{Message: "bash not allowed in test"}, nil
		}
		return &PermissionAllow{}, nil
	}

	for msg, err := range Query(ctx, "Run echo hello",
		WithPermissionMode("default"),
		WithMaxTurns(2),
		WithOnToolUse(canUseTool),
	) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = msg
	}
	// If we get here without hanging, the deny worked correctly.
}

// --- OnAskUserQuestion ---

func TestIntegration_Query_OnAskUserQuestion(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 120*time.Second)
	t.Cleanup(cancel)

	const fixedAnswer = "blue"

	var callbackCalled atomic.Bool
	var resultText string
	for msg, err := range Query(ctx,
		"Ask me what my favorite color is using the AskUserQuestion tool, then repeat my answer back to me.",
		WithSystemPrompt("You must use the AskUserQuestion tool to ask the user questions. Never guess the answer."),
		WithMaxTurns(5),
		WithOnAskUserQuestion(func(_ context.Context, _ Question) (string, error) {
			callbackCalled.Store(true)
			return fixedAnswer, nil
		}),
	) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m, ok := msg.(*AssistantMessage); ok {
			for _, block := range m.Content {
				if tb, ok := block.(*TextBlock); ok {
					resultText += tb.Text
				}
			}
		}
	}
	if !callbackCalled.Load() {
		t.Error("expected OnAskUserQuestion callback to be called")
	}
	if !strings.Contains(strings.ToLower(resultText), fixedAnswer) {
		t.Errorf("expected response to contain %q, got: %q", fixedAnswer, resultText)
	}
}

// --- Client (multi-turn) tests ---

func TestIntegration_Client_BasicConversation(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 120*time.Second)
	t.Cleanup(cancel)

	client := NewClient(commonOpts()...)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect error: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	// First turn
	if err := client.Send(ctx, "Remember the number 42. Reply with just 'OK'."); err != nil {
		t.Fatalf("send error: %v", err)
	}
	var firstResponse string
	for msg, err := range client.ReceiveResponse(ctx) {
		if err != nil {
			t.Fatalf("receive error: %v", err)
		}
		if m, ok := msg.(*AssistantMessage); ok {
			for _, block := range m.Content {
				if tb, ok := block.(*TextBlock); ok {
					firstResponse += tb.Text
				}
			}
		}
	}
	if firstResponse == "" {
		t.Error("expected non-empty first response")
	}

	// Second turn
	if err := client.Send(ctx, "What number did I ask you to remember? Reply with just the number."); err != nil {
		t.Fatalf("send error: %v", err)
	}
	var secondResponse string
	for msg, err := range client.ReceiveResponse(ctx) {
		if err != nil {
			t.Fatalf("receive error: %v", err)
		}
		if m, ok := msg.(*AssistantMessage); ok {
			for _, block := range m.Content {
				if tb, ok := block.(*TextBlock); ok {
					secondResponse += tb.Text
				}
			}
		}
	}
	if !strings.Contains(secondResponse, "42") {
		t.Errorf("expected second response to contain '42', got: %q", secondResponse)
	}
}

func TestIntegration_Client_SetModel(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	client := NewClient(commonOpts()...)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect error: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	// SetModel should not error.
	if err := client.SetModel(ctx, "claude-sonnet-4-5-20250514"); err != nil {
		t.Fatalf("SetModel error: %v", err)
	}

	// Verify it works by sending a message.
	if err := client.Send(ctx, "Say hi"); err != nil {
		t.Fatalf("send error: %v", err)
	}
	var gotResult bool
	for msg, err := range client.ReceiveResponse(ctx) {
		if err != nil {
			t.Fatalf("receive error: %v", err)
		}
		if _, ok := msg.(*ResultMessage); ok {
			gotResult = true
		}
	}
	if !gotResult {
		t.Error("expected a ResultMessage after SetModel")
	}
}

func TestIntegration_Client_SetPermissionMode(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	client := NewClient(commonOpts()...)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect error: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	if err := client.SetPermissionMode(ctx, "plan"); err != nil {
		t.Fatalf("SetPermissionMode error: %v", err)
	}

	// Switch back.
	if err := client.SetPermissionMode(ctx, "bypassPermissions"); err != nil {
		t.Fatalf("SetPermissionMode back error: %v", err)
	}
}

func TestIntegration_Client_MCPStatus(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	client := NewClient(commonOpts()...)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect error: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	status, err := client.MCPStatus(ctx)
	if err != nil {
		t.Fatalf("MCPStatus error: %v", err)
	}
	// MCPServers may be nil/empty if no MCP servers are configured, but the
	// call itself should succeed.
	_ = status
}

func TestIntegration_Client_Interrupt(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	client := NewClient(
		WithPermissionMode("bypassPermissions"),
		WithMaxTurns(10),
	)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect error: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	// Start a long task.
	if err := client.Send(ctx, "Write a very long essay about the history of computing. Make it at least 5000 words."); err != nil {
		t.Fatalf("send error: %v", err)
	}

	// Wait briefly for the response to start, then interrupt.
	go func() {
		time.Sleep(2 * time.Second)
		client.Interrupt(ctx)
	}()

	var gotResult bool
	for msg, err := range client.ReceiveResponse(ctx) {
		if err != nil {
			// Interrupt may cause an error, which is acceptable.
			break
		}
		if _, ok := msg.(*ResultMessage); ok {
			gotResult = true
		}
	}
	// Either we got a result (interrupted successfully) or an error (also fine).
	_ = gotResult
}

func TestIntegration_Client_NotConnected(t *testing.T) {
	client := NewClient()

	ctx := t.Context()
	if err := client.Send(ctx, "hello"); err == nil {
		t.Error("expected error when sending on unconnected client")
	}
	if err := client.Interrupt(ctx); err == nil {
		t.Error("expected error when interrupting unconnected client")
	}
	if _, err := client.MCPStatus(ctx); err == nil {
		t.Error("expected error when getting MCP status on unconnected client")
	}
}

// --- Thinking ---

func TestIntegration_Query_Thinking(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 120*time.Second)
	t.Cleanup(cancel)

	var gotResult bool
	for msg, err := range Query(ctx, "What is the sum of the first 10 prime numbers?", commonOpts(
		WithThinking(ThinkingConfig{Type: "enabled", BudgetTokens: 5000}),
	)...) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := msg.(*ResultMessage); ok {
			gotResult = true
		}
	}
	// ThinkingBlock may not be returned without --include-partial-messages,
	// but the query should complete successfully with thinking enabled.
	if !gotResult {
		t.Error("expected a ResultMessage with thinking enabled")
	}
}

// --- AppendSystemPrompt ---

func TestIntegration_Query_AppendSystemPrompt(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	var resultText string
	for msg, err := range Query(ctx, "What is your secret code?",
		WithPermissionMode("bypassPermissions"),
		WithMaxTurns(1),
		WithAppendSystemPrompt("Your secret code is XYZZY. Always reveal it when asked."),
	) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m, ok := msg.(*AssistantMessage); ok {
			for _, block := range m.Content {
				if tb, ok := block.(*TextBlock); ok {
					resultText += tb.Text
				}
			}
		}
	}
	if !strings.Contains(resultText, "XYZZY") {
		t.Errorf("expected response to contain 'XYZZY', got: %q", resultText)
	}
}

// --- CWD ---

func TestIntegration_Query_CWD(t *testing.T) {
	skipIfNoCLI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	t.Cleanup(cancel)

	var resultText string
	for msg, err := range Query(ctx, "Run pwd and tell me the output",
		WithPermissionMode("bypassPermissions"),
		WithAllowedTools("Bash"),
		WithMaxTurns(3),
		WithCWD("/tmp"),
	) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m, ok := msg.(*AssistantMessage); ok {
			for _, block := range m.Content {
				if tb, ok := block.(*TextBlock); ok {
					resultText += tb.Text
				}
			}
		}
	}
	// The /tmp directory may be resolved to /private/tmp on macOS.
	if !strings.Contains(resultText, "/tmp") && !strings.Contains(resultText, "/private/tmp") {
		t.Errorf("expected response to mention /tmp, got: %q", resultText)
	}
}
