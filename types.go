package agent

import "context"

// Message is the interface implemented by all message types returned from the CLI.
type Message interface {
	messageType() string
}

// ContentBlock is the interface implemented by all content block types.
type ContentBlock interface {
	blockType() string
}

// --- Content Blocks ---

// TextBlock represents a text content block.
type TextBlock struct {
	Text string
}

func (*TextBlock) blockType() string { return "text" }

// ThinkingBlock represents an extended thinking content block.
type ThinkingBlock struct {
	Thinking  string
	Signature string
}

func (*ThinkingBlock) blockType() string { return "thinking" }

// ToolUseBlock represents a tool invocation content block.
type ToolUseBlock struct {
	ID    string
	Name  string
	Input map[string]any
}

func (*ToolUseBlock) blockType() string { return "tool_use" }

// ToolResultBlock represents the result of a tool invocation.
type ToolResultBlock struct {
	ToolUseID string
	Content   any  // string or []map[string]any
	IsError   bool
}

func (*ToolResultBlock) blockType() string { return "tool_result" }

// --- Messages ---

// UserMessage represents a message from the user.
type UserMessage struct {
	Content         any    // string or []ContentBlock
	UUID            string
	ParentToolUseID string
	ToolUseResult   map[string]any
}

func (*UserMessage) messageType() string { return "user" }

// AssistantMessage represents a response from the assistant.
type AssistantMessage struct {
	Content         []ContentBlock
	Model           string
	ParentToolUseID string
	Error           string
	Usage           map[string]any
}

func (*AssistantMessage) messageType() string { return "assistant" }

// SystemMessage represents a system message with metadata.
type SystemMessage struct {
	Subtype string
	Data    map[string]any
}

func (*SystemMessage) messageType() string { return "system" }

// TaskUsage contains usage statistics for task messages.
type TaskUsage struct {
	TotalTokens int `json:"total_tokens"`
	ToolUses    int `json:"tool_uses"`
	DurationMS  int `json:"duration_ms"`
}

// TaskStartedMessage is emitted when a task starts.
type TaskStartedMessage struct {
	SystemMessage
	TaskID      string
	Description string
	UUID        string
	SessionID   string
	ToolUseID   string
	TaskType    string
}

// TaskProgressMessage is emitted while a task is in progress.
type TaskProgressMessage struct {
	SystemMessage
	TaskID       string
	Description  string
	Usage        TaskUsage
	UUID         string
	SessionID    string
	ToolUseID    string
	LastToolName string
}

// TaskNotificationMessage is emitted when a task completes, fails, or is stopped.
type TaskNotificationMessage struct {
	SystemMessage
	TaskID    string
	Status    string // "completed", "failed", "stopped"
	OutputFile string
	Summary   string
	UUID      string
	SessionID string
	ToolUseID string
	Usage     *TaskUsage
}

// ResultMessage contains the final result with cost and usage information.
type ResultMessage struct {
	Subtype          string
	DurationMS       int
	DurationAPIMS    int
	IsError          bool
	NumTurns         int
	SessionID        string
	StopReason       string
	TotalCostUSD     float64
	Usage            map[string]any
	Result           string
	StructuredOutput any
}

func (*ResultMessage) messageType() string { return "result" }

// StreamEvent represents a partial message update during streaming.
type StreamEvent struct {
	UUID            string
	SessionID       string
	Event           map[string]any
	ParentToolUseID string
}

func (*StreamEvent) messageType() string { return "stream_event" }

// RateLimitInfo contains rate limit status information.
type RateLimitInfo struct {
	Status                string  // "allowed", "allowed_warning", "rejected"
	ResetsAt              *int64
	RateLimitType         string  // "five_hour", "seven_day", etc.
	Utilization           *float64
	OverageStatus         string
	OverageResetsAt       *int64
	OverageDisabledReason string
	Raw                   map[string]any
}

// RateLimitEvent is emitted when rate limit status changes.
type RateLimitEvent struct {
	RateLimitInfo RateLimitInfo
	UUID          string
	SessionID     string
}

func (*RateLimitEvent) messageType() string { return "rate_limit_event" }

// --- Hook Types ---

// HookEvent represents hook event names.
type HookEvent string

const (
	HookPreToolUse        HookEvent = "PreToolUse"
	HookPostToolUse       HookEvent = "PostToolUse"
	HookPostToolUseFailure HookEvent = "PostToolUseFailure"
	HookUserPromptSubmit  HookEvent = "UserPromptSubmit"
	HookStop              HookEvent = "Stop"
	HookSubagentStop      HookEvent = "SubagentStop"
	HookPreCompact        HookEvent = "PreCompact"
	HookNotification      HookEvent = "Notification"
	HookSubagentStart     HookEvent = "SubagentStart"
	HookPermissionRequest HookEvent = "PermissionRequest"
)

// HookInput contains the input data passed to a hook callback.
// Fields are populated based on the HookEventName.
type HookInput struct {
	HookEventName  string
	SessionID      string
	TranscriptPath string
	CWD            string
	PermissionMode string

	// PreToolUse, PostToolUse, PostToolUseFailure, PermissionRequest
	ToolName  string
	ToolInput map[string]any
	ToolUseID string

	// PostToolUse
	ToolResponse any

	// PostToolUseFailure
	Error       string
	IsInterrupt bool

	// UserPromptSubmit
	Prompt string

	// Stop, SubagentStop
	StopHookActive bool

	// SubagentStart, SubagentStop
	AgentID             string
	AgentType           string
	AgentTranscriptPath string

	// Notification
	NotificationMessage string
	Title               string
	NotificationType    string

	// PreCompact
	Trigger            string
	CustomInstructions string

	// PermissionRequest
	PermissionSuggestions []any
}

// HookOutput is the return value from a hook callback.
type HookOutput struct {
	Continue           *bool          `json:"continue,omitempty"`
	SuppressOutput     bool           `json:"suppressOutput,omitempty"`
	StopReason         string         `json:"stopReason,omitempty"`
	Decision           string         `json:"decision,omitempty"`
	SystemMessage      string         `json:"systemMessage,omitempty"`
	Reason             string         `json:"reason,omitempty"`
	HookSpecificOutput map[string]any `json:"hookSpecificOutput,omitempty"`
}

// HookCallback is the function signature for hook implementations.
type HookCallback func(ctx context.Context, input HookInput, toolUseID string) (HookOutput, error)

// HookMatcher defines which hooks to run for a given event.
type HookMatcher struct {
	// Matcher is a tool name pattern (e.g., "Bash", "Write|Edit").
	Matcher string
	// Hooks are the callback functions to execute.
	Hooks []HookCallback
	// Timeout in seconds for all hooks in this matcher.
	Timeout float64
}

// --- Permission Types ---

// PermissionResult is the return value from a tool permission callback.
type PermissionResult interface {
	permissionBehavior() string
}

// PermissionAllow grants permission for a tool use.
type PermissionAllow struct {
	UpdatedInput       map[string]any
	UpdatedPermissions []PermissionUpdate
}

func (*PermissionAllow) permissionBehavior() string { return "allow" }

// PermissionDeny denies permission for a tool use.
type PermissionDeny struct {
	Message   string
	Interrupt bool
}

func (*PermissionDeny) permissionBehavior() string { return "deny" }

// PermissionUpdate represents a permission rule change.
type PermissionUpdate struct {
	Type        string // "addRules", "replaceRules", "removeRules", "setMode", "addDirectories", "removeDirectories"
	Rules       []PermissionRuleValue
	Behavior    string // "allow", "deny", "ask"
	Mode        string
	Directories []string
	Destination string // "userSettings", "projectSettings", "localSettings", "session"
}

// ToMap converts a PermissionUpdate to a map for the control protocol.
func (p *PermissionUpdate) ToMap() map[string]any {
	m := map[string]any{"type": p.Type}
	if p.Destination != "" {
		m["destination"] = p.Destination
	}
	switch p.Type {
	case "addRules", "replaceRules", "removeRules":
		if len(p.Rules) > 0 {
			rules := make([]map[string]any, len(p.Rules))
			for i, r := range p.Rules {
				rules[i] = map[string]any{"toolName": r.ToolName, "ruleContent": r.RuleContent}
			}
			m["rules"] = rules
		}
		if p.Behavior != "" {
			m["behavior"] = p.Behavior
		}
	case "setMode":
		if p.Mode != "" {
			m["mode"] = p.Mode
		}
	case "addDirectories", "removeDirectories":
		if len(p.Directories) > 0 {
			m["directories"] = p.Directories
		}
	}
	return m
}

// PermissionRuleValue represents a single permission rule.
type PermissionRuleValue struct {
	ToolName    string
	RuleContent string
}

// ToolPermissionContext provides context information for tool permission callbacks.
type ToolPermissionContext struct {
	Suggestions []PermissionUpdate
}

// CanUseToolFunc is the callback signature for tool permission decisions.
type CanUseToolFunc func(ctx context.Context, toolName string, input map[string]any, tctx ToolPermissionContext) (PermissionResult, error)

// --- MCP Server Config ---

// MCPServerConfig represents an external MCP server configuration.
type MCPServerConfig struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// --- Agent Definition ---

// Definition defines a custom agent.
type Definition struct {
	Description string
	Prompt      string
	Tools       []string
	Model       string // "sonnet", "opus", "haiku", "inherit"
	Skills      []string
	Memory      string // "user", "project", "local"
	MCPServers  []any  // server name (string) or inline config (map)
}

func (a *Definition) toMap() map[string]any {
	m := map[string]any{
		"description": a.Description,
		"prompt":      a.Prompt,
	}
	if len(a.Tools) > 0 {
		m["tools"] = a.Tools
	}
	if a.Model != "" {
		m["model"] = a.Model
	}
	if len(a.Skills) > 0 {
		m["skills"] = a.Skills
	}
	if a.Memory != "" {
		m["memory"] = a.Memory
	}
	if len(a.MCPServers) > 0 {
		m["mcpServers"] = a.MCPServers
	}
	return m
}

// --- MCP Status Types ---

// MCPStatusResponse is returned by Client.GetMCPStatus.
type MCPStatusResponse struct {
	MCPServers []MCPServerStatus `json:"mcpServers"`
}

// MCPServerStatus contains status information for an MCP server connection.
type MCPServerStatus struct {
	Name       string         `json:"name"`
	Status     string         `json:"status"` // "connected", "failed", "needs-auth", "pending", "disabled"
	ServerInfo map[string]any `json:"serverInfo,omitempty"`
	Error      string         `json:"error,omitempty"`
	Config     map[string]any `json:"config,omitempty"`
	Scope      string         `json:"scope,omitempty"`
	Tools      []MCPToolInfo  `json:"tools,omitempty"`
}

// MCPToolInfo contains information about a tool provided by an MCP server.
type MCPToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Annotations map[string]any `json:"annotations,omitempty"`
}
