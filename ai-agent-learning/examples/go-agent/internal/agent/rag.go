package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// KnowledgeDocument is intentionally tiny: retrieval is term overlap, not an
// embedding database. It makes the RAG/Tool boundary executable offline.
type KnowledgeDocument struct {
	Source string
	Tenant string
	Text   string
}

type KnowledgeTool struct {
	Documents []KnowledgeDocument
	Tenant    string
	MaxTopK   int
}

func (t KnowledgeTool) Definition() ToolDefinition {
	return ToolDefinition{Name: "search_knowledge", Description: "Search tenant-scoped knowledge and return cited snippets.", Schema: objectSchema(map[string]any{
		"query": map[string]any{"type": "string"},
		"topK":  map[string]any{"type": "integer", "minimum": 1, "maximum": 8},
	}, "query")}
}

func (t KnowledgeTool) Execute(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
	var input struct {
		Query string `json:"query"`
		TopK  int    `json:"topK"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return ToolResult{}, err
	}
	query := strings.TrimSpace(input.Query)
	if query == "" || len(query) > 1000 {
		return ToolResult{}, fmt.Errorf("query must contain 1-1000 characters")
	}
	topK := input.TopK
	if topK == 0 {
		topK = 3
	}
	maxTopK := t.MaxTopK
	if maxTopK <= 0 || maxTopK > 8 {
		maxTopK = 8
	}
	if topK < 1 || topK > maxTopK {
		return ToolResult{}, fmt.Errorf("topK must be between 1 and %d", maxTopK)
	}
	terms := tokenize(query)
	type hit struct {
		Source string  `json:"source"`
		Score  float64 `json:"score"`
		Text   string  `json:"snippet"`
	}
	hits := make([]hit, 0)
	for _, document := range t.Documents {
		if err := ctx.Err(); err != nil {
			return ToolResult{}, err
		}
		if t.Tenant != "" && document.Tenant != t.Tenant {
			continue
		}
		words := tokenize(document.Text)
		if len(words) == 0 {
			continue
		}
		matches := 0
		for _, term := range terms {
			for _, word := range words {
				if term == word {
					matches++
					break
				}
			}
		}
		if matches > 0 {
			hits = append(hits, hit{Source: document.Source, Score: float64(matches) / float64(len(terms)), Text: document.Text})
		}
	}
	sort.SliceStable(hits, func(left, right int) bool { return hits[left].Score > hits[right].Score })
	if len(hits) > topK {
		hits = hits[:topK]
	}
	encoded, err := json.Marshal(hits)
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{Content: string(encoded), Details: map[string]any{"hits": len(hits), "tenant": t.Tenant}}, nil
}

func tokenize(text string) []string {
	fields := strings.Fields(strings.ToLower(text))
	terms := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.Trim(field, ".,!?;:()[]{}\"'")
		if field != "" {
			terms = append(terms, field)
		}
	}
	return terms
}
