package engine

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"enygma_retail_payments/layout"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

// ChainState is what a replay must leave as it was: each vault's current
// Merkle tree and the public RaylsERC20 balances of the vault, the relayer
// and every actor.
type ChainState struct {
	Trees    []TreeState `json:"trees"`
	Balances []Balance   `json:"balances"`
}

// TreeState is one vault's current tree: its number, leaf count and root.
type TreeState struct {
	Vault     string `json:"vault"`
	Tree      int    `json:"tree"`
	LeafCount int    `json:"leafCount"`
	Root      string `json:"root"`
}

// Balance is one account's public RaylsERC20 balance, decimal: an actor's
// (by name), the vault's (NodeVault) or the relayer's (NodeRelayer).
type Balance struct {
	Owner string `json:"owner"`
	Value string `json:"value"`
}

// unchanged fails, naming the first difference, unless after is s.
func (s ChainState) unchanged(after ChainState) error {
	if len(after.Trees) != len(s.Trees) || len(after.Balances) != len(s.Balances) {
		return fmt.Errorf("the chain state has %d trees and %d balances, had %d and %d", len(after.Trees), len(after.Balances), len(s.Trees), len(s.Balances))
	}
	for i, t := range s.Trees {
		if a := after.Trees[i]; a != t {
			return fmt.Errorf("%s's tree went from tree %d, %d leaves, root %s to tree %d, %d leaves, root %s", t.Vault, t.Tree, t.LeafCount, t.Root, a.Tree, a.LeafCount, a.Root)
		}
	}
	for i, b := range s.Balances {
		if a := after.Balances[i]; a != b {
			return fmt.Errorf("%s's RaylsERC20 balance went from %s to %s", b.Owner, b.Value, a.Value)
		}
	}
	return nil
}

// account is one address whose balance a ChainState holds, and its owner.
type account struct {
	owner string
	addr  common.Address
}

// replayHop is the label of the replay's hop.
const replayHop = "replay payment 1"

// chainState reads e's ChainState now.
func (e *Engine) chainState(ctx context.Context) (ChainState, error) {
	c := e.chain
	var st ChainState
	for _, v := range []struct {
		node string
		addr common.Address
	}{{NodeVault, c.vaultAddr}, {NodeUsdrVault, c.usdrVaultAddr}} {
		snap, err := c.treeSnapshot(ctx, v.node, v.addr)
		if err != nil {
			return ChainState{}, err
		}
		st.Trees = append(st.Trees, TreeState{Vault: v.node, Tree: snap.Tree, LeafCount: snap.LeafCount, Root: snap.Root})
	}
	accounts := []account{{NodeVault, c.vaultAddr}, {NodeRelayer, c.relayerAddr}}
	for _, a := range e.actors {
		accounts = append(accounts, account{a.Name, a.Address})
	}
	for _, a := range accounts {
		var out []any
		if err := c.erc20.Call(&bind.CallOpts{Context: ctx}, &out, "balanceOf", a.addr); err != nil {
			return ChainState{}, fmt.Errorf("RaylsERC20.balanceOf(%s): %w", a.owner, err)
		}
		v, ok := out[0].(*big.Int)
		if len(out) != 1 || !ok {
			return ChainState{}, fmt.Errorf("RaylsERC20.balanceOf(%s) returned %v", a.owner, out)
		}
		st.Balances = append(st.Balances, Balance{Owner: a.owner, Value: v.String()})
	}
	return st, nil
}

// refusalOf is who refused a replay (NodeRelayer, or NodeVault when the
// vault's checks reverted the payment, in the relayer's simulation or on
// chain) and why, from the error the relayer's answer gave. An accepted
// replay (no error), a failure that is no refusal (a 5xx, no relayer), or a
// relayer refusal for anything but the nullifier (a root, its API key),
// which would show nothing about double spending, is an error. A vault
// revert is taken as is: its reason may be an undecoded custom error.
func refusalOf(err error) (by, reason string, _ error) {
	if err == nil {
		return "", "", errors.New("the relayer accepted the replay of payment 1: a spent note was spent again")
	}
	var he *httpError
	if !errors.As(err, &he) || he.status < 400 || he.status >= 500 {
		return "", "", fmt.Errorf("replay payment 1: %w", err)
	}
	msg := strings.ToLower(he.msg)
	if strings.Contains(msg, "revert") {
		return NodeVault, he.msg, nil
	}
	if !strings.Contains(msg, "nullifier") {
		return "", "", fmt.Errorf("the relayer refused the replay of payment 1, but not for its spent nullifier: %w", err)
	}
	return NodeRelayer, he.msg, nil
}

// replayPayment1 resubmits payment 1's exact relay request through the
// relayer once the presenter releases it. Its nullifier is already spent, so
// it must be refused: the hop names who refused it and why, and the vault's
// trees and every balance are checked unchanged. An accepted replay, or one
// that changed the chain, fails the walkthrough.
func (e *Engine) replayPayment1(ctx context.Context, s *step) error {
	p1 := e.payment1
	if p1 == nil {
		return errors.New("replay payment 1: payment 1 has not settled")
	}
	if err := e.awaitAction(ctx, s.id, "replay", p1.To); err != nil {
		return err
	}
	sender := ActorNode(e.sender().ID)
	before, err := e.chainState(ctx)
	if err != nil {
		return err
	}
	var out relayTxResponse
	exchange, posted := e.relayer.post(ctx, p1.relayReq.path, p1.relayReq.body, &out)
	by, reason, err := refusalOf(posted)
	if err != nil {
		if posted == nil {
			err = fmt.Errorf("%w (tx %s)", err, out.TxHash)
		}
		s.hopFailed(sender, NodeRelayer, replayHop, err)
		return err
	}
	after, err := e.chainState(ctx)
	if err != nil {
		return err
	}
	if err := before.unchanged(after); err != nil {
		err = fmt.Errorf("the refused replay of payment 1 changed the chain: %w", err)
		s.hopFailed(sender, NodeRelayer, replayHop, err)
		return err
	}
	who := "the relayer"
	if by == NodeVault {
		who = "the vault"
	}
	d := HopDetail{Kind: DetailRefusal, Vault: NodeVault, Tree: int(p1.stmt[layout.PaymentTreeNumbersOffset].Int64()),
		RefusedBy: by, Reason: reason, Nullifier: fieldHex(p1.stmt[layout.PaymentNullifiersOffset]), TxHash: p1.TxHash.Hex(),
		Before: &before, After: &after}
	s.hop(sender, NodeRelayer, replayHop, fmt.Sprintf("refused by %s: %s", who, reason), d, exchange)
	s.log("chain", "replay of payment 1 refused by %s: %s; trees and balances unchanged", who, reason)
	return nil
}
