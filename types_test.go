package agent

import (
	"testing"
)

func TestPermissionUpdate_ToMap_AddRules(t *testing.T) {
	pu := PermissionUpdate{
		Type:     "addRules",
		Behavior: "allow",
		Rules: []PermissionRuleValue{
			{ToolName: "Bash", RuleContent: "echo *"},
		},
		Destination: "session",
	}

	m := pu.ToMap()
	if m["type"] != "addRules" {
		t.Errorf("type = %v", m["type"])
	}
	if m["behavior"] != "allow" {
		t.Errorf("behavior = %v", m["behavior"])
	}
	if m["destination"] != "session" {
		t.Errorf("destination = %v", m["destination"])
	}

	rules, ok := m["rules"].([]map[string]any)
	if !ok {
		t.Fatalf("rules type = %T", m["rules"])
	}
	if len(rules) != 1 {
		t.Fatalf("rules length = %d", len(rules))
	}
	if rules[0]["toolName"] != "Bash" {
		t.Errorf("rules[0].toolName = %v", rules[0]["toolName"])
	}
}

func TestPermissionUpdate_ToMap_SetMode(t *testing.T) {
	pu := PermissionUpdate{
		Type: "setMode",
		Mode: "bypassPermissions",
	}

	m := pu.ToMap()
	if m["mode"] != "bypassPermissions" {
		t.Errorf("mode = %v", m["mode"])
	}
}

func TestPermissionUpdate_ToMap_AddDirectories(t *testing.T) {
	pu := PermissionUpdate{
		Type:        "addDirectories",
		Directories: []string{"/tmp", "/home"},
	}

	m := pu.ToMap()
	dirs, ok := m["directories"].([]string)
	if !ok {
		t.Fatalf("directories type = %T", m["directories"])
	}
	if len(dirs) != 2 {
		t.Fatalf("directories length = %d", len(dirs))
	}
}

func TestDefinition_toMap(t *testing.T) {
	ad := Definition{
		Description: "A test agent",
		Prompt:      "You are helpful",
		Tools:       []string{"Read", "Write"},
		Model:       "opus",
	}

	m := ad.toMap()
	if m["description"] != "A test agent" {
		t.Errorf("description = %v", m["description"])
	}
	if m["prompt"] != "You are helpful" {
		t.Errorf("prompt = %v", m["prompt"])
	}
	tools, ok := m["tools"].([]string)
	if !ok {
		t.Fatalf("tools type = %T", m["tools"])
	}
	if len(tools) != 2 {
		t.Errorf("tools = %v", tools)
	}
	if m["model"] != "opus" {
		t.Errorf("model = %v", m["model"])
	}
}

func TestDefinition_toMap_MinimalFields(t *testing.T) {
	ad := Definition{
		Description: "desc",
		Prompt:      "prompt",
	}

	m := ad.toMap()
	if _, ok := m["tools"]; ok {
		t.Error("tools should not be present when empty")
	}
	if _, ok := m["model"]; ok {
		t.Error("model should not be present when empty")
	}
	if _, ok := m["skills"]; ok {
		t.Error("skills should not be present when empty")
	}
}

func TestMessageInterface(t *testing.T) {
	// Verify all message types implement Message
	var messages []Message
	messages = append(messages, &UserMessage{})
	messages = append(messages, &AssistantMessage{})
	messages = append(messages, &SystemMessage{})
	messages = append(messages, &TaskStartedMessage{})
	messages = append(messages, &TaskProgressMessage{})
	messages = append(messages, &TaskNotificationMessage{})
	messages = append(messages, &ResultMessage{})
	messages = append(messages, &StreamEvent{})
	messages = append(messages, &RateLimitEvent{})

	for _, m := range messages {
		if m.messageType() == "" {
			t.Errorf("%T.messageType() returned empty string", m)
		}
	}
}

func TestContentBlockInterface(t *testing.T) {
	// Verify all content block types implement ContentBlock
	var blocks []ContentBlock
	blocks = append(blocks, &TextBlock{})
	blocks = append(blocks, &ThinkingBlock{})
	blocks = append(blocks, &ToolUseBlock{})
	blocks = append(blocks, &ToolResultBlock{})

	for _, b := range blocks {
		if b.blockType() == "" {
			t.Errorf("%T.blockType() returned empty string", b)
		}
	}
}
