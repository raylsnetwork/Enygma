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
	// Hop is one interaction From one node To another (both Nodes IDs),
	// with a short Label and an optional Detail; a key-material step is a
	// hop from an actor to itself.
	Hop       EventKind = "hop"
	RunDone   EventKind = "done"
	RunFailed EventKind = "failed"
)

// Event is one progress report from the engine.
type Event struct {
	Kind   EventKind `json:"kind"`
	Step   string    `json:"step,omitempty"`
	Label  string    `json:"label,omitempty"`
	Msg    string    `json:"msg,omitempty"`
	Cat    string    `json:"cat,omitempty"`
	Actor  int       `json:"actor"`
	Field  string    `json:"field,omitempty"`
	Value  string    `json:"value,omitempty"`
	From   string    `json:"from,omitempty"`
	To     string    `json:"to,omitempty"`
	Detail string    `json:"detail,omitempty"`
}

// The diagram's node IDs other than the actors' (see ActorNode). The page
// draws the same set.
const (
	NodeAuditor            = "auditor"
	NodeFunder             = "funder"
	NodeGnark              = "gnark"
	NodeRelayer            = "relayer"
	NodeUserRegistry       = "UserRegistry"
	NodeVault              = "Erc20CoinVault"
	NodeUsdrVault          = "UsdrCoinVault"
	NodeTagChannelRegistry = "TagChannelRegistry"
	NodeTagRegistry        = "TagRegistry"
	NodeDvp                = "EnygmaDvp"
)

// ActorNode is the node ID of the actor with panel index id.
func ActorNode(id int) string { return fmt.Sprintf("actor%d", id) }

// Node is one part of the system the diagram draws.
type Node struct {
	ID    string
	Group string // actor, service or contract
}

// Nodes is every node a hop can name: the issuer, the sender, the K
// participants, the auditor and the gas funder; the gnark server and the
// relayer; and the contracts.
var Nodes = func() []Node {
	var ns []Node
	for id := range 2 + K {
		ns = append(ns, Node{ActorNode(id), "actor"})
	}
	ns = append(ns, Node{NodeAuditor, "actor"}, Node{NodeFunder, "actor"},
		Node{NodeGnark, "service"}, Node{NodeRelayer, "service"})
	for _, id := range []string{NodeUserRegistry, NodeVault, NodeUsdrVault, NodeDvp, NodeTagChannelRegistry, NodeTagRegistry} {
		ns = append(ns, Node{id, "contract"})
	}
	return ns
}()

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

// hop reports an interaction from one node to another, both Nodes IDs.
func (s *step) hop(from, to, label, detail string) {
	s.e.sink.Emit(Event{Kind: Hop, Step: s.id, From: from, To: to, Label: label, Detail: detail})
}

func (s *step) proto(field, value string) {
	s.e.sink.Emit(Event{Kind: ProtocolField, Field: field, Value: value})
}
