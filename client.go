package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
)

// Client provides bidirectional, interactive conversations with Claude Code.
//
// Unlike [Query], Client supports multi-turn conversations, interrupts,
// dynamic permission changes, and other interactive features.
//
//	client := agent.NewClient(agent.WithPermissionMode("acceptEdits"))
//	if err := client.Connect(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// First turn
//	if err := client.Send(ctx, "Analyze this codebase"); err != nil {
//	    log.Fatal(err)
//	}
//	for msg, err := range client.ReceiveResponse(ctx) {
//	    // handle messages...
//	}
//
//	// Follow-up turn
//	if err := client.Send(ctx, "Now implement the fix"); err != nil {
//	    log.Fatal(err)
//	}
//	for msg, err := range client.ReceiveResponse(ctx) {
//	    // handle messages...
//	}
type Client struct {
	options   *Options
	transport Transport
	cs        *controlSession
}

// NewClient creates a new Client with the given options.
// Call [Client.Connect] to start the session.
func NewClient(opts ...Option) *Client {
	return &Client{
		options: applyOptions(opts),
	}
}

// Connect starts the Claude Code session. It launches the CLI process,
// initializes the control protocol, and prepares for message exchange.
func (c *Client) Connect(ctx context.Context) error {
	if c.options.CanUseTool != nil {
		c.options.PermissionPromptToolName = "stdio"
	}

	c.transport = newSubprocessTransport(c.options)
	if err := c.transport.Connect(ctx); err != nil {
		return err
	}

	c.cs = newControlSession(ctx, c.transport, c.options)
	c.cs.start()

	if _, err := c.cs.initialize(ctx); err != nil {
		c.Close()
		return err
	}

	return nil
}

// Close terminates the session and releases all resources.
func (c *Client) Close() error {
	var err error
	if c.cs != nil {
		err = c.cs.close()
		c.cs = nil
	}
	c.transport = nil
	return err
}

// Send sends a text message to Claude.
func (c *Client) Send(ctx context.Context, prompt string) error {
	return c.SendWithSessionID(ctx, prompt, "default")
}

// SendWithSessionID sends a text message with a specific session ID.
func (c *Client) SendWithSessionID(ctx context.Context, prompt string, sessionID string) error {
	if c.cs == nil {
		return &CLIConnectionError{SDKError{Message: "not connected; call Connect first"}}
	}

	msg := map[string]any{
		"type":               "user",
		"message":            map[string]any{"role": "user", "content": prompt},
		"parent_tool_use_id": nil,
		"session_id":         sessionID,
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return c.transport.Write(string(b) + "\n")
}

// ReceiveResponse returns an iterator over messages until a [ResultMessage]
// is received (inclusive). This is the primary way to consume a single
// response after calling [Client.Send].
func (c *Client) ReceiveResponse(ctx context.Context) iter.Seq2[Message, error] {
	return func(yield func(Message, error) bool) {
		if c.cs == nil {
			yield(nil, &CLIConnectionError{SDKError{Message: "not connected; call Connect first"}})
			return
		}

		for {
			select {
			case <-ctx.Done():
				yield(nil, ctx.Err())
				return
			case msg, ok := <-c.cs.msgCh:
				if !ok {
					if err := c.cs.readError(); err != nil {
						yield(nil, err)
					}
					return
				}
				if !yield(msg, nil) {
					return
				}
				if _, isResult := msg.(*ResultMessage); isResult {
					return
				}
			}
		}
	}
}

// ReceiveMessages returns an iterator over all messages without stopping at
// [ResultMessage]. Use this for continuous message processing across
// multiple turns.
func (c *Client) ReceiveMessages(ctx context.Context) iter.Seq2[Message, error] {
	return func(yield func(Message, error) bool) {
		if c.cs == nil {
			yield(nil, &CLIConnectionError{SDKError{Message: "not connected; call Connect first"}})
			return
		}

		for {
			select {
			case <-ctx.Done():
				yield(nil, ctx.Err())
				return
			case msg, ok := <-c.cs.msgCh:
				if !ok {
					if err := c.cs.readError(); err != nil {
						yield(nil, err)
					}
					return
				}
				if !yield(msg, nil) {
					return
				}
			}
		}
	}
}

// Interrupt sends an interrupt signal to the CLI.
func (c *Client) Interrupt(ctx context.Context) error {
	if c.cs == nil {
		return &CLIConnectionError{SDKError{Message: "not connected"}}
	}
	return c.cs.interrupt(ctx)
}

// SetPermissionMode changes the permission mode during a conversation.
// Valid modes: "default", "acceptEdits", "plan", "bypassPermissions".
func (c *Client) SetPermissionMode(ctx context.Context, mode string) error {
	if c.cs == nil {
		return &CLIConnectionError{SDKError{Message: "not connected"}}
	}
	return c.cs.setPermissionMode(ctx, mode)
}

// SetModel changes the AI model during a conversation.
func (c *Client) SetModel(ctx context.Context, model string) error {
	if c.cs == nil {
		return &CLIConnectionError{SDKError{Message: "not connected"}}
	}
	return c.cs.setModel(ctx, model)
}

// MCPStatus returns the current MCP server connection status.
func (c *Client) MCPStatus(ctx context.Context) (*MCPStatusResponse, error) {
	if c.cs == nil {
		return nil, &CLIConnectionError{SDKError{Message: "not connected"}}
	}
	raw, err := c.cs.mcpStatus(ctx)
	if err != nil {
		return nil, err
	}
	// Re-marshal and unmarshal to MCPStatusResponse
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal MCP status: %w", err)
	}
	var resp MCPStatusResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse MCP status: %w", err)
	}
	return &resp, nil
}

// ReconnectMCPServer reconnects a disconnected or failed MCP server.
func (c *Client) ReconnectMCPServer(ctx context.Context, serverName string) error {
	if c.cs == nil {
		return &CLIConnectionError{SDKError{Message: "not connected"}}
	}
	return c.cs.reconnectMCPServer(ctx, serverName)
}

// ToggleMCPServer enables or disables an MCP server.
func (c *Client) ToggleMCPServer(ctx context.Context, serverName string, enabled bool) error {
	if c.cs == nil {
		return &CLIConnectionError{SDKError{Message: "not connected"}}
	}
	return c.cs.toggleMCPServer(ctx, serverName, enabled)
}

// StopTask stops a running task.
func (c *Client) StopTask(ctx context.Context, taskID string) error {
	if c.cs == nil {
		return &CLIConnectionError{SDKError{Message: "not connected"}}
	}
	return c.cs.stopTask(ctx, taskID)
}

// RewindFiles rewinds tracked files to their state at a specific user message.
// Requires [Options.EnableFileCheckpointing] to be true.
func (c *Client) RewindFiles(ctx context.Context, userMessageID string) error {
	if c.cs == nil {
		return &CLIConnectionError{SDKError{Message: "not connected"}}
	}
	return c.cs.rewindFiles(ctx, userMessageID)
}
