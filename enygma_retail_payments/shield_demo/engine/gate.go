package engine

import (
	"context"
	"sync"
)

// Gate holds a walkthrough's engine before each action until the presenter
// releases it. An action is one real call: a key generation, a transaction
// (a registration, the shield), a proof request, a relay, a scan or one of
// the auditor's decryptions. A full run never waits on it.
type Gate struct {
	mu      sync.Mutex
	waiting bool
	release chan struct{}
}

// newGate is a closed gate.
func newGate() *Gate { return &Gate{release: make(chan struct{}, 1)} }

// Release lets the waiting engine perform its next action. It reports
// whether the engine was waiting; if not, it does nothing, so a release
// never carries over to an action that has not been announced yet.
func (g *Gate) Release() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.waiting {
		return false
	}
	g.waiting = false
	g.release <- struct{}{}
	return true
}

// open marks the engine as waiting; the caller then announces the action
// and calls wait. Opening before the announcement means a release by anyone
// who has seen it is never lost.
func (g *Gate) open() {
	g.mu.Lock()
	g.waiting = true
	g.mu.Unlock()
}

// wait blocks until Release, or until ctx ends.
func (g *Gate) wait(ctx context.Context) error {
	select {
	case <-g.release:
		return nil
	case <-ctx.Done():
		g.mu.Lock()
		g.waiting = false
		g.mu.Unlock()
		return ctx.Err()
	}
}

// awaitAction announces action of step id, about subject (a person's name,
// or empty), and waits for its release.
func (e *Engine) awaitAction(ctx context.Context, id, action, subject string) error {
	e.gate.open()
	e.sink.Emit(Event{Kind: ActionWaiting, Step: id, Action: action, Value: subject})
	return e.gate.wait(ctx)
}

// await waits for the release of action, about subject, if this step
// releases its actions one at a time (in a walkthrough); otherwise it
// returns at once.
func (s *step) await(ctx context.Context, action, subject string) error {
	if !s.gated {
		return nil
	}
	return s.e.awaitAction(ctx, s.id, action, subject)
}
