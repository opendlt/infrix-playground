// Copyright 2024 The Infrix Authors
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file or at
// https://opensource.org/licenses/MIT.

package api

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/opendlt/infrix-playground/worker"
)

// RunState is the lifecycle of a playground run.
type RunState string

const (
	StateCreated  RunState = "created"
	StateRunning  RunState = "running"
	StateComplete RunState = "complete"
	StateFailed   RunState = "failed"
)

// CanTransition reports whether a run may move from s to next. The state
// machine is deliberately tiny and forward-only: created → running →
// complete|failed. This is the contract the run-state-machine unit test pins.
func (s RunState) CanTransition(next RunState) bool {
	switch s {
	case StateCreated:
		return next == StateRunning
	case StateRunning:
		return next == StateComplete || next == StateFailed
	default:
		return false
	}
}

// Terminal reports whether the state is an end state.
func (s RunState) Terminal() bool { return s == StateComplete || s == StateFailed }

// Event is one item streamed over a run's SSE channel.
type Event struct {
	Type      string       `json:"type"` // "state" | "step" | "done" | "error"
	State     RunState     `json:"state,omitempty"`
	Step      *worker.Step `json:"step,omitempty"`
	ReceiptID string       `json:"receiptId,omitempty"`
	Error     string       `json:"error,omitempty"`
}

// Run is a single playground run. Its ID is ephemeral (process-local); the
// durable, shareable identifier is ReceiptID, assigned when the run completes.
type Run struct {
	ID   string
	Mode worker.Mode

	mu        sync.Mutex
	state     RunState
	steps     []worker.Step
	receiptID string
	errMsg    string
	subs      map[chan Event]struct{}
	history   []Event
}

// Snapshot is an immutable view of a run for the status endpoint.
type Snapshot struct {
	ID        string        `json:"id"`
	Mode      worker.Mode   `json:"mode"`
	State     RunState      `json:"state"`
	Steps     []worker.Step `json:"steps"`
	ReceiptID string        `json:"receiptId,omitempty"`
	Error     string        `json:"error,omitempty"`
}

func (r *Run) snapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return Snapshot{
		ID:        r.ID,
		Mode:      r.Mode,
		State:     r.state,
		Steps:     append([]worker.Step(nil), r.steps...),
		ReceiptID: r.receiptID,
		Error:     r.errMsg,
	}
}

// transition advances the state, enforcing the state machine. An illegal
// transition is a programming error and is rejected (state unchanged, returns
// false).
func (r *Run) transition(next RunState) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.state.CanTransition(next) {
		return false
	}
	r.state = next
	r.publishLocked(Event{Type: "state", State: next})
	return true
}

func (r *Run) addStep(s worker.Step) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps = append(r.steps, s)
	step := s
	r.publishLocked(Event{Type: "step", Step: &step})
}

func (r *Run) finishOK(receiptID string) {
	r.mu.Lock()
	r.receiptID = receiptID
	r.mu.Unlock()
	r.transition(StateComplete)
	r.mu.Lock()
	r.publishLocked(Event{Type: "done", State: StateComplete, ReceiptID: receiptID})
	r.closeSubsLocked()
	r.mu.Unlock()
}

func (r *Run) finishErr(msg string) {
	r.mu.Lock()
	r.errMsg = msg
	r.mu.Unlock()
	r.transition(StateFailed)
	r.mu.Lock()
	r.publishLocked(Event{Type: "error", State: StateFailed, Error: msg})
	r.closeSubsLocked()
	r.mu.Unlock()
}

// publishLocked records the event in history and fans it out to subscribers.
// Caller holds r.mu. Non-blocking: a slow subscriber drops events (it can
// re-sync via the status endpoint).
func (r *Run) publishLocked(e Event) {
	r.history = append(r.history, e)
	for ch := range r.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

func (r *Run) closeSubsLocked() {
	for ch := range r.subs {
		close(ch)
		delete(r.subs, ch)
	}
}

// Subscribe returns a channel of events plus an unsubscribe func. The caller
// first receives the buffered history (so a late subscriber still sees prior
// steps), then live events until the run terminates and the channel closes.
func (r *Run) Subscribe() (<-chan Event, func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch := make(chan Event, len(r.history)+16)
	for _, e := range r.history {
		ch <- e
	}
	if r.state.Terminal() {
		close(ch)
		return ch, func() {}
	}
	if r.subs == nil {
		r.subs = map[chan Event]struct{}{}
	}
	r.subs[ch] = struct{}{}
	unsub := func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		if _, ok := r.subs[ch]; ok {
			delete(r.subs, ch)
			close(ch)
		}
	}
	return ch, unsub
}

// RunManager owns the run registry and drives runs to completion, persisting
// the resulting receipt + bundle in the store.
type RunManager struct {
	runner  *worker.Runner
	store   *ReceiptStore
	metrics *Metrics
	counter int64

	mu   sync.RWMutex
	runs map[string]*Run
}

// NewRunManager builds a manager.
func NewRunManager(runner *worker.Runner, store *ReceiptStore, metrics *Metrics) *RunManager {
	return &RunManager{runner: runner, store: store, metrics: metrics, runs: map[string]*Run{}}
}

// Get returns a run by its ephemeral ID.
func (rm *RunManager) Get(id string) (*Run, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	r, ok := rm.runs[id]
	return r, ok
}

// Start creates a run and launches its execution in the background. It returns
// immediately with the created run so the caller can stream progress.
func (rm *RunManager) Start(ctx context.Context, mode worker.Mode) *Run {
	id := fmt.Sprintf("run-%d", atomic.AddInt64(&rm.counter, 1))
	r := &Run{ID: id, Mode: mode, state: StateCreated}

	rm.mu.Lock()
	rm.runs[id] = r
	rm.mu.Unlock()

	rm.metrics.Inc(MetricRunsStarted)
	r.transition(StateRunning)

	go rm.execute(ctx, r, mode)
	return r
}

// execute runs the worker, streams steps, and stores the result.
func (rm *RunManager) execute(ctx context.Context, r *Run, mode worker.Mode) {
	result, err := rm.runner.Run(ctx, mode, func(s worker.Step) { r.addStep(s) })
	if err != nil {
		rm.metrics.Inc(MetricRunsFailed)
		r.finishErr(err.Error())
		return
	}

	receiptJSON, _ := result.Receipt.MarshalJSONIndent()
	id, serr := rm.store.Put(&StoredRun{
		Mode:       string(result.Mode),
		Network:    result.NetworkLabel,
		ProofLabel: result.ProofLabel,
		Receipt:    result.Receipt,
		BundleJSON: result.BundleJSON,
	})
	if serr != nil {
		rm.metrics.Inc(MetricRunsFailed)
		r.finishErr("could not store run: " + serr.Error())
		return
	}
	_ = receiptJSON

	rm.metrics.Inc(MetricRunsCompleted)
	r.finishOK(id)
}
