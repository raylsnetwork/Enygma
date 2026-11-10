package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"enygma_retail_payments/shield_demo/engine"
)

// narrationBlock is the page's step narration data block.
var narrationBlock = regexp.MustCompile(`(?s)<script type="application/json" id="narration">(.*?)</script>`)

// narration is the page's narration block: the step narration by step kind,
// the walkthrough's action narration by step kind and action,
// the concept moments by the step id each plays before, the data chips by
// hop label, and the step kinds whose repeated hops collapse.
type narration struct {
	Steps       map[string]json.RawMessage            `json:"steps"`
	Walkthrough map[string]map[string]json.RawMessage `json:"walkthrough"`
	Moments     map[string][]json.RawMessage          `json:"moments"`
	Chips       map[string]chip                       `json:"chips"`
	Collapse    []string                              `json:"collapse"`
}

// chip is a data chip: what a hop moves, its kinds, and whether it stays
// inside a wallet.
type chip struct {
	Text    string   `json:"text"`
	Kinds   []string `json:"kinds"`
	Private bool     `json:"private"`
}

// servedNarration returns the served page's narration block.
func servedNarration(t *testing.T) narration {
	t.Helper()
	_, url := startDemo(t, "")
	m := narrationBlock.FindStringSubmatch(getPage(t, url))
	if m == nil {
		t.Fatal("the page has no narration block")
	}
	var n narration
	if err := json.Unmarshal([]byte(m[1]), &n); err != nil {
		t.Fatalf("narration block: %v", err)
	}
	return n
}

// combos are the four USDr fee × tags combinations a run can have.
var combos = []struct {
	name          string
	usdrFee, tags bool
}{
	{"fee and tags", true, true},
	{"fee, no tags", true, false},
	{"tags, no fee", false, true},
	{"no fee, no tags", false, false},
}

// comboOptions are the default options with the given USDr fee and tags.
func comboOptions(usdrFee, tags bool) engine.Options {
	opts := engine.DefaultOptions()
	opts.UsdrFee, opts.Tags = usdrFee, tags
	return opts
}

// stepSuffix is the _N a step id ends with; without it, the id is its kind.
var stepSuffix = regexp.MustCompile(`_\d+$`)

// Every step a run announces, for each USDr fee × tags combination, and a
// manual transfer, has its narration in the page.
func TestEveryStepKindHasANarration(t *testing.T) {
	steps := servedNarration(t).Steps
	for _, tc := range combos {
		t.Run(tc.name, func(t *testing.T) {
			for _, id := range engine.StepIDs(comboOptions(tc.usdrFee, tc.tags)) {
				if kind := stepSuffix.ReplaceAllString(id, ""); steps[kind] == nil {
					t.Errorf("step %s: no narration for kind %q", id, kind)
				}
			}
		})
	}
	if steps["manual"] == nil {
		t.Error("no narration for a manual transfer")
	}
}

// Every concept moment plays before a step that a tags-on run announces, for
// either USDr fee: a renamed step can't silently drop one. A tags-off run
// announces fewer steps, so plays fewer moments: TestARunPlaysItsConceptMoments
// checks each of its moments' step ids.
func TestEveryConceptMomentPlaysBeforeAnAnnouncedStep(t *testing.T) {
	moments := servedNarration(t).Moments
	if len(moments) == 0 {
		t.Fatal("the narration block has no concept moments")
	}
	for _, tc := range combos {
		if !tc.tags {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			announced := map[string]bool{}
			for _, id := range engine.StepIDs(comboOptions(tc.usdrFee, tc.tags)) {
				announced[id] = true
			}
			for id, ms := range moments {
				if !announced[id] {
					t.Errorf("%d concept moment(s) before step %q, which the run doesn't announce", len(ms), id)
				}
			}
		})
	}
}

// Every step kind whose repeated hops collapse is one a default run (fee and
// tags) announces: a renamed step can't silently stop collapsing.
func TestEveryCollapsingStepKindIsAnnounced(t *testing.T) {
	collapse := servedNarration(t).Collapse
	if len(collapse) == 0 {
		t.Fatal("the narration block names no step kinds to collapse")
	}
	announced := map[string]bool{}
	for _, id := range engine.StepIDs(engine.DefaultOptions()) {
		announced[stepSuffix.ReplaceAllString(id, "")] = true
	}
	for _, kind := range collapse {
		if !announced[kind] {
			t.Errorf("collapse names step kind %q, which a default run doesn't announce", kind)
		}
	}
}

// momentKicker is the part of a concept moment the tests read.
type momentKicker struct {
	Kicker string `json:"kicker"`
}

// A tags-on run plays six concept moments, "Tag channel" first, before its
// Tag channels step; a tags-off run plays the other five, in the same
// order, for either USDr fee. Each is "step id: kicker".
func TestARunPlaysItsConceptMoments(t *testing.T) {
	moments := servedNarration(t).Moments
	for _, tc := range combos {
		want := []string{"shield: Hidden on chain", "fanout_1: Proof", "fanout_1: Relayer", "discover: Finding your notes", "audit: Auditor"}
		if tc.tags {
			want = append([]string{"channels: Tag channel"}, want...)
		}
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, id := range engine.StepIDs(comboOptions(tc.usdrFee, tc.tags)) {
				for _, raw := range moments[id] {
					var m momentKicker
					if err := json.Unmarshal(raw, &m); err != nil {
						t.Fatalf("moment before %s: %v", id, err)
					}
					got = append(got, id+": "+m.Kicker)
				}
			}
			if !slices.Equal(got, want) {
				t.Errorf("moments %q, want %q", got, want)
			}
		})
	}
}

// momentTexts are the texts of raw, a narration value, that a run with the
// given USDr fee and tags, and no walkthrough, shows: a {"when", "text"}
// part is kept only when its options hold, as the page does.
func momentTexts(t *testing.T, raw any, usdrFee, tags bool) []string {
	t.Helper()
	switch v := raw.(type) {
	case string:
		return []string{v}
	case []any:
		var out []string
		for _, x := range v {
			out = append(out, momentTexts(t, x, usdrFee, tags)...)
		}
		return out
	case map[string]any:
		if when, ok := v["when"]; ok {
			holds := map[string]bool{"usdrFee": usdrFee, "!usdrFee": !usdrFee, "tags": tags, "!tags": !tags, "walkthrough": false, "!walkthrough": true}
			var conds []any
			if c, ok := when.(string); ok {
				conds = []any{c}
			} else {
				conds, _ = when.([]any)
			}
			for _, c := range conds {
				h, known := holds[fmt.Sprint(c)]
				if !known {
					t.Errorf("unknown condition %q", c)
				}
				if !h {
					return nil
				}
			}
			return momentTexts(t, v["text"], usdrFee, tags)
		}
		var out []string
		for _, x := range v {
			out = append(out, momentTexts(t, x, usdrFee, tags)...)
		}
		return out
	}
	return nil
}

// On a run without the USDr fee, no concept moment mentions a fee.
func TestNoConceptMomentMentionsAFeeOnAFeesOffRun(t *testing.T) {
	moments := servedNarration(t).Moments
	for _, tags := range []bool{true, false} {
		for id, ms := range moments {
			for _, raw := range ms {
				var m any
				if err := json.Unmarshal(raw, &m); err != nil {
					t.Fatalf("moment before %s: %v", id, err)
				}
				for _, s := range momentTexts(t, m, false, tags) {
					if strings.Contains(strings.ToLower(s), "fee") {
						t.Errorf("tags %v, moment before %s: %q mentions a fee", tags, id, s)
					}
				}
			}
		}
	}
}

// chipKinds is the closed set of things a hop can move.
var chipKinds = map[string]bool{
	"key": true, "gas": true, "ciphertext": true, "commitment": true,
	"note": true, "proof": true, "nullifier": true,
}

// Every data chip has text and only known chip kinds: the chip vocabulary
// stays closed.
func TestEveryDataChipHasAKnownKind(t *testing.T) {
	chips := servedNarration(t).Chips
	if len(chips) == 0 {
		t.Fatal("the narration block has no data chips")
	}
	for label, c := range chips {
		if c.Text == "" {
			t.Errorf("hop %q: chip has no text", label)
		}
		if len(c.Kinds) == 0 {
			t.Errorf("hop %q: chip has no kind", label)
		}
		for _, k := range c.Kinds {
			if !chipKinds[k] {
				t.Errorf("hop %q: unknown chip kind %q", label, k)
			}
		}
	}
}

// walkthroughOptions are the options of a walkthrough.
func walkthroughOptions() engine.Options {
	opts := engine.DefaultOptions()
	opts.Walkthrough = true
	return opts
}

// Every action a walkthrough's gate releases, in every step it announces,
// has its narration in the page's walkthrough section, and so does every
// step: a new action can't ship without its explanation.
func TestEveryWalkthroughActionHasANarration(t *testing.T) {
	n := servedNarration(t)
	actions := engine.StepActions(walkthroughOptions())
	for _, id := range engine.StepIDs(walkthroughOptions()) {
		kind := stepSuffix.ReplaceAllString(id, "")
		if n.Steps[kind] == nil {
			t.Errorf("walkthrough step %s: no step narration for kind %q", id, kind)
		}
		if len(actions[id]) == 0 {
			t.Errorf("walkthrough step %s releases no action", id)
		}
		for _, a := range actions[id] {
			if v := n.Walkthrough[kind][a]; len(v) == 0 || string(v) == `""` {
				t.Errorf("walkthrough step %s: no narration for action %q of kind %q", id, a, kind)
			}
		}
	}
}

// The on-chain proof check a walkthrough offers once payment 1 settles has
// its narration in the walkthrough section, keyed by its protocol field:
// the check itself, and Tamper.
func TestTheOnChainProofCheckHasANarration(t *testing.T) {
	check := servedNarration(t).Walkthrough[engine.VerifyProofField]
	for _, part := range []string{"check", "tamper"} {
		var text string
		if err := json.Unmarshal(check[part], &text); err != nil || strings.TrimSpace(text) == "" {
			t.Errorf("walkthrough.%s.%s: no narration text (%s)", engine.VerifyProofField, part, check[part])
		}
	}
}

// Payment 1's two columns, what stays in the Sender's wallet (its witness)
// and what goes on chain (its public inputs), have their narration in the
// walkthrough section, keyed by the witness hop detail kind.
func TestTheWitnessAndPublicColumnsHaveANarration(t *testing.T) {
	columns := servedNarration(t).Walkthrough[string(engine.DetailWitness)]
	for _, part := range []string{"stays", "chain"} {
		var text string
		if err := json.Unmarshal(columns[part], &text); err != nil || strings.TrimSpace(text) == "" {
			t.Errorf("walkthrough.%s.%s: no narration text (%s)", engine.DetailWitness, part, columns[part])
		}
	}
}

// Replay payment 1's refusal has its narration in the walkthrough section,
// keyed by the refusal hop detail kind: why it was refused, and that the
// chain is unchanged. The click itself is fanout's replay action, which
// TestEveryWalkthroughActionHasANarration covers.
func TestTheReplayRefusalHasANarration(t *testing.T) {
	n := servedNarration(t)
	refusal := n.Walkthrough[string(engine.DetailRefusal)]
	for _, part := range []string{"refused", "unchanged"} {
		var text string
		if err := json.Unmarshal(refusal[part], &text); err != nil || strings.TrimSpace(text) == "" {
			t.Errorf("walkthrough.%s.%s: no narration text (%s)", engine.DetailRefusal, part, refusal[part])
		}
	}
	if v := n.Walkthrough["fanout"]["replay"]; len(v) == 0 {
		t.Error("walkthrough.fanout.replay: no narration for the Replay payment 1 click")
	}
}
