package agent

import (
	"context"
	"errors"
	"sync"
)

// Agent adds lifecycle and message queues around AgentLoop. Queue messages are
// not inserted into a request that is already in flight; they are drained at a
// turn boundary.
type Agent struct {
	Loop AgentLoop

	mu       sync.Mutex
	steering []Message
	followUp []Message
	nextRun  []Message
	running  bool
	cancel   context.CancelFunc
}

func NewAgent(model Model, registry *Registry, options LoopOptions) *Agent {
	return &Agent{Loop: AgentLoop{Model: model, Tools: registry, Options: options}}
}

func (a *Agent) Prompt(ctx context.Context, text string) (RunResult, error) {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return RunResult{}, errors.New("agent is already running; use Steer or FollowUp")
	}
	initial := append([]Message(nil), a.nextRun...)
	a.nextRun = nil
	initial = append(initial, Message{Role: RoleUser, Content: text})
	runContext, cancel := a.beginRunLocked(ctx)
	a.mu.Unlock()
	return a.execute(runContext, cancel, initial)
}

// Run accepts an already assembled history. Harness uses this to restore a
// Session before adding the current user message.
func (a *Agent) Run(ctx context.Context, initial []Message) (RunResult, error) {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return RunResult{}, errors.New("agent is already running; use Steer or FollowUp")
	}
	runContext, cancel := a.beginRunLocked(ctx)
	a.mu.Unlock()
	return a.execute(runContext, cancel, initial)
}

func (a *Agent) beginRunLocked(ctx context.Context) (context.Context, context.CancelFunc) {
	runContext, cancel := context.WithCancel(ctx)
	a.running = true
	a.cancel = cancel
	return runContext, cancel
}

func (a *Agent) execute(runContext context.Context, cancel context.CancelFunc, initial []Message) (RunResult, error) {
	defer func() {
		cancel()
		a.mu.Lock()
		a.running = false
		a.cancel = nil
		a.mu.Unlock()
	}()

	options := a.Loop.Options
	options.Steering = a.drainSteering
	options.FollowUps = a.drainFollowUps
	loop := a.Loop
	loop.Options = options
	return loop.Run(runContext, initial)
}

func (a *Agent) Steer(text string) error {
	return a.enqueue(&a.steering, Message{Role: RoleUser, Content: text}, true)
}

func (a *Agent) FollowUp(text string) error {
	return a.enqueue(&a.followUp, Message{Role: RoleUser, Content: text}, true)
}

func (a *Agent) NextRun(text string) error {
	return a.enqueue(&a.nextRun, Message{Role: RoleUser, Content: text}, false)
}

func (a *Agent) Abort() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.running || a.cancel == nil {
		return errors.New("no active run")
	}
	// Steering/follow-up express intent for this run and are dropped on abort;
	// nextRun is deliberately preserved.
	a.steering = nil
	a.followUp = nil
	a.cancel()
	return nil
}

func (a *Agent) Running() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

func (a *Agent) enqueue(queue *[]Message, message Message, activeOnly bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if activeOnly && !a.running {
		return errors.New("queue requires an active run")
	}
	*queue = append(*queue, message)
	return nil
}

func (a *Agent) drainSteering() []Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	messages := append([]Message(nil), a.steering...)
	a.steering = nil
	return messages
}

func (a *Agent) drainFollowUps() []Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	messages := append([]Message(nil), a.followUp...)
	a.followUp = nil
	return messages
}
