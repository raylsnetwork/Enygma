package engine

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	tags "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// tagWindow is how many consecutive blocks a published tag covers, so it
// still lands under a key its recipient derives if the relayer's
// transaction is mined a block or two later than predicted.
const tagWindow = 3

// tagPublishedTopic is TagRegistry's TagPublished event.
var tagPublishedTopic = crypto.Keccak256Hash([]byte("TagPublished(uint256,bytes32,address,bytes)"))

// tagState is an actor's private-tag state with tags on: the channels they
// opened to recipients, the channels they found addressed to them, and how
// far they have scanned each registry.
type tagState struct {
	// out holds the shared secret of the channel this actor opened to each recipient.
	out map[*Actor][]byte
	// in are the channels to this actor that their channel scans found.
	in []tags.Channel
	// nextChannel is the first channel record not yet scanned; lastBlock
	// the last block whose tags have been.
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
		if _, err := e.scanChannels(s, a); err != nil {
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
	ss, c1, c2, bitmap, err := tags.PrepareChannelSetup(to.View.EncapsKey,
		[]byte("Payment channel ready"), []byte(from.Address.Hex()), tags.PrivacyFull, users, idx, nil)
	if err != nil {
		return fmt.Errorf("channel %s → %s: %w", from.Name, to.Name, err)
	}
	resp, err := e.relayer.channel(ctx, c1, c2, bitmap)
	if err != nil {
		s.hopFailed(ActorNode(from.ID), NodeRelayer, "open channel", err)
		return fmt.Errorf("channel %s → %s: %w", from.Name, to.Name, err)
	}
	from.tags.out[to] = ss
	s.hop(ActorNode(from.ID), NodeRelayer, "open channel", fmt.Sprintf("a shared secret encapsulated to %s's view key", to.Name))
	s.hop(NodeRelayer, NodeTagChannelRegistry, "openChannel", fmt.Sprintf("channel #%d, block %d, gas %s; the relayer is its on-chain sender", resp.ChannelIdx, resp.BlockNumber, formatNum(resp.GasUsed)))
	s.log("tag", "TagChannelRegistry.openChannel(%s → %s) via relayer → channel #%d  block %d  gas %s",
		from.Name, to.Name, resp.ChannelIdx, resp.BlockNumber, formatNum(resp.GasUsed))
	return nil
}

// maxChannelScan bounds one channel scan; a run opens a few dozen channels.
const maxChannelScan = 1 << 20

// scanChannels has a check every channel record published since their last
// scan, keeping the ones their view key opens, and returns how many it found.
func (e *Engine) scanChannels(s *step, a *Actor) (int, error) {
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
		s.hop(NodeTagChannelRegistry, ActorNode(a.ID), "scan channels", fmt.Sprintf("%d records checked, %d opened with %s's view key", checked, len(found), a.Name))
		s.log("tag", "%s checked %d channel records, opened %d with their ML-KEM view key", a.Name, checked, len(found))
		s.actor(a, "channels", fmt.Sprintf("%d incoming", len(a.tags.in)))
	}
	return len(found), nil
}

// ensureChannel opens a channel from from to to if from has none yet (a
// manual transfer between a new pair), and has to find it.
func (e *Engine) ensureChannel(ctx context.Context, s *step, from, to *Actor) error {
	if _, ok := from.tags.out[to]; ok {
		return nil
	}
	if err := e.openChannel(ctx, s, from, to); err != nil {
		return err
	}
	if found, err := e.scanChannels(s, to); err != nil {
		return err
	} else if found != 1 {
		return fmt.Errorf("%s found %d new channels by scanning, want the one %s opened", to.Name, found, from.Name)
	}
	return nil
}

// publishTag notifies t's recipient of the payment through the relayer: a
// tag only the recipient can recognise, carrying the note's opening
// encrypted under the channel's per-block key. It records the tag's
// transaction on t.
func (e *Engine) publishTag(ctx context.Context, s *step, from, to *Actor, t *sent) error {
	ss, ok := from.tags.out[to]
	if !ok {
		return fmt.Errorf("%s has no tag channel to %s", from.Name, to.Name)
	}
	start, window, ctxts, err := tags.PreparePaymentTag(e.chain.client, tagWindow, to.Spend.PublicKey, ss, t.Amount, tokenId, t.payeeSalt)
	if err != nil {
		return err
	}
	req := relayTagRequest{StartBlock: start}
	for i := range window {
		req.Tags = append(req.Tags, "0x"+hex.EncodeToString(window[i][:]))
		req.Ctxts = append(req.Ctxts, "0x"+hex.EncodeToString(ctxts[i]))
	}
	resp, err := e.relayer.tag(ctx, req)
	if err != nil {
		s.hopFailed(ActorNode(from.ID), NodeRelayer, "tag", err)
		return err
	}
	t.TagTxHash = common.HexToHash(resp.TxHash)
	t.tagBlock = resp.BlockNumber
	s.hop(ActorNode(from.ID), NodeRelayer, "tag", fmt.Sprintf("a tag only %s can recognise, carrying the note's opening", to.Name))
	s.hop(NodeRelayer, NodeTagRegistry, "publishTag", fmt.Sprintf("block %d, gas %s", resp.BlockNumber, formatNum(resp.GasUsed)))
	s.log("tag", "TagRegistry.publishTag() via relayer → %s's tag in block %d  gas %s", to.Name, resp.BlockNumber, formatNum(resp.GasUsed))
	return nil
}

// scanTags has a check every tag published since their last scan against
// their channels, decrypt only the notes whose tags match, and keep each
// that opens to a commitment in the vault. It returns how many notes it
// found and how many tags it checked.
func (e *Engine) scanTags(ctx context.Context, a *Actor) (found, checked int, err error) {
	c := e.chain
	tip, err := c.client.BlockNumber(ctx)
	if err != nil {
		return 0, 0, err
	}
	if tip <= a.tags.lastBlock {
		return 0, 0, nil
	}
	logs, err := c.client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(a.tags.lastBlock + 1),
		ToBlock:   new(big.Int).SetUint64(tip),
		Addresses: []common.Address{c.tagRegistryAddr},
		Topics:    [][]common.Hash{{tagPublishedTopic}},
	})
	if err != nil {
		return 0, 0, fmt.Errorf("TagPublished events: %w", err)
	}
	for _, l := range logs {
		if len(l.Topics) < 4 {
			continue
		}
		checked++
		for _, ch := range a.tags.in {
			pn, err := openTag(l, ch)
			if err != nil {
				return found, checked, fmt.Errorf("%s: %w", a.Name, err)
			}
			if pn == nil {
				continue
			}
			n, err := e.tagNote(ctx, a, pn)
			if err != nil {
				return found, checked, err
			}
			if !a.notes.has(n.commitment) {
				a.notes = append(a.notes, n)
				found++
			}
		}
	}
	a.tags.lastBlock = tip
	return found, checked, nil
}

// openTag decrypts the payment note a TagPublished log carries if it is
// ch's tag for its block, and returns nil if it is not.
func openTag(l ethtypes.Log, ch tags.Channel) (*tags.PaymentNote, error) {
	// Topics: [0] signature, [1] block number, [2] tag, [3] publisher.
	if len(l.Topics) < 4 {
		return nil, nil
	}
	want, err := tags.DeriveTag(l.BlockNumber, ch.PkSpend, ch.SharedSecret)
	if err != nil || common.Hash(want) != l.Topics[2] {
		return nil, nil
	}
	vals, err := abiBytes().Unpack(l.Data)
	if err != nil || len(vals) != 1 {
		return nil, fmt.Errorf("decode the tag in block %d: %v", l.BlockNumber, err)
	}
	return tags.DecryptPaymentNote(ch.SharedSecret, ch.PkSpend, l.BlockNumber, vals[0].([]byte))
}

// tagNote is the note a tag's payload opens for a, once its commitment is
// confirmed in the vault: a tag alone proves nothing was paid.
func (e *Engine) tagNote(ctx context.Context, a *Actor, pn *tags.PaymentNote) (*note, error) {
	if pn.TokenId.Cmp(tokenId) != 0 {
		return nil, fmt.Errorf("%s: tagged note is for token %s, want %s", a.Name, pn.TokenId, tokenId)
	}
	cm, err := rpcore.Erc20CommitmentV2(a.Spend.PublicKey, pn.Salt, pn.Amount, tokenId)
	if err != nil {
		return nil, err
	}
	logs, err := e.chain.client.FilterLogs(ctx, ethereum.FilterQuery{
		Addresses: []common.Address{e.chain.vaultAddr},
		Topics:    [][]common.Hash{{commitmentTopic}, nil, {common.BigToHash(cm)}},
	})
	if err != nil {
		return nil, fmt.Errorf("vault Commitment events: %w", err)
	}
	if len(logs) == 0 {
		return nil, fmt.Errorf("%s: tagged note 0x%s… is not in the vault", a.Name, cm.Text(16)[:12])
	}
	return &note{commitment: cm, salt: pn.Salt, amount: pn.Amount}, nil
}

// discoverByTag has a scan for tags and describes what the scan checked;
// want is how many new notes the scan must find.
func (e *Engine) discoverByTag(ctx context.Context, s *step, a *Actor, want int) (string, error) {
	found, checked, err := e.scanTags(ctx, a)
	if err != nil {
		return "", err
	}
	if found != want {
		return "", fmt.Errorf("%s found %d new notes by their tags, want %d", a.Name, found, want)
	}
	scan := fmt.Sprintf("%d tags checked → %d decrypted", checked, found)
	s.hop(NodeTagRegistry, ActorNode(a.ID), "scan tags", scan)
	s.hop(NodeVault, ActorNode(a.ID), "open note", fmt.Sprintf("the tagged note's commitment confirmed in the vault; %s decrypts only it", a.Name))
	s.log("tag", "%s checked %d tags, matched %d and decrypted only those notes", a.Name, checked, found)
	s.actor(a, "scan", scan)
	return scan, nil
}
