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
	Hop EventKind = "hop"
	// ActionWaiting is a walkthrough's engine waiting, before Action of
	// step Step, about the person Value names (or no one), for its Gate.
	ActionWaiting EventKind = "action"
	RunDone       EventKind = "done"
	RunFailed     EventKind = "failed"
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
	Action string    `json:"action,omitempty"`

	HopDetail []HopDetail `json:"hopDetail,omitempty"`
}

// HopDetailKind says what a HopDetail shows.
//
// A new kind is also handled where nothing will make the compiler point:
//   - test/13_shield_demo_scenario_test.go: checkProofDetails and
//     checkSettleChecks fail on a kind they don't list;
//   - index.html: SENTENCE (keyed by a hop's first detail), WIRE_KINDS
//     (kinds that don't lengthen a hop's hold), and rowsOf or WIRE (how the
//     hop detail panel draws it), and columns if it belongs beside
//     payment 1's public inputs (the witness);
//   - HAND_CHECK.md: the pace probe's hasDetail, which mirrors WIRE_KINDS.
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
	// DetailSettleChecks is what EnygmaDvp has the vault check of one proof
	// before it settles it: each spent note's Merkle root is one the vault
	// has had, its nullifier is unused, and the proof verifies; then the
	// nullifiers the vault publishes, one per spent note.
	DetailSettleChecks HopDetailKind = "settleChecks"
	// DetailWitness is a proof request's witness: the private inputs the
	// payer's wallet sent its prover, with their values. Only payment 1 of a
	// walkthrough carries one, beside its publicInputs; every walkthrough key
	// is a throwaway demo key.
	DetailWitness HopDetailKind = "witness"
	// DetailHTTP is one HTTP request a hop sent, to the gnark server or the
	// relayer, and the response it got back, both bodies as they went over
	// the wire.
	DetailHTTP HopDetailKind = "http"
	// DetailReceipt is the receipt of the transaction a hop mined: its hash,
	// block, gas and the events it logged.
	DetailReceipt HopDetailKind = "receipt"
)

// noteOpening names the parts of a note's opening, in the order the
// commitment hashes them. Only the names go to the page: the values are
// private to the note's owner.
var noteOpening = []string{"pk_spend", "salt", "amount", "token"}

// HopDetail is what a hop does to the protocol state or, for the http and
// receipt kinds, what it sent over the wire or mined on chain. Field
// elements are 0x-hex.
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

	// witness: the circuit and its private inputs' values. Vault is the
	// vault it spends from and Tree its first spent note's tree.
	Witness *Witness `json:"witness,omitempty"`

	// settleChecks: the circuit whose proof is checked and the transaction
	// that settles it (TxHash), and its spent notes. Vault is the vault it
	// spends from and Tree its first spent note's tree.
	Spent []SpentNote `json:"spent,omitempty"`

	// http: the request's method and URL, the response's status, and both
	// bodies as sent. Headers are left out.
	Method   string `json:"method,omitempty"`
	URL      string `json:"url,omitempty"`
	Status   int    `json:"status,omitempty"`
	Request  string `json:"request,omitempty"`
	Response string `json:"response,omitempty"`

	// receipt: the transaction (TxHash), its block, its gas and its logs.
	Block   uint64        `json:"block,omitempty"`
	GasUsed uint64        `json:"gas,omitempty"`
	Events  []LoggedEvent `json:"events,omitempty"`
}

// Witness is a payment proof's private inputs as the payer's wallet sent
// them to its prover, grouped by note. Field elements are 0x-hex, amounts
// decimal.
type Witness struct {
	// SpendKey is the payer's spend private key (PrivateKeysIn), the same
	// for every spent note.
	SpendKey string `json:"spendKey"`
	// Token is the token id (TokenId) every note's commitment hashes.
	Token string `json:"token"`
	// In are the spent notes (ValuesIn, SaltsIn, PathElements, PathIndices).
	In []NoteOpening `json:"in"`
	// Out are the new notes, in CommitmentsOut order: the payee's, then the
	// payer's change (SpendPublicKeysOut, ValuesOut, SaltsOut).
	Out []NoteOpening `json:"out"`
}

// NoteOpening is one note of a witness: whose it is, its amount and salt,
// and, for a new note, its owner's spend public key (a spent note's is
// derived in the circuit from the spend key) or, for a spent note, its
// Merkle path.
type NoteOpening struct {
	Owner   string    `json:"owner"`
	PkSpend string    `json:"pkSpend,omitempty"`
	Amount  string    `json:"amount"`
	Salt    string    `json:"salt"`
	Path    *NotePath `json:"path,omitempty"`
}

// NotePath is a spent note's Merkle path in its tree: its leaf index
// (PathIndices) and the sibling at each level from the leaf up
// (PathElements).
type NotePath struct {
	Tree     int      `json:"tree"`
	Index    int      `json:"index"`
	Siblings []string `json:"siblings"`
}

// LoggedEvent is one log of a transaction's receipt: the contract that
// logged it (a diagram node, a token's name, or its address), its address,
// and its event's name and arguments in declaration order.
type LoggedEvent struct {
	Contract string     `json:"contract"`
	Address  string     `json:"address"`
	Name     string     `json:"name"`
	Args     []EventArg `json:"args"`
}

// EventArg is one argument of a logged event.
type EventArg struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SpentNote is one note a settled proof spends, as EnygmaDvp checks it: its
// Merkle root, checked as one the vault has had in tree Tree, and its
// nullifier, checked unused, then published.
type SpentNote struct {
	Tree      int    `json:"tree"`
	Root      string `json:"root"`
	Nullifier string `json:"nullifier"`
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
	// gated releases each action inside the step on its own (see await).
	gated bool
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
