package main

import (
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"enygma_gnark_shared/statement"

	"enygma_dvp/gnark_circuits/templates"
)

// paymentSol is retail's PaymentStatementLayout, rendered by
// enygma_retail_payments/gnark_circuits/cmd/genlayout from retail's copies of
// the Payment-family templates.
const paymentSol = "../../../contracts/core/contracts/PaymentStatementLayout.sol"

// retailTarget is retail genlayout's Target, so that enygma_dvp's
// Payment-family layouts render exactly as retail's would.
var retailTarget = statement.Target{
	Generator:       "enygma_retail_payments/gnark_circuits/cmd/genlayout",
	Subject:         "the retail Payment-family circuits'",
	GoPackage:       "layout",
	SolidityLibrary: "PaymentStatementLayout",
}

var solConstant = regexp.MustCompile(`constant (\w+) = (\d+);`)

// enygma_dvp keeps its own copies of the Payment-family templates, while the
// contracts read retail's PaymentStatementLayout. The copies must keep
// deriving exactly that library.
func TestPaymentFamilyLayoutsMatchRetailPaymentStatementLayout(t *testing.T) {
	committed, err := os.ReadFile(paymentSol)
	if err != nil {
		t.Fatal(err)
	}
	ours := retailTarget.RenderSolidity(templates.PaymentStatementLayouts())
	if string(ours) == string(committed) {
		return
	}
	got, want := constants(string(ours)), constants(string(committed))
	named := false
	for _, name := range slices.Sorted(maps.Keys(want)) {
		w := want[name]
		if g, ok := got[name]; !ok {
			t.Errorf("%s: PaymentStatementLayout has it (= %s), enygma_dvp's templates do not derive it", name, w)
			named = true
		} else if g != w {
			t.Errorf("%s: enygma_dvp's templates derive %s, PaymentStatementLayout has %s", name, g, w)
			named = true
		}
	}
	for _, name := range slices.Sorted(maps.Keys(got)) {
		if _, ok := want[name]; !ok {
			t.Errorf("%s: enygma_dvp's templates derive it (= %s), PaymentStatementLayout does not have it", name, got[name])
			named = true
		}
	}
	if !named {
		ol, cl := strings.Split(string(ours), "\n"), strings.Split(string(committed), "\n")
		for i := 0; i < max(len(ol), len(cl)); i++ {
			if i >= len(ol) || i >= len(cl) || ol[i] != cl[i] {
				t.Errorf("same fields, but line %d differs from %s (field names in the documentation, or the Target changed)", i+1, paymentSol)
				break
			}
		}
	}
}

// constants maps each Solidity constant's name to its value.
func constants(src string) map[string]string {
	m := map[string]string{}
	for _, c := range solConstant.FindAllStringSubmatch(src, -1) {
		m[c[1]] = c[2]
	}
	return m
}
