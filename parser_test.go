package agent

import (
	"errors"
	"testing"
)

func TestParseMessage_UserMessage_StringContent(t *testing.T) {
	data := map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": "hello",
		},
		"uuid":               "abc-123",
		"parent_tool_use_id": "tool-1",
	}

	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	um, ok := msg.(*UserMessage)
	if !ok {
		t.Fatalf("expected *UserMessage, got %T", msg)
	}
	if um.Content != "hello" {
		t.Errorf("Content = %v, want %q", um.Content, "hello")
	}
	if um.UUID != "abc-123" {
		t.Errorf("UUID = %q, want %q", um.UUID, "abc-123")
	}
	if um.ParentToolUseID != "tool-1" {
		t.Errorf("ParentToolUseID = %q, want %q", um.ParentToolUseID, "tool-1")
	}
}

func TestParseMessage_UserMessage_BlockContent(t *testing.T) {
	data := map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": []any{
				map[string]any{"type": "text", "text": "hi"},
				map[string]any{"type": "tool_result", "tool_use_id": "t1", "content": "result", "is_error": false},
			},
		},
	}

	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	um, ok := msg.(*UserMessage)
	if !ok {
		t.Fatalf("expected *UserMessage, got %T", msg)
	}
	blocks, ok := um.Content.([]ContentBlock)
	if !ok {
		t.Fatalf("expected []ContentBlock, got %T", um.Content)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}

	tb, ok := blocks[0].(*TextBlock)
	if !ok {
		t.Fatalf("block[0]: expected *TextBlock, got %T", blocks[0])
	}
	if tb.Text != "hi" {
		t.Errorf("block[0].Text = %q, want %q", tb.Text, "hi")
	}

	tr, ok := blocks[1].(*ToolResultBlock)
	if !ok {
		t.Fatalf("block[1]: expected *ToolResultBlock, got %T", blocks[1])
	}
	if tr.ToolUseID != "t1" {
		t.Errorf("block[1].ToolUseID = %q, want %q", tr.ToolUseID, "t1")
	}
}

func TestParseMessage_AssistantMessage(t *testing.T) {
	data := map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"model": "claude-opus-4-20250514",
			"content": []any{
				map[string]any{"type": "text", "text": "The answer is 4."},
				map[string]any{"type": "thinking", "thinking": "Let me think...", "signature": "sig123"},
				map[string]any{"type": "tool_use", "id": "tu1", "name": "Bash", "input": map[string]any{"command": "echo hi"}},
			},
			"usage": map[string]any{"input_tokens": float64(100), "output_tokens": float64(50)},
		},
		"parent_tool_use_id": "parent-1",
	}

	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	am, ok := msg.(*AssistantMessage)
	if !ok {
		t.Fatalf("expected *AssistantMessage, got %T", msg)
	}
	if am.Model != "claude-opus-4-20250514" {
		t.Errorf("Model = %q", am.Model)
	}
	if am.ParentToolUseID != "parent-1" {
		t.Errorf("ParentToolUseID = %q", am.ParentToolUseID)
	}
	if len(am.Content) != 3 {
		t.Fatalf("expected 3 content blocks, got %d", len(am.Content))
	}

	// TextBlock
	tb, ok := am.Content[0].(*TextBlock)
	if !ok {
		t.Fatalf("content[0]: expected *TextBlock, got %T", am.Content[0])
	}
	if tb.Text != "The answer is 4." {
		t.Errorf("content[0].Text = %q", tb.Text)
	}

	// ThinkingBlock
	thb, ok := am.Content[1].(*ThinkingBlock)
	if !ok {
		t.Fatalf("content[1]: expected *ThinkingBlock, got %T", am.Content[1])
	}
	if thb.Thinking != "Let me think..." {
		t.Errorf("content[1].Thinking = %q", thb.Thinking)
	}
	if thb.Signature != "sig123" {
		t.Errorf("content[1].Signature = %q", thb.Signature)
	}

	// ToolUseBlock
	tub, ok := am.Content[2].(*ToolUseBlock)
	if !ok {
		t.Fatalf("content[2]: expected *ToolUseBlock, got %T", am.Content[2])
	}
	if tub.ID != "tu1" {
		t.Errorf("content[2].ID = %q", tub.ID)
	}
	if tub.Name != "Bash" {
		t.Errorf("content[2].Name = %q", tub.Name)
	}
	if cmd, _ := tub.Input["command"].(string); cmd != "echo hi" {
		t.Errorf("content[2].Input[command] = %q", cmd)
	}

	// Usage
	if am.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if am.Usage["input_tokens"] != float64(100) {
		t.Errorf("Usage[input_tokens] = %v", am.Usage["input_tokens"])
	}
}

func TestParseMessage_SystemMessage_Generic(t *testing.T) {
	data := map[string]any{
		"type":    "system",
		"subtype": "init",
		"foo":     "bar",
	}

	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sm, ok := msg.(*SystemMessage)
	if !ok {
		t.Fatalf("expected *SystemMessage, got %T", msg)
	}
	if sm.Subtype != "init" {
		t.Errorf("Subtype = %q", sm.Subtype)
	}
}

func TestParseMessage_TaskStartedMessage(t *testing.T) {
	data := map[string]any{
		"type":        "system",
		"subtype":     "task_started",
		"task_id":     "task-1",
		"description": "Running tests",
		"uuid":        "u1",
		"session_id":  "s1",
		"tool_use_id": "tu1",
		"task_type":   "background",
	}

	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tsm, ok := msg.(*TaskStartedMessage)
	if !ok {
		t.Fatalf("expected *TaskStartedMessage, got %T", msg)
	}
	if tsm.TaskID != "task-1" {
		t.Errorf("TaskID = %q", tsm.TaskID)
	}
	if tsm.Description != "Running tests" {
		t.Errorf("Description = %q", tsm.Description)
	}
	if tsm.TaskType != "background" {
		t.Errorf("TaskType = %q", tsm.TaskType)
	}
	// Should also be a SystemMessage
	if tsm.Subtype != "task_started" {
		t.Errorf("Subtype = %q", tsm.Subtype)
	}
}

func TestParseMessage_TaskProgressMessage(t *testing.T) {
	data := map[string]any{
		"type":        "system",
		"subtype":     "task_progress",
		"task_id":     "task-1",
		"description": "Still running",
		"uuid":        "u2",
		"session_id":  "s1",
		"usage": map[string]any{
			"total_tokens": float64(500),
			"tool_uses":    float64(3),
			"duration_ms":  float64(1200),
		},
		"last_tool_name": "Bash",
	}

	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tpm, ok := msg.(*TaskProgressMessage)
	if !ok {
		t.Fatalf("expected *TaskProgressMessage, got %T", msg)
	}
	if tpm.Usage.TotalTokens != 500 {
		t.Errorf("Usage.TotalTokens = %d", tpm.Usage.TotalTokens)
	}
	if tpm.LastToolName != "Bash" {
		t.Errorf("LastToolName = %q", tpm.LastToolName)
	}
}

func TestParseMessage_TaskNotificationMessage(t *testing.T) {
	data := map[string]any{
		"type":        "system",
		"subtype":     "task_notification",
		"task_id":     "task-1",
		"status":      "completed",
		"output_file": "/tmp/out.txt",
		"summary":     "Task done",
		"uuid":        "u3",
		"session_id":  "s1",
		"usage": map[string]any{
			"total_tokens": float64(1000),
			"tool_uses":    float64(5),
			"duration_ms":  float64(3000),
		},
	}

	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tnm, ok := msg.(*TaskNotificationMessage)
	if !ok {
		t.Fatalf("expected *TaskNotificationMessage, got %T", msg)
	}
	if tnm.Status != "completed" {
		t.Errorf("Status = %q", tnm.Status)
	}
	if tnm.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if tnm.Usage.TotalTokens != 1000 {
		t.Errorf("Usage.TotalTokens = %d", tnm.Usage.TotalTokens)
	}
}

func TestParseMessage_ResultMessage(t *testing.T) {
	data := map[string]any{
		"type":            "result",
		"subtype":         "success",
		"duration_ms":     float64(5000),
		"duration_api_ms": float64(4000),
		"is_error":        false,
		"num_turns":       float64(3),
		"session_id":      "sess-1",
		"stop_reason":     "end_turn",
		"total_cost_usd":  0.0123,
		"result":          "4",
		"usage":           map[string]any{"total_tokens": float64(200)},
	}

	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rm, ok := msg.(*ResultMessage)
	if !ok {
		t.Fatalf("expected *ResultMessage, got %T", msg)
	}
	if rm.DurationMS != 5000 {
		t.Errorf("DurationMS = %d", rm.DurationMS)
	}
	if rm.DurationAPIMS != 4000 {
		t.Errorf("DurationAPIMS = %d", rm.DurationAPIMS)
	}
	if rm.IsError {
		t.Error("IsError should be false")
	}
	if rm.NumTurns != 3 {
		t.Errorf("NumTurns = %d", rm.NumTurns)
	}
	if rm.StopReason != "end_turn" {
		t.Errorf("StopReason = %q", rm.StopReason)
	}
	if rm.TotalCostUSD != 0.0123 {
		t.Errorf("TotalCostUSD = %f", rm.TotalCostUSD)
	}
	if rm.Result != "4" {
		t.Errorf("Result = %q", rm.Result)
	}
	if rm.SessionID != "sess-1" {
		t.Errorf("SessionID = %q", rm.SessionID)
	}
}

func TestParseMessage_StreamEvent(t *testing.T) {
	data := map[string]any{
		"type":               "stream_event",
		"uuid":               "u1",
		"session_id":         "s1",
		"event":              map[string]any{"type": "content_block_delta"},
		"parent_tool_use_id": "p1",
	}

	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	se, ok := msg.(*StreamEvent)
	if !ok {
		t.Fatalf("expected *StreamEvent, got %T", msg)
	}
	if se.UUID != "u1" {
		t.Errorf("UUID = %q", se.UUID)
	}
	if se.ParentToolUseID != "p1" {
		t.Errorf("ParentToolUseID = %q", se.ParentToolUseID)
	}
	if se.Event["type"] != "content_block_delta" {
		t.Errorf("Event[type] = %v", se.Event["type"])
	}
}

func TestParseMessage_RateLimitEvent(t *testing.T) {
	data := map[string]any{
		"type":       "rate_limit_event",
		"uuid":       "u1",
		"session_id": "s1",
		"rate_limit_info": map[string]any{
			"status":        "allowed_warning",
			"resetsAt":      float64(1700000000),
			"rateLimitType": "five_hour",
			"utilization":   0.85,
		},
	}

	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rle, ok := msg.(*RateLimitEvent)
	if !ok {
		t.Fatalf("expected *RateLimitEvent, got %T", msg)
	}
	if rle.RateLimitInfo.Status != "allowed_warning" {
		t.Errorf("Status = %q", rle.RateLimitInfo.Status)
	}
	if rle.RateLimitInfo.ResetsAt == nil || *rle.RateLimitInfo.ResetsAt != 1700000000 {
		t.Errorf("ResetsAt = %v", rle.RateLimitInfo.ResetsAt)
	}
	if rle.RateLimitInfo.RateLimitType != "five_hour" {
		t.Errorf("RateLimitType = %q", rle.RateLimitInfo.RateLimitType)
	}
	if rle.RateLimitInfo.Utilization == nil || *rle.RateLimitInfo.Utilization != 0.85 {
		t.Errorf("Utilization = %v", rle.RateLimitInfo.Utilization)
	}
}

func TestParseMessage_UnknownType(t *testing.T) {
	data := map[string]any{
		"type": "future_type",
		"data": "something",
	}

	msg, err := parseMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg != nil {
		t.Fatalf("expected nil for unknown type, got %T", msg)
	}
}

func TestParseMessage_MissingType(t *testing.T) {
	data := map[string]any{
		"data": "something",
	}

	_, err := parseMessage(data)
	if err == nil {
		t.Fatal("expected error for missing type")
	}
	var parseErr *MessageParseError
	if !errors.As(err, &parseErr) {
		t.Errorf("expected *MessageParseError, got %T", err)
	}
}

func TestParseMessage_MissingMessageField(t *testing.T) {
	data := map[string]any{
		"type": "user",
	}

	_, err := parseMessage(data)
	if err == nil {
		t.Fatal("expected error for missing message field")
	}
}

func TestParseMessage_RateLimitEvent_MissingInfo(t *testing.T) {
	data := map[string]any{
		"type":       "rate_limit_event",
		"uuid":       "u1",
		"session_id": "s1",
	}

	_, err := parseMessage(data)
	if err == nil {
		t.Fatal("expected error for missing rate_limit_info")
	}
}

func TestParseHookInput(t *testing.T) {
	data := map[string]any{
		"hook_event_name": "PreToolUse",
		"session_id":      "s1",
		"transcript_path": "/tmp/transcript",
		"cwd":             "/home/user",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "ls"},
		"tool_use_id":     "tu1",
	}

	input := parseHookInput(data)
	if input.HookEventName != "PreToolUse" {
		t.Errorf("HookEventName = %q", input.HookEventName)
	}
	if input.ToolName != "Bash" {
		t.Errorf("ToolName = %q", input.ToolName)
	}
	if cmd, _ := input.ToolInput["command"].(string); cmd != "ls" {
		t.Errorf("ToolInput[command] = %q", cmd)
	}
	if input.CWD != "/home/user" {
		t.Errorf("CWD = %q", input.CWD)
	}
}

func TestParseContentBlocks_Empty(t *testing.T) {
	blocks, err := parseContentBlocks(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocks != nil {
		t.Errorf("expected nil, got %v", blocks)
	}
}

func TestParseContentBlocks_AllTypes(t *testing.T) {
	raw := []any{
		map[string]any{"type": "text", "text": "hello"},
		map[string]any{"type": "thinking", "thinking": "hmm", "signature": "s1"},
		map[string]any{"type": "tool_use", "id": "t1", "name": "Read", "input": map[string]any{"path": "/tmp"}},
		map[string]any{"type": "tool_result", "tool_use_id": "t1", "content": "file contents", "is_error": false},
		map[string]any{"type": "unknown_block"},
	}

	blocks, err := parseContentBlocks(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// unknown_block should be skipped
	if len(blocks) != 4 {
		t.Fatalf("expected 4 blocks, got %d", len(blocks))
	}

	if _, ok := blocks[0].(*TextBlock); !ok {
		t.Errorf("blocks[0]: expected *TextBlock, got %T", blocks[0])
	}
	if _, ok := blocks[1].(*ThinkingBlock); !ok {
		t.Errorf("blocks[1]: expected *ThinkingBlock, got %T", blocks[1])
	}
	if _, ok := blocks[2].(*ToolUseBlock); !ok {
		t.Errorf("blocks[2]: expected *ToolUseBlock, got %T", blocks[2])
	}
	if _, ok := blocks[3].(*ToolResultBlock); !ok {
		t.Errorf("blocks[3]: expected *ToolResultBlock, got %T", blocks[3])
	}
}
