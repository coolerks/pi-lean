package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// ToolDefinition is the part of a Tool exposed to the model.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"parameters"`
}

// ToolResult is converted to a RoleTool message after execution.
type ToolResult struct {
	Content   string
	Details   map[string]any
	IsError   bool
	Terminate bool
}

// Tool is the runtime side of a ToolDefinition.
type Tool interface {
	Definition() ToolDefinition
	Execute(context.Context, json.RawMessage) (ToolResult, error)
}

// BeforeToolHook can block a call after JSON/schema validation but before its
// side effect. Returning an error is treated as a blocked call by the loop.
type BeforeToolHook func(context.Context, ToolCall, ToolDefinition) error

// AfterToolHook can redact or annotate a completed result.
type AfterToolHook func(context.Context, ToolCall, ToolResult) (ToolResult, error)

// Registry owns the executable Tool set and performs the first validation gate.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(tool Tool) error {
	definition := tool.Definition()
	if definition.Name == "" {
		return errors.New("tool name cannot be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[definition.Name]; exists {
		return fmt.Errorf("tool %q is already registered", definition.Name)
	}
	r.tools[definition.Name] = tool
	return nil
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

func (r *Registry) Definitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	definitions := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		definitions = append(definitions, tool.Definition())
	}
	return definitions
}

func (r *Registry) Validate(call ToolCall) (Tool, error) {
	tool, ok := r.Get(call.Name)
	if !ok {
		return nil, fmt.Errorf("unknown tool %q", call.Name)
	}
	definition := tool.Definition()
	var object map[string]any
	if err := json.Unmarshal(call.Arguments, &object); err != nil {
		return nil, fmt.Errorf("tool %q arguments are invalid JSON: %w", call.Name, err)
	}
	if object == nil {
		return nil, fmt.Errorf("tool %q arguments must be a JSON object", call.Name)
	}
	if required, ok := definition.Schema["required"].([]any); ok {
		for _, item := range required {
			name, ok := item.(string)
			if ok {
				if _, present := object[name]; !present {
					return nil, fmt.Errorf("tool %q is missing required argument %q", call.Name, name)
				}
			}
		}
	}
	return tool, nil
}
