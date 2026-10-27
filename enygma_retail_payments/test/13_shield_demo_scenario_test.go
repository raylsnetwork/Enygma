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
	"math/big"
	"os"
	"strings"
	"sync"
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

var (
	paymentEventSig    = crypto.Keccak256Hash([]byte("Payment(uint256,uint256,bytes,bytes)"))
	nullifierEventSig  = crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)"))
	commitmentEventSig = crypto.Keccak256Hash([]byte("Commitment(uint256,uint256)"))
	encryptedNoteSig   = crypto.Keccak256Hash([]byte("EncryptedNote(uint256,uint256,bytes,bytes)"))
)

// runShieldScenario plays one scenario with opts and returns its engine.
func runShieldScenario(t *testing.T, cfg engine.Config, opts engine.Options) *engine.Engine {
	t.Helper()
	sink := &capturingSink{}
	e := engine.New(cfg, sink)
	t.Cleanup(e.Close)
	if err := e.Run(context.Background(), opts); err != nil {
		t.Fatalf("Run: %v (failed steps: %s)", err, sink.failures())
	}
	return e
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

	opts := engine.DefaultOptions()
	if !opts.UsdrFee {
		t.Fatal("DefaultOptions should pay fees in USDr")
	}
	e := runShieldScenario(t, cfg, opts)

	client := mustDial(t)
	defer client.Close()
	receipts := loadOnchainReceipts(t)
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)
	usdrVaultAddr := common.HexToAddress(receipts["UsdrCoinVault"].ContractAddress)

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

	t.Run("each participant scans the amount the sender assigned", func(t *testing.T) {
		for i, p := range e.Participants() {
			if got, want := e.Balance(p), big.NewInt(opts.Payouts[i]); got.Cmp(want) != 0 {
				t.Errorf("%s: scanned balance %s, want %s", p.Name, got, want)
			}
		}
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

	t.Run("sender's USDr after the fan-out is the grant minus one fee per transfer", func(t *testing.T) {
		// 20 granted − 6 transfers × 1 USDr fee.
		if got := e.UsdrBalance(e.Sender()); got.Cmp(big.NewInt(14)) != 0 {
			t.Errorf("sender's USDr balance %s, want 14", got)
		}
	})

	t.Run("with the fee toggle off the fan-out takes the plain route", func(t *testing.T) {
		plain := engine.DefaultOptions()
		plain.UsdrFee = false
		e := runShieldScenario(t, cfg, plain)
		for i, s := range e.FanOut() {
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
		if got := e.UsdrBalance(e.Sender()); got.Cmp(grant) != 0 {
			t.Errorf("sender's USDr balance %s, want the untouched grant %s", got, grant)
		}
	})
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
