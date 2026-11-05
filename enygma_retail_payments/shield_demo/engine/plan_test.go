package engine

import (
	"slices"
	"strings"
	"testing"
)

// The step ids a run would announce, for each USDr fee × tags combination,
// with no chain.
func TestTagsAddAChannelsStepAndTheFeeAddsNone(t *testing.T) {
	head := []string{"prereqs", "keygen", "register", "audit_keys"}
	middle := []string{
		"usdr", "shield",
		"fanout_1", "fanout_2", "fanout_3", "fanout_4", "fanout_5", "fanout_6",
		"discover",
		"transfer_1", "transfer_2", "transfer_3", "transfer_4", "transfer_5", "transfer_6",
		"audit", "totals",
	}
	withTags := slices.Concat(head, []string{"channels"}, middle)
	withoutTags := slices.Concat(head, middle)

	for _, tc := range []struct {
		name          string
		usdrFee, tags bool
		want          []string
	}{
		{"fee and tags", true, true, withTags},
		{"fee, no tags", true, false, withoutTags},
		{"tags, no fee", false, true, withTags},
		{"no fee, no tags", false, false, withoutTags},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := DefaultOptions()
			opts.UsdrFee, opts.Tags = tc.usdrFee, tc.tags
			if got := StepIDs(opts); !slices.Equal(got, tc.want) {
				t.Errorf("StepIDs:\n got %v\nwant %v", got, tc.want)
			}
		})
	}
	if n := len(StepIDs(DefaultOptions())); n != 22 {
		t.Errorf("a default run announces %d steps, want 22", n)
	}
}

// The transfers follow the script: one step per scripted transfer.
func TestStepIDsFollowTheScript(t *testing.T) {
	opts := DefaultOptions()
	opts.Script = []Transfer{{"Ana", "Bruno", 50}, {"Bruno", "Carla", 20}}
	ids := StepIDs(opts)
	var transfers []string
	for _, id := range ids {
		if strings.HasPrefix(id, "transfer_") {
			transfers = append(transfers, id)
		}
	}
	if want := []string{"transfer_1", "transfer_2"}; !slices.Equal(transfers, want) {
		t.Errorf("transfer steps %v, want %v", transfers, want)
	}
}

// A walkthrough follows the Sender's path, with the USDr fee and tags as
// the options set them: no prereqs, channels, transfers or totals step, and
// a usdr step only with the fee on. Payment 1 opens its tag channel, proves
// its fee and publishes its tag as actions of their own; with both off, the
// gates are the ones a walkthrough had before it followed them.
func TestAWalkthroughFollowsTheFeeAndTagsOptions(t *testing.T) {
	head := []string{"keygen", "register", "audit_keys"}
	tail := []string{"fanout_1", "fanout_2", "fanout_3", "fanout_4", "fanout_5", "fanout_6", "discover", "audit"}
	for _, tc := range []struct {
		name          string
		usdrFee, tags bool
		usdr          bool
		payment1      []string
	}{
		{"fee and tags", true, true, true, []string{"channel", "prove", "proof", "fee", "relay", "settle", "tag"}},
		{"fee, no tags", true, false, true, []string{"prove", "proof", "fee", "relay", "settle"}},
		{"tags, no fee", false, true, false, []string{"channel", "prove", "proof", "relay", "settle", "tag"}},
		{"neither", false, false, false, []string{"prove", "proof", "relay", "settle"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := DefaultOptions()
			opts.UsdrFee, opts.Tags, opts.Walkthrough = tc.usdrFee, tc.tags, true
			want := slices.Clone(head)
			if tc.usdr {
				want = append(want, "usdr")
			}
			want = append(append(want, "shield"), tail...)
			if got := StepIDs(opts); !slices.Equal(got, want) {
				t.Errorf("StepIDs:\n got %v\nwant %v", got, want)
			}
			actions := StepActions(opts)
			wantActions := map[string][]string{
				"keygen": {"auditor", "keys"}, "register": {"register"}, "audit_keys": {"recover"}, "shield": {"shield"},
				"fanout_1": tc.payment1, "fanout_2": {"payment", "replay"},
				"fanout_3": {"payment"}, "fanout_4": {"payment"}, "fanout_5": {"payment"}, "fanout_6": {"payment"},
				"discover": {"scan"}, "audit": {"audit"},
			}
			if tc.usdr {
				wantActions["usdr"] = []string{"credit"}
			}
			if len(actions) != len(wantActions) {
				t.Errorf("StepActions has %d steps %v, want %d", len(actions), actions, len(wantActions))
			}
			for id, as := range wantActions {
				if !slices.Equal(actions[id], as) {
					t.Errorf("%s's actions %v, want %v", id, actions[id], as)
				}
			}
		})
	}
}
