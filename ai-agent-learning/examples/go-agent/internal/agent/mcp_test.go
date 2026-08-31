package agent_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"

	agent "example.com/ai-agent-learning/go-agent/internal/agent"
)

func TestMCPClientListsAndCallsRemoteTool(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		defer serverConn.Close()
		scanner := bufio.NewScanner(serverConn)
		for scanner.Scan() {
			var request struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      int             `json:"id"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params"`
			}
			if json.Unmarshal(scanner.Bytes(), &request) != nil {
				return
			}
			var result string
			switch request.Method {
			case "tools/list":
				result = `{"tools":[{"name":"remote_add","description":"add remotely","inputSchema":{"type":"object","required":["value"]}}]}`
			case "tools/call":
				result = `{"content":[{"type":"text","text":"remote result"}],"isError":false}`
			default:
				result = `null`
			}
			response := fmt.Sprintf("{\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":%s}\n", request.ID, result)
			if _, err := serverConn.Write([]byte(response)); err != nil {
				return
			}
		}
	}()

	client := agent.NewMCPClient(clientConn, clientConn, clientConn.Close)
	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Definition().Name != "remote_add" {
		t.Fatalf("unexpected tools: %#v", tools)
	}
	result, err := tools[0].Execute(context.Background(), json.RawMessage(`{"value":1}`))
	if err != nil || !strings.Contains(result.Content, "remote result") {
		t.Fatalf("remote call = %#v, err = %v", result, err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	<-serverDone
}
