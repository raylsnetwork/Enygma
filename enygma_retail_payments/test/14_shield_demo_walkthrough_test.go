package tests

// TestShieldDemoWalkthrough drives the shield demo's engine through a
// walkthrough against the live stack, releasing its gate one action at a
// time as the page's Next does, with a sink that captures its events. It
// checks the actions the gate releases, and what the chain, the actors'
// scans and the auditor show.
//
// Prerequisites: the stack shield_demo/run.sh --no-demo brings up (see
// TestShieldDemoScenario).
//
// Run:
//
//	cd test && CC=/usr/bin/clang go test -run TestShieldDemoWalkthrough -v -timeout 1800s

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"enygma_retail_payments/shield_demo/engine"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// snapshot is every event the sink has captured so far.
func (s *capturingSink) snapshot() []engine.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]engine.Event(nil), s.events...)
}

// released is one action a walkthrough's gate released: the event that
// announced it and the hops that followed, up to the next announcement or
// the run's end.
type released struct {
	at   engine.Event
	hops []engine.Event
}

// actionHops are the hop labels each walkthrough action plays, by action.
// A person's keys fund their gas in the same click, except the issuer's,
// whose account is the gas funder.
var actionHops = map[string][]string{
	"auditor":  {"auditor key"},
	"keys":     {"spend key", "view key", "fund gas"},
	"register": {"audit ciphertexts", "register"},
	"recover":  slices.Repeat([]string{"recover view key"}, 1+engine.K),
	"shield":   {"shield"},
	"prove":    {"prove Payment"},
	"relay":    {"relay", "payment", "notes in"},
	"payment":  {"prove Payment", "relay", "payment", "notes in"},
	"scan":     {"scan"},
	"audit":    {"audit"},
}

// walkthroughOpts are the default options with the walkthrough on.
func walkthroughOpts() engine.Options {
	opts := engine.DefaultOptions()
	opts.Walkthrough = true
	return opts
}

// walk plays a walkthrough with opts, releasing each action once the engine
// waits for it. Before each release it checks that no hop arrives while
// the gate is closed. It returns the engine, the sink and the actions in
// the order released.
func walk(t *testing.T, cfg engine.Config, opts engine.Options) (*engine.Engine, *capturingSink, []released) {
	t.Helper()
	sink := &capturingSink{}
	e := engine.New(cfg, sink)
	t.Cleanup(e.Close)
	errc := make(chan error, 1)
	go func() { errc <- e.Run(context.Background(), opts) }()

	var acts []released
	seen := 0 // events already read
	var runErr error
	ended := false
	for !ended {
		// Wait for the next announcement, or the run's end.
		deadline := time.Now().Add(5 * time.Minute)
		var at *engine.Event
		for at == nil && !ended {
			select {
			case runErr = <-errc:
				ended = true
			case <-time.After(20 * time.Millisecond):
			}
			evs := sink.snapshot()
			for ; seen < len(evs); seen++ {
				ev := evs[seen]
				if ev.Kind == engine.Hop && len(acts) > 0 {
					acts[len(acts)-1].hops = append(acts[len(acts)-1].hops, ev)
				}
				if ev.Kind == engine.Hop && len(acts) == 0 {
					t.Errorf("hop %q (step %s) before the first release", ev.Label, ev.Step)
				}
				if ev.Kind == engine.ActionWaiting {
					at = &evs[seen]
					seen++
					break
				}
			}
			if at == nil && time.Now().After(deadline) {
				t.Fatalf("no action announced within 5 minutes after %d releases (failed steps: %s)", len(acts), sink.failures())
			}
		}
		if at == nil {
			break
		}
		// The gate is closed: nothing happens until it is released.
		time.Sleep(300 * time.Millisecond)
		if evs := sink.snapshot(); len(evs) > seen {
			for _, ev := range evs[seen:] {
				if ev.Kind == engine.Hop {
					t.Errorf("hop %q (step %s) arrived while the gate was closed before %s/%s", ev.Label, ev.Step, at.Step, at.Action)
				}
			}
		}
		acts = append(acts, released{at: *at})
		if !e.Gate().Release() {
			t.Fatalf("the gate was not waiting after announcing %s/%s", at.Step, at.Action)
		}
	}
	if runErr != nil {
		t.Fatalf("Run: %v (failed steps: %s)", runErr, sink.failures())
	}
	return e, sink, acts
}

func TestShieldDemoWalkthrough(t *testing.T) {
	cfg := engine.DefaultConfig("..")
	if missing := engine.Unreachable(cfg); len(missing) > 0 {
		t.Skipf("services not reachable: %s", strings.Join(missing, ", "))
	}
	receipts := loadOnchainReceipts(t)
	usdrVaultAddr := common.HexToAddress(receipts["UsdrCoinVault"].ContractAddress)
	client := mustDial(t)
	defer client.Close()

	t.Run("a walkthrough whose presenter leaves at its first gate ends with an error and sends nothing", func(t *testing.T) {
		sink := &capturingSink{}
		e := engine.New(cfg, sink)
		t.Cleanup(e.Close)
		ctx, cancel := context.WithCancel(context.Background())
		errc := make(chan error, 1)
		go func() { errc <- e.Run(ctx, walkthroughOpts()) }()
		for deadline := time.Now().Add(time.Minute); ; time.Sleep(20 * time.Millisecond) {
			if slices.ContainsFunc(sink.snapshot(), func(ev engine.Event) bool { return ev.Kind == engine.ActionWaiting }) {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("no action announced within a minute")
			}
		}
		cancel()
		if err := <-errc; !errors.Is(err, context.Canceled) {
			t.Errorf("Run after its context ended: %v, want context.Canceled", err)
		}
		if hops := sink.hops(); len(hops) != 0 {
			t.Errorf("%d hops from a walkthrough never released, want none", len(hops))
		}
		if !slices.ContainsFunc(sink.snapshot(), func(ev engine.Event) bool { return ev.Kind == engine.RunFailed }) {
			t.Error("no failed event: the page would not end the walkthrough")
		}
	})

	opts := walkthroughOpts()
	e, sink, acts := walk(t, cfg, opts)
	t.Logf("a walkthrough takes %d releases (the page's Nexts, before concept moments)", len(acts))

	t.Run("the steps are the Sender's path, in order", func(t *testing.T) {
		var started []string
		for _, ev := range sink.snapshot() {
			if ev.Kind == engine.StepStarted {
				started = append(started, ev.Step)
			}
		}
		want := []string{"keygen", "register", "audit_keys", "shield",
			"fanout_1", "fanout_2", "fanout_3", "fanout_4", "fanout_5", "fanout_6", "discover", "audit"}
		if !slices.Equal(started, want) {
			t.Errorf("steps started %v, want %v", started, want)
		}
		if !slices.Equal(engine.StepIDs(opts), want) {
			t.Errorf("StepIDs %v, want %v", engine.StepIDs(opts), want)
		}
	})

	t.Run("each release runs exactly one action, in its step", func(t *testing.T) {
		declared := engine.StepActions(opts)
		for i, a := range acts {
			if !slices.Contains(declared[a.at.Step], a.at.Action) {
				t.Errorf("release %d: action %q of step %s, which StepActions doesn't declare (%v)", i+1, a.at.Action, a.at.Step, declared[a.at.Step])
			}
			want := actionHops[a.at.Action]
			if a.at.Action == "keys" && a.at.Value == "Issuer" {
				want = want[:2]
			}
			var got []string
			for _, h := range a.hops {
				got = append(got, h.Label)
				if h.Step != a.at.Step {
					t.Errorf("release %d (%s/%s): hop %q is step %s's", i+1, a.at.Step, a.at.Action, h.Label, h.Step)
				}
			}
			if !slices.Equal(got, want) {
				t.Errorf("release %d (%s/%s %s): hops %q, want %q", i+1, a.at.Step, a.at.Action, a.at.Value, got, want)
			}
		}
		// One per person for keys, registration and scans; one per payment
		// after the first; the first payment's proof and relay apart.
		count := map[string]int{}
		for _, a := range acts {
			count[a.at.Action]++
		}
		want := map[string]int{"auditor": 1, "keys": 2 + engine.K, "register": 1 + engine.K, "recover": 1, "shield": 1,
			"prove": 1, "relay": 1, "payment": engine.K - 1, "scan": engine.K, "audit": 1}
		for action, n := range want {
			if count[action] != n {
				t.Errorf("%d releases of %q, want %d", count[action], action, n)
			}
		}
	})

	t.Run("a person's action is about them", func(t *testing.T) {
		node := map[string]string{"Issuer": engine.ActorNode(e.Issuer().ID), "Sender": engine.ActorNode(e.Sender().ID)}
		for _, p := range e.Participants() {
			node[p.Name] = engine.ActorNode(p.ID)
		}
		for _, a := range acts {
			if a.at.Action != "keys" && a.at.Action != "register" && a.at.Action != "scan" {
				continue
			}
			n, ok := node[a.at.Value]
			if !ok {
				t.Errorf("%s/%s about %q, who is no one in the run", a.at.Step, a.at.Action, a.at.Value)
				continue
			}
			for _, h := range a.hops {
				if !touches(h, n) {
					t.Errorf("%s/%s about %s: hop %q goes %s → %s", a.at.Step, a.at.Action, a.at.Value, h.Label, h.From, h.To)
				}
			}
		}
	})

	t.Run("no USDr fee and no tags", func(t *testing.T) {
		for _, h := range sink.hops() {
			if touches(h, engine.NodeUsdrVault) || touches(h, engine.NodeTagRegistry) || touches(h, engine.NodeTagChannelRegistry) {
				t.Errorf("hop %q (step %s) goes %s → %s", h.Label, h.Step, h.From, h.To)
			}
		}
		if n := len(e.Transfers()); n != 0 {
			t.Errorf("%d transfers between participants, want none", n)
		}
		fanOut := e.FanOut()
		if len(fanOut) != engine.K {
			t.Fatalf("%d fan-out payments, want %d", len(fanOut), engine.K)
		}
		for i, s := range fanOut {
			if s.UsdrFeeNote != nil || s.TagTxHash != (common.Hash{}) {
				t.Errorf("payment %d to %s paid a USDr fee or published a tag", i+1, s.To)
			}
			logs := txLogs(t, client, s.TxHash)
			if n := countTopic(logs, paymentEventSig); n != 1 {
				t.Errorf("payment %d: %d Payment events, want exactly one proof's", i+1, n)
			}
			if inserted := usdrCommitmentsIn(logs, usdrVaultAddr); len(inserted) != 0 {
				t.Errorf("payment %d: USDr vault inserted %d notes, want none", i+1, len(inserted))
			}
		}
	})

	t.Run("every participant discovers exactly their payout", func(t *testing.T) {
		paid := new(big.Int)
		for i, p := range e.Participants() {
			want := big.NewInt(opts.Payouts[i])
			paid.Add(paid, want)
			if got := e.Balance(p); got.Cmp(want) != 0 {
				t.Errorf("%s scanned %s RaylsERC20, was paid %s", p.Name, got, want)
			}
		}
		if got, want := e.Balance(e.Sender()), new(big.Int).Sub(big.NewInt(opts.ShieldTotal), paid); got.Cmp(want) != 0 {
			t.Errorf("Sender keeps %s, want the shielded %d less the payouts, %s", got, opts.ShieldTotal, want)
		}
	})

	t.Run("payment 1's proof checks true on the Verifier the vault uses, and false with a tampered output commitment", func(t *testing.T) {
		var call *engine.VerifyCall
		var sentIn string // the release the call arrived in
		for _, ev := range sink.snapshot() {
			if ev.Kind == engine.ActionWaiting {
				sentIn = ev.Step + "/" + ev.Action
			}
			if ev.Kind == engine.ProtocolField && ev.Field == engine.VerifyProofField {
				if call != nil {
					t.Fatal("more than one verifyProof field: only payment 1's is checked")
				}
				call = &engine.VerifyCall{}
				if err := json.Unmarshal([]byte(ev.Value), call); err != nil {
					t.Fatalf("verifyProof field: %v", err)
				}
				if sentIn != "fanout_1/relay" {
					t.Errorf("verifyProof field sent in release %s, want once payment 1 settles (fanout_1/relay)", sentIn)
				}
			}
		}
		if call == nil {
			t.Fatal("the walkthrough sent no verifyProof field")
		}

		v, err := engine.DeployedVerifier(cfg)
		if err != nil {
			t.Fatalf("DeployedVerifier: %v", err)
		}
		if v.Address != common.HexToAddress(receipts["Verifier"].ContractAddress).Hex() || v.RPCURL != cfg.RPCURL {
			t.Errorf("DeployedVerifier %+v, want the receipts' Verifier at %s", v, cfg.RPCURL)
		}
		vault := bindContract(t, client, common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress), loadOnchainABI(t, "Erc20CoinVault"))
		var slot []any
		if err := vault.Call(nil, &slot, "VK_ID_ERC20_JOINSPLIT"); err != nil {
			t.Fatalf("VK_ID_ERC20_JOINSPLIT: %v", err)
		}
		if got := slot[0].(*big.Int); got.Int64() != int64(v.PaymentVK) || call.VK != v.PaymentVK {
			t.Errorf("the vault checks Payment proofs against slot %s; /defaults says %d, the call uses %d", got, v.PaymentVK, call.VK)
		}

		// The call's inputs are the ones payment 1 was proved with.
		var proved []engine.PublicInput
		for _, h := range sink.hops() {
			if h.Step == "fanout_1" && h.Label == "prove Payment" {
				for _, d := range detailsOf(h, engine.DetailPublicInputs) {
					proved = d.Inputs
				}
			}
		}
		if len(proved) == 0 || !slices.Equal(call.Inputs, proved) {
			t.Errorf("call inputs %v, want payment 1's public inputs %v", call.Inputs, proved)
		}
		if tm := call.Tamper; tm.Name != "CommitmentsOut[0]" || tm.From != proved[tm.Index].Value || tm.To == tm.From {
			t.Errorf("tamper %+v, want CommitmentsOut[0] changed from payment 1's %s", tm, proved[tm.Index].Value)
		}

		// The data encodes the slot, the proof and the inputs the page shows.
		verifierABI := loadOnchainABI(t, "Verifier")
		args, err := verifierABI.Methods["verifyProof"].Inputs.Unpack(hexutil.MustDecode(call.Data)[4:])
		if err != nil {
			t.Fatalf("unpack the call's data: %v", err)
		}
		packed, _ := json.Marshal(args[1])
		var proof struct {
			A struct{ X, Y *big.Int }
			B struct{ X, Y [2]*big.Int }
			C struct{ X, Y *big.Int }
		}
		if err := json.Unmarshal(packed, &proof); err != nil {
			t.Fatalf("the call's proof: %v", err)
		}
		gotProof := []*big.Int{proof.A.X, proof.A.Y, proof.B.X[0], proof.B.X[1], proof.B.Y[0], proof.B.Y[1], proof.C.X, proof.C.Y}
		for i, p := range gotProof {
			if p.Cmp(hexField(t, call.Proof[i])) != 0 {
				t.Errorf("the call's data has proof[%d] %s, the page shows %s", i, p, call.Proof[i])
			}
		}
		if slot := args[0].(*big.Int); slot.Int64() != int64(call.VK) {
			t.Errorf("the call's data uses slot %s, the page shows %d", slot, call.VK)
		}
		gotInputs := args[2].([]*big.Int)
		if len(gotInputs) != len(call.Inputs) {
			t.Fatalf("the call's data has %d inputs, the page shows %d", len(gotInputs), len(call.Inputs))
		}
		for i, in := range call.Inputs {
			if gotInputs[i].Cmp(hexField(t, in.Value)) != 0 {
				t.Errorf("the call's data has input %d %s, the page shows %s", i, gotInputs[i], in.Value)
			}
		}

		// The page's (and curl's) JSON-RPC request, byte for byte as the
		// page's JSON.stringify writes it.
		ethCall := func(data string) string {
			body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"eth_call","params":[{"to":"%s","data":"%s"},"latest"]}`, v.Address, data)
			res, err := http.Post(v.RPCURL, "application/json", strings.NewReader(body))
			if err != nil {
				t.Fatalf("eth_call: %v", err)
			}
			defer res.Body.Close()
			var out struct {
				Result string          `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if err := json.NewDecoder(res.Body).Decode(&out); err != nil || out.Error != nil {
				t.Fatalf("eth_call: %v %s", err, out.Error)
			}
			return out.Result
		}
		const (
			isTrue  = "0x0000000000000000000000000000000000000000000000000000000000000001"
			isFalse = "0x0000000000000000000000000000000000000000000000000000000000000000"
		)
		if got := ethCall(call.Data); got != isTrue {
			t.Errorf("verifyProof(payment 1) = %s, want true", got)
		}
		if got := ethCall(call.Tamper.Data); got != isFalse {
			t.Errorf("verifyProof(payment 1, tampered) = %s, want false", got)
		}
	})

	t.Run("the audit reconciles every balance", func(t *testing.T) {
		a, err := e.Audit(context.Background())
		if err != nil {
			t.Fatalf("Audit: %v", err)
		}
		if !a.Inferred {
			t.Error("the walkthrough's audit names payers it has no tags for")
		}
		if want := 1 + engine.K; len(a.Entries) != want {
			t.Errorf("audit lists %d transactions, want the shield and %d payments", len(a.Entries), engine.K)
		}
		for _, u := range append([]engine.Actor{e.Sender()}, e.Participants()...) {
			if got, own := a.Balances[u.Name], e.Balance(u); got == nil || got.Cmp(own) != 0 {
				t.Errorf("%s: auditor's balance %v, the user's own scan %s", u.Name, got, own)
			}
		}
		if !slices.ContainsFunc(sink.snapshot(), func(ev engine.Event) bool { return ev.Kind == engine.RunDone }) {
			t.Error("the walkthrough did not finish")
		}
	})
}
