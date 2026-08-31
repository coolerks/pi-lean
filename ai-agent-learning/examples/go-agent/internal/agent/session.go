package agent

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Session is an append-oriented conversation store. The example uses JSONL so
// a line is independently inspectable; production stores need stronger
// transaction and corruption recovery rules.
type Session struct {
	mu       sync.RWMutex
	Messages []Message
}

func NewSession() *Session { return &Session{} }

func (s *Session) Append(messages ...Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = append(s.Messages, messages...)
}

func (s *Session) Snapshot() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Message(nil), s.Messages...)
}

func (s *Session) Replace(messages []Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = append([]Message(nil), messages...)
}

func (s *Session) Save(path string) error {
	if path == "" {
		return errors.New("session path cannot be empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".session-*.tmp")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer func() {
		temporary.Close()
		os.Remove(temporaryName)
	}()
	encoder := json.NewEncoder(temporary)
	for _, message := range s.Snapshot() {
		if err := encoder.Encode(message); err != nil {
			return err
		}
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}

func LoadSession(path string) (*Session, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewSession(), nil
		}
		return nil, err
	}
	defer file.Close()
	session := NewSession()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 4<<20)
	line := 0
	for scanner.Scan() {
		line++
		var message Message
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			return nil, fmt.Errorf("decode session line %d: %w", line, err)
		}
		if message.Role == "" {
			return nil, fmt.Errorf("session line %d has no role", line)
		}
		session.Append(message)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return session, nil
}

// EstimateTokens is deliberately approximate. It is a control signal for the
// lesson, not a tokenizer or a billing calculation.
func EstimateTokens(messages []Message) int {
	characters := 0
	for _, message := range messages {
		characters += len(message.Content)
		for _, call := range message.ToolCalls {
			characters += len(call.Name) + len(call.Arguments)
		}
	}
	return (characters + 3) / 4
}

// Compact keeps the first system message and a recent tail, replacing the
// omitted section with a deterministic summary. A production Harness would
// call a model for the summary and persist the compaction entry atomically.
func Compact(messages []Message, maxTokens int) ([]Message, error) {
	if maxTokens <= 0 {
		return nil, errors.New("maxTokens must be positive")
	}
	if EstimateTokens(messages) <= maxTokens {
		return append([]Message(nil), messages...), nil
	}
	start := 0
	var prefix []Message
	if len(messages) > 0 && messages[0].Role == RoleSystem {
		prefix = append(prefix, messages[0])
		start = 1
	}
	keep := make([]Message, 0, len(messages))
	for index := len(messages) - 1; index >= start; index-- {
		candidate := append([]Message(nil), messages[index])
		candidate = append(candidate, keep...)
		if EstimateTokens(append(append([]Message(nil), prefix...), candidate...)) > maxTokens/2 && len(keep) > 0 {
			break
		}
		keep = candidate
	}
	dropped := len(messages) - start - len(keep)
	if dropped < 0 {
		dropped = 0
	}
	summary := Message{Role: RoleAssistant, Content: fmt.Sprintf("[compaction summary: %d earlier messages omitted; retain task decisions and verify files before acting]", dropped)}
	result := append(prefix, summary)
	result = append(result, keep...)
	return result, nil
}
