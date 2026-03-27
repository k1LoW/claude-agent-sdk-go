# claude-agent-sdk-go

[![Go Reference](https://pkg.go.dev/badge/github.com/k1LoW/claude-agent-sdk-go.svg)](https://pkg.go.dev/github.com/k1LoW/claude-agent-sdk-go) ![Coverage](https://raw.githubusercontent.com/k1LoW/octocovs/main/badges/k1LoW/claude-agent-sdk-go/coverage.svg) ![Code to Test Ratio](https://raw.githubusercontent.com/k1LoW/octocovs/main/badges/k1LoW/claude-agent-sdk-go/ratio.svg) ![Test Execution Time](https://raw.githubusercontent.com/k1LoW/octocovs/main/badges/k1LoW/claude-agent-sdk-go/time.svg)

`claude-agent-sdk-go` is an **unofficial** Go SDK for [Claude Agent](https://docs.anthropic.com/en/docs/claude-code) (Claude Code).

It communicates with the Claude Code CLI via a subprocess, supporting both one-shot queries and interactive bidirectional sessions.

## Usage

### Simple query

``` go
package main

import (
	"context"
	"fmt"
	"log"

	agent "github.com/k1LoW/claude-agent-sdk-go"
)

func main() {
	ctx := context.Background()
	for msg, err := range agent.Query(ctx, "What is 2 + 2?") {
		if err != nil {
			log.Fatal(err)
		}
		switch m := msg.(type) {
		case *agent.AssistantMessage:
			for _, block := range m.Content {
				if tb, ok := block.(*agent.TextBlock); ok {
					fmt.Print(tb.Text)
				}
			}
		case *agent.ResultMessage:
			fmt.Printf("\nCost: $%.4f\n", m.TotalCostUSD)
		}
	}
}
```

### With options

``` go
for msg, err := range agent.Query(ctx, "Hello",
	agent.WithSystemPrompt("You are a helpful assistant"),
	agent.WithMaxTurns(3),
	agent.WithPermissionMode("bypassPermissions"),
) {
	// ...
}
```

### Interactive client

``` go
client := agent.NewClient(
	agent.WithSystemPrompt("You are a helpful assistant"),
)
if err := client.Connect(ctx); err != nil {
	log.Fatal(err)
}
defer client.Close()

// First turn
if err := client.Send(ctx, "What is the capital of France?"); err != nil {
	log.Fatal(err)
}
for msg, err := range client.ReceiveResponse(ctx) {
	if err != nil {
		log.Fatal(err)
	}
	// handle msg...
}

// Follow-up turn
if err := client.Send(ctx, "And Germany?"); err != nil {
	log.Fatal(err)
}
for msg, err := range client.ReceiveResponse(ctx) {
	// ...
}
```

### Hooks

``` go
checkBash := func(_ context.Context, input agent.HookInput, _ string) (agent.HookOutput, error) {
	if input.ToolName != "Bash" {
		return agent.HookOutput{}, nil
	}
	command, _ := input.ToolInput["command"].(string)
	if strings.Contains(command, "rm -rf") {
		return agent.HookOutput{
			HookSpecificOutput: map[string]any{
				"hookEventName":            "PreToolUse",
				"permissionDecision":       "deny",
				"permissionDecisionReason": "dangerous command",
			},
		}, nil
	}
	return agent.HookOutput{}, nil
}

for msg, err := range agent.Query(ctx, "Run echo hello",
	agent.WithAllowedTools("Bash"),
	agent.WithHooks(map[agent.HookEvent][]agent.HookMatcher{
		agent.HookPreToolUse: {
			{Matcher: "Bash", Hooks: []agent.HookCallback{checkBash}},
		},
	}),
) {
	// ...
}
```

### Tool permissions

``` go
// Allow specific tools to be auto-approved
for msg, err := range agent.Query(ctx, "Read main.go and fix the bug",
	agent.WithAllowedTools("Read", "Glob", "Grep"),
	agent.WithDisallowedTools("Bash"),
) {
	// ...
}
```

``` go
// Fine-grained control with a permission callback
canUseTool := func(_ context.Context, toolName string, input map[string]any, _ agent.ToolPermissionContext) (agent.PermissionResult, error) {
	if toolName == "Write" {
		return &agent.PermissionDeny{Message: "file writes are not allowed"}, nil
	}
	return &agent.PermissionAllow{}, nil
}

for msg, err := range agent.Query(ctx, "Create a file called hello.txt",
	agent.WithCanUseTool(canUseTool),
) {
	// ...
}
```

### Answering user questions

``` go
for msg, err := range agent.Query(ctx, "Help me design a logo",
	agent.WithAnswerUserQuestions(func(ctx context.Context, questions []agent.Question) (map[string]string, error) {
		answers := map[string]string{}
		for _, q := range questions {
			fmt.Printf("%s\n", q.Question)
			for i, opt := range q.Options {
				fmt.Printf("  %d) %s - %s\n", i+1, opt.Label, opt.Description)
			}
			fmt.Print("> ")
			var choice string
			fmt.Scanln(&choice)
			answers[q.Question] = choice
		}
		return answers, nil
	}),
) {
	// ...
}
```

### MCP servers

``` go
for msg, err := range agent.Query(ctx, "List my GitHub notifications",
	agent.WithMCPServers(map[string]agent.MCPServerConfig{
		"github": {
			Type:    "stdio",
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-github"},
			Env:     map[string]string{"GITHUB_TOKEN": os.Getenv("GITHUB_TOKEN")},
		},
	}),
	agent.WithAllowedTools("mcp__github__*"),
) {
	// ...
}
```

### Custom agents

``` go
for msg, err := range agent.Query(ctx, "Analyze this codebase",
	agent.WithAgents(map[string]*agent.Definition{
		"reviewer": {
			Description: "Code review specialist",
			Prompt:      "You review code for bugs and improvements.",
			Tools:       []string{"Read", "Glob", "Grep"},
			Model:       "sonnet",
		},
	}),
) {
	// ...
}
```

## Prerequisites

- Go 1.23+
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) installed and available in PATH

## References

- [claude-agent-sdk-python](https://github.com/anthropics/claude-agent-sdk-python) - The official Python SDK that this package is based on.
