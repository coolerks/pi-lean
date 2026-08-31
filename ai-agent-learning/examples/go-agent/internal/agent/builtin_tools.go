package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func objectSchema(properties map[string]any, required ...string) map[string]any {
	requiredValues := make([]any, len(required))
	for index, name := range required {
		requiredValues[index] = name
	}
	return map[string]any{"type": "object", "properties": properties, "required": requiredValues}
}

type ReadTool struct {
	Policy   PermissionPolicy
	MaxBytes int
}

func (t ReadTool) Definition() ToolDefinition {
	return ToolDefinition{Name: "read", Description: "Read a UTF-8 text file in the workspace.", Schema: objectSchema(map[string]any{
		"path": map[string]any{"type": "string"},
	}, "path")}
}

func (t ReadTool) Execute(_ context.Context, raw json.RawMessage) (ToolResult, error) {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return ToolResult{}, err
	}
	path, err := t.Policy.ResolvePath(input.Path)
	if err != nil {
		return ToolResult{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return ToolResult{}, err
	}
	maxBytes := t.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 32 << 10
	}
	result := string(content)
	truncated := false
	if len(content) > maxBytes {
		result = string(content[:maxBytes])
		truncated = true
	}
	if truncated {
		result += fmt.Sprintf("\n[output truncated at %d bytes]", maxBytes)
	}
	return ToolResult{Content: result, Details: map[string]any{"path": path, "truncated": truncated}}, nil
}

type WriteTool struct{ Policy PermissionPolicy }

func (t WriteTool) Definition() ToolDefinition {
	return ToolDefinition{Name: "write", Description: "Write UTF-8 text to a workspace file.", Schema: objectSchema(map[string]any{
		"path":    map[string]any{"type": "string"},
		"content": map[string]any{"type": "string"},
	}, "path", "content")}
}

func (t WriteTool) Execute(_ context.Context, raw json.RawMessage) (ToolResult, error) {
	var input struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return ToolResult{}, err
	}
	path, err := t.Policy.ResolvePath(input.Path)
	if err != nil {
		return ToolResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ToolResult{}, err
	}
	if err := os.WriteFile(path, []byte(input.Content), 0o644); err != nil {
		return ToolResult{}, err
	}
	return ToolResult{Content: fmt.Sprintf("wrote %d bytes to %s", len(input.Content), input.Path), Details: map[string]any{"path": path}}, nil
}

type EditTool struct{ Policy PermissionPolicy }

func (t EditTool) Definition() ToolDefinition {
	return ToolDefinition{Name: "edit", Description: "Replace one unique occurrence in a workspace file.", Schema: objectSchema(map[string]any{
		"path":    map[string]any{"type": "string"},
		"oldText": map[string]any{"type": "string"},
		"newText": map[string]any{"type": "string"},
	}, "path", "oldText", "newText")}
}

func (t EditTool) Execute(_ context.Context, raw json.RawMessage) (ToolResult, error) {
	var input struct {
		Path    string `json:"path"`
		OldText string `json:"oldText"`
		NewText string `json:"newText"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return ToolResult{}, err
	}
	path, err := t.Policy.ResolvePath(input.Path)
	if err != nil {
		return ToolResult{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return ToolResult{}, err
	}
	count := strings.Count(string(content), input.OldText)
	if count != 1 {
		return ToolResult{}, fmt.Errorf("oldText must occur exactly once, found %d", count)
	}
	updated := strings.Replace(string(content), input.OldText, input.NewText, 1)
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return ToolResult{}, err
	}
	return ToolResult{Content: fmt.Sprintf("edited %s", input.Path), Details: map[string]any{"path": path}}, nil
}

type GrepTool struct {
	Policy   PermissionPolicy
	MaxHits  int
	MaxBytes int
}

func (t GrepTool) Definition() ToolDefinition {
	return ToolDefinition{Name: "grep", Description: "Search text files under a workspace path.", Schema: objectSchema(map[string]any{
		"query": map[string]any{"type": "string"},
		"path":  map[string]any{"type": "string"},
	}, "query")}
}

func (t GrepTool) Execute(_ context.Context, raw json.RawMessage) (ToolResult, error) {
	var input struct {
		Query string `json:"query"`
		Path  string `json:"path"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return ToolResult{}, err
	}
	if input.Query == "" {
		return ToolResult{}, fmt.Errorf("query cannot be empty")
	}
	root, err := t.Policy.ResolvePath(input.Path)
	if err != nil {
		return ToolResult{}, err
	}
	maxHits := t.MaxHits
	if maxHits <= 0 {
		maxHits = 50
	}
	var lines []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || len(lines) >= maxHits {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil || bytes.IndexByte(data, 0) >= 0 {
			return nil
		}
		for index, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, input.Query) {
				lines = append(lines, fmt.Sprintf("%s:%d:%s", path, index+1, line))
				if len(lines) >= maxHits {
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{Content: strings.Join(lines, "\n"), Details: map[string]any{"hits": len(lines)}}, nil
}

type ShellTool struct {
	Policy   PermissionPolicy
	MaxBytes int
}

func (t ShellTool) Definition() ToolDefinition {
	return ToolDefinition{Name: "shell", Description: "Run a command in the workspace after permission checks.", Schema: objectSchema(map[string]any{
		"command": map[string]any{"type": "string"},
	}, "command")}
}

func (t ShellTool) Execute(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
	var input struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return ToolResult{}, err
	}
	if err := t.Policy.CheckCommand(input.Command); err != nil {
		return ToolResult{}, err
	}
	command := exec.CommandContext(ctx, "sh", "-c", input.Command)
	command.Dir = t.Policy.Workspace
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	maxBytes := t.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 32 << 10
	}
	content := output.String()
	if len(content) > maxBytes {
		content = content[:maxBytes] + fmt.Sprintf("\n[output truncated at %d bytes]", maxBytes)
	}
	if err != nil {
		return ToolResult{Content: content, IsError: true, Details: map[string]any{"exit": command.ProcessState.ExitCode()}}, err
	}
	return ToolResult{Content: content, Details: map[string]any{"exit": 0}}, nil
}
