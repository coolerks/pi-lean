package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPClient demonstrates the protocol boundary over newline-delimited stdio.
// It intentionally does not attempt to be a complete MCP SDK.
type MCPClient struct {
	mu     sync.Mutex
	read   *bufio.Reader
	write  io.WriteCloser
	nextID int
	close  func() error
}

func NewMCPClient(reader io.Reader, writer io.WriteCloser, close func() error) *MCPClient {
	return &MCPClient{read: bufio.NewReader(reader), write: writer, close: close, nextID: 1}
}

func (c *MCPClient) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.read == nil || c.write == nil {
		return nil, errors.New("MCP client is closed")
	}
	id := c.nextID
	c.nextID++
	encodedParams, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	request := rpcMessage{JSONRPC: "2.0", ID: id, Method: method, Params: encodedParams}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	if _, err := c.write.Write(append(payload, '\n')); err != nil {
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line, err := c.read.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		var response rpcMessage
		if err := json.Unmarshal(line, &response); err != nil {
			return nil, fmt.Errorf("decode MCP response: %w", err)
		}
		if response.ID != id {
			// Notifications and another request's response can be skipped in this
			// serial educational client. A multiplexed client needs a dispatcher.
			continue
		}
		if response.Error != nil {
			return nil, fmt.Errorf("MCP error %d: %s", response.Error.Code, response.Error.Message)
		}
		return response.Result, nil
	}
}

type RemoteTool struct {
	client      *MCPClient
	name        string
	description string
	schema      map[string]any
}

func (t RemoteTool) Definition() ToolDefinition {
	return ToolDefinition{Name: t.name, Description: t.description, Schema: t.schema}
}

func (t RemoteTool) Execute(ctx context.Context, arguments json.RawMessage) (ToolResult, error) {
	var args map[string]any
	if err := json.Unmarshal(arguments, &args); err != nil {
		return ToolResult{}, err
	}
	result, err := t.client.Call(ctx, "tools/call", map[string]any{"name": t.name, "arguments": args})
	if err != nil {
		return ToolResult{}, err
	}
	var decoded struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &decoded); err != nil {
		return ToolResult{}, err
	}
	text := ""
	for _, item := range decoded.Content {
		if item.Type == "text" {
			text += item.Text
		}
	}
	return ToolResult{Content: text, IsError: decoded.IsError}, nil
}

func (c *MCPClient) ListTools(ctx context.Context) ([]Tool, error) {
	result, err := c.Call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var decoded struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &decoded); err != nil {
		return nil, err
	}
	tools := make([]Tool, 0, len(decoded.Tools))
	for _, item := range decoded.Tools {
		if item.Name == "" {
			continue
		}
		tools = append(tools, RemoteTool{client: c, name: item.Name, description: item.Description, schema: item.InputSchema})
	}
	return tools, nil
}

func (c *MCPClient) Close() error {
	c.mu.Lock()
	closeFn := c.close
	c.close = nil
	c.read = nil
	c.write = nil
	c.mu.Unlock()
	if closeFn != nil {
		return closeFn()
	}
	return nil
}

// StartStdioMCP starts a server whose stdout contains only JSON-RPC messages.
func StartStdioMCP(command string, args ...string) (*MCPClient, error) {
	process := exec.Command(command, args...)
	stdin, err := process.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := process.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := process.Start(); err != nil {
		return nil, err
	}
	return NewMCPClient(stdout, stdin, func() error {
		_ = stdin.Close()
		_ = stdout.Close()
		return process.Wait()
	}), nil
}
