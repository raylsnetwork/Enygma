package main

import (
	"fmt"
	"regexp"
	"testing"

	"enygma_retail_payments/layout"
)

// pageDepth is the page's declaration of the vaults' tree depth.
var pageDepth = regexp.MustCompile(`(?m)^const DEPTH = (\d+), TREE_LEAVES = 2 \*\* DEPTH;$`)

// The page draws the vaults' trees and writes their depth into the Merkle
// path and nullifier cards from its own DEPTH, while the engine builds the
// trees and paths at layout.MerkleTreeDepth, the depth the circuits are
// compiled at. The two must agree.
func TestThePageDrawsTheTreesAtTheCircuitsDepth(t *testing.T) {
	m := pageDepth.FindStringSubmatch(indexHTML)
	if m == nil {
		t.Fatalf("index.html has no line %q", pageDepth)
	}
	if m[1] != fmt.Sprint(layout.MerkleTreeDepth) {
		t.Errorf("index.html's DEPTH is %s, the circuits' depth (layout.MerkleTreeDepth) is %d", m[1], layout.MerkleTreeDepth)
	}
}
