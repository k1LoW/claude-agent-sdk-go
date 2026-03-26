package main

import (
	"context"
	"fmt"
	"log"

	agent "github.com/k1LoW/claude-agent-sdk-go"
)

func main() {
	ctx := context.Background()

	client := agent.NewClient(agent.WithSystemPrompt("You are a helpful assistant"), agent.WithMaxTurns(3))
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
		printMessage(msg)
	}

	fmt.Println("\n---")

	// Second turn
	if err := client.Send(ctx, "And what about Germany?"); err != nil {
		log.Fatal(err)
	}
	for msg, err := range client.ReceiveResponse(ctx) {
		if err != nil {
			log.Fatal(err)
		}
		printMessage(msg)
	}
}

func printMessage(msg agent.Message) {
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
