package agent

import "fmt"

// SDKError is the base error type for all Claude Agent SDK errors.
type SDKError struct {
	Message string
	Cause   error
}

func (e *SDKError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *SDKError) Unwrap() error {
	return e.Cause
}

// CLINotFoundError is returned when the Claude Code CLI binary cannot be found.
type CLINotFoundError struct {
	SDKError
	CLIPath string
}

// CLIConnectionError is returned when unable to connect to or communicate with the CLI.
type CLIConnectionError struct {
	SDKError
}

// ProcessError is returned when the CLI process exits with an error.
type ProcessError struct {
	SDKError
	ExitCode int
	Stderr   string
}

func (e *ProcessError) Error() string {
	msg := e.Message
	if e.ExitCode != 0 {
		msg = fmt.Sprintf("%s (exit code: %d)", msg, e.ExitCode)
	}
	if e.Stderr != "" {
		msg = fmt.Sprintf("%s\nstderr: %s", msg, e.Stderr)
	}
	return msg
}

// JSONDecodeError is returned when unable to decode JSON from CLI output.
type JSONDecodeError struct {
	SDKError
	Line string
}

// MessageParseError is returned when unable to parse a message from CLI output.
type MessageParseError struct {
	SDKError
	Data map[string]any
}
