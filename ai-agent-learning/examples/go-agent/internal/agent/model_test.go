package agent_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	agent "example.com/ai-agent-learning/go-agent/internal/agent"
)

func TestHTTPModelCompleteAndStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("authorization = %q", got)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		writer.Header().Set("Content-Type", "application/json")
		if strings.Contains(string(body), `"stream":true`) {
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n"))
			_, _ = writer.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}]}\n\n"))
			_, _ = writer.Write([]byte("data: [DONE]\n\n"))
			return
		}
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`))
	}))
	defer server.Close()

	model := &agent.HTTPModel{Client: server.Client(), URL: server.URL, APIKey: "test-key", ModelID: "test-model"}
	request := agent.Request{Messages: []agent.Message{{Role: agent.RoleUser, Content: "hi"}}}
	response, err := model.Complete(context.Background(), request)
	if err != nil || response.Message.Content != "hello" || response.Usage.TotalTokens != 3 {
		t.Fatalf("complete = %#v, err = %v", response, err)
	}

	var deltas strings.Builder
	streamed, err := model.Stream(context.Background(), request, func(delta string) {
		_, _ = deltas.WriteString(delta)
	})
	if err != nil || deltas.String() != "hello" || streamed.StopReason != "stop" {
		t.Fatalf("streamed = %#v, deltas = %q, err = %v", streamed, deltas.String(), err)
	}
}

func TestHTTPModelRejectsResponseWithoutChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	model := &agent.HTTPModel{Client: server.Client(), URL: server.URL, ModelID: "test-model"}
	_, err := model.Complete(context.Background(), agent.Request{})
	if err == nil || !strings.Contains(err.Error(), "no choices") {
		t.Fatalf("expected no-choices error, got %v", err)
	}
}
