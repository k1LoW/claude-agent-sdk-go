// Package agent provides a Go SDK for Claude Agent (Claude Code).
//
// It communicates with the Claude Code CLI via a subprocess, supporting
// both one-shot queries and interactive bidirectional sessions.
//
// # Quick Start
//
//	for msg, err := range agent.Query(ctx, "What is 2 + 2?") {
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    if m, ok := msg.(*agent.AssistantMessage); ok {
//	        for _, block := range m.Content {
//	            if tb, ok := block.(*agent.TextBlock); ok {
//	                fmt.Print(tb.Text)
//	            }
//	        }
//	    }
//	}
//
// # With Options
//
//	for msg, err := range agent.Query(ctx, "Hello",
//	    agent.WithSystemPrompt("You are a helpful assistant"),
//	    agent.WithMaxTurns(1),
//	) {
//	    // ...
//	}
//
// # Interactive Client
//
//	client := agent.NewClient(agent.WithPermissionMode("bypassPermissions"))
//	if err := client.Connect(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	if err := client.Send(ctx, "Hello"); err != nil {
//	    log.Fatal(err)
//	}
//	for msg, err := range client.ReceiveResponse(ctx) {
//	    // ...
//	}
package agent

import (
	"context"
	"iter"
)

// Query performs a one-shot query to Claude Code and returns an iterator
// over the response messages. The iterator handles all setup and cleanup
// automatically — when the range loop exits (normally or via break), the
// CLI process is terminated and resources are released.
//
// Errors from connecting, reading, or parsing are yielded as (nil, err).
// Unrecognized message types from newer CLI versions are silently skipped.
func Query(ctx context.Context, prompt string, opts ...Option) iter.Seq2[Message, error] {
	return func(yield func(Message, error) bool) {
		options := applyOptions(opts)

		transport := newSubprocessTransport(options)
		if err := transport.Connect(ctx); err != nil {
			yield(nil, err)
			return
		}

		cs := newControlSession(ctx, transport, options)
		cs.start()
		defer cs.close()

		// Initialize the session
		if _, err := cs.initialize(ctx); err != nil {
			yield(nil, err)
			return
		}

		// Send the user message
		if err := cs.sendUserMessage(prompt, ""); err != nil {
			yield(nil, err)
			return
		}

		// Wait for result and close stdin in background
		go func() {
			if err := cs.waitForResultAndEndInput(); err != nil {
				cs.setReadErr(err)
			}
		}()

		// Yield messages to the caller
		for msg := range cs.msgCh {
			if !yield(msg, nil) {
				return
			}
		}

		// If the reader goroutine encountered an error, yield it
		if err := cs.readError(); err != nil {
			yield(nil, err)
		}
	}
}
