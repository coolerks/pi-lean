package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	agent "example.com/ai-agent-learning/go-agent/internal/agent"
)

func main() {
	demo := flag.Bool("demo", false, "run the offline scripted calculator demo")
	prompt := flag.String("prompt", "请计算 2 加 3", "user prompt for HTTP mode")
	baseURL := flag.String("url", os.Getenv("AGENT_MODEL_URL"), "OpenAI-compatible /chat/completions URL")
	modelID := flag.String("model", os.Getenv("AGENT_MODEL"), "model id for HTTP mode")
	flag.Parse()

	workspace, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	sessionPath := filepath.Join(workspace, ".agent-session.jsonl")
	var model agent.Model
	if *demo {
		model = &agent.ScriptedModel{Responses: []agent.Response{
			{Message: agent.Message{Role: agent.RoleAssistant, ToolCalls: []agent.ToolCall{
				agent.NewToolCall("call-1", "calculator", map[string]any{"operation": "add", "left": 2, "right": 3}),
			}}, StopReason: "tool_use"},
			{Message: agent.Message{Role: agent.RoleAssistant, Content: "2 + 3 = 5"}, StopReason: "stop"},
		}}
	} else {
		if *baseURL == "" || *modelID == "" || os.Getenv("AGENT_API_KEY") == "" {
			fmt.Fprintln(os.Stderr, "HTTP mode requires -url, -model, and AGENT_API_KEY; use --demo for an offline run")
			os.Exit(2)
		}
		model = &agent.HTTPModel{URL: *baseURL, ModelID: *modelID, APIKey: os.Getenv("AGENT_API_KEY")}
	}

	harness, err := agent.NewHarness(model, workspace, sessionPath)
	if err != nil {
		fatal(err)
	}
	if *demo {
		if err := harness.Agent.Loop.Tools.Register(agent.CalculatorTool{}); err != nil {
			fatal(err)
		}
	}
	harness.Agent.Loop.Options.OnEvent = func(event agent.Event) {
		if event.Type == "assistant" && event.Message != nil && event.Message.Content != "" {
			fmt.Printf("assistant> %s\n", event.Message.Content)
		}
		if event.Type == "tool_result" && event.ToolResult != nil {
			fmt.Printf("tool[%s]> %s\n", event.ToolCall.Name, event.ToolResult.Content)
		}
	}
	result, err := harness.Prompt(context.Background(), *prompt)
	if err != nil {
		fatal(err)
	}
	if result.Final.Content != "" {
		fmt.Printf("final> %s\n", result.Final.Content)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
