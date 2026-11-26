package engine

import (
	"context"
	"crypto/mlkem"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	tags "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// The assets an AuditEntry moves.
const (
	assetErc20 = "RaylsERC20"
	assetUsdr  = "USDr"
)

// AuditKind is what an audited transaction did.
type AuditKind string

const (
	// AuditShield is a deposit of public RaylsERC20 into a private note.
	AuditShield AuditKind = "shield"
	// AuditUsdrGrant is the issuer's deposit of a USDr note to a user.
	AuditUsdrGrant AuditKind = "usdr grant"
	// AuditTransfer is a private RaylsERC20 payment between users.
	AuditTransfer AuditKind = "transfer"
	// AuditUsdrFee is the USDr fee leg paid to the relayer with a transfer.
	AuditUsdrFee AuditKind = "usdr fee"
)

// paidPrivately reports whether the payer of an entry of kind k paid from
// their private notes (a shield and a grant are paid from public funds).
func (k AuditKind) paidPrivately() bool { return k == AuditTransfer || k == AuditUsdrFee }

// AuditEntry is one transaction as the auditor reconstructs it from the chain.
type AuditEntry struct {
	Kind  AuditKind `json:"kind"`
	Asset string    `json:"asset"`
	// From is the payer; empty when the auditor cannot tell (a transfer
	// without a private tag: only its recipient's note is published).
	From   string      `json:"from"`
	To     string      `json:"to"`
	Amount *big.Int    `json:"amount"`
	TxHash common.Hash `json:"tx"`
	Block  uint64      `json:"block"`
}

// Audit is what the auditor sees of the run, from the chain and the view
// keys it recovers from the UserRegistry alone.
type Audit struct {
	// Users are the registered users whose view keys the auditor recovered.
	Users   []string     `json:"users"`
	Entries []AuditEntry `json:"entries"`
	// Received is the RaylsERC20 each user was paid, shield included.
	Received map[string]*big.Int `json:"received"`
	// Balances and UsdrBalances are each user's private balances, received
	// minus paid; nil unless the auditor could name every transfer's payer.
	// A payer's change note is never published, so the auditor reconstructs
	// what they kept from what they paid.
	Balances     map[string]*big.Int `json:"balances"`
	UsdrBalances map[string]*big.Int `json:"usdrBalances"`
	// Inferred says the auditor could not name the payers and inferred the
	// balances by conservation instead (a walkthrough, see conserve).
	Inferred bool `json:"inferred,omitempty"`

	// opened are the notes the auditor decrypted, one per entry but the fee
	// legs, which carry no ciphertext; the audit hop shows them.
	opened []DecryptedNote
}

// auditUser is a registered user as the auditor knows them: their public
// spend key from the registry and the view key it recovered.
type auditUser struct {
	name    string
	spendPk *big.Int
	view    *mlkem.DecapsulationKey768
	// audit are the audit ciphertexts it opened the view key from.
	audit AuditCiphertexts
}

// recoverViewKeys has the auditor check the audit ciphertexts of every
// registration in the UserRegistry and recover the view keys they open to
// (see sortAuditChecks).
func (e *Engine) recoverViewKeys() ([]auditUser, unreadable, error) {
	regs, err := rpcore.AuditRegistrations(e.chain.client, e.chain.registryAddr, e.auditor.DecapsKey)
	if err != nil {
		return nil, unreadable{}, err
	}
	return e.sortAuditChecks(regs)
}

// unreadable names the accounts outside the run whose notes the auditor
// cannot read: the contract checks only the audit ciphertexts' lengths, so
// a registration's may not open (an earlier run's user, encrypted to another
// auditor, or a user opting out: the auditor cannot tell which), or open to
// a view key other than the one registered.
type unreadable struct {
	unopened, mismatched []string
}

// sortAuditChecks keeps the view key of each registration whose audit
// ciphertexts open to the view key it registered. A run's own user whose
// ciphertexts do not stops the run; any other account's is named.
func (e *Engine) sortAuditChecks(regs []rpcore.AuditedRegistration) (users []auditUser, others unreadable, err error) {
	for _, r := range regs {
		name, own := e.nameOf(r.User), e.registers(r.User)
		switch {
		case r.Opens() && r.Matches:
			users = append(users, auditUser{name: name, spendPk: r.Keys.SpendKey, view: r.ViewKey, audit: auditCiphertexts(r.MlKemCt, r.AesCt)})
		case own && !r.Opens():
			return nil, unreadable{}, fmt.Errorf("%s's audit ciphertexts do not open to the auditor", name)
		case own:
			return nil, unreadable{}, fmt.Errorf("%s registered a view key other than the one encrypted to the auditor", name)
		case !r.Opens():
			others.unopened = append(others.unopened, name)
		default:
			others.mismatched = append(others.mismatched, name)
		}
	}
	return users, others, nil
}

// registers says addr is one of the run's users it registers.
func (e *Engine) registers(addr common.Address) bool {
	for _, a := range e.registrable() {
		if a.Address == addr {
			return true
		}
	}
	return false
}

// nameOf is the run's name for an account, or its address.
func (e *Engine) nameOf(addr common.Address) string {
	for _, a := range e.actors {
		if a.Address == addr {
			return a.Name
		}
	}
	return addr.Hex()
}

// auditorKeysStep is the run's step after registration: the auditor
// recovers every registered user's view key.
func (e *Engine) auditorKeysStep(ctx context.Context, s *step) error {
	users, others, err := e.recoverViewKeys()
	if err != nil {
		return err
	}
	if len(users) != len(e.registrable()) {
		return fmt.Errorf("auditor recovered %d view keys, want %d", len(users), len(e.registrable()))
	}
	e.auditUsers = users
	for _, u := range users {
		s.hop(NodeUserRegistry, NodeAuditor, "recover view key", "opens "+u.name+"'s audit ciphertexts",
			HopDetail{Kind: DetailViewKeyRecovery, Recovery: &ViewKeyRecovery{Owner: u.name, Audit: u.audit,
				ViewPk: bytesHex(u.view.EncapsulationKey().Bytes()), ViewKey: shortSecret(bytesHex(u.view.Bytes()))}})
		s.log("key", "auditor opened %s's audit ciphertexts → ML-KEM view key, matches the registered one", u.name)
	}
	if n := len(others.unopened); n > 0 {
		s.log("key", "auditor cannot open the audit ciphertexts of %d other registrations (earlier runs' users, encrypted to other auditors, or users opting out of audit): %s",
			n, strings.Join(others.unopened, ", "))
	}
	if n := len(others.mismatched); n > 0 {
		s.log("key", "%d other registrations' audit ciphertexts open to a view key other than the one registered: %s",
			n, strings.Join(others.mismatched, ", "))
	}
	s.done("Auditor recovered the view keys of the sender and %d participants from the UserRegistry", K)
	return nil
}

// Audit is the auditor's view of the run so far: with the view keys it
// recovered, it scans both vaults from the run's first block and decrypts
// every note addressed to a user, and with tags on it opens each payment's
// channel to name its payer.
func (e *Engine) Audit(ctx context.Context) (*Audit, error) {
	if e.auditUsers == nil {
		return nil, fmt.Errorf("the auditor has not recovered the users' view keys yet")
	}
	c := e.chain
	channels, err := e.auditChannels()
	if err != nil {
		return nil, err
	}
	logs, err := c.logsSince(ctx, e.startBlock, ethereum.FilterQuery{
		Addresses: []common.Address{c.dvpAddr, c.vaultAddr, c.usdrVaultAddr},
		Topics:    [][]common.Hash{{paymentTopic, encryptedNoteTopic, channelPaymentTopic}},
	})
	if err != nil {
		return nil, fmt.Errorf("note events: %w", err)
	}

	a := &Audit{Users: []string{}, Entries: []AuditEntry{}, Received: map[string]*big.Int{}}
	for _, u := range e.auditUsers {
		a.Users = append(a.Users, u.name)
		a.Received[u.name] = new(big.Int)
	}
	// payerIn holds each audited transfer's payer by transaction, and feeLegs
	// the USDr Payment events no view key opens: the fee leg of a transfer
	// in the same transaction, once that transfer is audited.
	payerIn := map[common.Hash]string{}
	var feeLegs []AuditEntry
	args := abiBytesPair()
	for _, l := range logs {
		if len(l.Topics) < 3 {
			continue
		}
		if l.Topics[0] == channelPaymentTopic {
			// A transfer over a tag channel: the auditor opens it with the
			// channel's key, and the channel names its payer.
			ch, pn, f := auditChannelPayment(l, channels)
			if ch == nil || pn.TokenId.Cmp(tokenId) != 0 {
				continue // a payment to an earlier run's user, or junk
			}
			en := AuditEntry{Kind: AuditTransfer, Asset: assetErc20, From: ch.payer, To: ch.payee, Amount: pn.Amount, TxHash: l.TxHash, Block: l.BlockNumber}
			a.Entries = append(a.Entries, en)
			a.opened = append(a.opened, DecryptedNote{Owner: ch.payee, Asset: assetErc20, Kind: string(AuditTransfer), Tx: l.TxHash.Hex(),
				Commitment: fieldHex(f.Commitment), Via: viaTag, Sealed: bytesHex(f.Ctxt),
				Amount: pn.Amount.String(), Salt: fieldHex(pn.Salt), Token: fieldHex(pn.TokenId)})
			payerIn[l.TxHash] = en.From
			a.Received[en.To].Add(a.Received[en.To], en.Amount)
			continue
		}
		vals, err := args.Unpack(l.Data)
		if err != nil || len(vals) != 2 {
			continue
		}
		ev := dvpcore.OnChainErc20Event{Commitment: l.Topics[2].Big(), CipherText: vals[0].([]byte), EncTxData: vals[1].([]byte)}
		usdr := l.Topics[1].Big().Cmp(usdrVaultId) == 0
		asset := assetErc20
		if usdr {
			asset = assetUsdr
		}
		en := AuditEntry{Asset: asset, TxHash: l.TxHash, Block: l.BlockNumber}
		owner, owned := e.auditOpen(ev)
		var amount *big.Int
		if owner != "" {
			amount = owned.Amount
		}

		switch {
		case l.Topics[0] == encryptedNoteTopic && owner != "":
			en.Kind, en.To, en.Amount = AuditShield, owner, amount
			if usdr {
				en.Kind = AuditUsdrGrant
			}
			if en.From, err = e.txSender(ctx, l.TxHash); err != nil {
				return nil, err
			}
		case l.Topics[0] == paymentTopic && !usdr && owner != "":
			// A transfer without a tag channel: its payer stays hidden.
			en.Kind, en.To, en.Amount = AuditTransfer, owner, amount
		case l.Topics[0] == paymentTopic && usdr && owner == "":
			// The fee leg's note is the relayer's, published with no
			// ciphertext; its amount is the governance-set fixed fee.
			en.Kind, en.To, en.Amount = AuditUsdrFee, "Relayer", e.usdrFixedFee
			feeLegs = append(feeLegs, en)
			continue
		default:
			continue // a note of an earlier run's users
		}
		a.Entries = append(a.Entries, en)
		n := decrypted(owner, asset, ev, owned)
		n.Kind, n.Tx = string(en.Kind), l.TxHash.Hex()
		a.opened = append(a.opened, n)
		if en.Kind == AuditTransfer {
			payerIn[l.TxHash] = en.From
		}
		if en.Asset == assetErc20 {
			a.Received[en.To].Add(a.Received[en.To], en.Amount)
		}
	}
	for _, en := range feeLegs {
		if payer, ok := payerIn[en.TxHash]; ok {
			en.From = payer
			a.Entries = append(a.Entries, en)
		}
	}
	a.reconstructBalances()
	if a.Balances == nil && e.opts.Walkthrough {
		a.conserve()
	}
	return a, nil
}

// conserve infers the balances of a walkthrough, which has no tags to name
// payers by: every note in it comes from one user's shield, and only that
// user pays. So they keep what they shielded less what the others received,
// and the USDr they were granted less each relayer fee; each other user
// holds what they received. It infers none if more than one user shielded.
func (a *Audit) conserve() {
	shielder := ""
	for _, en := range a.Entries {
		if en.Kind == AuditShield {
			if shielder != "" && en.To != shielder {
				return
			}
			shielder = en.To
		}
	}
	if shielder == "" {
		return
	}
	a.Balances, a.UsdrBalances = map[string]*big.Int{}, map[string]*big.Int{}
	for _, u := range a.Users {
		a.Balances[u], a.UsdrBalances[u] = new(big.Int).Set(a.Received[u]), new(big.Int)
	}
	for _, en := range a.Entries {
		switch {
		case en.Kind == AuditUsdrFee: // a USDr entry too, so before the next case
			a.UsdrBalances[shielder].Sub(a.UsdrBalances[shielder], en.Amount)
		case en.Asset == assetUsdr:
			if to := a.UsdrBalances[en.To]; to != nil && !en.Kind.paidPrivately() {
				to.Add(to, en.Amount)
			}
		case en.Kind == AuditTransfer && en.To != shielder:
			a.Balances[shielder].Sub(a.Balances[shielder], en.Amount)
		}
	}
	a.Inferred = true
}

// reconstructBalances sets each user's balances to what they received less
// what they paid, if every payment's payer is known.
func (a *Audit) reconstructBalances() {
	for _, en := range a.Entries {
		if en.Kind.paidPrivately() && en.From == "" {
			return
		}
	}
	a.Balances, a.UsdrBalances = map[string]*big.Int{}, map[string]*big.Int{}
	for _, u := range a.Users {
		a.Balances[u], a.UsdrBalances[u] = new(big.Int), new(big.Int)
	}
	for _, en := range a.Entries {
		balances := a.Balances
		if en.Asset == assetUsdr {
			balances = a.UsdrBalances
		}
		if to := balances[en.To]; to != nil {
			to.Add(to, en.Amount)
		}
		if from := balances[en.From]; from != nil && en.Kind.paidPrivately() {
			from.Sub(from, en.Amount)
		}
	}
}

// auditOpen tries every recovered view key on ev and returns the user it
// opens for and the note it opens to, or "" if it opens for none.
func (e *Engine) auditOpen(ev dvpcore.OnChainErc20Event) (string, dvpcore.OwnedErc20Note) {
	for _, u := range e.auditUsers {
		owned, err := dvpcore.ScanForErc20Notes(u.view, u.spendPk, []dvpcore.OnChainErc20Event{ev})
		if err == nil && len(owned) == 1 {
			return u.name, owned[0]
		}
	}
	return "", dvpcore.OwnedErc20Note{}
}

// txSender names the account that signed tx: a deposit is public.
func (e *Engine) txSender(ctx context.Context, hash common.Hash) (string, error) {
	tx, _, err := e.chain.client.TransactionByHash(ctx, hash)
	if err != nil {
		return "", fmt.Errorf("transaction %s: %w", hash.Hex(), err)
	}
	from, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(e.chain.chainID), tx)
	if err != nil {
		return "", err
	}
	return e.nameOf(from), nil
}

// auditChannels is every tag channel to a user the auditor recovered the
// view key of, opened since the run started, each with the payer it names:
// the payer names themselves in the channel they open.
func (e *Engine) auditChannels() ([]auditChannel, error) {
	c := e.chain
	if c.channelRegistryAddr == (common.Address{}) {
		return nil, nil
	}
	var channels []auditChannel
	for _, u := range e.auditUsers {
		found, _, err := tags.ScanChannelsFromCursor(c.client, c.channelRegistryAddr, u.view,
			&tags.ScanCursor{LastChannelIdx: e.startChannel}, maxChannelScan)
		if err != nil {
			return nil, fmt.Errorf("auditor: scan %s's channels: %w", u.name, err)
		}
		for _, f := range found {
			payer := ""
			if common.IsHexAddress(string(f.SenderId)) {
				payer = e.nameOf(common.HexToAddress(string(f.SenderId)))
			}
			channels = append(channels, auditChannel{Channel: tags.Channel{SharedSecret: f.SharedSecret, PkSpend: u.spendPk}, payee: u.name, payer: payer})
		}
	}
	return channels, nil
}

// auditChannel is a tag channel the auditor opened, its payee and the payer
// it names.
type auditChannel struct {
	tags.Channel
	payee, payer string
}

// auditChannelPayment opens l, a ChannelPayment event, if its tag is one of
// channels' for its block_n: the channel it rides on, the note its sealed
// note opens to, checked against the commitment it settled, and the payment
// as found. A payment whose tag matches no channel, or whose note does not
// open (anyone with a channel can pay junk on it), opens to nothing.
func auditChannelPayment(l ethtypes.Log, channels []auditChannel) (*auditChannel, *tags.PaymentNote, tags.FoundChannelPayment) {
	none := tags.FoundChannelPayment{}
	// Only a RaylsERC20 vault payment is a transfer the audit counts.
	if len(l.Topics) != 4 || l.Topics[1].Big().Cmp(erc20VaultId) != 0 {
		return nil, nil, none
	}
	vals, err := channelPaymentData().Unpack(l.Data)
	if err != nil || len(vals) != 2 {
		return nil, nil, none
	}
	block := l.Topics[2].Big().Uint64()
	for i := range channels {
		ch := &channels[i]
		t, err := tags.ChannelTag(ch.SharedSecret, block, ch.PkSpend)
		if err != nil || common.Hash(t) != l.Topics[3] {
			continue
		}
		f := tags.FoundChannelPayment{BlockNumber: block, Commitment: vals[0].(*big.Int), Ctxt: vals[1].([]byte),
			SharedSecret: ch.SharedSecret, PkSpend: ch.PkSpend}
		pn, err := f.Open()
		if err != nil {
			return nil, nil, none
		}
		return ch, pn, f
	}
	return nil, nil, none
}

// channelPaymentData decodes a ChannelPayment event's data: the commitment
// and the sealed note.
func channelPaymentData() abi.Arguments {
	u256, _ := abi.NewType("uint256", "", nil)
	bs, _ := abi.NewType("bytes", "", nil)
	return abi.Arguments{{Type: u256}, {Type: bs}}
}

// publishAudit sends the auditor's current view to the UI.
func (e *Engine) publishAudit(ctx context.Context, s *step) error {
	a, err := e.Audit(ctx)
	if err != nil {
		return err
	}
	view := struct {
		*Audit
		// Own and OwnUsdr are each user's balances from their own scans,
		// beside the auditor's.
		Own     map[string]string `json:"own"`
		OwnUsdr map[string]string `json:"ownUsdr"`
	}{Audit: a, Own: map[string]string{}, OwnUsdr: map[string]string{}}
	for _, u := range e.registrable() {
		view.Own[u.Name] = u.notes.balance().String()
		view.OwnUsdr[u.Name] = u.usdr.balance().String()
	}
	data, err := json.Marshal(view)
	if err != nil {
		return err
	}
	s.proto("audit", string(data))
	return nil
}

// reconcile is the run's last step: the auditor's balances must equal each
// user's own, or, without tags, the auditor reports what it could see.
func (e *Engine) reconcile(ctx context.Context, s *step) error {
	a, err := e.Audit(ctx)
	if err != nil {
		return err
	}
	s.log("key", "auditor decrypted %d transactions across both vaults with the recovered view keys", len(a.Entries))
	details := []HopDetail{decryption(a.opened)}
	// With tags, each payment's note is sealed under its tag channel's key,
	// with no capsule, so the audit is the run's first hop a view key opens
	// a note's ciphertext by: it carries the note encryption card, a
	// walkthrough's opening the Sender's shielded note with the Sender's
	// view key, which the auditor recovered, a full run's by name only.
	if e.opts.Tags {
		d := formulaNamed(FormulaNoteEncryption, NodeVault, 0)
		if e.opts.Walkthrough {
			sender := e.sender()
			if len(sender.notes) == 0 {
				return errors.New("note encryption card: the Sender has not shielded")
			}
			if d, err = e.chain.noteEncryptionFormula(ctx, sender, sender.notes[0].commitment); err != nil {
				return err
			}
		}
		details = append(details, d)
	}
	s.hop(NodeVault, NodeAuditor, "audit", fmt.Sprintf("opens %d notes with the %d view keys it recovered", len(a.opened), len(a.Users)), details...)
	if a.Balances == nil {
		s.done("Auditor sees all %d transactions and what each user received; without tags the payers stay hidden, so it cannot reconstruct balances", len(a.Entries))
		return nil
	}
	for _, u := range e.registrable() {
		if own := u.notes.balance(); a.Balances[u.Name].Cmp(own) != 0 {
			return fmt.Errorf("auditor reconstructs %s RaylsERC20 for %s, who scans %s", a.Balances[u.Name], u.Name, own)
		}
		if own := u.usdr.balance(); a.UsdrBalances[u.Name].Cmp(own) != 0 {
			return fmt.Errorf("auditor reconstructs %s USDr for %s, who scans %s", a.UsdrBalances[u.Name], u.Name, own)
		}
	}
	if a.Inferred {
		s.done("Auditor saw all %d transactions and, with no tags to name payers, inferred every user's balance by conservation: each matches their own scan", len(a.Entries))
		return nil
	}
	s.done("Auditor saw all %d transactions, named every payer from the tag channels, and reconstructs every user's balances exactly", len(a.Entries))
	return nil
}
