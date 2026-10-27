package engine

import "fmt"

// EventKind says what an Event reports.
type EventKind string

const (
	StepStarted   EventKind = "step_started"
	StepSucceeded EventKind = "step_succeeded"
	StepFailed    EventKind = "step_failed"
	// Log is a protocol log line; Cat is key|chain|zk|tag or empty.
	Log EventKind = "log"
	// ActorField sets Field to Value on actor Actor's panel.
	ActorField EventKind = "actor"
	// ProtocolField sets a run-wide Field (a contract address, a total, …).
	ProtocolField EventKind = "proto"
	RunDone       EventKind = "done"
	RunFailed     EventKind = "failed"
)

// Event is one progress report from the engine.
type Event struct {
	Kind  EventKind `json:"kind"`
	Step  string    `json:"step,omitempty"`
	Label string    `json:"label,omitempty"`
	Msg   string    `json:"msg,omitempty"`
	Cat   string    `json:"cat,omitempty"`
	Actor int       `json:"actor"`
	Field string    `json:"field,omitempty"`
	Value string    `json:"value,omitempty"`
}

// Sink receives the engine's events, in order, from the goroutine running
// the scenario.
type Sink interface {
	Emit(Event)
}

// step reports progress for the step being run.
type step struct {
	e         *Engine
	id, label string
}

func (s *step) start(msg string) {
	s.e.sink.Emit(Event{Kind: StepStarted, Step: s.id, Label: s.label, Msg: msg})
}

func (s *step) done(format string, args ...any) {
	s.e.sink.Emit(Event{Kind: StepSucceeded, Step: s.id, Label: s.label, Msg: fmt.Sprintf(format, args...)})
}

func (s *step) log(cat, format string, args ...any) {
	s.e.sink.Emit(Event{Kind: Log, Step: s.id, Cat: cat, Msg: fmt.Sprintf(format, args...)})
}

func (s *step) actor(a *Actor, field, value string) {
	s.e.sink.Emit(Event{Kind: ActorField, Actor: a.ID, Field: field, Value: value})
}

func (s *step) proto(field, value string) {
	s.e.sink.Emit(Event{Kind: ProtocolField, Field: field, Value: value})
}
