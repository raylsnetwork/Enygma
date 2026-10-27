// Package engine is the shield demo's scenario engine: an issuer, a sender
// and K participants on a fresh retail deployment. The sender shields public
// RaylsERC20 into one note and fans it out to the participants with chained
// Payment transfers through the relayer; each participant finds their note
// by scanning the chain with their view key.
//
// The engine does every step through the retail core library and the live
// services, and reports progress only through a Sink, so the web UI and the
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
}

// DefaultOptions pays the participants different amounts and leaves the
// sender some change.
func DefaultOptions() Options {
	return Options{
		ShieldTotal: 1000,
		Payouts:     [K]int64{120, 90, 150, 60, 200, 80},
	}
}

func (o Options) validate() error {
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
	return nil
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
	notes  []*note
}

// note is a vault note an actor can open.
type note struct {
	commitment *big.Int
	salt       *big.Int
	amount     *big.Int
	spent      bool
}

// Engine runs the scenario on one fresh deployment.
type Engine struct {
	cfg  Config
	sink Sink

	auditor *rpcore.AuditorKeyPair
	actors  []*Actor // issuer, sender, participants

	chain   *chain
	relayer *relayer
}

// New returns an engine that reports to sink.
func New(cfg Config, sink Sink) *Engine {
	return &Engine{cfg: cfg, sink: sink, relayer: &relayer{baseURL: cfg.RelayerURL, apiKey: cfg.RelayerAPIKey}}
}

// Issuer, Sender and Participants return the run's actors; they are set
// once Run has generated the keys.
func (e *Engine) Issuer() Actor { return *e.actors[0] }
func (e *Engine) Sender() Actor { return *e.actors[1] }
func (e *Engine) Participants() []Actor {
	out := make([]Actor, 0, K)
	for _, a := range e.actors[2:] {
		out = append(out, *a)
	}
	return out
}

// Auditor returns the key pair the actors' view keys are encrypted to.
func (e *Engine) Auditor() *rpcore.AuditorKeyPair { return e.auditor }

// Balance is the total of the unspent notes the actor can open.
func (e *Engine) Balance(a Actor) *big.Int {
	return e.actors[a.ID].balance()
}

func (a *Actor) balance() *big.Int {
	sum := new(big.Int)
	for _, n := range a.notes {
		if !n.spent {
			sum.Add(sum, n.amount)
		}
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
// register the sender and participants, shield the sender's public balance,
// fan it out to the participants, and let each participant discover their
// note by scanning.
func (e *Engine) Run(ctx context.Context, opts Options) error {
	if err := opts.validate(); err != nil {
		return e.fail("prereqs", "Check prerequisites", err)
	}
	steps := []struct {
		id, label, msg string
		run            func(context.Context, *step) error
	}{
		{"prereqs", "Check prerequisites", "Probing chain · gnark server · relayer…", e.checkPrereqs},
		{"keygen", "Generate keys", "Issuer, sender and participants generate spend and view keys…", e.generateKeys},
		{"register", "Register in UserRegistry", "Sender and participants publish their keys…", e.register},
	}
	announced := make([]map[string]string, len(steps))
	for i, s := range steps {
		announced[i] = map[string]string{"id": s.id, "label": s.label}
	}
	stepsJSON, _ := json.Marshal(announced)
	e.sink.Emit(Event{Kind: ProtocolField, Field: "steps", Value: string(stepsJSON)})

	for _, s := range steps {
		st := &step{e: e, id: s.id, label: s.label}
		st.start(s.msg)
		if err := s.run(ctx, st); err != nil {
			return e.fail(s.id, s.label, err)
		}
		if e.cfg.StepPause > 0 {
			time.Sleep(e.cfg.StepPause)
		}
	}
	e.sink.Emit(Event{Kind: RunDone, Msg: "Scenario complete"})
	return nil
}

// Close releases the run's chain connection.
func (e *Engine) Close() { e.chain.close() }

func (e *Engine) fail(id, label string, err error) error {
	e.sink.Emit(Event{Kind: StepFailed, Step: id, Label: label, Msg: err.Error()})
	e.sink.Emit(Event{Kind: RunFailed, Msg: err.Error()})
	return fmt.Errorf("%s: %w", id, err)
}
