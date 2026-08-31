package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	agent "example.com/ai-agent-learning/go-agent/internal/agent"
)

type countingTool struct{ calls atomic.Int32 }

func (t *countingTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{Name: "count", Description: "count calls", Schema: map[string]any{
		"type": "object", "properties": map[string]any{"value": map[string]any{"type": "string"}}, "required": []any{"value"},
	}}
}

func (t *countingTool) Execute(_ context.Context, _ json.RawMessage) (agent.ToolResult, error) {
	t.calls.Add(1)
	return agent.ToolResult{Content: "called"}, nil
}

func scriptedLoop(model agent.Model, registry *agent.Registry, options agent.LoopOptions) agent.AgentLoop {
	return agent.AgentLoop{Model: model, Tools: registry, Options: options}
}

func TestToolCallIsExecutedAndReturnedToModel(t *testing.T) {
	model := &agent.ScriptedModel{Responses: []agent.Response{
		{Message: agent.Message{Role: agent.RoleAssistant, ToolCalls: []agent.ToolCall{
			agent.NewToolCall("1", "calculator", map[string]any{"operation": "add", "left": 2, "right": 3}),
		}}},
		{Message: agent.Message{Role: agent.RoleAssistant, Content: "5"}},
	}}
	registry := agent.NewRegistry()
	if err := registry.Register(agent.CalculatorTool{}); err != nil {
		t.Fatal(err)
	}
	loop := scriptedLoop(model, registry, agent.LoopOptions{MaxSteps: 4, ToolMode: agent.Sequential})
	result, err := loop.Run(context.Background(), []agent.Message{{Role: agent.RoleUser, Content: "2+3"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Final.Content != "5" || len(model.Requests) != 2 {
		t.Fatalf("final=%q requests=%d", result.Final.Content, len(model.Requests))
	}
	request := model.Requests[1]
	if len(request.Messages) == 0 || request.Messages[len(request.Messages)-1].Role != agent.RoleTool {
		t.Fatalf("tool result was not appended: %#v", request.Messages)
	}
	if request.Messages[len(request.Messages)-1].Content != "5" {
		t.Fatalf("unexpected tool result: %#v", request.Messages[len(request.Messages)-1])
	}
}

func TestUnknownToolDoesNotExecuteAnything(t *testing.T) {
	model := &agent.ScriptedModel{Responses: []agent.Response{
		{Message: agent.Message{Role: agent.RoleAssistant, ToolCalls: []agent.ToolCall{{ID: "1", Name: "missing", Arguments: json.RawMessage(`{}`)}}}},
		{Message: agent.Message{Role: agent.RoleAssistant, Content: "handled"}},
	}}
	counter := &countingTool{}
	registry := agent.NewRegistry()
	if err := registry.Register(counter); err != nil {
		t.Fatal(err)
	}
	result, err := scriptedLoop(model, registry, agent.LoopOptions{MaxSteps: 3}).Run(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if counter.calls.Load() != 0 || result.Final.Content != "handled" {
		t.Fatalf("unknown tool executed or loop failed: calls=%d final=%q", counter.calls.Load(), result.Final.Content)
	}
}

func TestMultipleToolCallsPreserveSourceOrder(t *testing.T) {
	model := &agent.ScriptedModel{Responses: []agent.Response{
		{Message: agent.Message{Role: agent.RoleAssistant, ToolCalls: []agent.ToolCall{
			agent.NewToolCall("a", "calculator", map[string]any{"operation": "add", "left": 1, "right": 2}),
			agent.NewToolCall("b", "calculator", map[string]any{"operation": "multiply", "left": 3, "right": 4}),
		}}},
		{Message: agent.Message{Role: agent.RoleAssistant, Content: "done"}},
	}}
	registry := agent.NewRegistry()
	if err := registry.Register(agent.CalculatorTool{}); err != nil {
		t.Fatal(err)
	}
	result, err := scriptedLoop(model, registry, agent.LoopOptions{MaxSteps: 3, ToolMode: agent.Parallel}).Run(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Messages) < 4 || result.Messages[1].Content != "3" || result.Messages[2].Content != "12" {
		t.Fatalf("tool results were not source ordered: %#v", result.Messages)
	}
}

func TestInvalidArgumentsDoNotInvokeTool(t *testing.T) {
	counter := &countingTool{}
	model := &agent.ScriptedModel{Responses: []agent.Response{
		{Message: agent.Message{Role: agent.RoleAssistant, ToolCalls: []agent.ToolCall{{ID: "1", Name: "count", Arguments: json.RawMessage(`{}`)}}}},
		{Message: agent.Message{Role: agent.RoleAssistant, Content: "handled"}},
	}}
	registry := agent.NewRegistry()
	if err := registry.Register(counter); err != nil {
		t.Fatal(err)
	}
	result, err := scriptedLoop(model, registry, agent.LoopOptions{MaxSteps: 2}).Run(context.Background(), nil)
	if err != nil || counter.calls.Load() != 0 || !result.Messages[1].IsError {
		t.Fatalf("invalid call executed or was not returned as error: err=%v calls=%d messages=%#v", err, counter.calls.Load(), result.Messages)
	}
}

func TestRetryAndMaxSteps(t *testing.T) {
	model := &agent.ScriptedModel{
		Errors:    []error{errors.New("temporary 503")},
		Responses: []agent.Response{{Message: agent.Message{Role: agent.RoleAssistant, Content: "ok"}}},
	}
	loop := scriptedLoop(model, agent.NewRegistry(), agent.LoopOptions{MaxSteps: 1, Retry: agent.RetryPolicy{MaxAttempts: 2, BaseDelay: time.Millisecond}})
	result, err := loop.Run(context.Background(), []agent.Message{{Role: agent.RoleUser, Content: "hello"}})
	if err != nil || result.Final.Content != "ok" || len(model.Requests) != 2 {
		t.Fatalf("retry failed: err=%v final=%q requests=%d", err, result.Final.Content, len(model.Requests))
	}

	loop = scriptedLoop(&agent.ScriptedModel{Responses: []agent.Response{{Message: agent.Message{Role: agent.RoleAssistant, ToolCalls: []agent.ToolCall{{ID: "1", Name: "missing", Arguments: json.RawMessage(`{}`)}}}}}}, agent.NewRegistry(), agent.LoopOptions{MaxSteps: 1})
	if _, err := loop.Run(context.Background(), nil); err == nil {
		t.Fatal("expected max-steps error")
	}
}

func TestAbortStopsInFlightModel(t *testing.T) {
	model := &agent.ScriptedModel{Delay: time.Second, Responses: []agent.Response{{Message: agent.Message{Role: agent.RoleAssistant, Content: "late"}}}}
	a := agent.NewAgent(model, agent.NewRegistry(), agent.LoopOptions{MaxSteps: 1})
	finished := make(chan error, 1)
	go func() {
		_, err := a.Prompt(context.Background(), "wait")
		finished <- err
	}()
	time.Sleep(20 * time.Millisecond)
	if err := a.Abort(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("abort did not settle")
	}
}

func TestSessionPersistenceAndCompaction(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "session.jsonl")
	session := agent.NewSession()
	session.Append(agent.Message{Role: agent.RoleUser, Content: "one"}, agent.Message{Role: agent.RoleAssistant, Content: "two"})
	if err := session.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := agent.LoadSession(path)
	if err != nil || len(loaded.Snapshot()) != 2 {
		t.Fatalf("load failed: err=%v messages=%d", err, len(loaded.Snapshot()))
	}
	compacted, err := agent.Compact([]agent.Message{{Role: agent.RoleUser, Content: "a very long message"}, {Role: agent.RoleAssistant, Content: "another long response"}}, 3)
	if err != nil || len(compacted) == 0 {
		t.Fatalf("compact failed: err=%v result=%#v", err, compacted)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestKnowledgeToolFiltersTenantAndReturnsCitations(t *testing.T) {
	tool := agent.KnowledgeTool{Tenant: "team-a", Documents: []agent.KnowledgeDocument{
		{Source: "a.md", Tenant: "team-a", Text: "retry policy for model requests"},
		{Source: "secret.md", Tenant: "team-b", Text: "retry policy for another tenant"},
	}}
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"query":"retry policy","topK":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "a.md") || strings.Contains(result.Content, "secret.md") {
		t.Fatalf("unexpected tenant-filtered result: %s", result.Content)
	}
}

func TestHarnessPersistsAndReloads(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "harness.jsonl")
	model := &agent.ScriptedModel{Responses: []agent.Response{{Message: agent.Message{Role: agent.RoleAssistant, Content: "ok"}}}}
	harness, err := agent.NewHarness(model, directory, path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := harness.Prompt(context.Background(), "hello")
	if err != nil || result.Final.Content != "ok" {
		t.Fatalf("harness prompt failed: err=%v final=%q", err, result.Final.Content)
	}
	loaded, err := agent.LoadHarness(&agent.ScriptedModel{}, directory, path)
	if err != nil {
		t.Fatal(err)
	}
	if messages := loaded.Session.Snapshot(); len(messages) != 2 || messages[0].Role != agent.RoleUser || messages[1].Content != "ok" {
		t.Fatalf("unexpected reloaded session: %#v", messages)
	}
}

func TestSkillDiscoveryAndPluginRegistration(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: review\ndescription: review code\n---\n# Details\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	skills, err := agent.DiscoverSkills(root)
	if err != nil || len(skills) != 1 || skills[0].Name != "review" {
		t.Fatalf("skill discovery failed: err=%v skills=%#v", err, skills)
	}
	registry := agent.NewRegistry()
	manager := &agent.PluginManager{}
	plugin := testPlugin{}
	if err := manager.Load(plugin, registry); err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Get("count"); !ok {
		t.Fatal("plugin did not register tool")
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
}

type testPlugin struct{}

func (testPlugin) Name() string                            { return "test" }
func (testPlugin) Register(registry *agent.Registry) error { return registry.Register(&countingTool{}) }
func (testPlugin) Close() error                            { return nil }
