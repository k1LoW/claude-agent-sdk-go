package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	agent "github.com/k1LoW/claude-agent-sdk-go"
)

func main() {
	ctx := context.Background()

	// Hook that blocks dangerous bash commands
	checkBash := func(_ context.Context, input agent.HookInput, _ string) (agent.HookOutput, error) {
		if input.ToolName != "Bash" {
			return agent.HookOutput{}, nil
		}
		command, _ := input.ToolInput["command"].(string)
		blocked := []string{"rm -rf", "sudo"}
		for _, pattern := range blocked {
			if strings.Contains(command, pattern) {
				return agent.HookOutput{
					HookSpecificOutput: map[string]any{
						"hookEventName":          "PreToolUse",
						"permissionDecision":     "deny",
						"permissionDecisionReason": fmt.Sprintf("blocked pattern: %s", pattern),
					},
				}, nil
			}
		}
		return agent.HookOutput{}, nil
	}

	for msg, err := range agent.Query(ctx, "Run echo hello",
		agent.WithAllowedTools("Bash"),
		agent.WithHooks(map[agent.HookEvent][]agent.HookMatcher{
			agent.HookPreToolUse: {
				{Matcher: "Bash", Hooks: []agent.HookCallback{checkBash}},
			},
		})) {
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
			fmt.Printf("\nDone (turns: %d)\n", m.NumTurns)
		}
	}
}
