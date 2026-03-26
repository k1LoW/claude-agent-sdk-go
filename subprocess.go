package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// subprocessTransport communicates with the Claude Code CLI via stdin/stdout.
type subprocessTransport struct {
	options *Options
	cliPath string

	cmd     *exec.Cmd
	stdin   io.WriteCloser
	decoder *json.Decoder
	stderr  io.ReadCloser

	writeMu sync.Mutex
	closed  bool
}

func newSubprocessTransport(options *Options) *subprocessTransport {
	return &subprocessTransport{options: options}
}

func (t *subprocessTransport) Connect(ctx context.Context) error {
	cliPath, err := t.findCLI()
	if err != nil {
		return err
	}
	t.cliPath = cliPath

	args := t.buildArgs()
	t.cmd = exec.CommandContext(ctx, t.cliPath, args...) //nolint:gosec // cliPath is from findCLI (user-configured or well-known paths)

	// Environment
	env := os.Environ()
	env = append(env, "CLAUDE_CODE_ENTRYPOINT=sdk-go")
	if t.options.Env != nil {
		for k, v := range t.options.Env {
			env = append(env, k+"="+v)
		}
	}
	if t.options.EnableFileCheckpointing {
		env = append(env, "CLAUDE_CODE_ENABLE_SDK_FILE_CHECKPOINTING=true")
	}
	if t.options.CWD != "" {
		env = append(env, "PWD="+t.options.CWD)
	}
	t.cmd.Env = env

	if t.options.CWD != "" {
		t.cmd.Dir = t.options.CWD
	}

	// Pipes
	stdin, err := t.cmd.StdinPipe()
	if err != nil {
		return &CLIConnectionError{SDKError{Message: "failed to create stdin pipe", Cause: err}}
	}
	t.stdin = stdin

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return &CLIConnectionError{SDKError{Message: "failed to create stdout pipe", Cause: err}}
	}
	t.decoder = json.NewDecoder(stdout)

	if t.options.Stderr != nil {
		stderr, err := t.cmd.StderrPipe()
		if err != nil {
			return &CLIConnectionError{SDKError{Message: "failed to create stderr pipe", Cause: err}}
		}
		t.stderr = stderr
		go t.readStderr()
	}

	if err := t.cmd.Start(); err != nil {
		return &CLINotFoundError{SDKError: SDKError{Message: "failed to start Claude Code CLI", Cause: err}, CLIPath: t.cliPath}
	}

	return nil
}

func (t *subprocessTransport) Write(data string) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	if t.closed || t.stdin == nil {
		return &CLIConnectionError{SDKError{Message: "transport is closed"}}
	}
	_, err := io.WriteString(t.stdin, data)
	if err != nil {
		return &CLIConnectionError{SDKError{Message: "failed to write to stdin", Cause: err}}
	}
	return nil
}

func (t *subprocessTransport) ReadMessage() (map[string]any, error) {
	var msg map[string]any
	if err := t.decoder.Decode(&msg); err != nil {
		if errors.Is(err, io.EOF) {
			// Check process exit status
			if waitErr := t.cmd.Wait(); waitErr != nil {
				var exitErr *exec.ExitError
				if errors.As(waitErr, &exitErr) {
					return nil, &ProcessError{
						SDKError: SDKError{Message: "CLI process failed"},
						ExitCode: exitErr.ExitCode(),
					}
				}
				return nil, &ProcessError{SDKError: SDKError{Message: "CLI process failed", Cause: waitErr}}
			}
			return nil, io.EOF
		}
		return nil, &JSONDecodeError{SDKError: SDKError{Message: "failed to decode JSON", Cause: err}}
	}
	return msg, nil
}

func (t *subprocessTransport) EndInput() error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	if t.stdin != nil {
		err := t.stdin.Close()
		t.stdin = nil
		return err
	}
	return nil
}

func (t *subprocessTransport) Close() error {
	t.writeMu.Lock()
	t.closed = true
	if t.stdin != nil {
		t.stdin.Close()
		t.stdin = nil
	}
	t.writeMu.Unlock()

	if t.cmd != nil && t.cmd.Process != nil {
		done := make(chan error, 1)
		go func() { done <- t.cmd.Wait() }()

		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.cmd.Process.Kill() //nolint:errcheck // Best-effort: process may have already exited.
			<-done
		}
	}
	return nil
}

func (t *subprocessTransport) readStderr() {
	if t.stderr == nil || t.options.Stderr == nil {
		return
	}
	buf := make([]byte, 4096)
	for {
		n, err := t.stderr.Read(buf)
		if n > 0 {
			for line := range strings.SplitSeq(strings.TrimRight(string(buf[:n]), "\n"), "\n") {
				if line != "" {
					t.options.Stderr(line)
				}
			}
		}
		if err != nil {
			return
		}
	}
}

func (t *subprocessTransport) findCLI() (string, error) {
	if t.options.CLIPath != "" {
		return t.options.CLIPath, nil
	}

	// Check PATH
	if p, err := exec.LookPath("claude"); err == nil {
		return p, nil
	}

	// Check common installation locations
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	candidates := []string{
		filepath.Join(home, ".npm-global", "bin", "claude"),
		"/usr/local/bin/claude",
		filepath.Join(home, ".local", "bin", "claude"),
		filepath.Join(home, "node_modules", ".bin", "claude"),
		filepath.Join(home, ".yarn", "bin", "claude"),
		filepath.Join(home, ".claude", "local", "claude"),
	}

	if runtime.GOOS == "windows" {
		for i, c := range candidates {
			candidates[i] = c + ".exe"
		}
	}

	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
	}

	return "", &CLINotFoundError{
		SDKError: SDKError{
			Message: "Claude Code CLI not found. Install with: npm install -g @anthropic-ai/claude-code",
		},
	}
}

func (t *subprocessTransport) buildArgs() []string {
	o := t.options
	args := []string{"--output-format", "stream-json", "--verbose"}

	// System prompt
	if o.AppendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", o.AppendSystemPrompt)
	} else if o.SystemPrompt != nil {
		args = append(args, "--system-prompt", *o.SystemPrompt)
	} else {
		args = append(args, "--system-prompt", "")
	}

	// Tools
	if o.Tools != nil { //nostyle:nilslices // nil=use default tools, empty=no tools
		if len(o.Tools) == 0 {
			args = append(args, "--tools", "")
		} else {
			args = append(args, "--tools", strings.Join(o.Tools, ","))
		}
	}

	if len(o.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(o.AllowedTools, ","))
	}
	if len(o.DisallowedTools) > 0 {
		args = append(args, "--disallowedTools", strings.Join(o.DisallowedTools, ","))
	}
	if o.MaxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(o.MaxTurns))
	}
	if o.MaxBudgetUSD != nil {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%g", *o.MaxBudgetUSD))
	}
	if o.Model != "" {
		args = append(args, "--model", o.Model)
	}
	if o.FallbackModel != "" {
		args = append(args, "--fallback-model", o.FallbackModel)
	}
	if len(o.Betas) > 0 {
		args = append(args, "--betas", strings.Join(o.Betas, ","))
	}
	if o.PermissionPromptToolName != "" {
		args = append(args, "--permission-prompt-tool", o.PermissionPromptToolName)
	}
	if o.PermissionMode != "" {
		args = append(args, "--permission-mode", o.PermissionMode)
	}
	if o.ContinueConversation {
		args = append(args, "--continue")
	}
	if o.Resume != "" {
		args = append(args, "--resume", o.Resume)
	}
	if o.Settings != "" {
		args = append(args, "--settings", o.Settings)
	}
	for _, dir := range o.AddDirs {
		args = append(args, "--add-dir", dir)
	}

	// MCP servers
	if len(o.MCPServers) > 0 {
		config := map[string]any{"mcpServers": o.MCPServers}
		if b, err := json.Marshal(config); err == nil {
			args = append(args, "--mcp-config", string(b))
		}
	} else if o.MCPConfigPath != "" {
		args = append(args, "--mcp-config", o.MCPConfigPath)
	}

	if o.IncludePartialMessages {
		args = append(args, "--include-partial-messages")
	}
	if o.ForkSession {
		args = append(args, "--fork-session")
	}

	// Setting sources
	if o.SettingSources != nil { //nostyle:nilslices // nil=use default sources, empty=disable all sources
		args = append(args, "--setting-sources", strings.Join(o.SettingSources, ","))
	} else {
		args = append(args, "--setting-sources", "")
	}

	// Extra args (sorted for deterministic output)
	if o.ExtraArgs != nil {
		flags := make([]string, 0, len(o.ExtraArgs))
		for flag := range o.ExtraArgs {
			flags = append(flags, flag)
		}
		slices.Sort(flags)
		for _, flag := range flags {
			if val := o.ExtraArgs[flag]; val == nil {
				args = append(args, "--"+flag)
			} else {
				args = append(args, "--"+flag, *val)
			}
		}
	}

	// Thinking
	var maxThinkingTokens *int
	if o.Thinking != nil {
		switch o.Thinking.Type {
		case "adaptive":
			v := 32000
			maxThinkingTokens = &v
		case "enabled":
			v := o.Thinking.BudgetTokens
			maxThinkingTokens = &v
		case "disabled":
			v := 0
			maxThinkingTokens = &v
		}
	}
	if maxThinkingTokens != nil {
		args = append(args, "--max-thinking-tokens", strconv.Itoa(*maxThinkingTokens))
	}
	if o.Effort != "" {
		args = append(args, "--effort", o.Effort)
	}

	// Output format (structured output / JSON schema)
	if o.OutputFormat != nil {
		if t, _ := o.OutputFormat["type"].(string); t == "json_schema" {
			if schema, ok := o.OutputFormat["schema"]; ok {
				if b, err := json.Marshal(schema); err == nil {
					args = append(args, "--json-schema", string(b))
				}
			}
		}
	}

	// Always use streaming input
	args = append(args, "--input-format", "stream-json")

	return args
}
