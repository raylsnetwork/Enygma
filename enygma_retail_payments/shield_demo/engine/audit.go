package engine

import (
	"bytes"
	"context"
	"crypto/mlkem"
	"encoding/json"
	"fmt"
	"math/big"

	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	tags "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum"
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
}

// auditUser is a registered user as the auditor knows them: their public
// spend key from the registry and the view key it recovered.
type auditUser struct {
	name    string
	spendPk *big.Int
	view    *mlkem.DecapsulationKey768
}

// recoverViewKeys has the auditor open the audit ciphertexts of every user
// in the UserRegistry, keeping the view keys encrypted to it (users of
// earlier runs were encrypted to other auditors), and checking each against
// the view key the user registered. It returns how many registrations were
// encrypted to other auditors.
func (e *Engine) recoverViewKeys() (users []auditUser, others int, err error) {
	c := e.chain
	count, err := rpcore.GetUserCount(c.client, c.registryAddr)
	if err != nil {
		return nil, 0, err
	}
	for i := range count {
		addr, err := rpcore.GetUserAt(c.client, c.registryAddr, i)
		if err != nil {
			return nil, 0, err
		}
		mlKemCt, aesCt, err := rpcore.LookupAuditCiphertexts(c.client, c.registryAddr, addr)
		if err != nil {
			return nil, 0, err
		}
		view, err := rpcore.AuditorDecryptViewKey(e.auditor.DecapsKey, mlKemCt, aesCt)
		if err != nil {
			others++
			continue
		}
		keys, err := rpcore.LookupKeys(c.client, c.registryAddr, addr)
		if err != nil {
			return nil, 0, err
		}
		if !bytes.Equal(view.EncapsulationKey().Bytes(), keys.ViewKey) {
			return nil, 0, fmt.Errorf("%s registered a view key other than the one encrypted to the auditor", e.nameOf(addr))
		}
		users = append(users, auditUser{name: e.nameOf(addr), spendPk: keys.SpendKey, view: view})
	}
	return users, others, nil
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
		s.log("key", "auditor opened %s's audit ciphertexts → ML-KEM view key, matches the registered one", u.name)
	}
	if others > 0 {
		s.log("key", "%d earlier registrations are encrypted to other auditors", others)
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
	payers, err := e.auditPayers(ctx)
	if err != nil {
		return nil, err
	}
	logs, err := c.client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(e.startBlock),
		Addresses: []common.Address{c.dvpAddr, c.vaultAddr, c.usdrVaultAddr},
		Topics:    [][]common.Hash{{paymentTopic, encryptedNoteTopic}},
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
		owner, amount := e.auditOpen(ev)

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
			en.Kind, en.To, en.Amount = AuditTransfer, owner, amount
			en.From = payers[ev.Commitment.String()]
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
	return a, nil
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
// opens for and the amount, or "" if it opens for none.
func (e *Engine) auditOpen(ev dvpcore.OnChainErc20Event) (string, *big.Int) {
	for _, u := range e.auditUsers {
		owned, err := dvpcore.ScanForErc20Notes(u.view, u.spendPk, []dvpcore.OnChainErc20Event{ev})
		if err == nil && len(owned) == 1 {
			return u.name, owned[0].Amount
		}
	}
	return "", nil
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

// auditPayers names the payer of each payment that carries a private tag,
// by its output commitment. With each recipient's view key the auditor opens
// the channels addressed to them, which name the payer who opened each,
// then matches the run's tags to those channels and opens each tagged note.
func (e *Engine) auditPayers(ctx context.Context) (map[string]string, error) {
	c := e.chain
	payers := map[string]string{}
	if c.channelRegistryAddr == (common.Address{}) || c.tagRegistryAddr == (common.Address{}) {
		return payers, nil
	}
	type channel struct {
		tags.Channel
		payer string
	}
	var channels []channel
	for _, u := range e.auditUsers {
		found, _, err := tags.ScanChannelsFromCursor(c.client, c.channelRegistryAddr, u.view,
			&tags.ScanCursor{LastChannelIdx: e.startChannel}, maxChannelScan)
		if err != nil {
			return nil, fmt.Errorf("auditor: scan %s's channels: %w", u.name, err)
		}
		for _, f := range found {
			// The payer names themselves in the channel they open.
			payer := ""
			if common.IsHexAddress(string(f.SenderId)) {
				payer = e.nameOf(common.HexToAddress(string(f.SenderId)))
			}
			channels = append(channels, channel{Channel: tags.Channel{SharedSecret: f.SharedSecret, PkSpend: u.spendPk}, payer: payer})
		}
	}
	if len(channels) == 0 {
		return payers, nil
	}

	logs, err := c.client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(e.startBlock),
		Addresses: []common.Address{c.tagRegistryAddr},
		Topics:    [][]common.Hash{{tagPublishedTopic}},
	})
	if err != nil {
		return nil, fmt.Errorf("TagPublished events: %w", err)
	}
	for _, l := range logs {
		for _, ch := range channels {
			pn, err := openTag(l, ch.Channel)
			if err != nil {
				return nil, fmt.Errorf("auditor: %w", err)
			}
			if pn == nil {
				continue
			}
			// A tag only names the payer of a note the auditor also decrypts
			// from a Payment event, so an unpaid tag names no one.
			cm, err := rpcore.Erc20CommitmentV2(ch.PkSpend, pn.Salt, pn.Amount, pn.TokenId)
			if err != nil {
				return nil, err
			}
			payers[cm.String()] = ch.payer
		}
	}
	return payers, nil
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
	s.done("Auditor saw all %d transactions, named every payer from the tag channels, and reconstructs every user's balances exactly", len(a.Entries))
	return nil
}
