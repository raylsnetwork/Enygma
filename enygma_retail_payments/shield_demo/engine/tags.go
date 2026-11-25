package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"

	tags "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// channelPaymentTopic is EnygmaDvp's ChannelPayment event.
var channelPaymentTopic = crypto.Keccak256Hash([]byte("ChannelPayment(uint256,uint256,bytes32,uint256,bytes)"))

// tagState is an actor's private-tag state with tags on: the channels they
// opened to recipients, the channels they found addressed to them, and how
// far they have scanned each registry.
type tagState struct {
	// out holds the shared secret of the channel this actor opened to each recipient.
	out map[*Actor][]byte
	// in are the channels to this actor that their channel scans found.
	in []tags.Channel
	// nextChannel is the first channel record not yet scanned; lastBlock
	// the last block_n whose tags have been: the payee's scan computes,
	// for each block after it, the tag of every channel they hold.
	nextChannel uint64
	lastBlock   uint64
}

// startTags sets every registered actor's scan cursors to the registries'
// current ends: they just registered, so nothing earlier is addressed to them.
func (e *Engine) startTags(ctx context.Context) error {
	c := e.chain
	channels, err := tags.GetChannelCount(c.client, c.channelRegistryAddr)
	if err != nil {
		return err
	}
	block, err := c.client.BlockNumber(ctx)
	if err != nil {
		return err
	}
	for _, a := range e.registrable() {
		a.tags = tagState{out: map[*Actor][]byte{}, nextChannel: channels, lastBlock: block}
	}
	return nil
}

// pair is a payer and the recipient they pay.
type pair struct{ from, to *Actor }

// channelPairs are the pairs the run will transact between: the sender to
// each participant, then each scripted pair once.
func (e *Engine) channelPairs() ([]pair, error) {
	var pairs []pair
	seen := map[pair]bool{}
	add := func(from, to *Actor) {
		if p := (pair{from, to}); !seen[p] {
			seen[p] = true
			pairs = append(pairs, p)
		}
	}
	for _, p := range e.participants() {
		add(e.sender(), p)
	}
	for _, t := range e.opts.Script {
		from, err := e.actorNamed(t.From)
		if err != nil {
			return nil, err
		}
		to, err := e.actorNamed(t.To)
		if err != nil {
			return nil, err
		}
		add(from, to)
	}
	return pairs, nil
}

// openChannels opens a tag channel for every pair the run transacts
// between, then has each recipient find theirs by scanning the channel
// registry with their view key.
func (e *Engine) openChannels(ctx context.Context, s *step) error {
	if err := e.startTags(ctx); err != nil {
		return err
	}
	pairs, err := e.channelPairs()
	if err != nil {
		return err
	}
	for _, p := range pairs {
		if err := e.openChannel(ctx, s, p.from, p.to); err != nil {
			return err
		}
	}
	for _, a := range e.registrable() {
		if _, err := e.scanChannels(ctx, s, a); err != nil {
			return err
		}
	}
	s.done("%d tag channels opened through the relayer; each recipient found theirs with their view key", len(pairs))
	return nil
}

// openChannel has from encapsulate a shared secret to to's view key and
// publish the channel through the relayer, so the registry shows the
// relayer as its sender.
func (e *Engine) openChannel(ctx context.Context, s *step, from, to *Actor) error {
	c := e.chain
	users, err := tags.GetUserCount(c.client, c.registryAddr)
	if err != nil {
		return err
	}
	idx, err := tags.GetRecipientIndex(c.client, c.registryAddr, to.Address)
	if err != nil {
		return err
	}
	msg, senderId := []byte("Payment channel ready"), []byte(from.Address.Hex())
	ss, c1, c2, bitmap, err := tags.PrepareChannelSetup(to.View.EncapsKey, msg, senderId, tags.PrivacyFull, users, idx, nil)
	if err != nil {
		return fmt.Errorf("channel %s → %s: %w", from.Name, to.Name, err)
	}
	resp, ex, err := e.relayer.channel(ctx, c1, c2, bitmap)
	if err != nil {
		s.hopFailed(ActorNode(from.ID), NodeRelayer, "open channel", err)
		return fmt.Errorf("channel %s → %s: %w", from.Name, to.Name, err)
	}
	from.tags.out[to] = ss
	receipt, err := e.chain.receiptOf(ctx, common.HexToHash(resp.TxHash))
	if err != nil {
		return fmt.Errorf("channel %s → %s: %w", from.Name, to.Name, err)
	}
	details := []HopDetail{ex}
	// The Sender's tag channel to payment 1's payee carries the tag channel
	// setup card: a walkthrough's with its values, checked against the
	// record, a full run's by name only.
	if e.carriesTagChannelCards(from, to) {
		d := formulaNamed(FormulaTagChannelSetup, NodeTagChannelRegistry, 0)
		if e.opts.Walkthrough {
			if d, err = c.tagChannelSetupFormula(ctx, from, to, resp.ChannelIdx, ss, c1, c2, bitmap, msg, senderId); err != nil {
				return err
			}
		}
		details = append(details, d)
	}
	s.hop(ActorNode(from.ID), NodeRelayer, "open channel", fmt.Sprintf("a shared secret encapsulated to %s's view key", to.Name), details...)
	s.hop(NodeRelayer, NodeTagChannelRegistry, "openChannel", fmt.Sprintf("channel #%d, block %d, gas %s; the relayer is its on-chain sender", resp.ChannelIdx, resp.BlockNumber, formatNum(resp.GasUsed)), receipt)
	s.log("tag", "TagChannelRegistry.openChannel(%s → %s) via relayer → channel #%d  block %d  gas %s",
		from.Name, to.Name, resp.ChannelIdx, resp.BlockNumber, formatNum(resp.GasUsed))
	return nil
}

// carriesTagChannelCards says whether the tag channel from from to to is the
// Sender's to payment 1's payee, the first participant: the one whose
// setup, detection, first tag (payment 1's) and that tag's payload carry
// the tag channel's four formula cards.
func (e *Engine) carriesTagChannelCards(from, to *Actor) bool {
	return from == e.sender() && to == e.participants()[0]
}

// maxChannelScan bounds one channel scan; a run opens a few dozen channels.
const maxChannelScan = 1 << 20

// scanChannels has a check every channel record published since their last
// scan, keeping the ones their view key opens, and returns how many it found.
func (e *Engine) scanChannels(ctx context.Context, s *step, a *Actor) (int, error) {
	c := e.chain
	cursor := &tags.ScanCursor{LastChannelIdx: a.tags.nextChannel}
	found, next, err := tags.ScanChannelsFromCursor(c.client, c.channelRegistryAddr, a.View.DecapsKey, cursor, maxChannelScan)
	if err != nil {
		return 0, fmt.Errorf("%s: scan channels: %w", a.Name, err)
	}
	for _, f := range found {
		a.tags.in = append(a.tags.in, tags.Channel{SharedSecret: f.SharedSecret, PkSpend: a.Spend.PublicKey})
	}
	checked := next.LastChannelIdx - a.tags.nextChannel
	a.tags.nextChannel = next.LastChannelIdx
	if len(found) > 0 {
		var details []HopDetail
		// The scan that finds the Sender's tag channel to payment 1's payee
		// carries the tag channel detection card: a walkthrough's with its
		// values, a full run's by name only.
		if sender := e.sender(); e.carriesTagChannelCards(sender, a) {
			for _, f := range found {
				if !bytes.Equal(f.SharedSecret, sender.tags.out[a]) {
					continue
				}
				d := formulaNamed(FormulaTagChannelDetection, NodeTagChannelRegistry, 0)
				if e.opts.Walkthrough {
					if d, err = c.tagChannelDetectionFormula(ctx, sender, a, e.participants()[1], f, checked, len(found)); err != nil {
						return 0, err
					}
				}
				details = append(details, d)
			}
		}
		s.hop(NodeTagChannelRegistry, ActorNode(a.ID), "scan channels", fmt.Sprintf("%d records checked, %d opened with %s's view key", checked, len(found), a.Name), details...)
		s.log("tag", "%s checked %d channel records, opened %d with their ML-KEM view key", a.Name, checked, len(found))
		s.actor(a, "channels", fmt.Sprintf("%d incoming", len(a.tags.in)))
	}
	return len(found), nil
}

// ensureChannel opens a channel from from to to if from has none yet (a
// manual transfer between a new pair, or each payment of a walkthrough,
// which has no channels step), and has to find it. The run's first opens
// the actors' scan cursors.
func (e *Engine) ensureChannel(ctx context.Context, s *step, from, to *Actor) error {
	if _, ok := from.tags.out[to]; ok {
		return nil
	}
	if err := s.await(ctx, "channel", to.Name); err != nil {
		return err
	}
	if from.tags.out == nil {
		if err := e.startTags(ctx); err != nil {
			return err
		}
	}
	if err := e.openChannel(ctx, s, from, to); err != nil {
		return err
	}
	if found, err := e.scanChannels(ctx, s, to); err != nil {
		return err
	} else if found != 1 {
		return fmt.Errorf("%s found %d new channels by scanning, want the one %s opened", to.Name, found, from.Name)
	}
	return nil
}

// prepareChannelPayment is, with tags on, the channel payment of amount
// from from to to (tags.PrepareChannelPayment): tagged for the block after
// the head, the one the relayer's transaction lands in on an automining
// chain, and settled up to tags.ChannelPaymentWindow blocks later. It is
// nil with tags off.
func (e *Engine) prepareChannelPayment(ctx context.Context, from, to *Actor, amount *big.Int) (*tags.ChannelPayment, error) {
	if !e.opts.Tags {
		return nil, nil
	}
	ss, ok := from.tags.out[to]
	if !ok {
		return nil, fmt.Errorf("%s has no tag channel to %s", from.Name, to.Name)
	}
	head, err := e.chain.client.BlockNumber(ctx)
	if err != nil {
		return nil, err
	}
	return tags.PrepareChannelPayment(ss, head+1, to.Spend.PublicKey, amount, tokenId)
}

// taggedNote is a channel payment a scan found by its tag: the note as
// decrypted, the tag channel whose tag matched, the block_n the tag is for,
// the sealed note, what it opened to, and the block the payment settled in.
type taggedNote struct {
	DecryptedNote
	ch      tags.Channel
	block   uint64
	payload []byte
	opened  *tags.PaymentNote
	cmBlock uint64
}

// errBadTag marks a channel payment that matches one of an actor's channels
// but opens to no RaylsERC20 note of theirs: a sealed note that does not
// decrypt, or does not open to the commitment it settled, or a note for
// another token or in another vault.
// Anyone with a channel to a user can pay them junk, so a scan skips such a
// payment rather than stopping at it.
var errBadTag = errors.New("bad tag")

// scanTags has a compute, for each block_n since their last scan up to the
// head, the tag of every channel they hold, and find the channel payments
// EnygmaDvp settled with those tags (tags.ScanChannelPayments); it keeps
// each one whose sealed note opens to its commitment. The demo's payments
// land in their block_n, before the scan that looks for them, so no
// payment for an already scanned block_n comes later. It returns the new
// notes, how many tags it computed, and how many matching payments it
// skipped as bad. Only an RPC error stops the scan, and then it keeps no
// notes and the cursor stays put, so the next scan reads the same blocks
// again and finds the same notes.
func (e *Engine) scanTags(ctx context.Context, a *Actor) (found []taggedNote, computed, skipped int, err error) {
	c := e.chain
	tip, err := c.client.BlockNumber(ctx)
	if err != nil {
		return nil, 0, 0, err
	}
	if tip <= a.tags.lastBlock || len(a.tags.in) == 0 {
		a.tags.lastBlock = max(a.tags.lastBlock, tip)
		return nil, 0, 0, nil
	}
	var fresh purse
	for b := a.tags.lastBlock + 1; b <= tip; b++ {
		paid, err := tags.ScanChannelPayments(ctx, c.client, c.dvpAddr, b, a.tags.in)
		if err != nil {
			return nil, 0, 0, err
		}
		computed += len(a.tags.in)
		for _, f := range paid {
			// A payment into another vault (the USDr vault's notes are
			// token 0 too) is no RaylsERC20 note, whatever it opens to.
			if f.VaultId.Cmp(erc20VaultId) != 0 {
				skipped++
				continue
			}
			pn, err := f.Open()
			if err == nil && pn.TokenId.Cmp(tokenId) != 0 {
				err = fmt.Errorf("%w: %s: the note is for token %s, want %s", errBadTag, a.Name, pn.TokenId, tokenId)
			}
			if err != nil {
				skipped++
				continue
			}
			if a.notes.has(f.Commitment) || fresh.has(f.Commitment) {
				continue
			}
			n := &note{commitment: f.Commitment, salt: pn.Salt, amount: pn.Amount}
			fresh = append(fresh, n)
			found = append(found, taggedNote{DecryptedNote: DecryptedNote{Owner: a.Name, Asset: assetErc20, Tx: f.TxHash.Hex(),
				Commitment: fieldHex(n.commitment), Via: viaTag, Sealed: bytesHex(f.Ctxt),
				Amount: n.amount.String(), Salt: fieldHex(n.salt), Token: fieldHex(pn.TokenId)},
				ch: tags.Channel{SharedSecret: f.SharedSecret, PkSpend: f.PkSpend}, block: f.BlockNumber, payload: f.Ctxt, opened: pn, cmBlock: f.MinedIn})
		}
	}
	a.notes = append(a.notes, fresh...)
	a.tags.lastBlock = tip
	return found, computed, skipped, nil
}

// logQueryBlocks is the most blocks one eth_getLogs query spans: hosted RPC
// providers reject wider ranges, some at 10,000 blocks, some lower.
const logQueryBlocks uint64 = 2000

// logsBetween is the logs q matches from block from to block to, asking for
// logQueryBlocks blocks of them per query.
func (c *chain) logsBetween(ctx context.Context, from, to uint64, q ethereum.FilterQuery) ([]ethtypes.Log, error) {
	var logs []ethtypes.Log
	for lo := from; lo <= to; lo += logQueryBlocks {
		q.FromBlock = new(big.Int).SetUint64(lo)
		q.ToBlock = new(big.Int).SetUint64(min(to, lo+logQueryBlocks-1))
		got, err := c.client.FilterLogs(ctx, q)
		if err != nil {
			return nil, err
		}
		logs = append(logs, got...)
	}
	return logs, nil
}

// logsSince is the logs q matches from block from to the chain's tip.
func (c *chain) logsSince(ctx context.Context, from uint64, q ethereum.FilterQuery) ([]ethtypes.Log, error) {
	tip, err := c.client.BlockNumber(ctx)
	if err != nil {
		return nil, err
	}
	return c.logsBetween(ctx, from, tip, q)
}

// vaultLog is the Commitment events a vault logged before block next,
// oldest first.
type vaultLog struct {
	next uint64
	logs []ethtypes.Log
}

// vaultCommitments is every Commitment event the vault at vaultAddr has
// logged since the deployment, oldest first. It reads each block once:
// a later call asks only for the blocks mined since. On an RPC error it
// keeps none of the new blocks' events, so the next call asks for them
// again.
func (c *chain) vaultCommitments(ctx context.Context, vaultAddr common.Address) ([]ethtypes.Log, error) {
	v, ok := c.commitments[vaultAddr]
	if !ok {
		v = &vaultLog{next: c.deployBlock}
	}
	tip, err := c.client.BlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("vault Commitment events: %w", err)
	}
	if tip >= v.next {
		logs, err := c.logsBetween(ctx, v.next, tip, ethereum.FilterQuery{
			Addresses: []common.Address{vaultAddr},
			Topics:    [][]common.Hash{{commitmentTopic}},
		})
		if err != nil {
			return nil, fmt.Errorf("vault Commitment events: %w", err)
		}
		v.logs, v.next = append(v.logs, logs...), tip+1
	}
	if c.commitments == nil {
		c.commitments = map[common.Address]*vaultLog{}
	}
	c.commitments[vaultAddr] = v
	return v.logs, nil
}

// discoverByTag has a scan for tags and describes what the scan checked;
// want is how many new notes the scan must find.
func (e *Engine) discoverByTag(ctx context.Context, s *step, a *Actor, want int) (string, error) {
	from := a.tags.lastBlock + 1
	found, computed, skipped, err := e.scanTags(ctx, a)
	if err != nil {
		return "", err
	}
	if skipped > 0 {
		s.log("tag", "%s skipped %d payments with their tag that opened to no note of theirs", a.Name, skipped)
	}
	if len(found) != want {
		return "", fmt.Errorf("%s found %d new notes by their tags, want %d", a.Name, len(found), want)
	}
	blocks := a.tags.lastBlock + 1 - from
	scan := fmt.Sprintf("%d tags computed for %d blocks → %d matched", computed, blocks, len(found))
	notes := make([]DecryptedNote, len(found))
	for i, f := range found {
		notes[i] = f.DecryptedNote
	}
	details := []HopDetail{decryption(notes)}
	for _, f := range found {
		// The note payment 1's tag carried, opened by its payee, carries the
		// tag payload card: a walkthrough's with its values, checked
		// against the commitment in the vault, a full run's by name only.
		if len(e.fanOut) == 0 || !e.carriesTagChannelCards(e.sender(), a) || common.HexToHash(f.Tx) != e.fanOut[0].TxHash {
			continue
		}
		d := formulaNamed(FormulaTagPayload, NodeVault, 0)
		if e.opts.Walkthrough {
			if d, err = e.chain.tagPayloadFormula(ctx, a, f); err != nil {
				return "", err
			}
		}
		details = append(details, d)
	}
	s.hop(NodeDvp, ActorNode(a.ID), "scan tags", scan)
	s.hop(NodeDvp, ActorNode(a.ID), "open note", fmt.Sprintf("%s opens only the matched payment's sealed note, to the commitment it settled", a.Name), details...)
	s.log("tag", "%s computed %d tags for %d blocks, matched %d payments and decrypted only those notes", a.Name, computed, blocks, len(found))
	s.actor(a, "scan", scan)
	return scan, nil
}
