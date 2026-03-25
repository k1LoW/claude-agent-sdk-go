package agent

import (
	"strings"
	"testing"
)

func TestBuildArgs_Defaults(t *testing.T) {
	tr := &subprocessTransport{options: &Options{}}
	args := tr.buildArgs()

	assertContains(t, args, "--output-format", "stream-json")
	assertContains(t, args, "--input-format", "stream-json")
	assertContains(t, args, "--system-prompt", "")
	assertContains(t, args, "--setting-sources", "")
	assertContains(t, args, "--verbose")
}

func TestBuildArgs_SystemPrompt(t *testing.T) {
	prompt := "You are a pirate"
	tr := &subprocessTransport{options: &Options{SystemPrompt: &prompt}}
	args := tr.buildArgs()

	assertContains(t, args, "--system-prompt", "You are a pirate")
}

func TestBuildArgs_AppendSystemPrompt(t *testing.T) {
	tr := &subprocessTransport{options: &Options{AppendSystemPrompt: "extra instructions"}}
	args := tr.buildArgs()

	assertContains(t, args, "--append-system-prompt", "extra instructions")
	// Should NOT have --system-prompt when append is used
	for i, a := range args {
		if a == "--system-prompt" {
			t.Errorf("found --system-prompt at index %d when --append-system-prompt is set", i)
		}
	}
}

func TestBuildArgs_Tools(t *testing.T) {
	tests := []struct {
		name     string
		tools    []string
		wantFlag string
		wantVal  string
	}{
		{"nil tools", nil, "", ""},
		{"empty tools", []string{}, "--tools", ""},
		{"some tools", []string{"Read", "Write", "Bash"}, "--tools", "Read,Write,Bash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &subprocessTransport{options: &Options{Tools: tt.tools}}
			args := tr.buildArgs()

			if tt.wantFlag == "" {
				for _, a := range args {
					if a == "--tools" {
						t.Error("found --tools flag when tools is nil")
					}
				}
			} else {
				assertContains(t, args, tt.wantFlag, tt.wantVal)
			}
		})
	}
}

func TestBuildArgs_AllowedAndDisallowedTools(t *testing.T) {
	tr := &subprocessTransport{options: &Options{
		AllowedTools:    []string{"Read", "Write"},
		DisallowedTools: []string{"Bash"},
	}}
	args := tr.buildArgs()

	assertContains(t, args, "--allowedTools", "Read,Write")
	assertContains(t, args, "--disallowedTools", "Bash")
}

func TestBuildArgs_MaxTurns(t *testing.T) {
	tr := &subprocessTransport{options: &Options{MaxTurns: 5}}
	args := tr.buildArgs()
	assertContains(t, args, "--max-turns", "5")
}

func TestBuildArgs_MaxTurns_Zero(t *testing.T) {
	tr := &subprocessTransport{options: &Options{MaxTurns: 0}}
	args := tr.buildArgs()
	for _, a := range args {
		if a == "--max-turns" {
			t.Error("found --max-turns when MaxTurns is 0")
		}
	}
}

func TestBuildArgs_MaxBudgetUSD(t *testing.T) {
	budget := 1.5
	tr := &subprocessTransport{options: &Options{MaxBudgetUSD: &budget}}
	args := tr.buildArgs()
	assertContains(t, args, "--max-budget-usd", "1.5")
}

func TestBuildArgs_Model(t *testing.T) {
	tr := &subprocessTransport{options: &Options{Model: "claude-sonnet-4-5-20250514"}}
	args := tr.buildArgs()
	assertContains(t, args, "--model", "claude-sonnet-4-5-20250514")
}

func TestBuildArgs_PermissionMode(t *testing.T) {
	tr := &subprocessTransport{options: &Options{PermissionMode: "bypassPermissions"}}
	args := tr.buildArgs()
	assertContains(t, args, "--permission-mode", "bypassPermissions")
}

func TestBuildArgs_ContinueAndResume(t *testing.T) {
	tr := &subprocessTransport{options: &Options{ContinueConversation: true}}
	args := tr.buildArgs()
	assertContains(t, args, "--continue")

	tr = &subprocessTransport{options: &Options{Resume: "session-123"}}
	args = tr.buildArgs()
	assertContains(t, args, "--resume", "session-123")
}

func TestBuildArgs_MCPServers(t *testing.T) {
	tr := &subprocessTransport{options: &Options{
		MCPServers: map[string]MCPServerConfig{
			"myserver": {Type: "stdio", Command: "python", Args: []string{"-m", "server"}},
		},
	}}
	args := tr.buildArgs()

	idx := indexOf(args, "--mcp-config")
	if idx < 0 {
		t.Fatal("missing --mcp-config flag")
	}
	if idx+1 >= len(args) {
		t.Fatal("--mcp-config has no value")
	}
	val := args[idx+1]
	if !strings.Contains(val, "myserver") {
		t.Errorf("mcp-config value should contain 'myserver': %s", val)
	}
}

func TestBuildArgs_MCPConfigPath(t *testing.T) {
	tr := &subprocessTransport{options: &Options{MCPConfigPath: "/path/to/config.json"}}
	args := tr.buildArgs()
	assertContains(t, args, "--mcp-config", "/path/to/config.json")
}

func TestBuildArgs_IncludePartialMessages(t *testing.T) {
	tr := &subprocessTransport{options: &Options{IncludePartialMessages: true}}
	args := tr.buildArgs()
	assertContains(t, args, "--include-partial-messages")
}

func TestBuildArgs_ForkSession(t *testing.T) {
	tr := &subprocessTransport{options: &Options{ForkSession: true}}
	args := tr.buildArgs()
	assertContains(t, args, "--fork-session")
}

func TestBuildArgs_Betas(t *testing.T) {
	tr := &subprocessTransport{options: &Options{Betas: []string{"context-1m-2025-08-07"}}}
	args := tr.buildArgs()
	assertContains(t, args, "--betas", "context-1m-2025-08-07")
}

func TestBuildArgs_Settings(t *testing.T) {
	tr := &subprocessTransport{options: &Options{Settings: `{"key":"value"}`}}
	args := tr.buildArgs()
	assertContains(t, args, "--settings", `{"key":"value"}`)
}

func TestBuildArgs_SettingSources(t *testing.T) {
	tr := &subprocessTransport{options: &Options{SettingSources: []string{"user", "project"}}}
	args := tr.buildArgs()
	assertContains(t, args, "--setting-sources", "user,project")
}

func TestBuildArgs_Thinking_Adaptive(t *testing.T) {
	tr := &subprocessTransport{options: &Options{Thinking: &ThinkingConfig{Type: "adaptive"}}}
	args := tr.buildArgs()
	assertContains(t, args, "--max-thinking-tokens", "32000")
}

func TestBuildArgs_Thinking_Enabled(t *testing.T) {
	tr := &subprocessTransport{options: &Options{Thinking: &ThinkingConfig{Type: "enabled", BudgetTokens: 16000}}}
	args := tr.buildArgs()
	assertContains(t, args, "--max-thinking-tokens", "16000")
}

func TestBuildArgs_Thinking_Disabled(t *testing.T) {
	tr := &subprocessTransport{options: &Options{Thinking: &ThinkingConfig{Type: "disabled"}}}
	args := tr.buildArgs()
	assertContains(t, args, "--max-thinking-tokens", "0")
}

func TestBuildArgs_Effort(t *testing.T) {
	tr := &subprocessTransport{options: &Options{Effort: "high"}}
	args := tr.buildArgs()
	assertContains(t, args, "--effort", "high")
}

func TestBuildArgs_ExtraArgs(t *testing.T) {
	val := "abc"
	tr := &subprocessTransport{options: &Options{ExtraArgs: map[string]*string{
		"debug-to-stderr":     nil,
		"replay-user-messages": &val,
	}}}
	args := tr.buildArgs()
	assertContains(t, args, "--debug-to-stderr")
	assertContains(t, args, "--replay-user-messages", "abc")
}

func TestBuildArgs_OutputFormat_JSONSchema(t *testing.T) {
	tr := &subprocessTransport{options: &Options{
		OutputFormat: map[string]any{
			"type": "json_schema",
			"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"answer": map[string]any{"type": "string"},
				},
			},
		},
	}}
	args := tr.buildArgs()
	idx := indexOf(args, "--json-schema")
	if idx < 0 {
		t.Fatal("missing --json-schema flag")
	}
	if idx+1 >= len(args) {
		t.Fatal("--json-schema has no value")
	}
	val := args[idx+1]
	if !strings.Contains(val, "answer") {
		t.Errorf("json-schema value should contain 'answer': %s", val)
	}
}

func TestBuildArgs_AddDirs(t *testing.T) {
	tr := &subprocessTransport{options: &Options{AddDirs: []string{"/dir1", "/dir2"}}}
	args := tr.buildArgs()

	count := 0
	for i, a := range args {
		if a == "--add-dir" {
			count++
			if i+1 < len(args) && args[i+1] != "/dir1" && args[i+1] != "/dir2" {
				t.Errorf("unexpected --add-dir value: %s", args[i+1])
			}
		}
	}
	if count != 2 {
		t.Errorf("expected 2 --add-dir flags, got %d", count)
	}
}

// --- helpers ---

// assertContains checks that args contains the given flag (and optionally its value).
func assertContains(t *testing.T, args []string, parts ...string) {
	t.Helper()
	if len(parts) == 0 {
		return
	}
	flag := parts[0]
	idx := indexOf(args, flag)
	if idx < 0 {
		t.Errorf("expected flag %q in args: %v", flag, args)
		return
	}
	if len(parts) > 1 {
		wantVal := parts[1]
		if idx+1 >= len(args) {
			t.Errorf("flag %q has no value", flag)
			return
		}
		if args[idx+1] != wantVal {
			t.Errorf("flag %q value = %q, want %q", flag, args[idx+1], wantVal)
		}
	}
}

func indexOf(args []string, target string) int {
	for i, a := range args {
		if a == target {
			return i
		}
	}
	return -1
}
