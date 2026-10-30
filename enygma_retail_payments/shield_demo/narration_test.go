package main

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"testing"

	"enygma_retail_payments/shield_demo/engine"
)

// narrationBlock is the page's step narration data block.
var narrationBlock = regexp.MustCompile(`(?s)<script type="application/json" id="narration">(.*?)</script>`)

// narration is the page's narration block: the step narration by step kind,
// the concept moments by the step id each plays before, and the data chips
// by hop label.
type narration struct {
	Steps   map[string]json.RawMessage   `json:"steps"`
	Moments map[string][]json.RawMessage `json:"moments"`
	Chips   map[string]chip              `json:"chips"`
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

// Every concept moment plays before a step that a run announces, for each
// USDr fee × tags combination: a renamed step can't silently drop one.
func TestEveryConceptMomentPlaysBeforeAnAnnouncedStep(t *testing.T) {
	moments := servedNarration(t).Moments
	if len(moments) == 0 {
		t.Fatal("the narration block has no concept moments")
	}
	for _, tc := range combos {
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

// momentKicker is the part of a concept moment the tests read.
type momentKicker struct {
	Kicker string `json:"kicker"`
}

// A run plays the five concept moments, in the spec's order, for each USDr
// fee × tags combination.
func TestARunPlaysTheFiveConceptMoments(t *testing.T) {
	moments := servedNarration(t).Moments
	want := []string{"Hidden on chain", "Proof", "Relayer", "Finding your notes", "Auditor"}
	for _, tc := range combos {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, id := range engine.StepIDs(comboOptions(tc.usdrFee, tc.tags)) {
				for _, raw := range moments[id] {
					var m momentKicker
					if err := json.Unmarshal(raw, &m); err != nil {
						t.Fatalf("moment before %s: %v", id, err)
					}
					got = append(got, m.Kicker)
				}
			}
			if !slices.Equal(got, want) {
				t.Errorf("moments %q, want %q", got, want)
			}
		})
	}
}

// momentTexts are the texts of raw, a narration value, that a run with the
// given USDr fee and tags shows: a {"when", "text"} part is kept only when
// its option holds, as the page does.
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
		if when, ok := v["when"].(string); ok {
			holds := map[string]bool{"usdrFee": usdrFee, "!usdrFee": !usdrFee, "tags": tags, "!tags": !tags}
			h, known := holds[when]
			if !known {
				t.Errorf("unknown condition %q", when)
			}
			if !h {
				return nil
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
