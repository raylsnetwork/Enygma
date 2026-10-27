// Package engine is the shield demo's scenario engine: an issuer, a sender
// and K participants on a fresh retail deployment. The issuer gives the
// sender and each participant a shielded USDr note to pay relayer fees with.
// The sender shields public RaylsERC20 into one note and fans it out to the
// participants with chained Payment transfers through the relayer, each
// paying the relayer a USDr fee in the same transaction; each participant
// finds their note by scanning the chain with their view key. A scripted
// sequence of transfers between participants then plays, and Transfer makes
// more by hand. Throughout, an auditor who recovered every user's view key
// from the UserRegistry decrypts each transaction (see Audit).
//
// The engine does every step through the retail core library and the live
// services (the UsdrFee proof, which has no core client, it requests from the
// gnark server directly), and reports progress only through a Sink, so the web UI and the
// integration test drive it the same way.
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"time"

	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/common"
)

// K is the number of participants the sender fans out to.
const K = 6

// merkleDepth is the vaults' Merkle tree depth (256 leaves per tree).
const merkleDepth = 8

// UsdrGrant is the shielded USDr the issuer gives the sender and each
// participant to pay relayer fees with.
const UsdrGrant = 20

// participantNames are the K participants' display names, in order.
var participantNames = [K]string{"Ana", "Bruno", "Carla", "Diego", "Elena", "Felipe"}

// Config locates the deployment and the services a run uses.
type Config struct {
	// RetailRoot is the enygma_retail_payments directory: build/receipts.json
	// and contracts/abis are read from it.
	RetailRoot    string
	RPCURL        string
	ChainID       int64
	GnarkURL      string
	RelayerURL    string
	RelayerAPIKey string
	// FunderKeyHex signs the issuer's public steps (it must hold the token
	// contracts' owner role) and funds each actor's account with gas.
	FunderKeyHex string
	// StepPause is slept after each step so a viewer can follow; 0 in tests.
	StepPause time.Duration
}

// DefaultConfig is the local stack shield_demo/run.sh starts.
func DefaultConfig(retailRoot string) Config {
	return Config{
		RetailRoot:    retailRoot,
		RPCURL:        "http://127.0.0.1:8545",
		ChainID:       1337,
		GnarkURL:      "http://localhost:8082",
		RelayerURL:    "http://localhost:8090",
		RelayerAPIKey: "test-api-key-dev-only",
		// Hardhat account[0] — the deployer, owner of RaylsERC20 (demo only).
		FunderKeyHex: "34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154",
	}
}

// Options are a run's scenario parameters.
type Options struct {
	// ShieldTotal is the public RaylsERC20 minted to the sender and shielded
	// into the sender's one note.
	ShieldTotal int64 `json:"shieldTotal"`
	// Payouts[i] is what the sender pays participant i in the fan-out.
	Payouts [K]int64 `json:"payouts"`
	// UsdrFee sends each transfer through the relayer's USDr-fee route, paying
	// the relayer the fixed USDr fee in the same transaction; off, transfers
	// take the plain route and pay no fee.
	UsdrFee bool `json:"usdrFee"`
	// Tags opens a private-tag channel between each pair that transacts and
	// publishes a tag for every fan-out transfer and transfer between
	// participants, so each recipient finds their notes by tag instead of
	// trying to decrypt every note on chain.
	Tags bool `json:"tags"`
	// Script is the transfers between participants played after the fan-out.
	Script []Transfer `json:"script"`
}

// DefaultOptions pays the participants different amounts, leaves the sender
// some change, plays the scripted transfers between participants (one a
// merge), pays relayer fees in USDr and tags every transfer.
func DefaultOptions() Options {
	return Options{
		ShieldTotal: 1000,
		Payouts:     [K]int64{120, 90, 150, 60, 200, 80},
		UsdrFee:     true,
		Tags:        true,
		Script:      append([]Transfer(nil), defaultScript...),
	}
}

// Validate checks the options before a run: positive payouts the shielded
// total covers, and a script each payer can pay from one note.
func (o Options) Validate() error {
	var sum int64
	for i, p := range o.Payouts {
		if p <= 0 {
			return fmt.Errorf("payout to %s must be positive, got %d", participantNames[i], p)
		}
		sum += p
	}
	if sum > o.ShieldTotal {
		return fmt.Errorf("payouts total %d exceeds the shielded total %d", sum, o.ShieldTotal)
	}
	return o.simulateScript()
}

// Role is an actor's part in the scenario.
type Role string

const (
	Issuer      Role = "issuer"
	Sender      Role = "sender"
	Participant Role = "participant"
)

// Actor is one person in the scenario, with their keys and the account that
// signs their public steps.
type Actor struct {
	ID      int // panel index: 0 issuer, 1 sender, 2.. participants
	Name    string
	Role    Role
	Address common.Address
	Spend   *rpcore.SpendKeyPair
	View    *rpcore.ViewKeyPair

	keyHex string
	notes  purse // RaylsERC20 vault notes
	usdr   purse // USDr vault notes
	tags   tagState
}

// note is a vault note an actor can open.
type note struct {
	commitment *big.Int
	salt       *big.Int
	amount     *big.Int
	spent      bool
}

// purse is the notes an actor can open in one vault.
type purse []*note

// Settlement is one transfer as it settled on chain.
type Settlement struct {
	From   string
	To     string
	Amount *big.Int
	TxHash common.Hash
	// UsdrFeeNote is the relayer's USDr fee note the transfer paid; nil on
	// the plain route.
	UsdrFeeNote *big.Int
	// TagTxHash is the transaction that published the transfer's private
	// tag; zero with tags off.
	TagTxHash common.Hash
}

// Engine runs the scenario on one fresh deployment.
type Engine struct {
	cfg  Config
	sink Sink
	opts Options

	auditor *rpcore.AuditorKeyPair
	actors  []*Actor // issuer, sender, participants

	chain   *chain
	relayer *relayer

	// usdrFixedFee is EnygmaDvp's fixed USDr fee and relayerFeePk the
	// relayer's fee spend key, both read when the run starts.
	usdrFixedFee *big.Int
	relayerFeePk *big.Int

	fanOut    []Settlement
	transfers []Settlement // the script's, then manual ones

	// proving and relaying add up the transfers' proof and relay times (see Totals).
	proving, relaying time.Duration

	// startBlock and startChannel are the chain's next block and the channel
	// registry's next record when the run started: the auditor scans from them.
	startBlock, startChannel uint64
	// auditUsers are the users whose view keys the auditor recovered.
	auditUsers []auditUser
}

// New returns an engine that reports to sink.
func New(cfg Config, sink Sink) *Engine {
	return &Engine{cfg: cfg, sink: sink, relayer: &relayer{baseURL: cfg.RelayerURL, apiKey: cfg.RelayerAPIKey}}
}

// Issuer, Sender and Participants return the run's actors; they are set
// once Run has generated the keys.
func (e *Engine) Issuer() Actor { return *e.issuer() }
func (e *Engine) Sender() Actor { return *e.sender() }
func (e *Engine) Participants() []Actor {
	out := make([]Actor, 0, K)
	for _, a := range e.participants() {
		out = append(out, *a)
	}
	return out
}

// actors holds the issuer, then the sender, then the K participants.
func (e *Engine) issuer() *Actor         { return e.actors[0] }
func (e *Engine) sender() *Actor         { return e.actors[1] }
func (e *Engine) participants() []*Actor { return e.actors[2:] }

// registrable are the actors registered in the UserRegistry: the sender and
// the participants.
func (e *Engine) registrable() []*Actor { return e.actors[1:] }

// Auditor returns the key pair the actors' view keys are encrypted to.
func (e *Engine) Auditor() *rpcore.AuditorKeyPair { return e.auditor }

// Balance is the total of the unspent RaylsERC20 notes the actor can open.
func (e *Engine) Balance(a Actor) *big.Int {
	return e.actors[a.ID].notes.balance()
}

// UsdrBalance is the total of the unspent USDr notes the actor can open.
func (e *Engine) UsdrBalance(a Actor) *big.Int {
	return e.actors[a.ID].usdr.balance()
}

// FanOut returns the run's fan-out transfers, in order.
func (e *Engine) FanOut() []Settlement {
	return append([]Settlement(nil), e.fanOut...)
}

// Transfers returns the transfers between participants after the fan-out —
// the script's, then manual ones — in order.
func (e *Engine) Transfers() []Settlement {
	return append([]Settlement(nil), e.transfers...)
}

func (p purse) balance() *big.Int {
	sum := new(big.Int)
	for _, n := range p.unspent() {
		sum.Add(sum, n.amount)
	}
	return sum
}

// Unreachable names each service in cfg that does not accept connections.
func Unreachable(cfg Config) []string {
	var missing []string
	for _, s := range []struct{ name, url string }{
		{"chain", cfg.RPCURL},
		{"gnark server", cfg.GnarkURL},
		{"relayer", cfg.RelayerURL},
	} {
		if !reachable(s.url) {
			missing = append(missing, fmt.Sprintf("%s (%s)", s.name, s.url))
		}
	}
	return missing
}

func reachable(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	conn, err := net.DialTimeout("tcp", u.Host, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Run plays the scenario: check the services, generate every actor's keys,
// register the sender and participants, give them USDr fee credit, shield
// the sender's public balance, fan it out to the participants one mined
// transfer at a time, let each participant discover their note by
// scanning, play the scripted transfers between participants, and show the
// run's totals. A step that fails ends the run with its error.
func (e *Engine) Run(ctx context.Context, opts Options) error {
	if err := opts.Validate(); err != nil {
		return e.fail("prereqs", "Check prerequisites", err)
	}
	e.opts = opts
	steps := []stepDef{
		{"prereqs", "Check prerequisites", "Probing chain · gnark server · relayer…", e.checkPrereqs},
		{"keygen", "Generate keys", "Issuer, sender and participants generate spend and view keys…", e.generateKeys},
		{"register", "Register in UserRegistry", "Sender and participants publish their keys…", e.register},
		{"audit_keys", "Auditor keys", "Auditor opens each registered user's audit ciphertexts in the UserRegistry…", e.auditorKeysStep},
	}
	if opts.Tags {
		steps = append(steps, stepDef{"channels", "Tag channels", "Each payer opens a private-tag channel to everyone they will pay, through the relayer…", e.openChannels})
	}
	steps = append(steps, []stepDef{
		{"usdr", "USDr fee credit", fmt.Sprintf("Issuer shields a %d USDr note to the sender and each participant…", UsdrGrant), e.grantUsdr},
		{"shield", "Shield", fmt.Sprintf("Sender gets %d public RaylsERC20 and shields it into one private note…", opts.ShieldTotal),
			func(ctx context.Context, s *step) error { return e.shield(ctx, s, opts) }},
	}...)
	route := "no fee"
	if opts.UsdrFee {
		route = "USDr fee to the relayer"
	}
	if opts.Tags {
		route += ", private tag"
	}
	for i, name := range participantNames {
		n, amount := i+1, big.NewInt(opts.Payouts[i])
		steps = append(steps, stepDef{
			fmt.Sprintf("fanout_%d", n), "Pay " + name,
			fmt.Sprintf("Fan-out %d/%d: sender pays %s privately, change back to the sender (%s)…", n, K, name, route),
			func(ctx context.Context, s *step) error {
				return e.payFanOut(ctx, s, n, e.participants()[i], amount, opts.UsdrFee)
			},
		})
	}
	discoverMsg := "Participants scan the chain with their view keys…"
	if opts.Tags {
		discoverMsg = "Participants scan the tag registry for their tags…"
	}
	steps = append(steps, stepDef{"discover", "Discover", discoverMsg, e.discover})
	for i, t := range opts.Script {
		steps = append(steps, stepDef{
			fmt.Sprintf("transfer_%d", i+1), fmt.Sprintf("%s → %s", t.From, t.To),
			fmt.Sprintf("Transfer %d/%d: %s pays %s %d privately (%s)…", i+1, len(opts.Script), t.From, t.To, t.Amount, route),
			func(ctx context.Context, s *step) error { return e.payByName(ctx, s, t) },
		})
	}
	steps = append(steps,
		stepDef{"audit", "Audit", "Auditor reconciles every user's balances from the transactions it decrypted…", e.reconcile},
		stepDef{"totals", "Totals", "Adding up the run's transactions, gas, proving and relay time…", e.showTotals},
	)
	announced := make([]map[string]string, len(steps))
	for i, s := range steps {
		announced[i] = map[string]string{"id": s.id, "label": s.label}
	}
	stepsJSON, _ := json.Marshal(announced)
	e.sink.Emit(Event{Kind: ProtocolField, Field: "steps", Value: string(stepsJSON)})

	for _, s := range steps {
		st := &step{e: e, id: s.id, label: s.label}
		st.start(s.msg)
		if err := runStep(ctx, s, st); err != nil {
			return e.fail(s.id, s.label, err)
		}
		// The auditor's view follows each step's transactions.
		if e.auditUsers != nil {
			if err := e.publishAudit(ctx, st); err != nil {
				return e.fail(s.id, s.label, fmt.Errorf("audit: %w", err))
			}
		}
		if e.cfg.StepPause > 0 {
			time.Sleep(e.cfg.StepPause)
		}
	}
	e.sink.Emit(Event{Kind: RunDone, Msg: "Scenario complete"})
	return nil
}

type stepDef struct {
	id, label, msg string
	run            func(context.Context, *step) error
}

// runStep runs s, turning a panic into the step's error, so a bug fails the
// run like any other error instead of taking the web server down with it.
func runStep(ctx context.Context, s stepDef, st *step) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return s.run(ctx, st)
}

// Close releases the run's chain connection.
func (e *Engine) Close() { e.chain.close() }

func (e *Engine) fail(id, label string, err error) error {
	e.sink.Emit(Event{Kind: StepFailed, Step: id, Label: label, Msg: err.Error()})
	e.sink.Emit(Event{Kind: RunFailed, Msg: err.Error()})
	return fmt.Errorf("%s: %w", id, err)
}
