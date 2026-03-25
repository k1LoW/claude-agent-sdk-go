package agent

import (
	"testing"
)

func TestApplyOptions_Defaults(t *testing.T) {
	o := applyOptions(nil)
	if o.SystemPrompt != nil {
		t.Errorf("SystemPrompt should be nil by default, got %v", o.SystemPrompt)
	}
	if o.MaxTurns != 0 {
		t.Errorf("MaxTurns should be 0 by default, got %d", o.MaxTurns)
	}
	if o.Model != "" {
		t.Errorf("Model should be empty by default, got %q", o.Model)
	}
}

func TestWithSystemPrompt(t *testing.T) {
	o := applyOptions([]Option{WithSystemPrompt("test prompt")})
	if o.SystemPrompt == nil {
		t.Fatal("SystemPrompt should not be nil")
	}
	if *o.SystemPrompt != "test prompt" {
		t.Errorf("SystemPrompt = %q, want %q", *o.SystemPrompt, "test prompt")
	}
}

func TestWithSystemPrompt_Empty(t *testing.T) {
	o := applyOptions([]Option{WithSystemPrompt("")})
	if o.SystemPrompt == nil {
		t.Fatal("SystemPrompt should not be nil")
	}
	if *o.SystemPrompt != "" {
		t.Errorf("SystemPrompt = %q, want empty string", *o.SystemPrompt)
	}
}

func TestWithAppendSystemPrompt(t *testing.T) {
	o := applyOptions([]Option{WithAppendSystemPrompt("extra")})
	if o.AppendSystemPrompt != "extra" {
		t.Errorf("AppendSystemPrompt = %q, want %q", o.AppendSystemPrompt, "extra")
	}
}

func TestWithAllowedTools(t *testing.T) {
	o := applyOptions([]Option{WithAllowedTools("Read", "Write", "Bash")})
	if len(o.AllowedTools) != 3 {
		t.Fatalf("AllowedTools length = %d, want 3", len(o.AllowedTools))
	}
	if o.AllowedTools[0] != "Read" || o.AllowedTools[1] != "Write" || o.AllowedTools[2] != "Bash" {
		t.Errorf("AllowedTools = %v", o.AllowedTools)
	}
}

func TestWithMaxTurns(t *testing.T) {
	o := applyOptions([]Option{WithMaxTurns(5)})
	if o.MaxTurns != 5 {
		t.Errorf("MaxTurns = %d, want 5", o.MaxTurns)
	}
}

func TestWithMaxBudgetUSD(t *testing.T) {
	o := applyOptions([]Option{WithMaxBudgetUSD(1.5)})
	if o.MaxBudgetUSD == nil {
		t.Fatal("MaxBudgetUSD should not be nil")
	}
	if *o.MaxBudgetUSD != 1.5 {
		t.Errorf("MaxBudgetUSD = %f, want 1.5", *o.MaxBudgetUSD)
	}
}

func TestWithModel(t *testing.T) {
	o := applyOptions([]Option{WithModel("claude-opus-4-20250514")})
	if o.Model != "claude-opus-4-20250514" {
		t.Errorf("Model = %q", o.Model)
	}
}

func TestWithPermissionMode(t *testing.T) {
	o := applyOptions([]Option{WithPermissionMode("bypassPermissions")})
	if o.PermissionMode != "bypassPermissions" {
		t.Errorf("PermissionMode = %q", o.PermissionMode)
	}
}

func TestWithCWD(t *testing.T) {
	o := applyOptions([]Option{WithCWD("/tmp/project")})
	if o.CWD != "/tmp/project" {
		t.Errorf("CWD = %q", o.CWD)
	}
}

func TestWithEnv(t *testing.T) {
	env := map[string]string{"FOO": "bar"}
	o := applyOptions([]Option{WithEnv(env)})
	if o.Env["FOO"] != "bar" {
		t.Errorf("Env[FOO] = %q", o.Env["FOO"])
	}
}

func TestWithThinking(t *testing.T) {
	o := applyOptions([]Option{WithThinking(ThinkingConfig{Type: "enabled", BudgetTokens: 16000})})
	if o.Thinking == nil {
		t.Fatal("Thinking should not be nil")
	}
	if o.Thinking.Type != "enabled" {
		t.Errorf("Thinking.Type = %q", o.Thinking.Type)
	}
	if o.Thinking.BudgetTokens != 16000 {
		t.Errorf("Thinking.BudgetTokens = %d", o.Thinking.BudgetTokens)
	}
}

func TestWithEffort(t *testing.T) {
	o := applyOptions([]Option{WithEffort("high")})
	if o.Effort != "high" {
		t.Errorf("Effort = %q", o.Effort)
	}
}

func TestMultipleOptions(t *testing.T) {
	o := applyOptions([]Option{
		WithModel("claude-sonnet-4-5-20250514"),
		WithMaxTurns(10),
		WithPermissionMode("acceptEdits"),
		WithCWD("/home/user"),
	})
	if o.Model != "claude-sonnet-4-5-20250514" {
		t.Errorf("Model = %q", o.Model)
	}
	if o.MaxTurns != 10 {
		t.Errorf("MaxTurns = %d", o.MaxTurns)
	}
	if o.PermissionMode != "acceptEdits" {
		t.Errorf("PermissionMode = %q", o.PermissionMode)
	}
	if o.CWD != "/home/user" {
		t.Errorf("CWD = %q", o.CWD)
	}
}

func TestOptionsOverride(t *testing.T) {
	o := applyOptions([]Option{
		WithModel("model-a"),
		WithModel("model-b"),
	})
	if o.Model != "model-b" {
		t.Errorf("Model = %q, want %q (last option should win)", o.Model, "model-b")
	}
}
