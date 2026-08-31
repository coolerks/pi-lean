package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Event is intentionally small. Applications can serialize it or attach a
// richer trace/span id without changing the model contract.
type Event struct {
	Type       string
	Step       int
	Message    *Message
	ToolCall   *ToolCall
	ToolResult *ToolResult
	Error      error
	At         time.Time
}

type EventSink func(Event)

type ExecutionMode string

const (
	Sequential ExecutionMode = "sequential"
	Parallel   ExecutionMode = "parallel"
)

type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
}

type LoopOptions struct {
	MaxSteps   int
	ToolMode   ExecutionMode
	Retry      RetryPolicy
	BeforeTool BeforeToolHook
	AfterTool  AfterToolHook
	OnEvent    EventSink
	Steering   func() []Message
	FollowUps  func() []Message
}

type RunResult struct {
	Messages []Message
	Final    Message
	Steps    int
}

type AgentLoop struct {
	Model   Model
	Tools   *Registry
	Options LoopOptions
}

func (l AgentLoop) Run(ctx context.Context, initial []Message) (RunResult, error) {
	options := l.Options
	if options.MaxSteps <= 0 {
		options.MaxSteps = 16
	}
	if options.ToolMode == "" {
		options.ToolMode = Parallel
	}
	if options.Retry.MaxAttempts <= 0 {
		options.Retry.MaxAttempts = 1
	}
	if l.Model == nil {
		return RunResult{}, errors.New("agent loop has no model")
	}
	if l.Tools == nil {
		l.Tools = NewRegistry()
	}
	// Store normalized defaults on this value so events emitted by helper
	// methods use the per-run callbacks too.
	l.Options = options

	history := append([]Message(nil), initial...)
	var final Message
	for step := 1; step <= options.MaxSteps; step++ {
		for _, message := range drainMessages(options.Steering) {
			history = append(history, message)
			l.emit(Event{Type: "steering", Step: step, Message: followUpPtr(message)})
		}
		l.emit(Event{Type: "step_start", Step: step})
		response, err := l.completeWithRetry(ctx, Request{Messages: history, Tools: l.Tools.Definitions()}, options.Retry, step)
		if err != nil {
			l.emit(Event{Type: "model_error", Step: step, Error: err})
			return RunResult{Messages: history, Steps: step}, err
		}
		message := response.Message
		if message.Role == "" {
			message.Role = RoleAssistant
		}
		if message.Timestamp.IsZero() {
			message.Timestamp = time.Now()
		}
		history = append(history, message)
		final = message
		l.emit(Event{Type: "assistant", Step: step, Message: &message})

		if len(message.ToolCalls) == 0 {
			followUps := drainMessages(options.FollowUps)
			if len(followUps) == 0 {
				l.emit(Event{Type: "run_end", Step: step, Message: &message})
				return RunResult{Messages: history, Final: final, Steps: step}, nil
			}
			for _, followUp := range followUps {
				history = append(history, followUp)
				l.emit(Event{Type: "follow_up", Step: step, Message: followUpPtr(followUp)})
			}
			continue
		}

		results := l.executeBatch(ctx, message, options, step)
		terminate := true
		for index := range results {
			result := results[index]
			toolMessage := Message{
				Role:       RoleTool,
				Content:    result.Content,
				ToolCallID: message.ToolCalls[index].ID,
				ToolName:   message.ToolCalls[index].Name,
				IsError:    result.IsError,
				Timestamp:  time.Now(),
			}
			history = append(history, toolMessage)
			l.emit(Event{Type: "tool_result", Step: step, ToolCall: callPtr(message.ToolCalls[index]), ToolResult: resultPtr(result), Message: &toolMessage})
			if !result.Terminate {
				terminate = false
			}
		}
		if terminate {
			followUps := drainMessages(options.FollowUps)
			if len(followUps) == 0 {
				l.emit(Event{Type: "run_end", Step: step, Message: &final})
				return RunResult{Messages: history, Final: final, Steps: step}, nil
			}
			for _, followUp := range followUps {
				history = append(history, followUp)
			}
		}
	}
	return RunResult{Messages: history, Final: final, Steps: options.MaxSteps}, fmt.Errorf("max steps exceeded (%d)", options.MaxSteps)
}

func (l AgentLoop) completeWithRetry(ctx context.Context, request Request, policy RetryPolicy, step int) (Response, error) {
	var lastErr error
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		l.emit(Event{Type: "model_attempt", Step: step})
		response, err := l.Model.Complete(ctx, request)
		if err == nil {
			return response, nil
		}
		lastErr = err
		if ctx.Err() != nil || attempt == policy.MaxAttempts {
			break
		}
		delay := policy.BaseDelay
		if delay <= 0 {
			delay = 50 * time.Millisecond
		}
		delay *= time.Duration(1 << (attempt - 1))
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return Response{}, ctx.Err()
		}
	}
	return Response{}, lastErr
}

func (l AgentLoop) executeBatch(ctx context.Context, assistant Message, options LoopOptions, step int) []ToolResult {
	results := make([]ToolResult, len(assistant.ToolCalls))
	if options.ToolMode == Sequential || len(assistant.ToolCalls) < 2 {
		for index, call := range assistant.ToolCalls {
			results[index] = l.executeOne(ctx, assistant, call, options, step)
		}
		return results
	}

	var waitGroup sync.WaitGroup
	for index, call := range assistant.ToolCalls {
		waitGroup.Add(1)
		go func(index int, call ToolCall) {
			defer waitGroup.Done()
			results[index] = l.executeOne(ctx, assistant, call, options, step)
		}(index, call)
	}
	waitGroup.Wait()
	return results
}

func (l AgentLoop) executeOne(ctx context.Context, assistant Message, call ToolCall, options LoopOptions, step int) ToolResult {
	l.emit(Event{Type: "tool_start", Step: step, ToolCall: callPtr(call)})
	tool, err := l.Tools.Validate(call)
	if err != nil {
		result := errorToolResult(err)
		l.emit(Event{Type: "tool_end", Step: step, ToolCall: callPtr(call), ToolResult: resultPtr(result), Error: err})
		return result
	}
	if options.BeforeTool != nil {
		if err := options.BeforeTool(ctx, call, tool.Definition()); err != nil {
			result := errorToolResult(err)
			l.emit(Event{Type: "tool_end", Step: step, ToolCall: callPtr(call), ToolResult: resultPtr(result), Error: err})
			return result
		}
	}
	result, err := tool.Execute(ctx, call.Arguments)
	if err != nil {
		result.IsError = true
		if result.Content == "" {
			result.Content = err.Error()
		}
	}
	if options.AfterTool != nil {
		hookResult, hookErr := options.AfterTool(ctx, call, result)
		if hookErr != nil {
			result = errorToolResult(hookErr)
			err = hookErr
		} else {
			result = hookResult
		}
	}
	l.emit(Event{Type: "tool_end", Step: step, ToolCall: callPtr(call), ToolResult: resultPtr(result), Error: err})
	return result
}

func (l AgentLoop) emit(event Event) {
	if l.Options.OnEvent != nil {
		event.At = time.Now()
		l.Options.OnEvent(event)
	}
}

func drainMessages(source func() []Message) []Message {
	if source == nil {
		return nil
	}
	return source()
}

func errorToolResult(err error) ToolResult {
	return ToolResult{Content: err.Error(), IsError: true}
}

func callPtr(call ToolCall) *ToolCall         { return &call }
func resultPtr(result ToolResult) *ToolResult { return &result }
func followUpPtr(message Message) *Message    { return &message }
