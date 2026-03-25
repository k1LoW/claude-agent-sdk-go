package agent

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestSDKError_Error(t *testing.T) {
	err := &SDKError{Message: "something failed"}
	if err.Error() != "something failed" {
		t.Errorf("Error() = %q", err.Error())
	}
}

func TestSDKError_ErrorWithCause(t *testing.T) {
	cause := fmt.Errorf("root cause")
	err := &SDKError{Message: "something failed", Cause: cause}
	if !strings.Contains(err.Error(), "root cause") {
		t.Errorf("Error() should contain cause: %q", err.Error())
	}
}

func TestSDKError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("root cause")
	err := &SDKError{Message: "wrapper", Cause: cause}
	if !errors.Is(errors.Unwrap(err), cause) {
		t.Errorf("Unwrap() = %v, want %v", errors.Unwrap(err), cause)
	}
}

func TestProcessError_Error(t *testing.T) {
	err := &ProcessError{
		SDKError: SDKError{Message: "command failed"},
		ExitCode: 1,
		Stderr:   "error output",
	}
	s := err.Error()
	if !strings.Contains(s, "exit code: 1") {
		t.Errorf("should contain exit code: %q", s)
	}
	if !strings.Contains(s, "error output") {
		t.Errorf("should contain stderr: %q", s)
	}
}

func TestProcessError_ErrorMinimal(t *testing.T) {
	err := &ProcessError{
		SDKError: SDKError{Message: "failed"},
	}
	if err.Error() != "failed" {
		t.Errorf("Error() = %q", err.Error())
	}
}

func TestCLINotFoundError(t *testing.T) {
	err := &CLINotFoundError{
		SDKError: SDKError{Message: "CLI not found"},
		CLIPath:  "/usr/local/bin/claude",
	}
	if err.CLIPath != "/usr/local/bin/claude" {
		t.Errorf("CLIPath = %q", err.CLIPath)
	}
}

func TestErrorHierarchy(t *testing.T) {
	// CLINotFoundError embeds SDKError and is accessible via type assertion
	err := &CLINotFoundError{SDKError: SDKError{Message: "not found"}}
	if err.Message != "not found" {
		t.Error("CLINotFoundError should embed SDKError")
	}

	// errors.As works for the concrete types themselves
	var notFoundErr *CLINotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Error("should match *CLINotFoundError via errors.As")
	}

	// CLIConnectionError embeds SDKError
	connErr := &CLIConnectionError{SDKError{Message: "connection failed"}}
	var connErrTarget *CLIConnectionError
	if !errors.As(connErr, &connErrTarget) {
		t.Error("should match *CLIConnectionError via errors.As")
	}

	// ProcessError embeds SDKError
	procErr := &ProcessError{SDKError: SDKError{Message: "process failed"}, ExitCode: 1}
	var procErrTarget *ProcessError
	if !errors.As(procErr, &procErrTarget) {
		t.Error("should match *ProcessError via errors.As")
	}
	if procErrTarget.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", procErrTarget.ExitCode)
	}
}

func TestJSONDecodeError(t *testing.T) {
	err := &JSONDecodeError{
		SDKError: SDKError{Message: "invalid json", Cause: fmt.Errorf("unexpected EOF")},
		Line:     `{"incomplete":`,
	}
	if err.Line != `{"incomplete":` {
		t.Errorf("Line = %q", err.Line)
	}
	if !strings.Contains(err.Error(), "unexpected EOF") {
		t.Errorf("should contain cause: %q", err.Error())
	}
}

func TestMessageParseError(t *testing.T) {
	err := &MessageParseError{
		SDKError: SDKError{Message: "missing field"},
		Data:     map[string]any{"type": "user"},
	}
	if err.Data["type"] != "user" {
		t.Errorf("Data = %v", err.Data)
	}
}
