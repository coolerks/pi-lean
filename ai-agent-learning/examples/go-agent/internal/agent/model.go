package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Role is the small internal message vocabulary used by the harness.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall is model output, not evidence that a function has executed.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// Message is deliberately provider-neutral. A real implementation may keep
// richer content blocks, images, and thinking signatures in separate fields.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolName   string     `json:"name,omitempty"`
	IsError    bool       `json:"is_error,omitempty"`
	Timestamp  time.Time  `json:"timestamp,omitempty"`
}

// Usage is kept separate from message text so accounting does not depend on
// whether a response is eventually shown to the user.
type Usage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	Cost         float64 `json:"cost,omitempty"`
}

type Response struct {
	Message    Message `json:"message"`
	StopReason string  `json:"stop_reason"`
	Usage      Usage   `json:"usage"`
}

type Request struct {
	Messages []Message        `json:"messages"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
}

// Model is the only dependency the AgentLoop has on a language model.
type Model interface {
	Complete(context.Context, Request) (Response, error)
}

// NewToolCall makes scripted responses readable and validates that arguments
// are JSON at construction time.
func NewToolCall(id, name string, arguments map[string]any) ToolCall {
	encoded, err := json.Marshal(arguments)
	if err != nil {
		panic(fmt.Sprintf("encode tool arguments: %v", err))
	}
	return ToolCall{ID: id, Name: name, Arguments: encoded}
}

// ScriptedModel is deterministic test infrastructure. It records requests so
// tests can prove that tool results were sent back to the model.
type ScriptedModel struct {
	mu        sync.Mutex
	Responses []Response
	Errors    []error
	Requests  []Request
	Delay     time.Duration
}

func (m *ScriptedModel) Complete(ctx context.Context, request Request) (Response, error) {
	m.mu.Lock()
	m.Requests = append(m.Requests, cloneRequest(request))
	var response Response
	var err error
	if len(m.Errors) > 0 {
		err = m.Errors[0]
		m.Errors = m.Errors[1:]
	}
	if err == nil && len(m.Responses) > 0 {
		response = m.Responses[0]
		m.Responses = m.Responses[1:]
	}
	m.mu.Unlock()

	if m.Delay > 0 {
		timer := time.NewTimer(m.Delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return Response{}, ctx.Err()
		}
	}
	if err != nil {
		return Response{}, err
	}
	if response.Message.Role == "" {
		return Response{}, errors.New("scripted model has no response left")
	}
	return response, nil
}

func cloneRequest(request Request) Request {
	copyRequest := Request{Tools: append([]ToolDefinition(nil), request.Tools...)}
	copyRequest.Messages = append([]Message(nil), request.Messages...)
	for index := range copyRequest.Messages {
		copyRequest.Messages[index].ToolCalls = append([]ToolCall(nil), copyRequest.Messages[index].ToolCalls...)
	}
	return copyRequest
}

// HTTPModel speaks the OpenAI-compatible Chat Completions shape. It is kept
// intentionally small so the provider boundary remains visible in the lesson.
type HTTPModel struct {
	Client  *http.Client
	URL     string
	APIKey  string
	ModelID string
}

func (m *HTTPModel) Complete(ctx context.Context, request Request) (Response, error) {
	payload := m.payload(request, false)
	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, m.URL, bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	m.addHeaders(httpRequest)
	client := m.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(httpRequest)
	if err != nil {
		return Response{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 16<<10))
		return Response{}, fmt.Errorf("model request failed (%s): %s", response.Status, strings.TrimSpace(string(message)))
	}
	var decoded chatResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return Response{}, fmt.Errorf("decode model response: %w", err)
	}
	return decoded.response()
}

// StreamingModel is an optional capability; the AgentLoop only needs Model.
type StreamingModel interface {
	Stream(context.Context, Request, func(string)) (Response, error)
}

func (m *HTTPModel) Stream(ctx context.Context, request Request, onDelta func(string)) (Response, error) {
	payload := m.payload(request, true)
	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, m.URL, bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	m.addHeaders(httpRequest)
	client := m.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(httpRequest)
	if err != nil {
		return Response{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 16<<10))
		return Response{}, fmt.Errorf("stream request failed (%s): %s", response.Status, strings.TrimSpace(string(message)))
	}

	var content strings.Builder
	var toolCalls []ToolCall
	finishReason := "stop"
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 1024), 4<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk chatChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return Response{}, fmt.Errorf("decode SSE data: %w", err)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		if delta.Content != "" {
			content.WriteString(delta.Content)
			if onDelta != nil {
				onDelta(delta.Content)
			}
		}
		if chunk.Choices[0].FinishReason != "" {
			finishReason = chunk.Choices[0].FinishReason
		}
		for _, call := range delta.ToolCalls {
			for len(toolCalls) <= call.Index {
				toolCalls = append(toolCalls, ToolCall{})
			}
			toolCalls[call.Index].ID = firstNonEmpty(toolCalls[call.Index].ID, call.ID)
			toolCalls[call.Index].Name = firstNonEmpty(toolCalls[call.Index].Name, call.Function.Name)
			toolCalls[call.Index].Arguments = append(toolCalls[call.Index].Arguments, []byte(call.Function.Arguments)...)
		}
	}
	if err := scanner.Err(); err != nil {
		return Response{}, err
	}
	for index := range toolCalls {
		if len(toolCalls[index].Arguments) == 0 {
			toolCalls[index].Arguments = json.RawMessage(`{}`)
		}
	}
	return Response{Message: Message{Role: RoleAssistant, Content: content.String(), ToolCalls: toolCalls, Timestamp: time.Now()}, StopReason: finishReason}, nil
}

func firstNonEmpty(current, next string) string {
	if current != "" {
		return current
	}
	return next
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Role      string         `json:"role"`
			Content   string         `json:"content"`
			ToolCalls []chatToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage usagePayload `json:"usage"`
}

type chatChunk struct {
	Choices []struct {
		Delta        chatDelta `json:"delta"`
		FinishReason string    `json:"finish_reason"`
	} `json:"choices"`
	Usage usagePayload `json:"usage"`
}

type chatDelta struct {
	Content   string         `json:"content"`
	ToolCalls []chatToolCall `json:"tool_calls"`
}

type chatToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type usagePayload struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (r chatResponse) response() (Response, error) {
	if len(r.Choices) == 0 {
		return Response{}, errors.New("model response has no choices")
	}
	choice := r.Choices[0]
	calls := make([]ToolCall, len(choice.Message.ToolCalls))
	for index, call := range choice.Message.ToolCalls {
		arguments := json.RawMessage(call.Function.Arguments)
		if len(arguments) == 0 {
			arguments = json.RawMessage(`{}`)
		}
		calls[index] = ToolCall{ID: call.ID, Name: call.Function.Name, Arguments: arguments}
	}
	return Response{
		Message:    Message{Role: RoleAssistant, Content: choice.Message.Content, ToolCalls: calls, Timestamp: time.Now()},
		StopReason: choice.FinishReason,
		Usage:      Usage{InputTokens: r.Usage.PromptTokens, OutputTokens: r.Usage.CompletionTokens, TotalTokens: r.Usage.TotalTokens},
	}, nil
}

func (m *HTTPModel) payload(request Request, stream bool) map[string]any {
	messages := make([]map[string]any, 0, len(request.Messages))
	for _, message := range request.Messages {
		entry := map[string]any{"role": string(message.Role), "content": message.Content}
		if message.ToolCallID != "" {
			entry["tool_call_id"] = message.ToolCallID
		}
		if message.ToolName != "" {
			entry["name"] = message.ToolName
		}
		if len(message.ToolCalls) > 0 {
			calls := make([]map[string]any, 0, len(message.ToolCalls))
			for _, call := range message.ToolCalls {
				calls = append(calls, map[string]any{
					"id":       call.ID,
					"type":     "function",
					"function": map[string]any{"name": call.Name, "arguments": string(call.Arguments)},
				})
			}
			entry["tool_calls"] = calls
		}
		messages = append(messages, entry)
	}
	payload := map[string]any{"model": m.ModelID, "messages": messages}
	if len(request.Tools) > 0 {
		tools := make([]map[string]any, 0, len(request.Tools))
		for _, tool := range request.Tools {
			tools = append(tools, map[string]any{"type": "function", "function": map[string]any{
				"name": tool.Name, "description": tool.Description, "parameters": tool.Schema,
			}})
		}
		payload["tools"] = tools
	}
	if stream {
		payload["stream"] = true
	}
	return payload
}

func (m *HTTPModel) addHeaders(request *http.Request) {
	request.Header.Set("Content-Type", "application/json")
	if m.APIKey != "" {
		request.Header.Set("Authorization", "Bearer "+m.APIKey)
	}
}
