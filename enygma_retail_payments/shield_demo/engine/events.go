package engine

import (
	"fmt"
	"math/big"
)

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
	// with a short Label, an optional Detail (its caption) and optional
	// HopDetail, what it does to the protocol state; a key-material step is
	// a hop from an actor to itself. A Failed hop is one that failed its
	// step, its Detail the error.
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
	Failed bool      `json:"failed,omitempty"`

	HopDetail []HopDetail `json:"hopDetail,omitempty"`
}

// HopDetailKind says what a HopDetail shows.
type HopDetailKind string

const (
	// DetailLeavesIn is new commitments landing as leaves in a vault's tree.
	DetailLeavesIn HopDetailKind = "leavesIn"
	// DetailCommitment is a deposit's new note: its commitment, public, and
	// its opening, private, landing as one leaf in the vault's tree.
	DetailCommitment HopDetailKind = "commitment"
	// DetailMerklePath is a spent note's Merkle path: its leaf, hashed up
	// through one sibling per level to a root the vault has had.
	DetailMerklePath HopDetailKind = "merklePath"
	// DetailPublicInputs is a proof's public inputs, named as in its
	// circuit's statement layout, and the names of what stays private.
	DetailPublicInputs HopDetailKind = "publicInputs"
)

// noteOpening names the parts of a note's opening, in the order the
// commitment hashes them. Only the names go to the page: the values are
// private to the note's owner.
var noteOpening = []string{"pk_spend", "salt", "amount", "token"}

// HopDetail is what a hop does to the protocol state. Field elements are
// 0x-hex.
type HopDetail struct {
	Kind HopDetailKind `json:"kind"`
	// Vault is the vault's node ID (NodeVault or NodeUsdrVault) and Tree
	// the number of its tree the hop touches.
	Vault string `json:"vault"`
	Tree  int    `json:"tree"`

	// leavesIn and commitment: the transaction that inserted the leaves,
	// the leaves, and the tree's root before and after them. TreeStarted is
	// set when the batch started a new tree, the previous one being too
	// full for it; RootBefore is then the empty tree's root.
	TxHash      string `json:"tx,omitempty"`
	Leaves      []Leaf `json:"leaves,omitempty"`
	RootBefore  string `json:"rootBefore,omitempty"`
	RootAfter   string `json:"rootAfter,omitempty"`
	TreeStarted bool   `json:"treeStarted,omitempty"`

	// commitment: the note's commitment (its one leaf) and the names of
	// its opening's parts.
	Commitment string   `json:"commitment,omitempty"`
	Opening    []string `json:"opening,omitempty"`

	// merklePath: the spent note's leaf, the sibling at each level from the
	// leaf up (on the right where the path's node at that level is even,
	// i.e. where the leaf index's bit is 0), and the root they hash to.
	Leaf     *Leaf    `json:"leaf,omitempty"`
	Siblings []string `json:"siblings,omitempty"`
	Root     string   `json:"root,omitempty"`

	// publicInputs: the circuit, its public signals in statement order, and
	// the names of its private witnesses. Vault is the vault it spends from
	// and Tree its first spent note's tree.
	Circuit string        `json:"circuit,omitempty"`
	Inputs  []PublicInput `json:"inputs,omitempty"`
	Private []string      `json:"private,omitempty"`
}

// PublicInput is one public signal of a proof: element Index of its
// statement layout's field Name.
type PublicInput struct {
	Name  string `json:"name"`
	Index int    `json:"index"`
	Value string `json:"value"`
}

// Leaf is one commitment in a vault's tree. Owner is the actor whose note
// it is: the chain does not know it, the demo does, holding every key.
type Leaf struct {
	Index      int    `json:"index"`
	Commitment string `json:"commitment"`
	Owner      string `json:"owner,omitempty"`
}

// TreeSnapshot is one vault's Merkle trees when a run starts, sent as the
// run's "trees" protocol field (a JSON list, one per vault): its current
// tree's number, leaves and root, and its earlier, full trees.
type TreeSnapshot struct {
	Vault     string     `json:"vault"`
	Tree      int        `json:"tree"`
	LeafCount int        `json:"leafCount"`
	Leaves    []Leaf     `json:"leaves"`
	Root      string     `json:"root"`
	FullTrees []FullTree `json:"fullTrees"`
}

// FullTree is a vault's tree that the vault has moved on from.
type FullTree struct {
	Tree      int    `json:"tree"`
	LeafCount int    `json:"leafCount"`
	Root      string `json:"root"`
}

// fieldHex is a field element as the hop details show it.
func fieldHex(v *big.Int) string { return fmt.Sprintf("0x%064x", v) }

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

// hop reports an interaction from one node to another, both Nodes IDs,
// with what it does to the protocol state, if anything.
func (s *step) hop(from, to, label, detail string, hd ...HopDetail) {
	s.e.sink.Emit(Event{Kind: Hop, Step: s.id, From: from, To: to, Label: label, Detail: detail, HopDetail: hd})
}

// hopFailed reports the interaction from one node to another that failed
// the step with err.
func (s *step) hopFailed(from, to, label string, err error) {
	s.e.sink.Emit(Event{Kind: Hop, Step: s.id, From: from, To: to, Label: label, Detail: err.Error(), Failed: true})
}

func (s *step) proto(field, value string) {
	s.e.sink.Emit(Event{Kind: ProtocolField, Field: field, Value: value})
}
