package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"enygma_retail_payments/layout"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// ErrRefused marks a transfer refused before proving: nothing was proved and
// no transaction was sent.
var ErrRefused = errors.New("transfer refused")

// Transfer is one transfer between registered actors, by name.
type Transfer struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Amount int64  `json:"amount"`
}

// defaultScript is the transfers between participants played after the
// fan-out of DefaultOptions' payouts. Each spends the payer's smallest note
// that covers it; the last, bigger than either of Diego's notes (60 and 100),
// merges them.
var defaultScript = []Transfer{
	{"Ana", "Bruno", 50},
	{"Elena", "Carla", 120},
	{"Carla", "Diego", 100},
	{"Bruno", "Felipe", 70},
	{"Felipe", "Ana", 30},
	{"Diego", "Elena", 150},
}

func refused(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrRefused, fmt.Sprintf(format, args...))
}

// smallestCovering returns the smallest unspent note holding at least
// amount, or nil if none does.
func (p purse) smallestCovering(amount *big.Int) *note {
	var best *note
	for _, n := range p.unspent() {
		if n.amount.Cmp(amount) >= 0 && (best == nil || n.amount.Cmp(best.amount) < 0) {
			best = n
		}
	}
	return best
}

// covers refuses an amount over owner's balance in p.
func (p purse) covers(owner, unit string, amount *big.Int) error {
	if balance := p.balance(); balance.Cmp(amount) < 0 {
		return refused("%s holds %s %s privately, cannot pay %s", owner, balance, unit, amount)
	}
	return nil
}

// twoLargest returns the two largest unspent notes, largest first; b is nil
// if there are fewer than two.
func (p purse) twoLargest() (a, b *note) {
	for _, n := range p.unspent() {
		switch {
		case a == nil || n.amount.Cmp(a.amount) > 0:
			a, b = n, a
		case b == nil || n.amount.Cmp(b.amount) > 0:
			b = n
		}
	}
	return a, b
}

// spendable picks the single note owner spends to pay amount from p (the
// USDr fee leg spends one note): the smallest that covers it. It refuses an
// amount over owner's balance, or bigger than every note they hold.
func (p purse) spendable(owner, unit string, amount *big.Int) (*note, error) {
	if err := p.covers(owner, unit, amount); err != nil {
		return nil, err
	}
	n := p.smallestCovering(amount)
	if n == nil {
		return nil, refused("%s holds no single %s note of %s or more", owner, unit, amount)
	}
	return n, nil
}

// inputs picks the notes owner spends to pay amount from p: the smallest
// single note that covers it, or else their two largest notes, merged. It
// refuses an amount over owner's balance, or one even their two largest notes
// can't cover, since a transfer merges at most two notes.
func (p purse) inputs(owner, unit string, amount *big.Int) ([]*note, error) {
	if err := p.covers(owner, unit, amount); err != nil {
		return nil, err
	}
	if n := p.smallestCovering(amount); n != nil {
		return []*note{n}, nil
	}
	a, b := p.twoLargest()
	if b == nil || new(big.Int).Add(a.amount, b.amount).Cmp(amount) < 0 {
		return nil, refused("%s's two largest %s notes cannot cover %s (a transfer merges at most two notes)", owner, unit, amount)
	}
	return []*note{a, b}, nil
}

// total is the sum of notes' amounts.
func total(notes []*note) *big.Int {
	sum := new(big.Int)
	for _, n := range notes {
		sum.Add(sum, n.amount)
	}
	return sum
}

// amounts lists notes' amounts, e.g. "100 + 60".
func amounts(notes []*note) string {
	parts := make([]string, len(notes))
	for i, n := range notes {
		parts[i] = n.amount.String()
	}
	return strings.Join(parts, " + ")
}

// actorNamed returns the registered actor (the sender or a participant) named name.
func (e *Engine) actorNamed(name string) (*Actor, error) {
	for _, a := range e.registrable() {
		if a.Name == name {
			return a, nil
		}
	}
	return nil, refused("no sender or participant named %q", name)
}

// sent is a transfer as it settled, with what the UI shows about it.
type sent struct {
	Settlement
	change           *big.Int
	proveMs, relayMs int64
	tx               *relayTxResponse
	method           string
	// merged is set when the transfer merged two notes (Payment2in).
	merged bool
	// payeeSalt opens the recipient's note; a private tag carries it.
	payeeSalt *big.Int
	// tagBlock is the block the transfer's tag was published in; 0 with tags off.
	tagBlock uint64
}

// send pays to amount through the relayer, change back to from: from's
// smallest note that covers it with a 1-in/2-out Payment transfer, or, if no
// single note does, their two largest notes merged with a 2-in/2-out
// Payment2in transfer. With payUsdrFee it also pays the relayer's USDr fee
// from from's USDr note in the same transaction, USDr change back to from.
// Anything it can tell will fail — over balance, no two notes that cover it,
// no USDr for the fee — it refuses with ErrRefused before proving. It returns
// once the transfer is mined, so from's change notes are already in the trees.
func (e *Engine) send(ctx context.Context, s *step, from, to *Actor, amount *big.Int, payUsdrFee bool) (*sent, error) {
	ins, err := e.checkSend(from, to, amount, payUsdrFee)
	if err != nil {
		return nil, err
	}
	c := e.chain
	change := new(big.Int).Sub(total(ins), amount)

	tree, err := c.vaultTree(ctx, c.vaultAddr)
	if err != nil {
		return nil, err
	}
	var values, salts, treeNums []*big.Int
	var keys []rpcore.KeyPair
	var proofs []*rpcore.MerkleProof
	for _, n := range ins {
		proof, err := tree.GenerateProof(n.commitment)
		if err != nil {
			return nil, fmt.Errorf("merkle proof for %s's %s note: %w", from.Name, n.amount, err)
		}
		values, salts = append(values, n.amount), append(salts, n.salt)
		keys = append(keys, rpcore.KeyPair{PrivateKey: from.Spend.PrivateKey, PublicKey: from.Spend.PublicKey})
		proofs, treeNums = append(proofs, proof), append(treeNums, big.NewInt(int64(proof.TreeNumber)))
	}
	s.log("key", "rebuilt vault tree from %d Commitment events, root 0x%s…", len(tree.GetLeaves()), proofs[0].Root.Text(16)[:12])

	proveStart := time.Now()
	res, err := rpcore.NewPaymentClient(e.cfg.GnarkURL).BoundPaymentProof(
		new(big.Int).SetBytes(c.vaultAddr.Bytes()),
		big.NewInt(0),
		values,
		keys,
		salts,
		[]*big.Int{amount, change},
		[]*big.Int{to.Spend.PublicKey, from.Spend.PublicKey},
		[][]byte{to.View.EncapsKey, from.View.EncapsKey},
		merkleDepth,
		proofs,
		treeNums,
		tokenId,
	)
	proveMs := time.Since(proveStart).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("prove payment: %w", err)
	}
	stmt := res.ContractStatement()
	circuit, stmtLen, nullifiersOffset, commitmentsOutOffset := "Payment", layout.PaymentLen, layout.PaymentNullifiersOffset, layout.PaymentCommitmentsOutOffset
	if len(ins) == 2 {
		circuit, stmtLen, nullifiersOffset, commitmentsOutOffset = "Payment2in", layout.Payment2inLen, layout.Payment2inNullifiersOffset, layout.Payment2inCommitmentsOutOffset
	}
	if len(stmt) != stmtLen {
		return nil, fmt.Errorf("%s statement has %d signals, want %d", circuit, len(stmt), stmtLen)
	}
	nullifiers := make([]string, len(ins))
	for i := range ins {
		nullifiers[i] = "0x" + stmt[nullifiersOffset+i].Text(16)[:12] + "…"
	}
	if len(ins) == 2 {
		s.log("zk", "Payment2in proof in %d ms — merges %s's %s notes, nullifiers %s",
			proveMs, from.Name, amounts(ins), strings.Join(nullifiers, ", "))
	} else {
		s.log("zk", "Payment proof in %d ms — spends %s's %s note, nullifier %s", proveMs, from.Name, ins[0].amount, nullifiers[0])
	}

	var fee *usdrFeeLeg
	if payUsdrFee {
		feeStart := time.Now()
		if fee, err = e.proveUsdrFee(ctx, from); err != nil {
			return nil, err
		}
		s.log("zk", "UsdrFee proof in %d ms — %s USDr fee note to the relayer, %s USDr change back to %s",
			time.Since(feeStart).Milliseconds(), e.usdrFixedFee, fee.change.amount, from.Name)
	}

	relayStart := time.Now()
	var tx *relayTxResponse
	if fee != nil {
		tx, err = e.relayer.paymentUsdrFee(ctx, res, fee)
	} else {
		tx, err = e.relayer.payment(ctx, res)
	}
	relayMs := time.Since(relayStart).Milliseconds()
	if err != nil {
		return nil, err
	}
	receipt, err := c.client.TransactionReceipt(ctx, common.HexToHash(tx.TxHash))
	if err != nil {
		return nil, fmt.Errorf("receipt for %s: %w", tx.TxHash, err)
	}
	if receipt.Status != ethtypes.ReceiptStatusSuccessful {
		return nil, fmt.Errorf("payment %s reverted", tx.TxHash)
	}

	changeNote := &note{commitment: stmt[commitmentsOutOffset+1], salt: res.SaltA, amount: change}
	if err := changeNote.check(from); err != nil {
		return nil, err
	}
	if !hasCommitmentLog(receipt.Logs, c.vaultAddr, changeNote.commitment) {
		return nil, fmt.Errorf("payment %s did not insert %s's change note", tx.TxHash, from.Name)
	}
	for _, n := range ins {
		n.spent = true
	}
	from.notes = append(from.notes, changeNote)

	out := &sent{
		Settlement: Settlement{From: from.Name, To: to.Name, Amount: amount, TxHash: common.HexToHash(tx.TxHash)},
		change:     change, proveMs: proveMs, relayMs: relayMs, tx: tx, method: "payment", merged: len(ins) == 2,
		payeeSalt: res.SaltB,
	}
	if fee != nil {
		if !hasCommitmentLog(receipt.Logs, c.usdrVaultAddr, fee.feeNote) || !hasCommitmentLog(receipt.Logs, c.usdrVaultAddr, fee.change.commitment) {
			return nil, fmt.Errorf("paymentWithUsdrFee %s did not insert the relayer's fee note and %s's USDr change", tx.TxHash, from.Name)
		}
		fee.in.spent = true
		from.usdr = append(from.usdr, fee.change)
		out.UsdrFeeNote = fee.feeNote
		out.method = "paymentWithUsdrFee"
	}
	s.log("chain", "EnygmaDvp.%s() via relayer → block %d  gas %s  round-trip %d ms", out.method, tx.BlockNumber, formatNum(tx.GasUsed), relayMs)
	return out, nil
}

// checkSend checks a transfer send would make before anything is proved or
// sent, and returns the notes from would spend. It refuses with ErrRefused
// paying oneself, a non-positive amount, an amount from's notes can't cover,
// or no USDr note for the fee; it fails if the relayer takes no USDr fees.
func (e *Engine) checkSend(from, to *Actor, amount *big.Int, payUsdrFee bool) ([]*note, error) {
	if from == to {
		return nil, refused("%s cannot pay themselves", from.Name)
	}
	if amount.Sign() <= 0 {
		return nil, refused("the amount must be positive, got %s", amount)
	}
	ins, err := from.notes.inputs(from.Name, "RaylsERC20", amount)
	if err != nil {
		return nil, err
	}
	if payUsdrFee {
		if e.relayerFeePk == nil {
			return nil, fmt.Errorf("the relayer publishes no fee spend key (set RELAYER_FEE_SPEND_PRIVATE_KEY)")
		}
		if _, err := from.usdr.spendable(from.Name, "USDr", e.usdrFixedFee); err != nil {
			return nil, fmt.Errorf("%w for the relayer's fee", err)
		}
	}
	return ins, nil
}

// row is the transfer's line in one of the UI's transfer tables.
// scan says how the recipient found it, if they have yet.
func (t *sent) row(n int, feeAmount *big.Int, scan string) string {
	fee, tag := "—", "—"
	if t.UsdrFeeNote != nil {
		fee = feeAmount.String() + " USDr"
	}
	if t.tagBlock != 0 {
		tag = fmt.Sprintf("block %d", t.tagBlock)
	}
	row, _ := json.Marshal(map[string]any{
		"n": n, "from": t.From, "to": t.To, "amount": t.Amount.String(), "fee": fee, "merged": t.merged, "tag": tag, "scan": scan,
		"proveMs": t.proveMs, "relayMs": t.relayMs, "gas": t.tx.GasUsed, "block": t.tx.BlockNumber, "tx": t.tx.TxHash,
	})
	return string(row)
}

// payByName is payTransfer between the registered actors t names.
func (e *Engine) payByName(ctx context.Context, s *step, t Transfer) error {
	from, err := e.actorNamed(t.From)
	if err != nil {
		return err
	}
	to, err := e.actorNamed(t.To)
	if err != nil {
		return err
	}
	return e.payTransfer(ctx, s, from, to, big.NewInt(t.Amount))
}

// payTransfer is one transfer between registered actors after the fan-out:
// send it, then let the recipient find it by scanning the chain. With tags
// on, the payer first opens a channel to the recipient if they have none,
// then tags the payment, and the recipient finds it by its tag.
func (e *Engine) payTransfer(ctx context.Context, s *step, from, to *Actor, amount *big.Int) error {
	if e.opts.Tags {
		// Refuse before opening a channel, so a refusal sends nothing.
		if _, err := e.checkSend(from, to, amount, e.opts.UsdrFee); err != nil {
			return err
		}
		if err := e.ensureChannel(ctx, s, from, to); err != nil {
			return err
		}
	}
	t, err := e.send(ctx, s, from, to, amount, e.opts.UsdrFee)
	if err != nil {
		return err
	}
	if e.opts.Tags {
		if err := e.publishTag(ctx, s, from, to, t); err != nil {
			return err
		}
	}
	e.transfers = append(e.transfers, t.Settlement)

	var scan string
	if e.opts.Tags {
		if scan, err = e.discoverByTag(ctx, s, to, 1); err != nil {
			return err
		}
	} else {
		events, err := e.chain.noteEvents(ctx, e.chain.vaultAddr, erc20VaultId)
		if err != nil {
			return err
		}
		if found := to.scan(events, &to.notes); found != 1 {
			return fmt.Errorf("%s found %d new notes by scanning, want the payment", to.Name, found)
		}
		scan = trialScan(len(events))
		s.log("key", "%s scanned %d encrypted notes and decrypted the payment with their ML-KEM view key", to.Name, len(events))
		s.actor(to, "scan", scan)
	}

	s.proto("transfer", t.row(len(e.transfers), e.usdrFixedFee, scan))
	for _, a := range []*Actor{from, to} {
		s.actor(a, "balance", a.notes.balance().String()+" RaylsERC20")
		s.actor(a, "note", "0x"+a.notes[len(a.notes)-1].commitment.Text(16))
	}
	merged := ""
	if t.merged {
		merged = ", merging two notes"
	}
	if t.UsdrFeeNote != nil {
		s.actor(from, "usdr", from.usdr.balance().String()+" USDr")
		s.proto("relayerUsdrIncome", e.relayerIncome().String()+" USDr")
		s.done("%s paid %s %s privately%s and the relayer %s USDr in one transaction (proof %d ms, relay %d ms, gas %s) — %s change back",
			from.Name, to.Name, amount, merged, e.usdrFixedFee, t.proveMs, t.relayMs, formatNum(t.tx.GasUsed), t.change)
		return nil
	}
	s.done("%s paid %s %s privately%s (proof %d ms, relay %d ms, gas %s) — %s change back",
		from.Name, to.Name, amount, merged, t.proveMs, t.relayMs, formatNum(t.tx.GasUsed), t.change)
	return nil
}

// trialScan describes a scan without tags: every note on chain tried.
func trialScan(notes int) string {
	return fmt.Sprintf("%d notes trial-decrypted", notes)
}

// Transfer makes a manual transfer between registered actors, by name, after
// Run has completed, with the run's fee setting. It reports through the sink
// as its own step; a transfer it can tell will fail it refuses with
// ErrRefused before proving.
func (e *Engine) Transfer(ctx context.Context, from, to string, amount int64) error {
	if e.chain == nil || len(e.actors) == 0 {
		return refused("run the scenario first")
	}
	st := &step{e: e, id: fmt.Sprintf("manual_%d", len(e.transfers)+1), label: "Manual transfer"}
	st.start(fmt.Sprintf("%s pays %s %d privately…", from, to, amount))
	if err := e.payByName(ctx, st, Transfer{From: from, To: to, Amount: amount}); err != nil {
		if errors.Is(err, ErrRefused) {
			st.log("", "✗ %v", err)
		}
		e.sink.Emit(Event{Kind: StepFailed, Step: st.id, Label: st.label, Msg: err.Error()})
		return err
	}
	return nil
}

// simulateScript checks the script against the fan-out's payouts before
// anything runs: each transfer must name two different registered actors and
// be payable from one of the payer's notes, or two merged, at that point.
func (o Options) simulateScript() error {
	purses := map[string]*purse{"Sender": {{amount: big.NewInt(o.ShieldTotal)}}}
	for _, name := range participantNames {
		purses[name] = &purse{}
	}
	pay := func(from, to string, amount *big.Int) error {
		fp, tp := purses[from], purses[to]
		if fp == nil || tp == nil || from == to {
			return refused("%s → %s does not name two different participants", from, to)
		}
		if amount.Sign() <= 0 {
			return refused("the amount must be positive, got %s", amount)
		}
		ins, err := fp.inputs(from, "RaylsERC20", amount)
		if err != nil {
			return err
		}
		for _, n := range ins {
			n.spent = true
		}
		*fp = append(*fp, &note{amount: new(big.Int).Sub(total(ins), amount)})
		*tp = append(*tp, &note{amount: amount})
		return nil
	}
	for i, p := range o.Payouts {
		if err := pay("Sender", participantNames[i], big.NewInt(p)); err != nil {
			return err
		}
	}
	for i, t := range o.Script {
		if err := pay(t.From, t.To, big.NewInt(t.Amount)); err != nil {
			return fmt.Errorf("scripted transfer %d: %w", i+1, err)
		}
	}
	return nil
}
