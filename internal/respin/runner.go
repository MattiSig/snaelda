package respin

import (
	"context"
	"log/slog"
	"sync"

	"github.com/MattiSig/snaelda/internal/generation"
)

// EventType tags a progress event streamed to a demo watcher.
type EventType string

const (
	EventStatus   EventType = "status"
	EventProgress EventType = "progress"
	EventComplete EventType = "complete"
	EventFailed   EventType = "failed"
)

// Event is a single progress event in an import's live stream. Late subscribers
// receive the buffered history first, so an SSE client that connects after the
// POST that started the run still sees every event (Spec 21 demo UI).
type Event struct {
	Type   EventType                `json:"type"`
	Status string                   `json:"status,omitempty"`
	Step   *generation.ProgressStep `json:"step,omitempty"`
	Result *RunResult               `json:"result,omitempty"`
	Error  string                   `json:"error,omitempty"`
}

// Runner owns a global concurrency cap on in-flight imports and an in-process
// progress hub so the decoupled start (POST) and watch (SSE GET) endpoints share
// one live run (Spec 21 resource caps). It is safe for concurrent use.
type Runner struct {
	sem    chan struct{}
	logger *slog.Logger

	mu   sync.Mutex
	runs map[string]*runState
}

// NewRunner builds a Runner bounded to maxConcurrent simultaneous imports.
func NewRunner(maxConcurrent int, logger *slog.Logger) *Runner {
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{
		sem:    make(chan struct{}, maxConcurrent),
		logger: logger,
		runs:   make(map[string]*runState),
	}
}

// Slot is a reserved concurrency slot. Acquire it before creating an import so an
// over-capacity request is rejected before any work (or a demo workspace) is
// created; hand it to Start, which releases it when the run finishes.
type Slot struct {
	runner   *Runner
	released bool
}

// TryAcquire reserves a concurrency slot without blocking. It returns (slot,
// true) when capacity is available and (nil, false) when the runner is at its
// in-flight cap — the endpoint maps the latter to a friendly busy response.
func (r *Runner) TryAcquire() (*Slot, bool) {
	select {
	case r.sem <- struct{}{}:
		return &Slot{runner: r}, true
	default:
		return nil, false
	}
}

func (s *Slot) release() {
	if s == nil || s.released {
		return
	}
	s.released = true
	<-s.runner.sem
}

// Release returns an unused slot to the pool (e.g. when import creation fails
// after acquisition). It is a no-op once the slot has been handed to Start.
func (s *Slot) Release() { s.release() }

type runState struct {
	mu     sync.Mutex
	events []Event
	subs   map[chan Event]struct{}
	done   bool
}

func (rs *runState) publish(ev Event) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.events = append(rs.events, ev)
	if ev.Type == EventComplete || ev.Type == EventFailed {
		rs.done = true
	}
	for ch := range rs.subs {
		select {
		case ch <- ev:
		default:
			// A slow subscriber must not stall the pipeline; it can still
			// recover the full history via the replay buffer on reconnect.
		}
	}
	if rs.done {
		for ch := range rs.subs {
			close(ch)
			delete(rs.subs, ch)
		}
	}
}

// RunFunc executes one import pipeline against the given sink. The public and
// session-bound flows supply different closures (budgeted vs unbudgeted
// analyzer) over the same runner, keeping the daily public LLM budget off the
// quota-accounted session-bound path (Spec 21).
type RunFunc func(ctx context.Context, sink ProgressSink) (RunResult, error)

// Start launches an already-created import in the background and returns
// immediately. It consumes the provided slot, releasing it when the run
// terminates. Progress is fanned out through the hub to any SSE subscriber.
func (r *Runner) Start(importID string, run RunFunc, slot *Slot) {
	rs := &runState{subs: make(map[chan Event]struct{})}
	r.mu.Lock()
	r.runs[importID] = rs
	r.mu.Unlock()

	sink := hubSink{state: rs}
	go func() {
		defer slot.release()
		// The run outlives the request that started it; use a background context
		// so a disconnecting watcher cannot cancel an in-flight generation.
		result, err := run(context.Background(), sink)
		if err != nil {
			rs.publish(Event{Type: EventFailed, Error: err.Error()})
		} else {
			res := result
			rs.publish(Event{Type: EventComplete, Result: &res})
		}
		// Retain the terminal history briefly for reconnecting clients, then drop
		// it so the map does not grow unbounded. The store remains the durable
		// source of truth for status after this point.
		r.mu.Lock()
		delete(r.runs, importID)
		r.mu.Unlock()
	}()
}

// Subscribe returns a channel replaying the run's buffered events followed by
// live ones until it terminates, plus whether an active run exists. When no run
// is tracked (cache hit, completed-and-evicted, or process restart) the caller
// falls back to the durable import status in the store.
func (r *Runner) Subscribe(importID string) (<-chan Event, bool) {
	r.mu.Lock()
	rs, ok := r.runs[importID]
	r.mu.Unlock()
	if !ok {
		return nil, false
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()
	ch := make(chan Event, len(rs.events)+16)
	for _, ev := range rs.events {
		ch <- ev
	}
	if rs.done {
		close(ch)
		return ch, true
	}
	rs.subs[ch] = struct{}{}
	return ch, true
}

// hubSink adapts pipeline progress into hub events.
type hubSink struct {
	state *runState
}

func (s hubSink) Status(status string) {
	s.state.publish(Event{Type: EventStatus, Status: status})
}

func (s hubSink) Step(step generation.ProgressStep) {
	st := step
	s.state.publish(Event{Type: EventProgress, Step: &st})
}
