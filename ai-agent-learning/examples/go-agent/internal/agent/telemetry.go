package agent

import (
	"sync"
	"time"
)

type Span struct {
	Name   string
	Start  time.Time
	End    time.Time
	Error  string
	Fields map[string]string
}

// Recorder stores bounded metadata rather than prompt or file contents.
type Recorder struct {
	mu    sync.Mutex
	Spans []Span
}

func (r *Recorder) Start(name string, fields map[string]string) func(error) {
	start := time.Now()
	return func(err error) {
		span := Span{Name: name, Start: start, End: time.Now(), Fields: fields}
		if err != nil {
			span.Error = err.Error()
		}
		r.mu.Lock()
		r.Spans = append(r.Spans, span)
		r.mu.Unlock()
	}
}

func (r *Recorder) Snapshot() []Span {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Span(nil), r.Spans...)
}
