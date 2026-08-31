package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Harness composes the pieces used in the lessons. It persists the complete
// local transcript after every accepted prompt; the durable v2 design in the
// Pi docs adds operation intents and crash recovery around this boundary.
type Harness struct {
	Agent       *Agent
	Session     *Session
	SessionPath string
	Telemetry   *Recorder
	Plugins     PluginManager
}

func NewHarness(model Model, workspace, sessionPath string) (*Harness, error) {
	if model == nil {
		return nil, errors.New("model is required")
	}
	registry := NewRegistry()
	policy := PermissionPolicy{Workspace: workspace}
	for _, tool := range []Tool{
		ReadTool{Policy: policy},
		WriteTool{Policy: policy},
		EditTool{Policy: policy},
		GrepTool{Policy: policy},
		ShellTool{Policy: policy},
	} {
		if err := registry.Register(tool); err != nil {
			return nil, err
		}
	}
	recorder := &Recorder{}
	options := LoopOptions{
		MaxSteps: 12,
		ToolMode: Parallel,
		Retry:    RetryPolicy{MaxAttempts: 2},
		OnEvent: func(event Event) {
			fields := map[string]string{"step": strconv.Itoa(event.Step)}
			finish := recorder.Start("agent."+event.Type, fields)
			finish(event.Error)
		},
	}
	return &Harness{
		Agent:       NewAgent(model, registry, options),
		Session:     NewSession(),
		SessionPath: sessionPath,
		Telemetry:   recorder,
	}, nil
}

func LoadHarness(model Model, workspace, sessionPath string) (*Harness, error) {
	harness, err := NewHarness(model, workspace, sessionPath)
	if err != nil {
		return nil, err
	}
	if sessionPath != "" {
		session, loadErr := LoadSession(sessionPath)
		if loadErr != nil {
			return nil, loadErr
		}
		harness.Session = session
	}
	return harness, nil
}

func (h *Harness) Prompt(ctx context.Context, text string) (RunResult, error) {
	if h.Session == nil || h.Agent == nil {
		return RunResult{}, errors.New("harness is not initialized")
	}
	history := h.Session.Snapshot()
	history = append(history, Message{Role: RoleUser, Content: text})
	result, err := h.Agent.Run(ctx, history)
	// Persist the successful prefix even when a model request fails. This is
	// the important difference from saving only after a final answer.
	h.Session.Replace(result.Messages)
	if h.SessionPath != "" {
		if saveErr := h.Session.Save(h.SessionPath); saveErr != nil && err == nil {
			return result, saveErr
		}
	}
	return result, err
}

func (h *Harness) Compact(maxTokens int) error {
	compacted, err := Compact(h.Session.Snapshot(), maxTokens)
	if err != nil {
		return err
	}
	h.Session.Replace(compacted)
	if h.SessionPath != "" {
		return h.Session.Save(h.SessionPath)
	}
	return nil
}

// CalculatorTool is used by the offline demo and tests.
type CalculatorTool struct{}

func (CalculatorTool) Definition() ToolDefinition {
	return ToolDefinition{Name: "calculator", Description: "Add, subtract, multiply, or divide two numbers.", Schema: objectSchema(map[string]any{
		"operation": map[string]any{"type": "string"},
		"left":      map[string]any{"type": "number"},
		"right":     map[string]any{"type": "number"},
	}, "operation", "left", "right")}
}

func (CalculatorTool) Execute(_ context.Context, raw json.RawMessage) (ToolResult, error) {
	var input struct {
		Operation string  `json:"operation"`
		Left      float64 `json:"left"`
		Right     float64 `json:"right"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return ToolResult{}, err
	}
	var value float64
	switch strings.ToLower(input.Operation) {
	case "add":
		value = input.Left + input.Right
	case "subtract":
		value = input.Left - input.Right
	case "multiply":
		value = input.Left * input.Right
	case "divide":
		if input.Right == 0 {
			return ToolResult{}, errors.New("division by zero")
		}
		value = input.Left / input.Right
	default:
		return ToolResult{}, fmt.Errorf("unknown operation %q", input.Operation)
	}
	return ToolResult{Content: strconv.FormatFloat(value, 'f', -1, 64), Details: map[string]any{"value": value}}, nil
}
