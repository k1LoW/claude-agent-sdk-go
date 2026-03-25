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
			fmt.Printf("\n---\nCost: $%.4f | Turns: %d\n", m.TotalCostUSD, m.NumTurns)
		}
	}
}
