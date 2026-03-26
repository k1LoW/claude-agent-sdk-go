package agent

import "context"

// Transport is the interface for communication with the Claude Code CLI.
//
// This is an internal abstraction exposed for custom transport implementations
// (e.g., remote connections). The interface may change in future releases.
type Transport interface {
	// Connect starts the transport (e.g., launches the subprocess).
	Connect(ctx context.Context) error

	// Write sends raw data to the transport (typically JSON + newline).
	Write(data []byte) error

	// ReadMessage reads the next JSON message from the transport.
	// Returns io.EOF when the stream ends.
	ReadMessage() (map[string]any, error)

	// Close shuts down the transport and releases resources.
	Close() error

	// EndInput closes the input stream (stdin for subprocess transports).
	EndInput() error
}
