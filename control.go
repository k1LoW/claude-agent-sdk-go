package agent

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"sync"
	"sync/atomic"
)

// controlSession handles the bidirectional control protocol on top of Transport.
// It routes control messages, manages hooks and tool permission callbacks,
// and forwards regular messages to the consumer.
type controlSession struct {
	ctx    context.Context //nostyle:contexts // used by readLoop/transportReader goroutines; passing via method args is not feasible
	cancel context.CancelFunc

	transport Transport
	options   *Options

	msgCh  chan Message
	doneCh chan struct{}

	mu              sync.Mutex
	pendingRequests map[string]chan controlResult
	requestCounter  int
	hookCallbacks   map[string]HookCallback
	nextCallbackID  int

	firstResultOnce sync.Once
	firstResultCh   chan struct{}

	readErrMu sync.Mutex
	readErr   error
	closed    atomic.Bool
}

type controlResult struct {
	response map[string]any
	err      error
}

func newControlSession(ctx context.Context, transport Transport, options *Options) *controlSession {
	ctx, cancel := context.WithCancel(ctx)
	return &controlSession{
		ctx:             ctx,
		cancel:          cancel,
		transport:       transport,
		options:         options,
		msgCh:           make(chan Message, 100),
		doneCh:          make(chan struct{}),
		pendingRequests: make(map[string]chan controlResult),
		hookCallbacks:   make(map[string]HookCallback),
		firstResultCh:   make(chan struct{}),
	}
}

// start begins reading messages from the transport in a goroutine.
func (cs *controlSession) start() {
	readCh := make(chan readResult, 1)
	go cs.transportReader(readCh)
	go cs.readLoop(readCh)
}

type readResult struct {
	msg map[string]any
	err error
}

// transportReader is a single goroutine that continuously reads from the
// transport and sends results to readCh. It exits when ReadMessage returns
// an error (including io.EOF) or when the transport is closed.
func (cs *controlSession) transportReader(readCh chan<- readResult) {
	defer close(readCh)
	for {
		raw, err := cs.transport.ReadMessage()
		select {
		case readCh <- readResult{raw, err}:
		case <-cs.ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

func (cs *controlSession) readLoop(readCh <-chan readResult) {
	defer close(cs.doneCh)
	defer close(cs.msgCh)

	for {
		var rr readResult
		var ok bool
		select {
		case rr, ok = <-readCh:
			if !ok {
				return
			}
		case <-cs.ctx.Done():
			cs.signalPendingRequests(cs.ctx.Err())
			return
		}

		if rr.err != nil {
			if !errors.Is(rr.err, io.EOF) {
				cs.setReadErr(rr.err)
			}
			cs.signalPendingRequests(fmt.Errorf("transport closed"))
			return
		}

		msgType, _ := rr.msg["type"].(string)

		switch msgType {
		case "control_response":
			cs.handleControlResponse(rr.msg)

		case "control_request":
			go cs.handleControlRequest(rr.msg)

		case "control_cancel_request":
			cs.handleControlCancelRequest(rr.msg)

		default:
			if msgType == "result" {
				cs.firstResultOnce.Do(func() { close(cs.firstResultCh) })
			}

			msg, err := parseMessage(rr.msg)
			if err != nil {
				// Parse errors for individual messages are not fatal
				continue
			}
			if msg == nil {
				// Unrecognized message type, skip
				continue
			}

			select {
			case cs.msgCh <- msg:
			case <-cs.ctx.Done():
				return
			}
		}
	}
}

func (cs *controlSession) handleControlCancelRequest(raw map[string]any) {
	requestID, _ := raw["request_id"].(string)
	if requestID == "" {
		return
	}

	cs.mu.Lock()
	ch, ok := cs.pendingRequests[requestID]
	if ok {
		delete(cs.pendingRequests, requestID)
	}
	cs.mu.Unlock()

	if ok {
		ch <- controlResult{err: fmt.Errorf("request canceled by CLI")}
	}
}

func (cs *controlSession) handleControlResponse(raw map[string]any) {
	resp, _ := raw["response"].(map[string]any)
	if resp == nil {
		return
	}
	requestID, _ := resp["request_id"].(string)
	if requestID == "" {
		return
	}

	cs.mu.Lock()
	ch, ok := cs.pendingRequests[requestID]
	if ok {
		delete(cs.pendingRequests, requestID)
	}
	cs.mu.Unlock()

	if !ok {
		return
	}

	subtype, _ := resp["subtype"].(string)
	if subtype == "error" {
		errMsg, _ := resp["error"].(string)
		ch <- controlResult{err: fmt.Errorf("control error: %s", errMsg)}
	} else {
		ch <- controlResult{response: resp}
	}
}

func (cs *controlSession) handleControlRequest(raw map[string]any) {
	requestID, _ := raw["request_id"].(string)
	request, _ := raw["request"].(map[string]any)
	if request == nil {
		return
	}
	subtype, _ := request["subtype"].(string)

	var responseData map[string]any
	var respErr error

	switch subtype {
	case "can_use_tool":
		responseData, respErr = cs.handleCanUseTool(request)
	case "hook_callback":
		responseData, respErr = cs.handleHookCallback(request)
	default:
		respErr = fmt.Errorf("unsupported control request subtype: %s", subtype)
	}

	var response map[string]any
	if respErr != nil {
		response = map[string]any{
			"type": "control_response",
			"response": map[string]any{
				"subtype":    "error",
				"request_id": requestID,
				"error":      respErr.Error(),
			},
		}
	} else {
		response = map[string]any{
			"type": "control_response",
			"response": map[string]any{
				"subtype":    "success",
				"request_id": requestID,
				"response":   responseData,
			},
		}
	}

	b, err := json.Marshal(response)
	if err != nil {
		return
	}
	if err := cs.transport.Write(append(b, '\n')); err != nil {
		cs.setReadErr(err)
	}
}

func (cs *controlSession) handleCanUseTool(request map[string]any) (map[string]any, error) {
	toolName, _ := request["tool_name"].(string)
	input, _ := request["input"].(map[string]any)
	if input == nil {
		input = map[string]any{}
	}

	if toolName == "AskUserQuestion" && cs.options.AnswerUserQuestions != nil {
		return cs.handleAskUserQuestion(input)
	}

	if cs.options.CanUseTool == nil {
		return map[string]any{
			"behavior":     "allow",
			"updatedInput": input,
		}, nil
	}

	tctx := ToolPermissionContext{}
	// TODO: parse permission_suggestions from request

	result, err := cs.options.CanUseTool(cs.ctx, toolName, input, tctx)
	if err != nil {
		return nil, err
	}

	switch r := result.(type) {
	case *PermissionAllow:
		resp := map[string]any{
			"behavior": "allow",
		}
		if r.UpdatedInput != nil {
			resp["updatedInput"] = r.UpdatedInput
		} else {
			resp["updatedInput"] = input
		}
		if len(r.UpdatedPermissions) > 0 {
			perms := make([]map[string]any, len(r.UpdatedPermissions))
			for i, p := range r.UpdatedPermissions {
				perms[i] = p.ToMap()
			}
			resp["updatedPermissions"] = perms
		}
		return resp, nil

	case *PermissionDeny:
		resp := map[string]any{
			"behavior": "deny",
			"message":  r.Message,
		}
		if r.Interrupt {
			resp["interrupt"] = true
		}
		return resp, nil

	default:
		return nil, fmt.Errorf("unexpected permission result type: %T", result)
	}
}

func (cs *controlSession) handleAskUserQuestion(input map[string]any) (map[string]any, error) {
	questionsRaw, questions := parseQuestions(input)

	answers, err := cs.options.AnswerUserQuestions(cs.ctx, questions)
	if err != nil {
		return nil, err
	}

	updatedInput := make(map[string]any, len(input)+1)
	maps.Copy(updatedInput, input)
	updatedInput["questions"] = questionsRaw
	updatedInput["answers"] = answers

	return map[string]any{
		"behavior":     "allow",
		"updatedInput": updatedInput,
	}, nil
}

func (cs *controlSession) handleHookCallback(request map[string]any) (map[string]any, error) {
	callbackID, _ := request["callback_id"].(string)

	cs.mu.Lock()
	callback, ok := cs.hookCallbacks[callbackID]
	cs.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("no hook callback found for ID: %s", callbackID)
	}

	inputData, _ := request["input"].(map[string]any)
	if inputData == nil {
		inputData = map[string]any{}
	}
	hookInput := parseHookInput(inputData)
	toolUseID, _ := request["tool_use_id"].(string)

	output, err := callback(cs.ctx, hookInput, toolUseID)
	if err != nil {
		return nil, err
	}

	// Convert to map
	result := make(map[string]any)
	if output.Continue != nil {
		result["continue"] = *output.Continue
	}
	if output.SuppressOutput {
		result["suppressOutput"] = true
	}
	if output.StopReason != "" {
		result["stopReason"] = output.StopReason
	}
	if output.Decision != "" {
		result["decision"] = output.Decision
	}
	if output.SystemMessage != "" {
		result["systemMessage"] = output.SystemMessage
	}
	if output.Reason != "" {
		result["reason"] = output.Reason
	}
	if output.HookSpecificOutput != nil {
		result["hookSpecificOutput"] = output.HookSpecificOutput
	}
	return result, nil
}

// sendControlRequest sends a control request and waits for the response.
func (cs *controlSession) sendControlRequest(ctx context.Context, request map[string]any) (map[string]any, error) {
	cs.mu.Lock()
	cs.requestCounter++
	randBytes := make([]byte, 4)
	rand.Read(randBytes)
	requestID := fmt.Sprintf("req_%d_%x", cs.requestCounter, randBytes)
	ch := make(chan controlResult, 1)
	cs.pendingRequests[requestID] = ch
	cs.mu.Unlock()

	controlRequest := map[string]any{
		"type":       "control_request",
		"request_id": requestID,
		"request":    request,
	}

	b, err := json.Marshal(controlRequest)
	if err != nil {
		cs.mu.Lock()
		delete(cs.pendingRequests, requestID)
		cs.mu.Unlock()
		return nil, err
	}

	if err := cs.transport.Write(append(b, '\n')); err != nil {
		cs.mu.Lock()
		delete(cs.pendingRequests, requestID)
		cs.mu.Unlock()
		return nil, err
	}

	select {
	case <-ctx.Done():
		cs.mu.Lock()
		delete(cs.pendingRequests, requestID)
		cs.mu.Unlock()
		return nil, ctx.Err()
	case result := <-ch:
		if result.err != nil {
			return nil, result.err
		}
		resp, _ := result.response["response"].(map[string]any)
		if resp == nil {
			resp = map[string]any{}
		}
		return resp, nil
	}
}

// initialize sends the initialize control request.
func (cs *controlSession) initialize(ctx context.Context) (map[string]any, error) {
	// Build hooks config with callback IDs
	var hooksConfig map[string]any
	if cs.options.Hooks != nil {
		hooksConfig = make(map[string]any)
		for event, matchers := range cs.options.Hooks {
			if len(matchers) == 0 {
				continue
			}
			matcherConfigs := make([]map[string]any, 0, len(matchers))
			for _, matcher := range matchers {
				callbackIDs := make([]string, 0, len(matcher.Hooks))
				cs.mu.Lock()
				for _, cb := range matcher.Hooks {
					id := fmt.Sprintf("hook_%d", cs.nextCallbackID)
					cs.nextCallbackID++
					cs.hookCallbacks[id] = cb
					callbackIDs = append(callbackIDs, id)
				}
				cs.mu.Unlock()

				mc := map[string]any{
					"matcher":         matcher.Matcher,
					"hookCallbackIds": callbackIDs,
				}
				if matcher.Timeout > 0 {
					mc["timeout"] = matcher.Timeout
				}
				matcherConfigs = append(matcherConfigs, mc)
			}
			hooksConfig[string(event)] = matcherConfigs
		}
	}

	request := map[string]any{
		"subtype": "initialize",
		"hooks":   hooksConfig,
	}

	// Add agents
	if cs.options.Agents != nil {
		agents := make(map[string]any)
		for name, def := range cs.options.Agents {
			agents[name] = def.toMap()
		}
		request["agents"] = agents
	}

	return cs.sendControlRequest(ctx, request)
}

// interrupt sends an interrupt control request.
func (cs *controlSession) interrupt(ctx context.Context) error {
	_, err := cs.sendControlRequest(ctx, map[string]any{"subtype": "interrupt"})
	return err
}

// setPermissionMode changes the permission mode.
func (cs *controlSession) setPermissionMode(ctx context.Context, mode string) error {
	_, err := cs.sendControlRequest(ctx, map[string]any{
		"subtype": "set_permission_mode",
		"mode":    mode,
	})
	return err
}

// setModel changes the AI model.
func (cs *controlSession) setModel(ctx context.Context, model string) error {
	_, err := cs.sendControlRequest(ctx, map[string]any{
		"subtype": "set_model",
		"model":   model,
	})
	return err
}

// mcpStatus gets the current MCP server connection status.
func (cs *controlSession) mcpStatus(ctx context.Context) (map[string]any, error) {
	return cs.sendControlRequest(ctx, map[string]any{"subtype": "mcp_status"})
}

// reconnectMCPServer reconnects a disconnected MCP server.
func (cs *controlSession) reconnectMCPServer(ctx context.Context, serverName string) error {
	_, err := cs.sendControlRequest(ctx, map[string]any{
		"subtype":    "mcp_reconnect",
		"serverName": serverName,
	})
	return err
}

// toggleMCPServer enables or disables an MCP server.
func (cs *controlSession) toggleMCPServer(ctx context.Context, serverName string, enabled bool) error {
	_, err := cs.sendControlRequest(ctx, map[string]any{
		"subtype":    "mcp_toggle",
		"serverName": serverName,
		"enabled":    enabled,
	})
	return err
}

// stopTask stops a running task.
func (cs *controlSession) stopTask(ctx context.Context, taskID string) error {
	_, err := cs.sendControlRequest(ctx, map[string]any{
		"subtype": "stop_task",
		"task_id": taskID,
	})
	return err
}

// rewindFiles rewinds tracked files to a specific user message state.
func (cs *controlSession) rewindFiles(ctx context.Context, userMessageID string) error {
	_, err := cs.sendControlRequest(ctx, map[string]any{
		"subtype":         "rewind_files",
		"user_message_id": userMessageID,
	})
	return err
}

// sendUserMessage marshals and writes a user message to the transport.
func (cs *controlSession) sendUserMessage(prompt string, sessionID string) error {
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
	return cs.transport.Write(append(b, '\n'))
}

// waitForResultAndEndInput waits for the first result then closes stdin.
func (cs *controlSession) waitForResultAndEndInput() error {
	needsWait := len(cs.hookCallbacks) > 0 || cs.options.CanUseTool != nil || cs.options.AnswerUserQuestions != nil

	if needsWait {
		select {
		case <-cs.firstResultCh:
		case <-cs.ctx.Done():
		}
	}

	return cs.transport.EndInput()
}

// signalPendingRequests notifies all pending control requests with the given error and closes firstResultCh.
func (cs *controlSession) signalPendingRequests(err error) {
	cs.mu.Lock()
	for id, ch := range cs.pendingRequests {
		ch <- controlResult{err: err}
		delete(cs.pendingRequests, id)
	}
	cs.mu.Unlock()
	cs.firstResultOnce.Do(func() { close(cs.firstResultCh) })
}

func (cs *controlSession) setReadErr(err error) {
	cs.readErrMu.Lock()
	cs.readErr = errors.Join(cs.readErr, err)
	cs.readErrMu.Unlock()
}

func (cs *controlSession) readError() error {
	cs.readErrMu.Lock()
	defer cs.readErrMu.Unlock()
	return cs.readErr
}

// close shuts down the control session.
func (cs *controlSession) close() error {
	cs.closed.Store(true)
	cs.cancel()
	<-cs.doneCh
	return cs.transport.Close()
}
