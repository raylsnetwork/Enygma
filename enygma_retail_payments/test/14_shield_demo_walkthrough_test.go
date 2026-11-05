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
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/iden3/go-iden3-crypto/poseidon"
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
	"proof":    {"proof"},
	"relay":    {"relay"},
	"settle":   {"payment", "notes in"},
	"payment":  {"prove Payment", "relay", "payment", "notes in"},
	"replay":   {"replay payment 1"},
	"scan":     {"scan"},
	"audit":    {"audit"},
}

// walkthroughOpts are the options the page's Walkthrough button sends with
// the form as /defaults fills it: the default amounts, no USDr fee, no tags
// and the walkthrough on (the server keeps the default script, which a
// walkthrough does not play).
func walkthroughOpts() engine.Options {
	opts := engine.DefaultOptions()
	opts.UsdrFee, opts.Tags, opts.Walkthrough = false, false, true
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

	// The walkthrough's replay fixture: /defaults as the page got it (with
	// the Verifier its on-chain proof check calls), the options it ran with,
	// and the node's answer to each verifyProof eth_call the check makes,
	// which the on-chain check subtest fills in.
	verifier, err := engine.DeployedVerifier(cfg)
	if err != nil {
		t.Fatalf("DeployedVerifier: %v", err)
	}
	ethCalls := map[string]string{} // call data → the node's result
	sink.writeReplayFixtureIfPassed(t, walkthroughFixture,
		fixtureField{"defaults", struct {
			engine.Options
			Chain engine.Verifier `json:"chain"`
		}{engine.DefaultOptions(), verifier}},
		fixtureField{"run", opts},
		fixtureField{"ethCall", ethCalls})

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
		// after the first; the first payment's four hops apart.
		count := map[string]int{}
		for _, a := range acts {
			count[a.at.Action]++
		}
		want := map[string]int{"auditor": 1, "keys": 2 + engine.K, "register": 1 + engine.K, "recover": 1, "shield": 1,
			"prove": 1, "proof": 1, "relay": 1, "settle": 1, "payment": engine.K - 1, "replay": 1, "scan": engine.K, "audit": 1}
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
			if inserted := commitmentsIn(logs, usdrVaultAddr); len(inserted) != 0 {
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

	t.Run("keygen shows every key pair, the auditor's too, registration what the UserRegistry got, and each recovery the view key the auditor opened", func(t *testing.T) {
		checkKeyDetails(t, e, sink.hops(), true)
		checkRegistryDetails(t, client, e, sink.hops())
	})

	t.Run("the Sender's registration carries the spend key formula: Poseidon of their sk_spend, the pk_spend the UserRegistry holds for them", func(t *testing.T) {
		sender := e.Sender()
		var reg []engine.Event
		for _, h := range sink.hops() {
			if h.Step == "register" && h.Label == "register" && h.From == engine.ActorNode(sender.ID) && h.To == engine.NodeUserRegistry && !h.Failed {
				reg = append(reg, h)
			}
		}
		if len(reg) != 1 {
			t.Fatalf("%d registration hops for the Sender, want 1", len(reg))
		}
		fs := detailsOf(reg[0], engine.DetailFormula)
		if len(fs) != 1 || fs[0].Formula == nil {
			t.Fatalf("the Sender's registration carries %d formula details, want one", len(fs))
		}
		if fs[0].Vault != engine.NodeUserRegistry {
			t.Errorf("the spend key formula is in the %s, want the %s", fs[0].Vault, engine.NodeUserRegistry)
		}
		f := fs[0].Formula
		if f.Chain == nil {
			t.Fatal("the spend key formula carries no chain value: a full run's, by name only")
		}
		if f.Name != engine.FormulaSpendKey {
			t.Errorf("formula %q, want %q", f.Name, engine.FormulaSpendKey)
		}
		if len(f.Inputs) != 1 || f.Inputs[0].Name != "sk_spend" {
			t.Fatalf("inputs %+v, want sk_spend alone", f.Inputs)
		}
		sk := hexField(t, f.Inputs[0].Value)
		if sk.Cmp(sender.Spend.PrivateKey) != 0 {
			t.Errorf("sk_spend 0x%x, want the Sender's 0x%x", sk, sender.Spend.PrivateKey)
		}
		// Recomputed independently of the engine, with the client library's Poseidon.
		want, err := poseidon.Hash([]*big.Int{sk})
		if err != nil {
			t.Fatalf("poseidon.Hash: %v", err)
		}
		if got := hexField(t, f.Result); got.Cmp(want) != 0 {
			t.Errorf("result 0x%x, Poseidon(sk_spend) is 0x%x", got, want)
		}
		keys, err := rpcore.LookupKeys(client, common.HexToAddress(receipts["UserRegistry"].ContractAddress), sender.Address)
		if err != nil {
			t.Fatalf("LookupKeys: %v", err)
		}
		if keys.SpendKey.Cmp(want) != 0 {
			t.Fatalf("the UserRegistry holds pk_spend 0x%x for the Sender, Poseidon(sk_spend) is 0x%x", keys.SpendKey, want)
		}
		if got := hexField(t, f.Chain.Value); got.Cmp(keys.SpendKey) != 0 || !f.OK {
			t.Errorf("the chain's value 0x%x (ok %v), want the registered pk_spend 0x%x, ok", got, f.OK, keys.SpendKey)
		}
		if !strings.Contains(f.Chain.Name, "UserRegistry") {
			t.Errorf("the chain's value is %q, want it to name the UserRegistry", f.Chain.Name)
		}
		// Only the Sender's registration carries it.
		for _, h := range sink.hops() {
			if h.Step == "register" && h.From != engine.ActorNode(sender.ID) && len(detailsOf(h, engine.DetailFormula)) > 0 {
				t.Errorf("%s's register hop %q carries a formula, want only the Sender's", h.From, h.Label)
			}
		}
	})

	t.Run("each participant's scan shows the note their view key opened: their payout, hashing to their payment's commitment", func(t *testing.T) {
		checkDiscoverDetails(t, client, e, sink.hops(), false)
	})

	t.Run("payment 1's proof request carries its witness, as sent to the prover, its output openings hashing to the commitments the vault logged", func(t *testing.T) {
		var prove []engine.Event
		for _, h := range sink.hops() {
			if h.Step == "fanout_1" && h.Label == "prove Payment" {
				prove = append(prove, h)
			}
		}
		if len(prove) != 1 {
			t.Fatalf("%d proof request hops in payment 1, want 1", len(prove))
		}
		ws, ins, wire := detailsOf(prove[0], engine.DetailWitness), detailsOf(prove[0], engine.DetailPublicInputs), detailsOf(prove[0], engine.DetailHTTP)
		if len(ws) != 1 || ws[0].Witness == nil || len(ins) != 1 || len(wire) != 1 {
			t.Fatalf("payment 1's proof request carries %d witness, %d publicInputs and %d http details, want one each", len(ws), len(ins), len(wire))
		}
		w := ws[0].Witness
		sender, payee := e.Sender(), e.Participants()[0]

		// The witness is what the Sender's wallet sent its prover.
		var req map[string]any
		if err := json.Unmarshal([]byte(wire[0].Request), &req); err != nil {
			t.Fatalf("payment 1's proof request body: %v", err)
		}
		dec := func(v any) *big.Int {
			n, ok := new(big.Int).SetString(fmt.Sprint(v), 10)
			if !ok {
				t.Fatalf("request value %v is not a decimal integer", v)
			}
			return n
		}
		list := func(k string) []any { l, _ := req[k].([]any); return l }
		eq := func(what, witness string, sent any) {
			if hexField(t, witness).Cmp(dec(sent)) != 0 {
				t.Errorf("witness %s %s, the request sent %v", what, witness, sent)
			}
		}
		if len(w.In) != 1 || len(w.Out) != 2 {
			t.Fatalf("witness has %d spent and %d new notes, want 1 and 2", len(w.In), len(w.Out))
		}
		eq("spend key", w.SpendKey, list("wtPrivateKeysIn")[0])
		eq("token", w.Token, req["wtTokenId"])
		in := w.In[0]
		if in.Owner != sender.Name || in.Amount != fmt.Sprint(opts.ShieldTotal) || dec(list("wtValuesIn")[0]).String() != in.Amount {
			t.Errorf("spent note %+v, want the Sender's shielded %d, as sent (%v)", in, opts.ShieldTotal, list("wtValuesIn")[0])
		}
		eq("spent salt", in.Salt, list("wtSaltsIn")[0])
		if in.Path == nil || len(in.Path.Siblings) != 8 {
			t.Fatalf("spent note's Merkle path %+v, want 8 siblings", in.Path)
		}
		eq("leaf index", fmt.Sprintf("0x%x", in.Path.Index), list("wtPathIndices")[0])
		sentPath, _ := list("wtPathElements")[0].([]any)
		for i, s := range in.Path.Siblings {
			eq(fmt.Sprintf("sibling %d", i), s, sentPath[i])
		}
		for i, want := range []struct {
			owner  string
			pk     *big.Int
			amount int64
		}{{payee.Name, payee.Spend.PublicKey, opts.Payouts[0]}, {sender.Name, sender.Spend.PublicKey, opts.ShieldTotal - opts.Payouts[0]}} {
			o := w.Out[i]
			if o.Owner != want.owner || o.Amount != fmt.Sprint(want.amount) || hexField(t, o.PkSpend).Cmp(want.pk) != 0 {
				t.Errorf("new note %d %+v, want %s's %d", i, o, want.owner, want.amount)
			}
			eq(fmt.Sprintf("new note %d's spend public key", i), o.PkSpend, list("wtSpendPublicKeysOut")[i])
			eq(fmt.Sprintf("new note %d's salt", i), o.Salt, list("wtSaltsOut")[i])
			if dec(list("wtValuesOut")[i]).String() != o.Amount {
				t.Errorf("new note %d's amount %s, the request sent %v", i, o.Amount, list("wtValuesOut")[i])
			}
		}

		// Each new note's opening hashes to its public CommitmentsOut, and
		// those are the commitments the vault logged for payment 1.
		var public []*big.Int
		for _, x := range ins[0].Inputs {
			if x.Name == "CommitmentsOut" {
				public = append(public, hexField(t, x.Value))
			}
		}
		logged := commitmentsIn(txLogs(t, client, e.FanOut()[0].TxHash), common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress))
		if len(public) != 2 || len(logged) != 2 {
			t.Fatalf("%d public CommitmentsOut and %d logged commitments, want 2 each", len(public), len(logged))
		}
		for i, o := range w.Out {
			amount, _ := new(big.Int).SetString(o.Amount, 10)
			c, err := rpcore.Erc20CommitmentV2(hexField(t, o.PkSpend), hexField(t, o.Salt), amount, hexField(t, w.Token))
			if err != nil {
				t.Fatalf("Erc20CommitmentV2: %v", err)
			}
			if c.Cmp(public[i]) != 0 {
				t.Errorf("new note %d's opening hashes to 0x%x, public CommitmentsOut[%d] is 0x%x", i, c, i, public[i])
			}
			if c.Cmp(logged[i]) != 0 {
				t.Errorf("new note %d's opening hashes to 0x%x, the vault logged 0x%x", i, c, logged[i])
			}
		}
	})

	t.Run("the shield's hop carries the commitment formula: Poseidon of the Sender's opening, the leaf its Commitment event inserted", func(t *testing.T) {
		var shield []engine.Event
		for _, h := range sink.hops() {
			if h.Step == "shield" && h.Label == "shield" {
				shield = append(shield, h)
			}
		}
		if len(shield) != 1 {
			t.Fatalf("%d shield hops, want 1", len(shield))
		}
		fs, cs := detailsOf(shield[0], engine.DetailFormula), detailsOf(shield[0], engine.DetailCommitment)
		if len(fs) != 1 || fs[0].Formula == nil || len(cs) != 1 {
			t.Fatalf("the shield hop carries %d formula and %d commitment details, want one each", len(fs), len(cs))
		}
		f := fs[0].Formula
		if f.Chain == nil {
			t.Fatal("the shield's commitment formula carries no chain value: a full run's, by name only")
		}
		if f.Name != engine.FormulaCommitment {
			t.Errorf("formula %q, want %q", f.Name, engine.FormulaCommitment)
		}
		var names []string
		in := map[string]string{}
		for _, x := range f.Inputs {
			names = append(names, x.Name)
			in[x.Name] = x.Value
		}
		if want := []string{"pk_spend", "salt", "amount", "token"}; !slices.Equal(names, want) {
			t.Fatalf("inputs %v, want %v, in the order the commitment hashes them", names, want)
		}
		pk, salt, token := hexField(t, in["pk_spend"]), hexField(t, in["salt"]), hexField(t, in["token"])
		amount, ok := new(big.Int).SetString(in["amount"], 10)
		if !ok {
			t.Fatalf("amount %q is not decimal", in["amount"])
		}
		// The opening is the Sender's: their spend public key, the total shielded, RaylsERC20's token id 0.
		if pk.Cmp(e.Sender().Spend.PublicKey) != 0 || amount.Int64() != opts.ShieldTotal || token.Sign() != 0 {
			t.Errorf("opening (pk_spend %s, amount %s, token %s), want the Sender's %s, %d, 0", pk, amount, token, e.Sender().Spend.PublicKey, opts.ShieldTotal)
		}
		// Recomputed independently of the engine, with the client library's Poseidon.
		want, err := poseidon.Hash([]*big.Int{pk, salt, amount, token})
		if err != nil {
			t.Fatalf("poseidon.Hash: %v", err)
		}
		if got := hexField(t, f.Result); got.Cmp(want) != 0 {
			t.Errorf("result 0x%x, Poseidon(pk_spend, salt, amount, token) is 0x%x", got, want)
		}
		logged := commitmentsIn(txLogs(t, client, common.HexToHash(cs[0].TxHash)), common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress))
		if len(logged) != 1 || logged[0].Cmp(want) != 0 {
			t.Fatalf("the deposit's Commitment events logged %v, want one, 0x%x", logged, want)
		}
		if got := hexField(t, f.Chain.Value); got.Cmp(logged[0]) != 0 || !f.OK {
			t.Errorf("the chain's value 0x%x (ok %v), want the logged leaf 0x%x, ok", got, f.OK, logged[0])
		}
		if !strings.Contains(f.Chain.Name, fmt.Sprintf("#%d", cs[0].Leaves[0].Index)) {
			t.Errorf("the chain's value is %q, want it to name leaf #%d", f.Chain.Name, cs[0].Leaves[0].Index)
		}
	})

	// payment1Formula is the one formula detail of payment 1's one hop
	// that is, from → to.
	payment1Formula := func(t *testing.T, what, from, to string) *engine.Formula {
		t.Helper()
		var hs []engine.Event
		for _, h := range sink.hops() {
			if h.Step == "fanout_1" && h.From == from && h.To == to && !h.Failed {
				hs = append(hs, h)
			}
		}
		if len(hs) != 1 {
			t.Fatalf("%d %s hops in payment 1, want 1", len(hs), what)
		}
		fs := detailsOf(hs[0], engine.DetailFormula)
		if len(fs) != 1 || fs[0].Formula == nil {
			t.Fatalf("payment 1's %s hop carries %d formula details, want 1", what, len(fs))
		}
		return fs[0].Formula
	}
	// payment1Request is payment 1's proof request as sent to the prover.
	payment1Request := func(t *testing.T) map[string]any {
		t.Helper()
		for _, h := range sink.hops() {
			if h.Step == "fanout_1" && h.Label == "prove Payment" {
				if wire := detailsOf(h, engine.DetailHTTP); len(wire) == 1 {
					var req map[string]any
					if err := json.Unmarshal([]byte(wire[0].Request), &req); err != nil {
						t.Fatalf("payment 1's proof request body: %v", err)
					}
					return req
				}
			}
		}
		t.Fatal("no proof request in payment 1")
		return nil
	}
	decimal := func(t *testing.T, v any) *big.Int {
		t.Helper()
		n, ok := new(big.Int).SetString(fmt.Sprint(v), 10)
		if !ok {
			t.Fatalf("value %v is not a decimal integer", v)
		}
		return n
	}
	vault := bindContract(t, client, common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress), loadOnchainABI(t, "Erc20CoinVault"))

	t.Run("payment 1's proof hop carries its Merkle path formula: the shielded leaf hashed up its 8 levels to a root the vault has had", func(t *testing.T) {
		f := payment1Formula(t, "proof", engine.NodeGnark, engine.ActorNode(e.Sender().ID))
		if f.Name != engine.FormulaMerklePath {
			t.Fatalf("formula %q, want %q", f.Name, engine.FormulaMerklePath)
		}
		// leaf, then per level its sibling and (below the root) the node it makes.
		if len(f.Inputs) != 1+8+7 {
			t.Fatalf("%d inputs, want the leaf, 8 siblings and 7 nodes below the root", len(f.Inputs))
		}
		// Independently of the engine: the leaf the shield's Commitment
		// event logged, the siblings and leaf index the prover was sent.
		var shieldTx string
		for _, h := range sink.hops() {
			if cs := detailsOf(h, engine.DetailCommitment); h.Step == "shield" && len(cs) == 1 {
				shieldTx = cs[0].TxHash
			}
		}
		leaves := commitmentsIn(txLogs(t, client, common.HexToHash(shieldTx)), common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress))
		if len(leaves) != 1 {
			t.Fatalf("the shield logged %d commitments, want 1", len(leaves))
		}
		req := payment1Request(t)
		index := int(decimal(t, req["wtPathIndices"].([]any)[0]).Int64())
		sent, _ := req["wtPathElements"].([]any)[0].([]any)
		if len(sent) != 8 {
			t.Fatalf("the prover was sent %d siblings, want 8", len(sent))
		}
		want := []string{fmt.Sprintf("0x%064x", leaves[0])}
		node := leaves[0]
		for level, s := range sent {
			sib := decimal(t, s)
			want = append(want, fmt.Sprintf("0x%064x", sib))
			pair := []*big.Int{node, sib}
			if index>>level&1 == 1 {
				pair = []*big.Int{sib, node}
			}
			var err error
			if node, err = poseidon.Hash(pair); err != nil {
				t.Fatalf("poseidon.Hash: %v", err)
			}
			if level < 7 {
				want = append(want, fmt.Sprintf("0x%064x", node))
			}
		}
		for i, x := range f.Inputs {
			if hexField(t, x.Value).Cmp(hexField(t, want[i])) != 0 {
				t.Errorf("input %d (%s) %s, recomputed %s", i, x.Name, x.Value, want[i])
			}
		}
		if !strings.Contains(f.Inputs[0].Name, fmt.Sprintf("#%d", index)) {
			t.Errorf("the leaf is named %q, want leaf #%d", f.Inputs[0].Name, index)
		}
		for level := range 8 {
			side := "right"
			if index>>level&1 == 1 {
				side = "left"
			}
			if name := f.Inputs[1+2*level].Name; !strings.HasSuffix(name, side) {
				t.Errorf("sibling at level %d is named %q, want it on the %s (leaf #%d)", level, name, side, index)
			}
		}
		if got := hexField(t, f.Result); got.Cmp(node) != 0 {
			t.Errorf("result %s, the path hashes up to 0x%x", f.Result, node)
		}
		// The root is the one the proof publishes, and one the vault has had.
		tree := int(decimal(t, req["stTreeNumbers"].([]any)[0]).Int64())
		if public := decimal(t, req["stMerkleRoots"].([]any)[0]); public.Cmp(node) != 0 {
			t.Errorf("the path hashes up to 0x%x, the proof publishes root 0x%x", node, public)
		}
		if !vaultHasRoot(t, vault, tree, fmt.Sprintf("0x%x", node)) {
			t.Errorf("root 0x%x is not one the vault has had in tree %d", node, tree)
		}
		if hexField(t, f.Chain.Value).Cmp(node) != 0 || !f.OK {
			t.Errorf("the chain's root %s (ok %v), want 0x%x, ok", f.Chain.Value, f.OK, node)
		}
		if !strings.Contains(f.Chain.Name, "isValidRoot") || !strings.Contains(f.Chain.Name, fmt.Sprintf("tree %d", tree)) {
			t.Errorf("the chain's value is %q, want it to name isValidRoot and tree %d", f.Chain.Name, tree)
		}
		// The crowd: the leaves tree T held when the root was taken, before
		// payment 1's settlement inserted its notes, counted from the
		// vault's Commitment events.
		settled, err := client.TransactionReceipt(context.Background(), e.FanOut()[0].TxHash)
		if err != nil {
			t.Fatalf("payment 1's receipt: %v", err)
		}
		sizes := treeSizes(t, client, common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress), settled.BlockNumber.Uint64())
		if tree >= len(sizes) || f.Leaves != sizes[tree] {
			t.Errorf("the formula's crowd is %d leaves, the vault's Commitment events put %v leaves in its trees, want tree %d's", f.Leaves, sizes, tree)
		}
		if f.Leaves <= index {
			t.Errorf("%d leaves in tree %d, but the spent note is leaf #%d", f.Leaves, tree, index)
		}
	})

	t.Run("payment 1's settle hop carries its nullifier formula: Poseidon of the Sender's spend key and the leaf's position, the Nullifier event the receipt logged", func(t *testing.T) {
		f := payment1Formula(t, "settle", engine.NodeRelayer, engine.NodeDvp)
		if f.Name != engine.FormulaNullifier {
			t.Fatalf("formula %q, want %q", f.Name, engine.FormulaNullifier)
		}
		in := map[string]string{}
		var names []string
		for _, x := range f.Inputs {
			names = append(names, x.Name)
			in[x.Name] = x.Value
		}
		if want := []string{"sk_spend", "tree", "leaf index", "tree · 2⁸ + leaf index"}; !slices.Equal(names, want) {
			t.Fatalf("inputs %v, want %v", names, want)
		}
		req := payment1Request(t)
		sk, index := e.Sender().Spend.PrivateKey, decimal(t, req["wtPathIndices"].([]any)[0])
		tree := decimal(t, req["stTreeNumbers"].([]any)[0])
		if hexField(t, in["sk_spend"]).Cmp(sk) != 0 || decimal(t, in["tree"]).Cmp(tree) != 0 || decimal(t, in["leaf index"]).Cmp(index) != 0 {
			t.Errorf("inputs %v, want the Sender's spend key, tree %s and leaf index %s, as sent to the prover", in, tree, index)
		}
		global := new(big.Int).Add(new(big.Int).Mul(tree, big.NewInt(256)), index)
		if decimal(t, in["tree · 2⁸ + leaf index"]).Cmp(global) != 0 {
			t.Errorf("tree · 2⁸ + leaf index %s, want %s", in["tree · 2⁸ + leaf index"], global)
		}
		want, err := poseidon.Hash([]*big.Int{sk, global})
		if err != nil {
			t.Fatalf("poseidon.Hash: %v", err)
		}
		if hexField(t, f.Result).Cmp(want) != 0 {
			t.Errorf("result %s, Poseidon(sk_spend, tree · 2⁸ + leaf index) is 0x%x", f.Result, want)
		}
		// Payment 1's settlement logged it: the vault's one Nullifier event.
		var logged []*big.Int
		for _, l := range txLogs(t, client, e.FanOut()[0].TxHash) {
			if l.Address == common.HexToAddress(receipts["Erc20CoinVault"].ContractAddress) && len(l.Topics) == 4 && l.Topics[0] == nullifierEventSig {
				logged = append(logged, l.Topics[3].Big())
			}
		}
		if len(logged) != 1 || logged[0].Cmp(want) != 0 {
			t.Fatalf("payment 1's receipt logged the vault's nullifiers %v, want one, 0x%x", logged, want)
		}
		if hexField(t, f.Chain.Value).Cmp(want) != 0 || !f.OK {
			t.Errorf("the chain's value %s (ok %v), want the logged nullifier 0x%x, ok", f.Chain.Value, f.OK, want)
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
				if sentIn != "fanout_1/settle" {
					t.Errorf("verifyProof field sent in release %s, want once payment 1 settles (fanout_1/settle)", sentIn)
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
			ethCalls[data] = out.Result
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

	t.Run("replaying payment 1 after payment 2 is refused for its spent nullifier, and changes nothing on chain", func(t *testing.T) {
		// The replay is released once, right after payment 2 settles.
		at := slices.IndexFunc(acts, func(a released) bool { return a.at.Action == "replay" })
		if at < 1 {
			t.Fatalf("replay released at %d, want after payment 2", at)
		}
		if a, prev := acts[at].at, acts[at-1].at; a.Step != "fanout_2" || prev.Step != "fanout_2" || prev.Action != "payment" {
			t.Errorf("replay released as %s/%s after %s/%s, want fanout_2/replay right after fanout_2/payment", a.Step, a.Action, prev.Step, prev.Action)
		}
		if who := acts[at].at.Value; who != e.Participants()[0].Name {
			t.Errorf("the replay is about %q, want payment 1's payee %s", who, e.Participants()[0].Name)
		}
		hops := acts[at].hops
		if len(hops) != 1 || hops[0].Failed {
			t.Fatalf("replay hops %v, want one refused (not failed) hop", hops)
		}
		h := hops[0]
		if h.From != engine.ActorNode(e.Sender().ID) || h.To != engine.NodeRelayer {
			t.Errorf("replay hop goes %s → %s, want Sender → relayer", h.From, h.To)
		}

		// It resubmits payment 1's exact relay request.
		var relay1 []engine.HopDetail
		for _, x := range sink.hops() {
			if x.Step == "fanout_1" && x.Label == "relay" {
				relay1 = detailsOf(x, engine.DetailHTTP)
			}
		}
		wire := detailsOf(h, engine.DetailHTTP)
		if len(relay1) != 1 || len(wire) != 1 {
			t.Fatalf("%d http details on payment 1's relay and %d on the replay, want one each", len(relay1), len(wire))
		}
		if wire[0].URL != relay1[0].URL || wire[0].Request != relay1[0].Request {
			t.Errorf("replay sent %s %q, want payment 1's request %s %q", wire[0].URL, wire[0].Request, relay1[0].URL, relay1[0].Request)
		}
		if wire[0].Status < 400 || wire[0].Status >= 500 {
			t.Errorf("relayer answered the replay %d, want a refusal (4xx)", wire[0].Status)
		}

		// The refusal names the relayer and the spent nullifier.
		rs := detailsOf(h, engine.DetailRefusal)
		if len(rs) != 1 {
			t.Fatalf("replay hop carries %d refusal details, want 1", len(rs))
		}
		r := rs[0]
		if r.RefusedBy != engine.NodeRelayer || !strings.Contains(r.Reason, "nullifier already spent") {
			t.Errorf("refused by %q because %q, want the relayer because the nullifier is already spent", r.RefusedBy, r.Reason)
		}
		var nullifier string
		for _, x := range sink.hops() {
			if x.Step == "fanout_1" && x.Label == "prove Payment" {
				for _, d := range detailsOf(x, engine.DetailPublicInputs) {
					for _, in := range d.Inputs {
						if in.Name == "Nullifiers" {
							nullifier = in.Value
						}
					}
				}
			}
		}
		if nullifier == "" || r.Nullifier != nullifier || r.TxHash != e.FanOut()[0].TxHash.Hex() {
			t.Errorf("refusal names nullifier %s spent by %s, want payment 1's %s spent by %s", r.Nullifier, r.TxHash, nullifier, e.FanOut()[0].TxHash.Hex())
		}

		// The trees and every balance are unchanged: as the engine read them,
		// and on chain, where nothing was mined between payments 2 and 3.
		if r.Before == nil || r.After == nil || len(r.Before.Trees) != 2 || len(r.Before.Balances) != 2+2+engine.K {
			t.Fatalf("refusal's chain state %+v → %+v, want both vaults' trees and the vault's, the relayer's and every actor's balance", r.Before, r.After)
		}
		if !slices.Equal(r.Before.Trees, r.After.Trees) || !slices.Equal(r.Before.Balances, r.After.Balances) {
			t.Errorf("chain state changed: %+v → %+v", r.Before, r.After)
		}
		block := func(tx common.Hash) uint64 {
			rc, err := client.TransactionReceipt(context.Background(), tx)
			if err != nil {
				t.Fatalf("receipt %s: %v", tx.Hex(), err)
			}
			return rc.BlockNumber.Uint64()
		}
		if b2, b3 := block(e.FanOut()[1].TxHash), block(e.FanOut()[2].TxHash); b3 != b2+1 {
			t.Errorf("payment 2 in block %d, payment 3 in %d: a transaction was mined between them", b2, b3)
		}
	})

	t.Run("the audit hop shows every note the auditor decrypted, each the ciphertext on chain and its owner's opening", func(t *testing.T) {
		checkAuditDetails(t, client, e, sink.hops())
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

// treeSizes counts the leaves in each of the vault at vaultAddr's trees,
// from its Commitment events in blocks before block before. As Merkle.sol
// does, it inserts a transaction's commitments as one batch and starts a
// new tree when the batch would fill the current one (count + batch ≥ 2⁸).
// It repeats the engine's rule on purpose: the test's own count, from the
// chain's events, is what the engine's crowd is checked against.
func treeSizes(t *testing.T, client *ethclient.Client, vaultAddr common.Address, before uint64) []int {
	t.Helper()
	logs, err := client.FilterLogs(context.Background(), ethereum.FilterQuery{
		Addresses: []common.Address{vaultAddr},
		Topics:    [][]common.Hash{{commitmentEventSig}},
		ToBlock:   new(big.Int).SetUint64(before - 1),
	})
	if err != nil {
		t.Fatalf("vault Commitment events: %v", err)
	}
	var batches []int
	var last common.Hash
	for i, l := range logs {
		if i == 0 || l.TxHash != last {
			batches = append(batches, 0)
		}
		batches[len(batches)-1]++
		last = l.TxHash
	}
	sizes := []int{0}
	for _, n := range batches {
		if sizes[len(sizes)-1]+n >= 1<<8 {
			sizes = append(sizes, 0)
		}
		sizes[len(sizes)-1] += n
	}
	return sizes
}
