package agent

import "fmt"

// parseMessage converts a raw JSON map from the CLI into a typed Message.
// Returns nil for unrecognized message types (forward compatibility).
func parseMessage(data map[string]any) (Message, error) {
	msgType, _ := data["type"].(string)
	if msgType == "" {
		return nil, &MessageParseError{
			SDKError: SDKError{Message: "message missing 'type' field"},
			Data:     data,
		}
	}

	switch msgType {
	case "user":
		return parseUserMessage(data)
	case "assistant":
		return parseAssistantMessage(data)
	case "system":
		return parseSystemMessage(data)
	case "result":
		return parseResultMessage(data)
	case "stream_event":
		return parseStreamEvent(data)
	case "rate_limit_event":
		return parseRateLimitEvent(data)
	default:
		// Forward-compatible: skip unrecognized message types.
		return nil, nil
	}
}

func parseUserMessage(data map[string]any) (*UserMessage, error) {
	msg := &UserMessage{
		UUID:            strVal(data, "uuid"),
		ParentToolUseID: strVal(data, "parent_tool_use_id"),
	}

	if tr, ok := data["tool_use_result"].(map[string]any); ok {
		msg.ToolUseResult = tr
	}

	message, ok := data["message"].(map[string]any)
	if !ok {
		return nil, &MessageParseError{SDKError: SDKError{Message: "user message missing 'message' field"}, Data: data}
	}

	content := message["content"]
	switch c := content.(type) {
	case string:
		msg.Content = c
	case []any:
		blocks, err := parseContentBlocks(c)
		if err != nil {
			return nil, err
		}
		msg.Content = blocks
	default:
		msg.Content = fmt.Sprintf("%v", content)
	}

	return msg, nil
}

func parseAssistantMessage(data map[string]any) (*AssistantMessage, error) {
	message, ok := data["message"].(map[string]any)
	if !ok {
		return nil, &MessageParseError{SDKError: SDKError{Message: "assistant message missing 'message' field"}, Data: data}
	}

	contentRaw, _ := message["content"].([]any)
	blocks, err := parseContentBlocks(contentRaw)
	if err != nil {
		return nil, err
	}

	msg := &AssistantMessage{
		Content:         blocks,
		Model:           strVal(message, "model"),
		ParentToolUseID: strVal(data, "parent_tool_use_id"),
		Error:           strVal(data, "error"),
	}
	if u, ok := message["usage"].(map[string]any); ok {
		msg.Usage = u
	}

	return msg, nil
}

func parseSystemMessage(data map[string]any) (Message, error) {
	subtype, _ := data["subtype"].(string)

	switch subtype {
	case "task_started":
		return &TaskStartedMessage{
			SystemMessage: SystemMessage{Subtype: subtype, Data: data},
			TaskID:        strVal(data, "task_id"),
			Description:   strVal(data, "description"),
			UUID:          strVal(data, "uuid"),
			SessionID:     strVal(data, "session_id"),
			ToolUseID:     strVal(data, "tool_use_id"),
			TaskType:      strVal(data, "task_type"),
		}, nil

	case "task_progress":
		return &TaskProgressMessage{
			SystemMessage: SystemMessage{Subtype: subtype, Data: data},
			TaskID:        strVal(data, "task_id"),
			Description:   strVal(data, "description"),
			Usage:         parseTaskUsage(data["usage"]),
			UUID:          strVal(data, "uuid"),
			SessionID:     strVal(data, "session_id"),
			ToolUseID:     strVal(data, "tool_use_id"),
			LastToolName:  strVal(data, "last_tool_name"),
		}, nil

	case "task_notification":
		var usage *TaskUsage
		if u := data["usage"]; u != nil {
			tu := parseTaskUsage(u)
			usage = &tu
		}
		return &TaskNotificationMessage{
			SystemMessage: SystemMessage{Subtype: subtype, Data: data},
			TaskID:        strVal(data, "task_id"),
			Status:        strVal(data, "status"),
			OutputFile:    strVal(data, "output_file"),
			Summary:       strVal(data, "summary"),
			UUID:          strVal(data, "uuid"),
			SessionID:     strVal(data, "session_id"),
			ToolUseID:     strVal(data, "tool_use_id"),
			Usage:         usage,
		}, nil

	default:
		return &SystemMessage{Subtype: subtype, Data: data}, nil
	}
}

func parseResultMessage(data map[string]any) (*ResultMessage, error) {
	msg := &ResultMessage{
		Subtype:       strVal(data, "subtype"),
		DurationMS:    intVal(data, "duration_ms"),
		DurationAPIMS: intVal(data, "duration_api_ms"),
		IsError:       boolVal(data, "is_error"),
		NumTurns:      intVal(data, "num_turns"),
		SessionID:     strVal(data, "session_id"),
		StopReason:    strVal(data, "stop_reason"),
		TotalCostUSD:  floatVal(data, "total_cost_usd"),
		Result:        strVal(data, "result"),
	}
	if u, ok := data["usage"].(map[string]any); ok {
		msg.Usage = u
	}
	msg.StructuredOutput = data["structured_output"]
	return msg, nil
}

func parseStreamEvent(data map[string]any) (*StreamEvent, error) {
	event, _ := data["event"].(map[string]any)
	return &StreamEvent{
		UUID:            strVal(data, "uuid"),
		SessionID:       strVal(data, "session_id"),
		Event:           event,
		ParentToolUseID: strVal(data, "parent_tool_use_id"),
	}, nil
}

func parseRateLimitEvent(data map[string]any) (*RateLimitEvent, error) {
	info, _ := data["rate_limit_info"].(map[string]any)
	if info == nil {
		return nil, &MessageParseError{SDKError: SDKError{Message: "rate_limit_event missing rate_limit_info"}, Data: data}
	}

	rli := RateLimitInfo{
		Status:                strVal(info, "status"),
		RateLimitType:         strVal(info, "rateLimitType"),
		OverageStatus:         strVal(info, "overageStatus"),
		OverageDisabledReason: strVal(info, "overageDisabledReason"),
		Raw:                   info,
	}
	if v, ok := numVal(info, "resetsAt"); ok {
		i := int64(v)
		rli.ResetsAt = &i
	}
	if v, ok := numVal(info, "utilization"); ok {
		rli.Utilization = &v
	}
	if v, ok := numVal(info, "overageResetsAt"); ok {
		i := int64(v)
		rli.OverageResetsAt = &i
	}

	return &RateLimitEvent{
		RateLimitInfo: rli,
		UUID:          strVal(data, "uuid"),
		SessionID:     strVal(data, "session_id"),
	}, nil
}

func parseContentBlocks(raw []any) ([]ContentBlock, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	blocks := make([]ContentBlock, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		blockType, _ := m["type"].(string)
		switch blockType {
		case "text":
			blocks = append(blocks, &TextBlock{Text: strVal(m, "text")})
		case "thinking":
			blocks = append(blocks, &ThinkingBlock{
				Thinking:  strVal(m, "thinking"),
				Signature: strVal(m, "signature"),
			})
		case "tool_use":
			input, _ := m["input"].(map[string]any)
			blocks = append(blocks, &ToolUseBlock{
				ID:    strVal(m, "id"),
				Name:  strVal(m, "name"),
				Input: input,
			})
		case "tool_result":
			blocks = append(blocks, &ToolResultBlock{
				ToolUseID: strVal(m, "tool_use_id"),
				Content:   m["content"],
				IsError:   boolVal(m, "is_error"),
			})
		}
	}
	return blocks, nil
}

func parseTaskUsage(v any) TaskUsage {
	m, ok := v.(map[string]any)
	if !ok {
		return TaskUsage{}
	}
	return TaskUsage{
		TotalTokens: intVal(m, "total_tokens"),
		ToolUses:    intVal(m, "tool_uses"),
		DurationMS:  intVal(m, "duration_ms"),
	}
}

func parseHookInput(data map[string]any) HookInput {
	input := HookInput{
		HookEventName:  strVal(data, "hook_event_name"),
		SessionID:      strVal(data, "session_id"),
		TranscriptPath: strVal(data, "transcript_path"),
		CWD:            strVal(data, "cwd"),
		PermissionMode: strVal(data, "permission_mode"),
		ToolName:       strVal(data, "tool_name"),
		ToolUseID:      strVal(data, "tool_use_id"),
		Error:          strVal(data, "error"),
		IsInterrupt:    boolVal(data, "is_interrupt"),
		Prompt:         strVal(data, "prompt"),
		StopHookActive: boolVal(data, "stop_hook_active"),
		AgentID:        strVal(data, "agent_id"),
		AgentType:      strVal(data, "agent_type"),
		AgentTranscriptPath: strVal(data, "agent_transcript_path"),
		Title:               strVal(data, "title"),
		NotificationType:    strVal(data, "notification_type"),
		Trigger:             strVal(data, "trigger"),
		CustomInstructions:  strVal(data, "custom_instructions"),
	}
	if ti, ok := data["tool_input"].(map[string]any); ok {
		input.ToolInput = ti
	}
	input.ToolResponse = data["tool_response"]
	if msg, ok := data["message"].(string); ok {
		input.NotificationMessage = msg
	}
	if ps, ok := data["permission_suggestions"].([]any); ok {
		input.PermissionSuggestions = ps
	}
	return input
}

// --- helpers ---

func strVal(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func intVal(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

func floatVal(m map[string]any, key string) float64 {
	v, _ := m[key].(float64)
	return v
}

func boolVal(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}

func numVal(m map[string]any, key string) (float64, bool) {
	v, ok := m[key].(float64)
	return v, ok
}
