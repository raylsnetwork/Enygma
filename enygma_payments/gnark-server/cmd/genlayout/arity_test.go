package main

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"enygma_payments/gnark-server/pkg/circuits/burn"
	"enygma_payments/gnark-server/pkg/circuits/deposit"
	enygma "enygma_payments/gnark-server/pkg/circuits/enygma"
	"enygma_payments/gnark-server/pkg/circuits/enygma_fee"
	"enygma_payments/gnark-server/pkg/circuits/usdr"
	"enygma_payments/gnark-server/pkg/circuits/withdraw"
)

const enygmaSol = "../../../contracts/enygma/contracts/Enygma.sol"

var (
	solFunction = regexp.MustCompile(`function (\w+)\(`)
	solSelector = regexp.MustCompile(`"verifyProof\(uint256\[8\],uint256\[(\d+)\]\)"`)
)

// Enygma.sol calls each verifier with a literal selector string, which
// Solidity cannot build from the generated *_LEN constants. This pins the
// arity in every such string to its circuit's statement length.
func TestEnygmaVerifierSelectorsMatchStatementLengths(t *testing.T) {
	want := map[string]int{
		"burn":                     burn.StatementLayout().Len,
		"withdraw":                 withdraw.StatementLayout().Len,
		"deposit":                  deposit.StatementLayout().Len,
		"_verifyTransferProof":     enygma.StatementLayout().Len,
		"_verifyUsdrTransferProof": usdr.StatementLayout().Len,
		"_verifyFeeTransferProof":  enygma_fee.StatementLayout().Len,
	}

	src, err := os.ReadFile(enygmaSol)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, m := range solSelector.FindAllSubmatchIndex(src, -1) {
		fn := enclosingFunction(src[:m[0]])
		got, _ := strconv.Atoi(string(src[m[2]:m[3]]))
		wantLen, ok := want[fn]
		if !ok {
			t.Errorf("verifyProof selector in %s: no statement layout pinned for this function", fn)
			continue
		}
		if got != wantLen {
			t.Errorf("verifyProof selector in %s: uint256[%d], want uint256[%d]", fn, got, wantLen)
		}
		seen[fn] = true
	}
	for fn := range want {
		if !seen[fn] {
			t.Errorf("no verifyProof selector found in %s", fn)
		}
	}
}

// enclosingFunction returns the name of the last Solidity function declared
// in before.
func enclosingFunction(before []byte) string {
	all := solFunction.FindAllSubmatch(before, -1)
	if len(all) == 0 {
		return ""
	}
	return string(all[len(all)-1][1])
}
