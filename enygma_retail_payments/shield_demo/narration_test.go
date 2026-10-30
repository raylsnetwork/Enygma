package main

import (
	"encoding/json"
	"regexp"
	"testing"

	"enygma_retail_payments/shield_demo/engine"
)

// narrationBlock is the page's step narration data block.
var narrationBlock = regexp.MustCompile(`(?s)<script type="application/json" id="narration">(.*?)</script>`)

// narration is the page's narration block: the step narration by step kind,
// and the concept moments by the step id each plays before.
type narration struct {
	Steps   map[string]json.RawMessage   `json:"steps"`
	Moments map[string][]json.RawMessage `json:"moments"`
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
