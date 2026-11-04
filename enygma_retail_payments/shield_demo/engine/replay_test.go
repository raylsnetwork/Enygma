package engine

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// A walkthrough offers Replay payment 1 as an action of payment 2, after
// its payment: once it has settled, and not before.
func TestAWalkthroughReplaysPayment1AfterPayment2(t *testing.T) {
	opts := DefaultOptions()
	opts.Walkthrough = true
	actions := StepActions(opts)
	if got, want := actions["fanout_2"], []string{"payment", "replay"}; !slices.Equal(got, want) {
		t.Errorf("fanout_2's actions %v, want %v", got, want)
	}
	for id, as := range actions {
		if id != "fanout_2" && slices.Contains(as, "replay") {
			t.Errorf("step %s also replays payment 1 (%v)", id, as)
		}
	}
	if as := StepActions(DefaultOptions()); len(as) != 0 {
		t.Errorf("a full run declares actions %v, want none", as)
	}
}

// refusalOf names who refused a replay and why from the relayer's answer;
// an accepted replay, or a failure that is not a refusal, is an error.
func TestARefusedReplayNamesWhoRefusedIt(t *testing.T) {
	for _, tc := range []struct {
		name      string
		err       error
		by        string
		reason    string
		wantError string
	}{
		{"the relayer's nullifier check", &httpError{url: "u", status: 400, msg: "nullifier already spent"}, NodeRelayer, "nullifier already spent", ""},
		{"the vault, in the relayer's simulation", &httpError{url: "u", status: 422, msg: "payment() would revert: execution reverted: Nullifier already spent"}, NodeVault, "payment() would revert: execution reverted: Nullifier already spent", ""},
		{"the vault, on chain", &httpError{url: "u", status: 400, msg: "transaction reverted on-chain"}, NodeVault, "transaction reverted on-chain", ""},
		{"the relayer, for another reason", &httpError{url: "u", status: 400, msg: "merkle root is not a known vault root"}, "", "", "not for its spent nullifier"},
		{"the relayer's API key", &httpError{url: "u", status: 401, msg: "unauthorized"}, "", "", "not for its spent nullifier"},
		{"accepted", nil, "", "", "accepted"},
		{"a relayer failure", &httpError{url: "u", status: 500, msg: "wait mined: boom"}, "", "", "500"},
		{"no relayer", errors.New("POST u: connection refused"), "", "", "connection refused"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			by, reason, err := refusalOf(tc.err)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Errorf("refusalOf: error %v, want one saying %q", err, tc.wantError)
				}
				return
			}
			if err != nil || by != tc.by || reason != tc.reason {
				t.Errorf("refusalOf = %q, %q, %v; want %q, %q", by, reason, err, tc.by, tc.reason)
			}
		})
	}
}

// A replay that changed a tree or a balance fails, naming what changed.
func TestAReplayThatChangedTheChainFails(t *testing.T) {
	before := ChainState{
		Trees:    []TreeState{{Vault: NodeVault, Tree: 0, LeafCount: 5, Root: "0x01"}},
		Balances: []Balance{{Owner: "Sender", Value: "0"}, {Owner: NodeVault, Value: "1000"}},
	}
	if err := before.unchanged(before); err != nil {
		t.Errorf("unchanged(same) = %v, want nil", err)
	}
	moreLeaves := before
	moreLeaves.Trees = []TreeState{{Vault: NodeVault, Tree: 0, LeafCount: 7, Root: "0x02"}}
	if err := before.unchanged(moreLeaves); err == nil || !strings.Contains(err.Error(), NodeVault) {
		t.Errorf("unchanged(new leaves) = %v, want an error naming %s", err, NodeVault)
	}
	paid := before
	paid.Balances = []Balance{{Owner: "Sender", Value: "0"}, {Owner: NodeVault, Value: "880"}}
	if err := before.unchanged(paid); err == nil || !strings.Contains(err.Error(), "1000") {
		t.Errorf("unchanged(balance moved) = %v, want an error naming the old balance", err)
	}
}
