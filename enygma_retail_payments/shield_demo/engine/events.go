package engine

import (
	"encoding/hex"
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
//   - test/14_shield_demo_walkthrough_test.go, for a walkthrough-only kind;
//   - index.html: SENTENCE (keyed by a hop's first detail), WIRE_KINDS
//     (kinds that don't lengthen a hop's hold), and rowsOf or WIRE (how the
//     hop detail panel draws it), columns if it belongs beside payment 1's
//     public inputs (the witness), and refusal if a card shows it (the
//     replay's refusal);
//   - HAND_CHECK.md: the pace probe's hasDetail, which mirrors WIRE_KINDS;
//   - testdata/default_run.events.json, which a passing make shield-test
//     rewrites, and testdata/walkthrough.events.json, which a passing
//     TestShieldDemoWalkthrough rewrites: commit them, or the replay test
//     (scripts/shield_demo_replay.test.js) never plays the new kind.
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
	// DetailRefusal is a walkthrough's replay of payment 1 refused: who
	// refused it (the relayer or the vault) and why, the spent nullifier it
	// reused, and the chain state before and after it, unchanged.
	DetailRefusal HopDetailKind = "refusal"
	// DetailKey is a key pair an actor or the auditor made: its scheme, its
	// public half in full and its private half shortened. Every key the demo
	// makes is a throwaway demo key.
	DetailKey HopDetailKind = "key"
	// DetailRegistration is what a user's registration sent the
	// UserRegistry: their spend and view public keys and the audit
	// ciphertexts that carry their view key to the auditor.
	DetailRegistration HopDetailKind = "registration"
	// DetailViewKeyRecovery is the auditor opening a user's audit
	// ciphertexts: the ciphertexts, as the UserRegistry holds them, and the
	// view key they open to, the one the user registered.
	DetailViewKeyRecovery HopDetailKind = "viewKeyRecovery"
	// DetailDecryption is notes a view key opened: each note's ciphertext as
	// the chain holds it and the opening (amount, salt, token) it decrypts
	// to. A participant's discovery opens their payment; the audit opens
	// every note it can.
	DetailDecryption HopDetailKind = "decryption"
	// DetailFormula is one of the protocol's equations, which the page
	// draws by its name as the privacy property it gives (see Formula). A
	// full run's carries only the name, once per run on the formula's first
	// hop: the spend key on the Sender's registration (the first hop the
	// UserRegistry holds it by), the commitment on the shield, the Merkle
	// path on payment 1's proof request (a full run has no hop of its own
	// for the proof coming back), the nullifier on payment 1's settlement.
	// A walkthrough's carries the run's real values too, checked against
	// what the chain holds, on the Sender's registration (the spend key:
	// its sk_spend is a throwaway demo key), its shield's hop (the
	// commitment: its values are the shield's opening, public in the
	// deposit's calldata) and payment 1's (the Merkle path on the proof
	// coming back, the nullifier on the settlement): values the page shows
	// there anyway, in payment 1's witness.
	DetailFormula HopDetailKind = "formula"
)

// FormulaName names the equation a formula hop detail shows.
type FormulaName string

const (
	// FormulaCommitment is a note's commitment, Poseidon(pk_spend, salt,
	// amount, token), the leaf its vault inserts.
	FormulaCommitment FormulaName = "commitment"
	// FormulaSpendKey is an actor's spend public key, pk_spend =
	// Poseidon(sk_spend), as the UserRegistry holds it.
	FormulaSpendKey FormulaName = "spendKey"
	// FormulaMerklePath is a spent note's path: its leaf hashed with one
	// sibling per level, Poseidon(left, right) ordered by the leaf index's
	// bit at that level, up to a root its vault has had.
	FormulaMerklePath FormulaName = "merklePath"
	// FormulaNullifier is a spent note's nullifier, Poseidon(sk_spend,
	// tree·2^depth + leaf index), as its vault publishes it.
	FormulaNullifier FormulaName = "nullifier"
)

// Formula is one equation with its real inputs, in the order the equation
// takes them, its real result, and the value the chain holds to check the
// result against. OK says whether the result equals it. Field elements are
// 0x-hex, amounts decimal. A full run's has only its Name: a full run never
// sends a note's opening to the page.
type Formula struct {
	Name   FormulaName    `json:"name"`
	Inputs []FormulaValue `json:"inputs,omitempty"`
	Result string         `json:"result,omitempty"`
	Chain  *FormulaValue  `json:"chain,omitempty"`
	OK     bool           `json:"ok,omitempty"`
	// Leaves, for a Merkle path, is how many leaves its tree held when its
	// root was taken: the crowd the spent note hides in.
	Leaves int `json:"leaves,omitempty"`
}

// FormulaValue is one named value of a formula: an input, or what the chain
// holds (Name then says where, e.g. "leaf #3, the vault's Commitment event").
type FormulaValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// noteOpening names the parts of a note's opening, in the order the
// commitment hashes them. A commitment detail sends only the names: the
// values are private to the note's owner (a walkthrough's shield sends them
// in a commitment formula, public in its calldata anyway).
var noteOpening = []string{"pk_spend", "salt", "amount", "token"}

// HopDetail is what a hop does to the protocol state or, for the http and
// receipt kinds, what it sent over the wire or mined on chain. Field
// elements are 0x-hex. A hop's details are fixed when it plays, so a detail
// can show only what exists by then: a transaction's receipt exists from its
// settle hop on, never on the proof hops before it.
type HopDetail struct {
	Kind HopDetailKind `json:"kind"`
	// Vault is the vault's node ID (NodeVault or NodeUsdrVault) and Tree
	// the number of its tree the hop touches. A spend key formula holds
	// NodeUserRegistry there, where the chain keeps pk_spend, and tree 0.
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

	// refusal: who refused the replay (NodeRelayer or NodeVault) and the
	// reason they gave, the nullifier it reused, payment 1's transaction
	// (TxHash), which spent it, and the chain state before and after the
	// replay. Vault and Tree are the vault and tree of the spent note.
	RefusedBy string      `json:"refusedBy,omitempty"`
	Reason    string      `json:"reason,omitempty"`
	Nullifier string      `json:"nullifier,omitempty"`
	Before    *ChainState `json:"before,omitempty"`
	After     *ChainState `json:"after,omitempty"`

	// key, registration, viewKeyRecovery: the key pair, the registration
	// and the recovery.
	Key          *KeyPair         `json:"key,omitempty"`
	Registration *Registration    `json:"registration,omitempty"`
	Recovery     *ViewKeyRecovery `json:"recovery,omitempty"`

	// formula: the equation and its values. Vault and Tree are where the
	// chain holds the value it is checked against.
	Formula *Formula `json:"formula,omitempty"`

	// decryption: the notes opened. Vault is the RaylsERC20 vault, even for
	// an audit's, which opens USDr notes too: each note names its asset.
	Notes []DecryptedNote `json:"notes,omitempty"`
}

// KeyPair is one key pair as a key hop shows it: whose it is, what it is
// for, its scheme, its public half in 0x-hex in full and its private half
// shortened (see shortSecret), never sent in full.
type KeyPair struct {
	Owner   string `json:"owner"`
	Name    string `json:"name"`
	Scheme  string `json:"scheme"`
	Public  string `json:"public"`
	Private string `json:"private"`
}

// Registration is what UserRegistry.register received from one user, all
// 0x-hex: the spend public key, the view public key (ML-KEM-768
// encapsulation key) and the audit ciphertexts (see AuditCiphertexts).
type Registration struct {
	Owner   string           `json:"owner"`
	SpendPk string           `json:"spendPk"`
	ViewPk  string           `json:"viewPk"`
	Audit   AuditCiphertexts `json:"audit"`
}

// AuditCiphertexts carry a user's view key to the auditor: an ML-KEM-768
// capsule to the auditor's key and the AES-256-GCM encryption, under the
// key it encapsulates, of the view key's 64-byte seed. Both 0x-hex.
type AuditCiphertexts struct {
	Capsule string `json:"capsule"`
	Sealed  string `json:"sealed"`
}

// ViewKeyRecovery is the auditor opening one user's audit ciphertexts, as
// the UserRegistry holds them, to the user's view key: its public half in
// full, matching the one the user registered, and its private half
// shortened.
type ViewKeyRecovery struct {
	Owner   string           `json:"owner"`
	Audit   AuditCiphertexts `json:"audit"`
	ViewPk  string           `json:"viewPk"`
	ViewKey string           `json:"viewKey"`
}

// DecryptedNote is one note a view key opened: its owner, its asset, the
// transaction that published it and its commitment; the ciphertext opened,
// either a Payment or EncryptedNote event's capsule and sealed amount (Via
// "view key") or, with tags, a TagPublished event's payload, in Sealed (Via
// "tag"); and its opening. Byte strings and field elements are 0x-hex, the
// amount decimal.
type DecryptedNote struct {
	Owner      string `json:"owner"`
	Asset      string `json:"asset"`
	Kind       string `json:"kind,omitempty"`
	Tx         string `json:"tx,omitempty"`
	Commitment string `json:"commitment"`
	Via        string `json:"via"`
	Capsule    string `json:"capsule,omitempty"`
	Sealed     string `json:"sealed"`
	Amount     string `json:"amount"`
	Salt       string `json:"salt"`
	Token      string `json:"token"`
}

// The ways a DecryptedNote's ciphertext reached its owner.
const (
	viaViewKey = "view key"
	viaTag     = "tag"
)

// bytesHex is a byte string as the hop details show it.
func bytesHex(b []byte) string { return "0x" + hex.EncodeToString(b) }

// shortSecret is a private key's 0x-hex shortened to its first 6 and last 4
// hex digits, all a key hop sends of it.
func shortSecret(h string) string {
	if len(h) <= 2+6+4 {
		return h
	}
	return h[:2+6] + "…" + h[len(h)-4:]
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
	// then are the actions released once the step's own work is done.
	then []string
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
