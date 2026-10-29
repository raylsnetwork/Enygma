package main

import (
	"encoding/json"
	"regexp"
	"testing"

	"enygma_retail_payments/shield_demo/engine"
)

// narrationBlock is the page's step narration data block.
var narrationBlock = regexp.MustCompile(`(?s)<script type="application/json" id="narration">(.*?)</script>`)

// servedNarration returns the step kinds the served page narrates.
func servedNarration(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	_, url := startDemo(t, "")
	m := narrationBlock.FindStringSubmatch(getPage(t, url))
	if m == nil {
		t.Fatal("the page has no narration block")
	}
	var n struct {
		Steps map[string]json.RawMessage `json:"steps"`
	}
	if err := json.Unmarshal([]byte(m[1]), &n); err != nil {
		t.Fatalf("narration block: %v", err)
	}
	return n.Steps
}

// stepSuffix is the _N a step id ends with; without it, the id is its kind.
var stepSuffix = regexp.MustCompile(`_\d+$`)

// Every step a run announces, for each USDr fee × tags combination, and a
// manual transfer, has its narration in the page.
func TestEveryStepKindHasANarration(t *testing.T) {
	steps := servedNarration(t)
	for _, tc := range []struct {
		name          string
		usdrFee, tags bool
	}{
		{"fee and tags", true, true},
		{"fee, no tags", true, false},
		{"tags, no fee", false, true},
		{"no fee, no tags", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := engine.DefaultOptions()
			opts.UsdrFee, opts.Tags = tc.usdrFee, tc.tags
			for _, id := range engine.StepIDs(opts) {
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
