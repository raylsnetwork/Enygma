package tests

// TestShieldDemoScenario drives the shield demo's scenario engine (the same
// engine the port-9092 web UI drives) against the live stack, with a sink
// that captures its events, and checks only what the chain, the relayer's
// fee-note store and the actors' scans show.
//
// Prerequisites — a fresh deployment, as shield_demo/run.sh brings up:
//
//	Hardhat node (8545), retail deploy + init, gnark server (8082), relayer (8090)
//
// The relayer's fee-note store is read from RELAYER_FEE_NOTES_PATH, default
// ../build/relayer_fee_notes.jsonl (where run.sh points the relayer).
//
// Run:
//
//	cd test && CC=/usr/bin/clang go test -run TestShieldDemoScenario -v -timeout 1800s

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"enygma_retail_payments/shield_demo/engine"
	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// capturingSink records every event the engine emits.
type capturingSink struct {
	mu     sync.Mutex
	events []engine.Event
}

func (s *capturingSink) Emit(e engine.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
}

// failures returns the messages of the steps that failed, for diagnostics.
func (s *capturingSink) failures() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, e := range s.events {
		if e.Kind == engine.StepFailed {
			out = append(out, e.Step+": "+e.Msg)
		}
	}
	return strings.Join(out, "; ")
}

// hops returns the hops the engine has emitted, in order.
func (s *capturingSink) hops() []engine.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []engine.Event
	for _, e := range s.events {
		if e.Kind == engine.Hop {
			out = append(out, e)
		}
	}
	return out
}

// transferSteps groups the hops of the fan-out and scripted transfer steps
// by step.
func transferSteps(hops []engine.Event) map[string][]engine.Event {
	out := map[string][]engine.Event{}
	for _, h := range hops {
		if strings.HasPrefix(h.Step, "fanout_") || strings.HasPrefix(h.Step, "transfer_") {
			out[h.Step] = append(out[h.Step], h)
		}
	}
	return out
}

// checkGrantShieldScanHops checks that the issuer's USDr grants, the
// shield and each participant's discovery scan play as hops.
func checkGrantShieldScanHops(t *testing.T, e *engine.Engine, hops []engine.Event) {
	t.Helper()
	issuer, sender := engine.ActorNode(e.Issuer().ID), engine.ActorNode(e.Sender().ID)
	grants, shields := 0, 0
	scanned := map[string]bool{}
	for _, h := range hops {
		switch {
		case h.From == issuer && h.To == engine.NodeUsdrVault:
			grants++
		case h.From == sender && h.To == engine.NodeVault:
			shields++
		case h.Step == "discover" && h.From == engine.NodeVault:
			scanned[h.To] = true
		}
	}
	if want := 1 + engine.K; grants != want {
		t.Errorf("%d grant hops from the issuer to the USDr vault, want one per registered user (%d)", grants, want)
	}
	if shields != 1 {
		t.Errorf("%d shield hops from the sender to the RaylsERC20 vault, want 1", shields)
	}
	for _, p := range e.Participants() {
		if !scanned[engine.ActorNode(p.ID)] {
			t.Errorf("%s's discovery has no hop from the RaylsERC20 vault", p.Name)
		}
	}
}

// touches reports whether hop h goes from or to node.
func touches(h engine.Event, node string) bool { return h.From == node || h.To == node }

var (
	paymentEventSig    = crypto.Keccak256Hash([]byte("Payment(uint256,uint256,bytes,bytes)"))
	nullifierEventSig  = crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)"))
	commitmentEventSig = crypto.Keccak256Hash([]byte("Commitment(uint256,uint256)"))
	encryptedNoteSig   = crypto.Keccak256Hash([]byte("EncryptedNote(uint256,uint256,bytes,bytes)"))
	tagPublishedSig    = crypto.Keccak256Hash([]byte("TagPublished(uint256,bytes32,address,bytes)"))
)

// zkLogs counts the proving log lines the engine has emitted.
func (s *capturingSink) zkLogs() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, e := range s.events {
		if e.Kind == engine.Log && e.Cat == "zk" {
			n++
		}
	}
	return n
}

// runShieldScenario plays one scenario with opts and returns its engine and
// the sink that captured its events.
func runShieldScenario(t *testing.T, cfg engine.Config, opts engine.Options) (*engine.Engine, *capturingSink) {
	t.Helper()
	sink := &capturingSink{}
	e := engine.New(cfg, sink)
	t.Cleanup(e.Close)
	if err := e.Run(context.Background(), opts); err != nil {
		t.Fatalf("Run: %v (failed steps: %s)", err, sink.failures())
	}
	return e, sink
}

func TestShieldDemoScenario(t *testing.T) {
	cfg := engine.DefaultConfig("..")
	if missing := engine.Unreachable(cfg); len(missing) > 0 {
		t.Skipf("services not reachable: %s", strings.Join(missing, ", "))
	}
	// Other tests (10_usdr_relayer_fee) change the fixed fee; the demo's is 1 USDr.
	if fee, err := engine.SetUsdrFixedFee(cfg, big.NewInt(1)); err != nil || fee.Int64() != 1 {
		t.Fatalf("set the USDr fixed fee to 1: fee=%v err=%v", fee, err)
	}

	receipts := loadOnchainReceipts(t)
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)
	vaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
	usdrVaultAddr := common.HexToAddress(receipts["UsdrCoinVault"].ContractAddress)
	tagRegistryAddr := common.HexToAddress(receipts["TagRegistry"].ContractAddress)

	opts := engine.DefaultOptions()
	if !opts.UsdrFee || !opts.Tags {
		t.Fatal("DefaultOptions should pay fees in USDr and tag every transfer")
	}
	client := mustDial(t)
	defer client.Close()
	runStart, err := client.BlockNumber(context.Background())
	if err != nil {
		t.Fatalf("BlockNumber: %v", err)
	}
	e, sink := runShieldScenario(t, cfg, opts)

	t.Run("the run's totals count every transaction it put on chain", func(t *testing.T) {
		totals, err := e.Totals(context.Background())
		if err != nil {
			t.Fatalf("Totals: %v", err)
		}
		txs, gas := chainActivitySince(t, client, runStart)
		if totals.Txs != txs || totals.Gas != gas {
			t.Errorf("totals report %d txs and %d gas, the chain shows %d txs and %d gas since the run started", totals.Txs, totals.Gas, txs, gas)
		}
		// At least the fan-out and the script, each one relayed transaction.
		if min := engine.K + len(opts.Script); totals.Txs < min {
			t.Errorf("totals report %d txs, want at least %d", totals.Txs, min)
		}
		if totals.ProveMs <= 0 || totals.RelayMs <= 0 {
			t.Errorf("totals report %d ms proving and %d ms relaying, want both positive", totals.ProveMs, totals.RelayMs)
		}
	})

	t.Run("every fan-out and scripted transfer is tagged", func(t *testing.T) {
		checkTagged(t, client, tagRegistryAddr, append(e.FanOut(), e.Transfers()...))
	})
	t.Run("sender and participants are registered", func(t *testing.T) {
		registered := append([]engine.Actor{e.Sender()}, e.Participants()...)
		if len(registered) != 1+engine.K {
			t.Fatalf("got %d registrable actors, want %d", len(registered), 1+engine.K)
		}
		for _, a := range registered {
			keys, err := rpcore.LookupKeys(client, registryAddr, a.Address)
			if err != nil {
				t.Fatalf("%s: LookupKeys: %v", a.Name, err)
			}
			if keys.SpendKey.Cmp(a.Spend.PublicKey) != 0 {
				t.Errorf("%s: registered pk_spend %s, want %s", a.Name, keys.SpendKey, a.Spend.PublicKey)
			}
			if !bytes.Equal(keys.ViewKey, a.View.EncapsKey) {
				t.Errorf("%s: registered pk_view differs from the actor's view key", a.Name)
			}
			mlKemCt, aesCt, err := rpcore.LookupAuditCiphertexts(client, registryAddr, a.Address)
			if err != nil {
				t.Fatalf("%s: LookupAuditCiphertexts: %v", a.Name, err)
			}
			recovered, err := rpcore.AuditorDecryptViewKey(e.Auditor().DecapsKey, mlKemCt, aesCt)
			if err != nil {
				t.Fatalf("%s: auditor cannot open the audit ciphertexts: %v", a.Name, err)
			}
			if !bytes.Equal(recovered.EncapsulationKey().Bytes(), a.View.EncapsKey) {
				t.Errorf("%s: auditor recovered a different view key", a.Name)
			}
		}
	})

	t.Run("every hop names a known node; each registration and view-key recovery is a hop", func(t *testing.T) {
		known := map[string]bool{}
		for _, n := range engine.Nodes {
			known[n.ID] = true
		}
		hops := sink.hops()
		toRegistry := map[string]bool{}
		recovered := 0
		for _, h := range hops {
			if !known[h.From] || !known[h.To] {
				t.Errorf("hop %q (step %s) goes %q → %q, not both known nodes", h.Label, h.Step, h.From, h.To)
			}
			if h.To == engine.NodeUserRegistry {
				toRegistry[h.From] = true
			}
			if h.From == engine.NodeUserRegistry && h.To == engine.NodeAuditor {
				recovered++
			}
		}
		registered := append([]engine.Actor{e.Sender()}, e.Participants()...)
		for _, a := range registered {
			if !toRegistry[engine.ActorNode(a.ID)] {
				t.Errorf("%s's registration has no hop to the UserRegistry", a.Name)
			}
		}
		if recovered != len(registered) {
			t.Errorf("got %d view-key recovery hops from the UserRegistry to the auditor, want one per registered user (%d)", recovered, len(registered))
		}
	})

	t.Run("each fee-paying transfer has one hop from the relayer to the contract it settles on", func(t *testing.T) {
		settles := map[string]int{}
		for _, h := range sink.hops() {
			if h.From == engine.NodeRelayer && h.To == engine.NodeDvp {
				settles[h.Step]++
			}
		}
		var feeSteps []string
		for i, s := range e.FanOut() {
			if s.UsdrFeeNote != nil {
				feeSteps = append(feeSteps, fmt.Sprintf("fanout_%d", i+1))
			}
		}
		for i, s := range e.Transfers() {
			if s.UsdrFeeNote != nil {
				feeSteps = append(feeSteps, fmt.Sprintf("transfer_%d", i+1))
			}
		}
		if len(feeSteps) != engine.K+len(opts.Script) {
			t.Fatalf("got %d fee-paying transfers, want every fan-out and scripted one (%d)", len(feeSteps), engine.K+len(opts.Script))
		}
		steps := transferSteps(sink.hops())
		for _, step := range feeSteps {
			if n := settles[step]; n != 1 {
				t.Errorf("step %s: %d hops from the relayer to EnygmaDvp, want 1", step, n)
			}
			if !slices.ContainsFunc(steps[step], func(h engine.Event) bool { return h.To == engine.NodeUsdrVault }) {
				t.Errorf("step %s: no hop lands the fee leg on the USDr vault", step)
			}
		}
	})

	t.Run("tag mode opens channels and publishes each transfer's tag as hops to the tag registries", func(t *testing.T) {
		toRegistry := map[string]int{}
		for _, h := range sink.hops() {
			if h.To == engine.NodeTagChannelRegistry || h.To == engine.NodeTagRegistry {
				toRegistry[h.To]++
			}
		}
		if toRegistry[engine.NodeTagChannelRegistry] == 0 {
			t.Error("no hop opens a tag channel in the TagChannelRegistry")
		}
		if n, want := toRegistry[engine.NodeTagRegistry], engine.K+len(opts.Script); n != want {
			t.Errorf("%d hops publish a tag to the TagRegistry, want one per fan-out and scripted transfer (%d)", n, want)
		}
	})

	t.Run("the grants, the shield and each participant's scan are hops", func(t *testing.T) {
		checkGrantShieldScanHops(t, e, sink.hops())
	})

	grant := big.NewInt(engine.UsdrGrant) // 20 USDr
	t.Run("sender and every participant scan a USDr grant note from the issuer", func(t *testing.T) {
		events := usdrVaultNoteEvents(t, client, usdrVaultAddr)
		for _, a := range append([]engine.Actor{e.Sender()}, e.Participants()...) {
			owned, err := dvpcore.ScanForErc20Notes(a.View.DecapsKey, a.Spend.PublicKey, events)
			if err != nil {
				t.Fatalf("%s: scan the USDr vault: %v", a.Name, err)
			}
			if len(owned) != 1 || owned[0].Amount.Cmp(grant) != 0 {
				t.Errorf("%s: found %d USDr deposit notes (%v), want one of %s", a.Name, len(owned), owned, grant)
			}
		}
	})

	t.Run("sender shields the whole public balance", func(t *testing.T) {
		erc20 := bindContract(t, client, common.HexToAddress(receipts["ERC20"].ContractAddress), loadOnchainABI(t, "RaylsERC20"))
		var out []any
		if err := erc20.Call(&bind.CallOpts{}, &out, "balanceOf", e.Sender().Address); err != nil {
			t.Fatalf("balanceOf: %v", err)
		}
		if public := out[0].(*big.Int); public.Sign() != 0 {
			t.Errorf("sender still holds %s public RaylsERC20, want 0", public)
		}
	})

	t.Run("after the scripted transfers each participant scans the expected balances", func(t *testing.T) {
		checkAfterScript(t, e)
	})

	t.Run("each scripted transfer settles with its USDr fee through the relayer", func(t *testing.T) {
		settled := e.Transfers()
		if len(settled) != len(opts.Script) {
			t.Fatalf("got %d transfers, want the script's %d", len(settled), len(opts.Script))
		}
		for i, s := range settled {
			if s.From != opts.Script[i].From || s.To != opts.Script[i].To {
				t.Errorf("transfer %d: %s→%s, want %s→%s", i+1, s.From, s.To, opts.Script[i].From, opts.Script[i].To)
			}
			if s.UsdrFeeNote == nil {
				t.Errorf("transfer %d: paid no USDr fee note", i+1)
			}
			if n := countTopic(txLogs(t, client, s.TxHash), paymentEventSig); n != 2 {
				t.Errorf("transfer %d: %d Payment events, want 2 (main + USDr)", i+1, n)
			}
		}
	})

	t.Run("the scripted merge settles as one Payment2in transfer spending both notes", func(t *testing.T) {
		merge := checkDiegoElenaMerge(t, client, e.Transfers(), vaultAddr, true)
		o, ok := relayerFeeNoteOpenings(t)[merge.UsdrFeeNote.String()]
		if !ok || o.Amount != "1" || !o.mined {
			t.Errorf("the relayer kept no mined 1 USDr opening for the merge's fee note %s", merge.UsdrFeeNote)
		}
	})

	participant := func(name string) engine.Actor { return participantOf(t, e, name) }

	t.Run("a manual transfer settles and the recipient scans it", func(t *testing.T) {
		// After the script Carla holds notes of 150 and 20, Diego one of 10.
		if err := e.Transfer(context.Background(), "Carla", "Diego", 15); err != nil {
			t.Fatalf("Transfer Carla→Diego 15: %v", err)
		}
		if got := e.Balance(participant("Diego")); got.Cmp(big.NewInt(25)) != 0 {
			t.Errorf("Diego's balance %s after the transfer, want 25", got)
		}
		if got := e.Balance(participant("Carla")); got.Cmp(big.NewInt(155)) != 0 {
			t.Errorf("Carla's balance %s after the transfer, want 155", got)
		}
	})

	// refusedWithNothingSent checks that from paying to amount is refused with
	// no proof made and no transaction sent.
	refusedWithNothingSent := func(t *testing.T, from, to string, amount int64) {
		t.Helper()
		blockBefore, err := client.BlockNumber(context.Background())
		if err != nil {
			t.Fatalf("BlockNumber: %v", err)
		}
		zkBefore := sink.zkLogs()
		err = e.Transfer(context.Background(), from, to, amount)
		if !errors.Is(err, engine.ErrRefused) {
			t.Fatalf("Transfer %s→%s %d: got %v, want a refusal", from, to, amount, err)
		}
		blockAfter, err := client.BlockNumber(context.Background())
		if err != nil {
			t.Fatalf("BlockNumber: %v", err)
		}
		if blockAfter != blockBefore {
			t.Errorf("chain moved from block %d to %d: a transaction was sent", blockBefore, blockAfter)
		}
		if n := sink.zkLogs(); n != zkBefore {
			t.Errorf("%d proofs made for a refused transfer", n-zkBefore)
		}
	}

	t.Run("an over-balance transfer is refused with no proof and no transaction", func(t *testing.T) {
		// Ana holds 100.
		refusedWithNothingSent(t, "Ana", "Bruno", 1000)
		if got := e.Balance(participant("Ana")); got.Cmp(big.NewInt(100)) != 0 {
			t.Errorf("Ana's balance %s after the refusal, want 100 unchanged", got)
		}
	})

	t.Run("a transfer two notes can't cover is refused with no proof and no transaction", func(t *testing.T) {
		// Diego holds notes of 10 and 15; Ana's 5 makes a third, so 30 in all
		// but 25 in his two largest.
		if err := e.Transfer(context.Background(), "Ana", "Diego", 5); err != nil {
			t.Fatalf("Transfer Ana→Diego 5: %v", err)
		}
		refusedWithNothingSent(t, "Diego", "Bruno", 28)
		if got := e.Balance(participant("Diego")); got.Cmp(big.NewInt(30)) != 0 {
			t.Errorf("Diego's balance %s after the refusal, want 30 unchanged", got)
		}
	})

	t.Run("manual transfers are tagged, opening a channel where the pair has none", func(t *testing.T) {
		// Carla→Diego reuses the script's channel; Ana→Diego opened a new one.
		manual := e.Transfers()[len(opts.Script):]
		if len(manual) != 2 {
			t.Fatalf("got %d manual transfers, want 2", len(manual))
		}
		checkTagged(t, client, tagRegistryAddr, manual)
	})

	t.Run("sender keeps the shielded total minus the payouts", func(t *testing.T) {
		// DefaultOptions: 1000 shielded, 120+90+150+60+200+80 = 700 paid out.
		if got, want := e.Balance(e.Sender()), big.NewInt(300); got.Cmp(want) != 0 {
			t.Errorf("sender's private balance %s, want %s", got, want)
		}
	})

	t.Run("each fan-out step settles the payment and the USDr fee in one transaction", func(t *testing.T) {
		settled := e.FanOut()
		if len(settled) != engine.K {
			t.Fatalf("got %d fan-out transfers, want %d", len(settled), engine.K)
		}
		for i, s := range settled {
			logs := txLogs(t, client, s.TxHash)
			if n := countTopic(logs, paymentEventSig); n != 2 {
				t.Errorf("step %d: %d Payment events, want 2 (main + USDr)", i+1, n)
			}
			if n := countTopic(logs, nullifierEventSig); n != 2 {
				t.Errorf("step %d: %d Nullifier events, want 2 (main + USDr)", i+1, n)
			}
		}
	})

	t.Run("the relayer gets one 1 USDr fee note per step and keeps its opening", func(t *testing.T) {
		info := fetchRelayerInfoFor10(t)
		relayerPk, ok := new(big.Int).SetString(info.FeeSpendPubKey, 10)
		if !ok {
			t.Fatalf("relayer publishes no fee spend key: %q", info.FeeSpendPubKey)
		}
		openings := relayerFeeNoteOpenings(t)
		for i, s := range e.FanOut() {
			if s.UsdrFeeNote == nil {
				t.Fatalf("step %d: no USDr fee note", i+1)
			}
			inserted := usdrCommitmentsIn(txLogs(t, client, s.TxHash), usdrVaultAddr)
			if len(inserted) != 2 || inserted[0].Cmp(s.UsdrFeeNote) != 0 {
				t.Errorf("step %d: USDr vault inserted %v, want [fee note %s, sender's change]", i+1, inserted, s.UsdrFeeNote)
				continue
			}
			o, ok := openings[s.UsdrFeeNote.String()]
			if !ok {
				t.Errorf("step %d: relayer kept no opening for fee note %s", i+1, s.UsdrFeeNote)
				continue
			}
			if o.Amount != "1" || !o.mined {
				t.Errorf("step %d: relayer's record amount=%s mined=%v, want 1 and mined", i+1, o.Amount, o.mined)
			}
			salt, _ := new(big.Int).SetString(o.Salt, 10)
			opened, err := rpcore.Erc20CommitmentV2(relayerPk, salt, big.NewInt(1), big.NewInt(0))
			if err != nil || opened.Cmp(s.UsdrFeeNote) != 0 {
				t.Errorf("step %d: the relayer's opening does not open fee note %s", i+1, s.UsdrFeeNote)
			}
		}
	})

	t.Run("the auditor names every payer and its balances match the users' own", func(t *testing.T) {
		checkAudit(t, e, true)
	})

	t.Run("sender's USDr after the fan-out is the grant minus one fee per transfer", func(t *testing.T) {
		// 20 granted − 6 transfers × 1 USDr fee.
		if got := e.UsdrBalance(e.Sender()); got.Cmp(big.NewInt(14)) != 0 {
			t.Errorf("sender's USDr balance %s, want 14", got)
		}
	})

	t.Run("with the fee toggle off the fan-out and the transfers take the plain route", func(t *testing.T) {
		plain := engine.DefaultOptions()
		plain.UsdrFee = false
		e, psink := runShieldScenario(t, cfg, plain)
		for step, hops := range transferSteps(psink.hops()) {
			proofs := 0
			for _, h := range hops {
				if touches(h, engine.NodeUsdrVault) {
					t.Errorf("step %s: hop %q goes %s → %s with fees off", step, h.Label, h.From, h.To)
				}
				if h.To == engine.NodeGnark {
					proofs++
				}
			}
			if proofs != 1 {
				t.Errorf("step %s: %d proofs requested from the gnark server with fees off, want 1 (no UsdrFee proof)", step, proofs)
			}
		}
		for i, s := range append(e.FanOut(), e.Transfers()...) {
			if s.UsdrFeeNote != nil {
				t.Errorf("step %d: paid a USDr fee note with fees off", i+1)
			}
			logs := txLogs(t, client, s.TxHash)
			if n := countTopic(logs, paymentEventSig); n != 1 {
				t.Errorf("step %d: %d Payment events, want 1", i+1, n)
			}
			if inserted := usdrCommitmentsIn(logs, usdrVaultAddr); len(inserted) != 0 {
				t.Errorf("step %d: USDr vault inserted %d notes, want none", i+1, len(inserted))
			}
		}
		checkTagged(t, client, tagRegistryAddr, append(e.FanOut(), e.Transfers()...))
		checkDiegoElenaMerge(t, client, e.Transfers(), vaultAddr, false)
		for name, want := range map[string]int64{"Diego": 10, "Elena": 230} {
			for _, p := range e.Participants() {
				if got := e.Balance(p); p.Name == name && got.Cmp(big.NewInt(want)) != 0 {
					t.Errorf("%s's balance %s after the merge with fees off, want %d", name, got, want)
				}
			}
		}
		if got := e.UsdrBalance(e.Sender()); got.Cmp(grant) != 0 {
			t.Errorf("sender's USDr balance %s, want the untouched grant %s", got, grant)
		}
		for _, p := range e.Participants() {
			if got := e.UsdrBalance(p); got.Cmp(grant) != 0 {
				t.Errorf("%s's USDr balance %s, want the untouched grant %s", p.Name, got, grant)
			}
		}
		checkAudit(t, e, true)
	})

	t.Run("with the tag toggle off recipients find every transfer by trial decryption and payers stay hidden", func(t *testing.T) {
		untagged := engine.DefaultOptions()
		untagged.Tags = false
		ue, usink := runShieldScenario(t, cfg, untagged)
		for _, h := range usink.hops() {
			if touches(h, engine.NodeTagChannelRegistry) || touches(h, engine.NodeTagRegistry) {
				t.Errorf("hop %q (step %s) goes %s → %s with tags off", h.Label, h.Step, h.From, h.To)
			}
		}
		for _, s := range append(ue.FanOut(), ue.Transfers()...) {
			if s.TagTxHash != (common.Hash{}) {
				t.Errorf("%s→%s: published a tag with tags off", s.From, s.To)
			}
			if s.UsdrFeeNote == nil {
				t.Errorf("%s→%s: paid no USDr fee note with fees on", s.From, s.To)
			}
		}
		checkAfterScript(t, ue)
		checkGrantShieldScanHops(t, ue, usink.hops())
		if got, want := ue.Balance(ue.Sender()), big.NewInt(300); got.Cmp(want) != 0 {
			t.Errorf("sender's private balance %s, want %s", got, want)
		}
		checkAudit(t, ue, false)
	})

	t.Run("a relayer that stops mid-run fails the step with its error and ends the run", func(t *testing.T) {
		relayer, err := url.Parse(cfg.RelayerURL)
		if err != nil {
			t.Fatalf("parse relayer URL: %v", err)
		}
		// The proxy passes requests to the relayer until the first fee-paying
		// payment, then drops every connection, as a stopped relayer would.
		var stopped atomic.Bool
		forward := httputil.NewSingleHostReverseProxy(relayer)
		proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/relay/payment_usdr_fee" {
				stopped.Store(true)
			}
			if stopped.Load() {
				conn, _, err := w.(http.Hijacker).Hijack()
				if err == nil {
					conn.Close()
				}
				return
			}
			forward.ServeHTTP(w, r)
		}))
		defer proxy.Close()

		stopping := cfg
		stopping.RelayerURL = proxy.URL
		short := engine.DefaultOptions()
		short.Script = nil
		sink := &capturingSink{}
		fe := engine.New(stopping, sink)
		defer fe.Close()
		if err := fe.Run(context.Background(), short); err == nil {
			t.Fatal("Run succeeded with the relayer stopped")
		}
		if len(fe.FanOut()) != 0 {
			t.Errorf("%d fan-out transfers settled with the relayer stopped, want none", len(fe.FanOut()))
		}
		sink.mu.Lock()
		defer sink.mu.Unlock()
		failed, failedStep := false, ""
		for _, ev := range sink.events {
			switch ev.Kind {
			case engine.StepFailed:
				failed, failedStep = true, ev.Step
				if ev.Msg == "" {
					t.Errorf("step %s failed with no error message", ev.Step)
				}
			case engine.RunDone:
				t.Error("the run reported done after a failed step")
			}
		}
		if !failed {
			t.Error("no step reported the failure")
		}
		failedHop := slices.ContainsFunc(sink.events, func(ev engine.Event) bool {
			return ev.Kind == engine.Hop && ev.Failed && ev.Step == failedStep && ev.To == engine.NodeRelayer && ev.Detail != ""
		})
		if !failedHop {
			t.Errorf("step %s shows no failed hop to the relayer with its error", failedStep)
		}
		if last := sink.events[len(sink.events)-1]; last.Kind != engine.RunFailed || last.Msg == "" {
			t.Errorf("last event %+v, want the run failing with its error", last)
		}
	})
}

// TestShieldDemoScenarioPastOneTree fills both vaults with plain deposits so
// that one of the run's two-note inserts in each would take the current tree
// to its last leaf, which makes the vault start a new tree for the whole
// batch, then checks that the run's transfers settle against both trees:
// notes inserted before the new tree are spent after it.
func TestShieldDemoScenarioPastOneTree(t *testing.T) {
	cfg := engine.DefaultConfig("..")
	if missing := engine.Unreachable(cfg); len(missing) > 0 {
		t.Skipf("services not reachable: %s", strings.Join(missing, ", "))
	}
	if fee, err := engine.SetUsdrFixedFee(cfg, big.NewInt(1)); err != nil || fee.Int64() != 1 {
		t.Fatalf("set the USDr fixed fee to 1: fee=%v err=%v", fee, err)
	}
	receipts := loadOnchainReceipts(t)
	client := mustDial(t)
	defer client.Close()
	vaultABI, erc20ABI := loadOnchainABI(t, "Erc20CoinVault"), loadOnchainABI(t, "RaylsERC20")
	bindAt := func(name string, a abi.ABI) *bind.BoundContract {
		return bindContract(t, client, common.HexToAddress(receipts[name].ContractAddress), a)
	}
	vault, usdrVault := bindAt("Erc20CoinVault", vaultABI), bindAt("UsdrCoinVault", vaultABI)

	// A run inserts the sender's shield into the RaylsERC20 vault and one
	// USDr grant per registered user into the USDr vault, then each transfer
	// inserts two notes into each. The second fan-out starts the RaylsERC20
	// vault's new tree, so the first's notes (the sender's change and Ana's
	// payout) are spent from the old one; the first fan-out starts the USDr
	// vault's, so every participant's grant is spent from the old one.
	erc20Tree := fillTreeTo(t, client, vault, bindAt("ERC20", erc20ABI), receipts["Erc20CoinVault"].ContractAddress, treeLeaves-2-(1+2))
	usdrTree := fillTreeTo(t, client, usdrVault, bindAt("UsdrERC20", erc20ABI), receipts["UsdrCoinVault"].ContractAddress, treeLeaves-2-(1+engine.K))

	opts := engine.DefaultOptions()
	e, _ := runShieldScenario(t, cfg, opts)

	if got := len(e.Transfers()); got != len(opts.Script) {
		t.Errorf("%d scripted transfers settled, want %d", got, len(opts.Script))
	}
	checkAfterScript(t, e)
	checkDiegoElenaMerge(t, client, e.Transfers(), common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress), true)
	for name, v := range map[string]struct {
		vault *bind.BoundContract
		tree  int64
	}{"RaylsERC20": {vault, erc20Tree}, "USDr": {usdrVault, usdrTree}} {
		if got := vaultTreeNumber(t, v.vault); got != v.tree+1 {
			t.Errorf("%s vault is on tree %d after the run, want %d: the run did not start a new tree", name, got, v.tree+1)
		}
	}
}

// treeLeaves is the leaf count of a vault's depth-8 Merkle tree.
const treeLeaves = 1 << 8

// fillTreeTo makes plain 1-unit deposits into vault, minting and approving
// them with the token owner's key, until the vault's current tree holds
// exactly leaves leaves, and returns that tree's number. It first deposits
// until the vault starts a new tree: the deposit that starts it is that
// tree's first leaf.
func fillTreeTo(t *testing.T, client *ethclient.Client, vault, token *bind.BoundContract, vaultAddr string, leaves int) int64 {
	t.Helper()
	owner := hardhatAuth(t, client)
	transact := func(c *bind.BoundContract, method string, args ...any) {
		t.Helper()
		tx, err := c.Transact(owner, method, args...)
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		if r, err := bind.WaitMined(context.Background(), client, tx); err != nil || r.Status != types.ReceiptStatusSuccessful {
			t.Fatalf("%s: not mined (%v)", method, err)
		}
	}
	// At most a whole tree to reach the next one, then leaves-1 more.
	allowance := big.NewInt(int64(treeLeaves + leaves))
	transact(token, "mint", owner.From, allowance)
	transact(token, "approve", common.HexToAddress(vaultAddr), allowance)
	deposit := func() {
		t.Helper()
		pk, err := dvpcore.RandomInField()
		if err != nil {
			t.Fatal(err)
		}
		salt, err := dvpcore.RandomInField()
		if err != nil {
			t.Fatal(err)
		}
		transact(vault, "deposit", []*big.Int{big.NewInt(1), pk, salt, big.NewInt(0)})
	}
	start := vaultTreeNumber(t, vault)
	for vaultTreeNumber(t, vault) == start {
		deposit()
	}
	for range leaves - 1 {
		deposit()
	}
	tree := vaultTreeNumber(t, vault)
	if tree != start+1 {
		t.Fatalf("vault %s went from tree %d to %d while filling, want %d", vaultAddr, start, tree, start+1)
	}
	return tree
}

// vaultTreeNumber is the number of vault's current Merkle tree.
func vaultTreeNumber(t *testing.T, vault *bind.BoundContract) int64 {
	t.Helper()
	var out []any
	if err := vault.Call(&bind.CallOpts{}, &out, "treeNumber"); err != nil {
		t.Fatalf("treeNumber: %v", err)
	}
	return out[0].(*big.Int).Int64()
}

// chainActivitySince counts the transactions and the gas they used in the
// blocks after block from.
func chainActivitySince(t *testing.T, client *ethclient.Client, from uint64) (txs int, gas uint64) {
	t.Helper()
	head, err := client.BlockNumber(context.Background())
	if err != nil {
		t.Fatalf("BlockNumber: %v", err)
	}
	for n := from + 1; n <= head; n++ {
		b, err := client.BlockByNumber(context.Background(), new(big.Int).SetUint64(n))
		if err != nil {
			t.Fatalf("block %d: %v", n, err)
		}
		txs += len(b.Transactions())
		gas += b.GasUsed()
	}
	return txs, gas
}

// checkAfterScript checks every participant's scanned balances after
// DefaultOptions' payouts and script. They pay out Ana 120, Bruno 90, Carla
// 150, Diego 60, Elena 200, Felipe 80, then play Ana→Bruno 50, Elena→Carla
// 120, Carla→Diego 100, Bruno→Felipe 70, Felipe→Ana 30 and Diego→Elena 150
// (merging Diego's 60 and 100); each payer pays a 1 USDr fee per transfer.
func checkAfterScript(t *testing.T, e *engine.Engine) {
	t.Helper()
	want := map[string]struct{ erc20, usdr int64 }{
		"Ana": {100, 19}, "Bruno": {70, 19}, "Carla": {170, 19},
		"Diego": {10, 19}, "Elena": {230, 19}, "Felipe": {120, 19},
	}
	for _, p := range e.Participants() {
		w := want[p.Name]
		if got := e.Balance(p); got.Cmp(big.NewInt(w.erc20)) != 0 {
			t.Errorf("%s: scanned RaylsERC20 balance %s, want %d", p.Name, got, w.erc20)
		}
		if got := e.UsdrBalance(p); got.Cmp(big.NewInt(w.usdr)) != 0 {
			t.Errorf("%s: scanned USDr balance %s, want %d", p.Name, got, w.usdr)
		}
	}
}

// checkAudit checks the auditor's view of e's run: it recovers every
// registered user's view key and lists the shield, each USDr grant, every
// transfer and every USDr fee leg with its amount and parties. With tags the
// auditor names each transfer's payer and reconstructs every user's balances,
// which must equal the users' own scans; without tags payers stay hidden and
// it reconstructs none.
func checkAudit(t *testing.T, e *engine.Engine, tagged bool) {
	t.Helper()
	a, err := e.Audit(context.Background())
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	registered := append([]engine.Actor{e.Sender()}, e.Participants()...)
	if len(a.Users) != len(registered) {
		t.Errorf("auditor recovered the view keys of %v, want the sender and %d participants", a.Users, engine.K)
	}
	for _, u := range registered {
		if !slices.Contains(a.Users, u.Name) {
			t.Errorf("auditor did not recover %s's view key", u.Name)
		}
	}

	find := func(kind engine.AuditKind, match func(engine.AuditEntry) bool) *engine.AuditEntry {
		for i, en := range a.Entries {
			if en.Kind == kind && match(en) {
				return &a.Entries[i]
			}
		}
		return nil
	}
	check := func(what string, en *engine.AuditEntry, from, to string, amount int64) {
		t.Helper()
		if en == nil {
			t.Errorf("%s: missing from the audit", what)
			return
		}
		if en.From != from || en.To != to || en.Amount.Cmp(big.NewInt(amount)) != 0 {
			t.Errorf("%s: audited %q → %q %s, want %q → %q %d", what, en.From, en.To, en.Amount, from, to, amount)
		}
	}
	want := 0
	check("shield", find(engine.AuditShield, func(engine.AuditEntry) bool { return true }), "Sender", "Sender", 1000)
	want++
	for _, u := range registered {
		check(u.Name+"'s USDr grant", find(engine.AuditUsdrGrant, func(en engine.AuditEntry) bool { return en.To == u.Name }), "Issuer", u.Name, engine.UsdrGrant)
		want++
	}
	for _, s := range append(e.FanOut(), e.Transfers()...) {
		from := ""
		if tagged {
			from = s.From
		}
		what := fmt.Sprintf("%s→%s %s", s.From, s.To, s.Amount)
		inTx := func(en engine.AuditEntry) bool { return en.TxHash == s.TxHash }
		check(what, find(engine.AuditTransfer, inTx), from, s.To, s.Amount.Int64())
		want++
		if s.UsdrFeeNote != nil {
			check(what+" USDr fee", find(engine.AuditUsdrFee, inTx), from, "Relayer", 1)
			want++
		}
	}
	if len(a.Entries) != want {
		t.Errorf("audit lists %d transactions, want %d", len(a.Entries), want)
	}

	if !tagged {
		if a.Balances != nil || a.UsdrBalances != nil {
			t.Errorf("auditor reconstructed balances without knowing who paid")
		}
		return
	}
	for _, u := range registered {
		if got, own := a.Balances[u.Name], e.Balance(u); got == nil || got.Cmp(own) != 0 {
			t.Errorf("%s: auditor's RaylsERC20 balance %v, the user's own scan %s", u.Name, got, own)
		}
		if got, own := a.UsdrBalances[u.Name], e.UsdrBalance(u); got == nil || got.Cmp(own) != 0 {
			t.Errorf("%s: auditor's USDr balance %v, the user's own scan %s", u.Name, got, own)
		}
	}
}

// checkTagged checks that each transfer's tag was published to the
// TagRegistry in its own transaction.
func checkTagged(t *testing.T, client *ethclient.Client, tagRegistry common.Address, settled []engine.Settlement) {
	t.Helper()
	for _, s := range settled {
		if s.TagTxHash == (common.Hash{}) {
			t.Errorf("%s→%s %s: published no tag", s.From, s.To, s.Amount)
			continue
		}
		if n := countTopicAt(txLogs(t, client, s.TagTxHash), tagRegistry, tagPublishedSig); n != 1 {
			t.Errorf("%s→%s %s: tag transaction published %d tags, want 1", s.From, s.To, s.Amount, n)
		}
	}
}

// participantOf returns e's participant named name.
func participantOf(t *testing.T, e *engine.Engine, name string) engine.Actor {
	t.Helper()
	for _, p := range e.Participants() {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no participant %s", name)
	return engine.Actor{}
}

// checkDiegoElenaMerge checks that the default script's Diego→Elena 150, bigger than
// either of Diego's notes, settled as one transaction that spent both notes
// (two nullifiers in the vault, plus the USDr fee note's with fees on) and
// inserted the payment and Diego's change. It returns the merge.
func checkDiegoElenaMerge(t *testing.T, client *ethclient.Client, settled []engine.Settlement, vault common.Address, usdrFee bool) engine.Settlement {
	t.Helper()
	for _, s := range settled {
		if s.From != "Diego" || s.To != "Elena" {
			continue
		}
		logs := txLogs(t, client, s.TxHash)
		if n := countTopicAt(logs, vault, nullifierEventSig); n != 2 {
			t.Errorf("merge: %d nullifiers spent in the vault, want 2 (both of Diego's notes)", n)
		}
		if n := countTopicAt(logs, vault, commitmentEventSig); n != 2 {
			t.Errorf("merge: %d notes inserted in the vault, want 2 (Elena's and Diego's change)", n)
		}
		wantNullifiers := 2
		if usdrFee {
			wantNullifiers = 3
		}
		if n := countTopic(logs, nullifierEventSig); n != wantNullifiers {
			t.Errorf("merge: %d nullifiers spent, want %d", n, wantNullifiers)
		}
		if (s.UsdrFeeNote != nil) != usdrFee {
			t.Fatalf("merge: paid a USDr fee note = %v, want %v", s.UsdrFeeNote != nil, usdrFee)
		}
		return s
	}
	t.Fatal("no Diego→Elena transfer settled")
	return engine.Settlement{}
}

func txLogs(t *testing.T, client *ethclient.Client, tx common.Hash) []*types.Log {
	t.Helper()
	r, err := client.TransactionReceipt(context.Background(), tx)
	if err != nil {
		t.Fatalf("receipt %s: %v", tx.Hex(), err)
	}
	if r.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("tx %s reverted", tx.Hex())
	}
	return r.Logs
}

func countTopic(logs []*types.Log, topic common.Hash) int {
	n := 0
	for _, l := range logs {
		if len(l.Topics) > 0 && l.Topics[0] == topic {
			n++
		}
	}
	return n
}

// countTopicAt counts the logs addr emitted with topic.
func countTopicAt(logs []*types.Log, addr common.Address, topic common.Hash) int {
	n := 0
	for _, l := range logs {
		if l.Address == addr && len(l.Topics) > 0 && l.Topics[0] == topic {
			n++
		}
	}
	return n
}

// usdrCommitmentsIn returns the commitments the USDr vault inserted, in order.
func usdrCommitmentsIn(logs []*types.Log, usdrVault common.Address) []*big.Int {
	var out []*big.Int
	for _, l := range logs {
		if l.Address == usdrVault && len(l.Topics) >= 3 && l.Topics[0] == commitmentEventSig {
			out = append(out, l.Topics[2].Big())
		}
	}
	return out
}

// usdrVaultNoteEvents returns the USDr vault's EncryptedNote (deposit) events.
func usdrVaultNoteEvents(t *testing.T, client *ethclient.Client, usdrVault common.Address) []dvpcore.OnChainErc20Event {
	t.Helper()
	logs, err := client.FilterLogs(context.Background(), ethereum.FilterQuery{
		Addresses: []common.Address{usdrVault},
		Topics:    [][]common.Hash{{encryptedNoteSig}},
	})
	if err != nil {
		t.Fatalf("USDr vault EncryptedNote events: %v", err)
	}
	bytesT, _ := abi.NewType("bytes", "", nil)
	args := abi.Arguments{{Type: bytesT}, {Type: bytesT}}
	var events []dvpcore.OnChainErc20Event
	for _, l := range logs {
		vals, err := args.Unpack(l.Data)
		if err != nil || len(l.Topics) < 3 {
			t.Fatalf("decode EncryptedNote: %v", err)
		}
		events = append(events, dvpcore.OnChainErc20Event{
			Commitment: l.Topics[2].Big(),
			CipherText: vals[0].([]byte),
			EncTxData:  vals[1].([]byte),
		})
	}
	return events
}

// feeNoteOpening is what the relayer's fee-note store says about one note.
type feeNoteOpening struct {
	Salt, Amount string
	mined        bool
}

// relayerFeeNoteOpenings reads the relayer's fee-note store, by commitment.
func relayerFeeNoteOpenings(t *testing.T) map[string]*feeNoteOpening {
	t.Helper()
	path := os.Getenv("RELAYER_FEE_NOTES_PATH")
	if path == "" {
		path = "../build/relayer_fee_notes.jsonl"
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open the relayer's fee-note store: %v", err)
	}
	defer f.Close()
	out := map[string]*feeNoteOpening{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r struct{ Status, Commitment, Salt, Amount string }
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("parse fee-note record %q: %v", sc.Text(), err)
		}
		o := out[r.Commitment]
		if o == nil {
			o = &feeNoteOpening{}
			out[r.Commitment] = o
		}
		switch r.Status {
		case "submitted":
			o.Salt, o.Amount = r.Salt, r.Amount
		case "mined":
			o.mined = true
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read the relayer's fee-note store: %v", err)
	}
	return out
}
