package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"enygma_retail_payments/layout"
	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// tokenId is the retail ERC20 vault's token id.
var tokenId = big.NewInt(0)

// erc20VaultId and usdrVaultId are the retail ERC20 and USDr vaults' ids on
// EnygmaDvp (scripts/cmd/init registers them in that order).
var (
	erc20VaultId = big.NewInt(0)
	usdrVaultId  = big.NewInt(1)
)

var (
	commitmentTopic    = crypto.Keccak256Hash([]byte("Commitment(uint256,uint256)"))
	paymentTopic       = crypto.Keccak256Hash([]byte("Payment(uint256,uint256,bytes,bytes)"))
	encryptedNoteTopic = crypto.Keccak256Hash([]byte("EncryptedNote(uint256,uint256,bytes,bytes)"))
)

// shield mints the sender's public RaylsERC20 and deposits all of it into
// one note the sender owns, encrypted to the sender's own view key.
func (e *Engine) shield(ctx context.Context, s *step, opts Options) error {
	c, issuer, sender := e.chain, e.issuer(), e.sender()
	total := big.NewInt(opts.ShieldTotal)

	if _, err := c.transact(ctx, issuer.keyHex, c.erc20, "mint", sender.Address, total); err != nil {
		return err
	}
	s.log("chain", "RaylsERC20.mint(%s) → Sender (public)", total)
	s.actor(sender, "public", total.String()+" RaylsERC20")

	if _, err := c.transact(ctx, sender.keyHex, c.erc20, "approve", c.vaultAddr, total); err != nil {
		return err
	}
	n, receipt, err := c.depositNote(ctx, sender.keyHex, c.vault, c.vaultAddr, sender, total)
	if err != nil {
		return err
	}
	sender.notes = append(sender.notes, n)

	s.log("chain", "Erc20CoinVault.depositV2(%s) → block %d  gas %s", total, receipt.BlockNumber, formatNum(receipt.GasUsed))
	s.log("key", "sender's note commitment 0x%s…", n.commitment.Text(16)[:16])
	s.proto("shieldGas", fmt.Sprint(receipt.GasUsed))
	s.actor(sender, "public", "0 RaylsERC20")
	s.actor(sender, "note", "0x"+n.commitment.Text(16))
	s.actor(sender, "balance", sender.notes.balance().String()+" RaylsERC20")
	s.done("Sender shielded %s RaylsERC20 into one private note (block %d)", total, receipt.BlockNumber)
	return nil
}

// depositNote deposits amount from payerKeyHex's account (which must have
// approved vault for it) into a note owner can open, encrypted to owner's
// view key, and returns the note.
func (c *chain) depositNote(ctx context.Context, payerKeyHex string, vault *bind.BoundContract, vaultAddr common.Address,
	owner *Actor, amount *big.Int) (*note, *ethtypes.Receipt, error) {
	ss, capsule, err := rpcore.Encapsulate(owner.View.EncapsKey)
	if err != nil {
		return nil, nil, fmt.Errorf("encapsulate: %w", err)
	}
	saltB, err := rpcore.DerivePaymentSalt(ss)
	if err != nil {
		return nil, nil, err
	}
	encKey, err := rpcore.DerivePaymentKey(ss)
	if err != nil {
		return nil, nil, err
	}
	salt := rpcore.SaltBToField(saltB)
	commitment, err := rpcore.Erc20CommitmentV2(owner.Spend.PublicKey, salt, amount, tokenId)
	if err != nil {
		return nil, nil, err
	}
	ctxt, err := rpcore.EncryptPayload(encKey, tokenId, amount)
	if err != nil {
		return nil, nil, err
	}
	receipt, err := c.transact(ctx, payerKeyHex, vault, "depositV2",
		[]*big.Int{amount, owner.Spend.PublicKey, salt, tokenId}, capsule, ctxt)
	if err != nil {
		return nil, nil, err
	}
	// The vault computes the commitment on-chain; it must be the one we can open.
	if !hasCommitmentLog(receipt.Logs, vaultAddr, commitment) {
		return nil, nil, fmt.Errorf("depositV2 did not insert %s's commitment 0x%s", owner.Name, commitment.Text(16))
	}
	return &note{commitment: commitment, salt: salt, amount: amount}, receipt, nil
}

// payFanOut pays participant p their amount from the sender's current note
// with a 1-in/2-out Payment transfer through the relayer, change back to the
// sender. With payUsdrFee it also pays the relayer's USDr fee from the sender's
// USDr note in the same transaction, USDr change back to the sender. It
// returns once the transfer is mined, so the next step's change notes are
// already in the trees.
func (e *Engine) payFanOut(ctx context.Context, s *step, n int, p *Actor, amount *big.Int, payUsdrFee bool) error {
	c, sender := e.chain, e.sender()
	in := sender.notes.unspent()
	if len(in) != 1 {
		return fmt.Errorf("sender holds %d unspent notes, want 1", len(in))
	}
	prev := in[0]
	change := new(big.Int).Sub(prev.amount, amount)
	if change.Sign() < 0 {
		return fmt.Errorf("sender's note holds %s, cannot pay %s", prev.amount, amount)
	}

	tree, err := c.vaultTree(ctx, c.vaultAddr)
	if err != nil {
		return err
	}
	proof, err := tree.GenerateProof(prev.commitment)
	if err != nil {
		return fmt.Errorf("merkle proof for the sender's note: %w", err)
	}
	s.log("key", "rebuilt vault tree from %d Commitment events, root 0x%s…", len(tree.GetLeaves()), proof.Root.Text(16)[:12])

	proveStart := time.Now()
	res, err := rpcore.NewPaymentClient(e.cfg.GnarkURL).BoundPaymentProof(
		new(big.Int).SetBytes(c.vaultAddr.Bytes()),
		big.NewInt(0),
		[]*big.Int{prev.amount},
		[]rpcore.KeyPair{{PrivateKey: sender.Spend.PrivateKey, PublicKey: sender.Spend.PublicKey}},
		[]*big.Int{prev.salt},
		[]*big.Int{amount, change},
		[]*big.Int{p.Spend.PublicKey, sender.Spend.PublicKey},
		[][]byte{p.View.EncapsKey, sender.View.EncapsKey},
		merkleDepth,
		[]*rpcore.MerkleProof{proof},
		[]*big.Int{big.NewInt(int64(proof.TreeNumber))},
		tokenId,
	)
	proveMs := time.Since(proveStart).Milliseconds()
	if err != nil {
		return fmt.Errorf("prove payment: %w", err)
	}
	stmt := res.ContractStatement()
	s.log("zk", "Payment proof in %d ms — nullifier 0x%s…", proveMs, stmt[layout.PaymentNullifiersOffset].Text(16)[:12])

	var fee *usdrFeeLeg
	if payUsdrFee {
		feeStart := time.Now()
		if fee, err = e.proveUsdrFee(ctx, sender); err != nil {
			return err
		}
		s.log("zk", "UsdrFee proof in %d ms — %s USDr fee note to the relayer, %s USDr change back to the sender",
			time.Since(feeStart).Milliseconds(), e.usdrFixedFee, fee.change.amount)
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
		return err
	}
	receipt, err := c.client.TransactionReceipt(ctx, common.HexToHash(tx.TxHash))
	if err != nil {
		return fmt.Errorf("receipt for %s: %w", tx.TxHash, err)
	}
	if receipt.Status != ethtypes.ReceiptStatusSuccessful {
		return fmt.Errorf("payment %s reverted", tx.TxHash)
	}

	prev.spent = true
	changeNote := &note{commitment: stmt[layout.PaymentCommitmentsOutOffset+1], salt: res.SaltA, amount: change}
	if err := changeNote.check(sender); err != nil {
		return err
	}
	sender.notes = append(sender.notes, changeNote)

	settled := Settlement{To: p.Name, TxHash: common.HexToHash(tx.TxHash)}
	method, feeCol := "payment", "—"
	if fee != nil {
		if !hasCommitmentLog(receipt.Logs, c.usdrVaultAddr, fee.feeNote) {
			return fmt.Errorf("paymentWithUsdrFee %s did not insert the relayer's fee note", tx.TxHash)
		}
		fee.in.spent = true
		sender.usdr = append(sender.usdr, fee.change)
		settled.UsdrFeeNote = fee.feeNote
		method, feeCol = "paymentWithUsdrFee", e.usdrFixedFee.String()+" USDr"
	}
	e.fanOut = append(e.fanOut, settled)

	row, _ := json.Marshal(map[string]any{
		"n": n, "to": p.Name, "amount": amount.String(), "fee": feeCol,
		"proveMs": proveMs, "relayMs": relayMs, "gas": tx.GasUsed, "block": tx.BlockNumber, "tx": tx.TxHash,
	})
	s.proto("fanout", string(row))
	s.log("chain", "EnygmaDvp.%s() via relayer → block %d  gas %s  round-trip %d ms", method, tx.BlockNumber, formatNum(tx.GasUsed), relayMs)
	s.actor(sender, "note", "0x"+changeNote.commitment.Text(16))
	s.actor(sender, "balance", sender.notes.balance().String()+" RaylsERC20")
	if fee != nil {
		s.actor(sender, "usdr", sender.usdr.balance().String()+" USDr")
		s.proto("relayerUsdrIncome", e.relayerIncome().String()+" USDr")
		s.done("Paid %s privately and the relayer %s USDr in one transaction (proof %d ms, relay %d ms, gas %s) — %s change back to the sender",
			p.Name, e.usdrFixedFee, proveMs, relayMs, formatNum(tx.GasUsed), change)
		return nil
	}
	s.done("Paid %s privately (proof %d ms, relay %d ms, gas %s) — %s change back to the sender",
		p.Name, proveMs, relayMs, formatNum(tx.GasUsed), change)
	return nil
}

// relayerIncome is the USDr the relayer has been paid in fee notes this run.
func (e *Engine) relayerIncome() *big.Int {
	income := new(big.Int)
	for _, st := range e.fanOut {
		if st.UsdrFeeNote != nil {
			income.Add(income, e.usdrFixedFee)
		}
	}
	return income
}

// discover has each participant scan the chain for notes addressed to them.
func (e *Engine) discover(ctx context.Context, s *step) error {
	events, err := e.chain.noteEvents(ctx, e.chain.vaultAddr, erc20VaultId)
	if err != nil {
		return err
	}
	s.log("chain", "%d encrypted notes on chain (Payment + EncryptedNote events)", len(events))
	for _, p := range e.participants() {
		found := p.scan(events, &p.notes)
		s.log("key", "%s decrypted %d note(s) with their ML-KEM view key, commitments recomputed", p.Name, found)
		s.actor(p, "balance", p.notes.balance().String()+" RaylsERC20")
		if len(p.notes) > 0 {
			s.actor(p, "note", "0x"+p.notes[len(p.notes)-1].commitment.Text(16))
		}
	}
	s.done("Each participant found their payment by scanning the chain with their view key")
	return nil
}

// scan adds to into the notes in events (one vault's) that a's view key
// opens, and returns how many were new. An event that fails to open or to
// match its commitment is someone else's.
func (a *Actor) scan(events []dvpcore.OnChainErc20Event, into *purse) int {
	found := 0
	for _, ev := range events {
		owned, err := dvpcore.ScanForErc20Notes(a.View.DecapsKey, a.Spend.PublicKey, []dvpcore.OnChainErc20Event{ev})
		if err != nil || len(owned) == 0 || into.has(ev.Commitment) {
			continue
		}
		*into = append(*into, &note{commitment: owned[0].Commitment, salt: owned[0].SaltBField, amount: owned[0].Amount})
		found++
	}
	return found
}

func (p purse) has(commitment *big.Int) bool {
	for _, n := range p {
		if n.commitment.Cmp(commitment) == 0 {
			return true
		}
	}
	return false
}

func (p purse) unspent() []*note {
	var out []*note
	for _, n := range p {
		if !n.spent {
			out = append(out, n)
		}
	}
	return out
}

// check confirms owner can open n: its commitment is the one the proof published.
func (n *note) check(owner *Actor) error {
	want, err := rpcore.Erc20CommitmentV2(owner.Spend.PublicKey, n.salt, n.amount, tokenId)
	if err != nil {
		return err
	}
	if want.Cmp(n.commitment) != 0 {
		return fmt.Errorf("%s's note 0x%s does not open to (%s, salt)", owner.Name, n.commitment.Text(16), n.amount)
	}
	return nil
}

// vaultTree rebuilds a vault's Merkle tree from its Commitment events.
func (c *chain) vaultTree(ctx context.Context, vaultAddr common.Address) (*rpcore.MerkleTree, error) {
	logs, err := c.client.FilterLogs(ctx, ethereum.FilterQuery{
		Addresses: []common.Address{vaultAddr},
		Topics:    [][]common.Hash{{commitmentTopic}},
	})
	if err != nil {
		return nil, fmt.Errorf("vault Commitment events: %w", err)
	}
	tree := rpcore.NewMerkleTree(merkleDepth)
	for _, l := range logs {
		if len(l.Topics) >= 3 {
			tree.InsertLeaf(l.Topics[2].Big())
		}
	}
	return tree, nil
}

// noteEvents returns every encrypted note published for one vault:
// EnygmaDvp's Payment events and the vault's EncryptedNote (deposit) events.
func (c *chain) noteEvents(ctx context.Context, vaultAddr common.Address, vaultId *big.Int) ([]dvpcore.OnChainErc20Event, error) {
	vaultIdTopic := common.BigToHash(vaultId)
	logs, err := c.client.FilterLogs(ctx, ethereum.FilterQuery{
		Addresses: []common.Address{c.dvpAddr, vaultAddr},
		Topics:    [][]common.Hash{{paymentTopic, encryptedNoteTopic}, {vaultIdTopic}},
	})
	if err != nil {
		return nil, fmt.Errorf("note events: %w", err)
	}
	args := abiBytesPair()
	var events []dvpcore.OnChainErc20Event
	for _, l := range logs {
		if len(l.Topics) < 3 {
			continue
		}
		vals, err := args.Unpack(l.Data)
		if err != nil || len(vals) != 2 {
			continue
		}
		events = append(events, dvpcore.OnChainErc20Event{
			Commitment: l.Topics[2].Big(),
			CipherText: vals[0].([]byte),
			EncTxData:  vals[1].([]byte),
		})
	}
	return events, nil
}
