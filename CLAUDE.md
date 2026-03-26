# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Unofficial Go SDK for Claude Agent (Claude Code). Provides a Go API to interact with the Claude Code CLI via a bidirectional JSON control protocol over stdio. Pure standard library — no external dependencies.

## Commands

```bash
# Run all tests with coverage
make test

# Run a single test
go test ./... -run TestFunctionName -count=1

# Lint (golangci-lint + gostyle)
make lint

# Install dev dependencies (gocredits, gostyle)
make depsdev
```

## Architecture

The SDK has two main entry points:

- **`Query()`** (agent.go) — One-shot API. Creates a subprocess, sends a prompt, returns an `iter.Seq2[Message, error]` iterator, and cleans up automatically.
- **`Client`** (client.go) — Interactive multi-turn API. `Connect()` / `Send()` / `ReceiveResponse()` / `Close()` lifecycle. Supports session management, interrupts, and MCP control.

Both use the same internal stack:

```
Query/Client
  └─ controlSession (control.go)  — Bidirectional message routing, control request/response tracking, hook/permission callbacks
       └─ Transport (transport.go) — Interface: Connect, Write, ReadMessage, Close, EndInput
            └─ subprocessTransport (subprocess.go) — Finds Claude CLI, launches process, manages stdio JSON streams
```

Key supporting modules:
- **parser.go** — Parses raw JSON maps into typed Message/ContentBlock values.
- **options.go** — Functional options pattern (`With*` functions) building the `Options` struct (40+ fields).
- **types.go** — All message types (User/Assistant/System/Result/StreamEvent), content blocks (Text/Thinking/ToolUse/ToolResult), hook system, permission system, MCP config, and custom agent definitions.
- **errors.go** — Structured error hierarchy: SDKError base with CLINotFoundError, CLIConnectionError, ProcessError, JSONDecodeError, MessageParseError.

## Conventions

- Go 1.25, uses `iter.Seq2` iterator pattern.
- Functional options pattern for configuration: `agent.WithSystemPrompt(...)`, `agent.WithMaxTurns(...)`, etc.
- Message handling via type switches on the `Message` interface.
- Linting: golangci-lint v2 config (errorlint, funcorder, godot, gosec, misspell, revive, modernize) + gostyle. Comments must end with a period (godot).
- `examples/` directory is excluded from linting.
