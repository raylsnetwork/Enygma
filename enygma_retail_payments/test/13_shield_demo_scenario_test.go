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

	opts := engine.DefaultOptions()
	if !opts.UsdrFee {
		t.Fatal("DefaultOptions should pay fees in USDr")
	}
	e, sink := runShieldScenario(t, cfg, opts)

	client := mustDial(t)
	defer client.Close()
	receipts := loadOnchainReceipts(t)
	registryAddr := common.HexToAddress(receipts["UserRegistry"].ContractAddress)
	vaultAddr := common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress)
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

	t.Run("after the scripted transfers each participant scans the expected balances", func(t *testing.T) {
		// DefaultOptions pays out Ana 120, Bruno 90, Carla 150, Diego 60,
		// Elena 200, Felipe 80, then plays Ana→Bruno 50, Elena→Carla 120,
		// Carla→Diego 100, Bruno→Felipe 70, Felipe→Ana 30 and Diego→Elena 150
		// (merging Diego's 60 and 100); each payer pays a 1 USDr fee per
		// transfer.
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

	participant := func(name string) engine.Actor {
		for _, p := range e.Participants() {
			if p.Name == name {
				return p
			}
		}
		t.Fatalf("no participant %s", name)
		return engine.Actor{}
	}

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

	t.Run("with the fee toggle off the fan-out and the transfers take the plain route", func(t *testing.T) {
		plain := engine.DefaultOptions()
		plain.UsdrFee = false
		e, _ := runShieldScenario(t, cfg, plain)
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
	})
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
