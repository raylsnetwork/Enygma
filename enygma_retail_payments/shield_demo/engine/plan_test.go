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
