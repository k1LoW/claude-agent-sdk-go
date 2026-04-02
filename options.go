package agent

// Options configures a query or client session.
type Options struct {
	// SystemPrompt overrides the system prompt. If nil, the SDK passes an empty
	// system prompt (disabling Claude Code's built-in prompt). Set a value to
	// use a custom prompt.
	SystemPrompt *string

	// AppendSystemPrompt appends to Claude Code's default system prompt
	// (uses --append-system-prompt). If set, SystemPrompt is ignored.
	AppendSystemPrompt string

	// Tools sets the base tool set. nil = default, empty = no tools.
	Tools []string

	// AllowedTools is a permission allowlist for auto-approved tools.
	AllowedTools []string

	// DisallowedTools blocks specific tools.
	DisallowedTools []string

	// MaxTurns limits the number of agent turns. 0 = no limit.
	MaxTurns int

	// MaxBudgetUSD sets a spending limit. nil = no limit.
	MaxBudgetUSD *float64

	// Model specifies the AI model to use.
	Model string

	// FallbackModel specifies a fallback model.
	FallbackModel string

	// PermissionMode controls tool execution permissions.
	// Valid: "default", "acceptEdits", "plan", "bypassPermissions".
	PermissionMode string

	// CWD sets the working directory for the CLI.
	CWD string

	// CLIPath overrides the path to the Claude Code CLI binary.
	CLIPath string

	// Env sets additional environment variables for the CLI process.
	Env map[string]string

	// MCPServers configures external MCP servers.
	// Keys are server names, values are MCPServerConfig.
	MCPServers map[string]MCPServerConfig

	// MCPConfigPath is a path to an MCP config file (alternative to MCPServers).
	MCPConfigPath string

	// Settings is a JSON string or file path for CLI settings.
	Settings string

	// AddDirs adds additional directories to the session.
	AddDirs []string

	// ContinueConversation resumes the most recent conversation.
	ContinueConversation bool

	// Resume resumes a specific session by ID.
	Resume string

	// ForkSession forks a resumed session to a new session ID.
	ForkSession bool

	// IncludePartialMessages enables partial message streaming.
	IncludePartialMessages bool

	// Agents defines custom agent configurations.
	Agents map[string]*Definition

	// SettingSources controls which setting sources to load.
	// Valid: "user", "project", "local".
	SettingSources []string

	// Hooks configures hook callbacks for agent lifecycle events.
	Hooks map[HookEvent][]HookMatcher

	// OnToolUse is a callback for tool use decisions.
	OnToolUse OnToolUseFunc

	// OnAskUserQuestion is a callback for handling AskUserQuestion tool calls.
	// When set, AskUserQuestion inputs are parsed and passed to this callback,
	// and the answers are returned as updatedInput with behavior "allow".
	OnAskUserQuestion OnAskUserQuestionFunc

	// PermissionPromptToolName sets the permission prompt tool.
	// Automatically set to "stdio" when OnToolUse or OnAskUserQuestion is provided.
	PermissionPromptToolName string

	// Betas enables beta features.
	Betas []string

	// Thinking controls extended thinking behavior.
	Thinking *ThinkingConfig

	// Effort sets the thinking depth level.
	// Valid: "low", "medium", "high", "max".
	Effort string

	// OutputFormat configures structured output.
	// Example: map[string]any{"type": "json_schema", "schema": map[string]any{...}}
	OutputFormat map[string]any

	// EnableFileCheckpointing tracks file changes for rewind support.
	EnableFileCheckpointing bool

	// ExtraArgs passes arbitrary CLI flags. Keys are flag names (without --),
	// values are flag values (nil for boolean flags).
	ExtraArgs map[string]*string

	// Stderr is called with each line of stderr output from the CLI.
	Stderr func(string)

}

// ThinkingConfig controls extended thinking behavior.
type ThinkingConfig struct {
	// Type is one of "adaptive", "enabled", "disabled".
	Type string
	// BudgetTokens is used when Type is "enabled".
	BudgetTokens int
}

// Option is a functional option for configuring Options.
type Option func(*Options)

// WithSystemPrompt sets a custom system prompt.
func WithSystemPrompt(prompt string) Option {
	return func(o *Options) { o.SystemPrompt = &prompt }
}

// WithAppendSystemPrompt appends to Claude Code's default system prompt.
func WithAppendSystemPrompt(s string) Option {
	return func(o *Options) { o.AppendSystemPrompt = s }
}

// WithTools sets the base tool set.
// nil = use default tools, empty slice = no tools.
func WithTools(tools ...string) Option {
	return func(o *Options) { o.Tools = tools }
}

// WithAllowedTools sets the tools to auto-approve.
func WithAllowedTools(tools ...string) Option {
	return func(o *Options) { o.AllowedTools = tools }
}

// WithDisallowedTools blocks specific tools.
func WithDisallowedTools(tools ...string) Option {
	return func(o *Options) { o.DisallowedTools = tools }
}

// WithMaxTurns limits the number of agent turns.
func WithMaxTurns(n int) Option {
	return func(o *Options) { o.MaxTurns = n }
}

// WithMaxBudgetUSD sets a spending limit.
func WithMaxBudgetUSD(usd float64) Option {
	return func(o *Options) { o.MaxBudgetUSD = &usd }
}

// WithModel specifies the AI model.
func WithModel(model string) Option {
	return func(o *Options) { o.Model = model }
}

// WithPermissionMode sets the permission mode.
func WithPermissionMode(mode string) Option {
	return func(o *Options) { o.PermissionMode = mode }
}

// WithCWD sets the working directory.
func WithCWD(dir string) Option {
	return func(o *Options) { o.CWD = dir }
}

// WithCLIPath overrides the CLI binary path.
func WithCLIPath(path string) Option {
	return func(o *Options) { o.CLIPath = path }
}

// WithEnv adds environment variables for the CLI process.
func WithEnv(env map[string]string) Option {
	return func(o *Options) { o.Env = env }
}

// WithMCPServers configures external MCP servers.
func WithMCPServers(servers map[string]MCPServerConfig) Option {
	return func(o *Options) { o.MCPServers = servers }
}

// WithSettings provides CLI settings as a JSON string or file path.
func WithSettings(settings string) Option {
	return func(o *Options) { o.Settings = settings }
}

// WithContinueConversation resumes the most recent conversation.
func WithContinueConversation() Option {
	return func(o *Options) { o.ContinueConversation = true }
}

// WithResume resumes a specific session by ID.
func WithResume(sessionID string) Option {
	return func(o *Options) { o.Resume = sessionID }
}

// WithAgents configures custom agents.
func WithAgents(agents map[string]*Definition) Option {
	return func(o *Options) { o.Agents = agents }
}

// WithHooks configures hook callbacks.
func WithHooks(hooks map[HookEvent][]HookMatcher) Option {
	return func(o *Options) { o.Hooks = hooks }
}

// WithOnToolUse sets the tool use callback.
func WithOnToolUse(fn OnToolUseFunc) Option {
	return func(o *Options) { o.OnToolUse = fn }
}

// WithOnAskUserQuestion sets a callback to handle AskUserQuestion tool calls.
// The callback is invoked once per parsed Question and returns a single
// answer string. The SDK loops over all questions, constructs the answers
// map and updatedInput, and allows the tool.
func WithOnAskUserQuestion(fn OnAskUserQuestionFunc) Option {
	return func(o *Options) { o.OnAskUserQuestion = fn }
}

// WithThinking configures extended thinking.
func WithThinking(cfg ThinkingConfig) Option {
	return func(o *Options) { o.Thinking = &cfg }
}

// WithEffort sets the thinking depth level.
func WithEffort(effort string) Option {
	return func(o *Options) { o.Effort = effort }
}

// WithIncludePartialMessages enables partial message streaming.
func WithIncludePartialMessages() Option {
	return func(o *Options) { o.IncludePartialMessages = true }
}

// WithOutputFormat configures structured output.
func WithOutputFormat(format map[string]any) Option {
	return func(o *Options) { o.OutputFormat = format }
}

func applyOptions(opts []Option) *Options {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	if o.OnToolUse != nil || o.OnAskUserQuestion != nil {
		o.PermissionPromptToolName = "stdio"
	}
	return o
}
