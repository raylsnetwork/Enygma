package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// -only names circuits keygen makes keys for; an empty list means all of
// them.
func TestParseOnly_ReadsTheCircuitsKeygenMakes(t *testing.T) {
	for list, want := range map[string][]string{
		"":                              nil,
		"DvPInitiator":                  {"DvPInitiator"},
		" DvPInitiator, DvPDestination": {"DvPDestination", "DvPInitiator"},
		"PrivateMint,,":                 {"PrivateMint"},
	} {
		only, err := parseOnly(list)
		if err != nil {
			t.Fatalf("parseOnly(%q): %v", list, err)
		}
		if got := slices.Sorted(maps.Keys(only)); !slices.Equal(got, want) {
			t.Errorf("parseOnly(%q) = %v, want %v", list, got, want)
		}
	}
}

// A circuit keygen makes no keys for is refused, rather than regenerating
// nothing: the Payment-family keys are retail's, which make dvp-keygen
// copies.
func TestParseOnly_RefusesACircuitKeygenMakesNoKeysFor(t *testing.T) {
	for _, list := range []string{"Payment", "DvPInitiator,UsdrFee", "PaymentFee", "Payment2in", "PaymentRelayerFeePublic", "dvpinitiator"} {
		if _, err := parseOnly(list); err == nil || !strings.Contains(err.Error(), "make dvp-keygen copies the Payment-family keys") {
			t.Errorf("parseOnly(%q) = %v, want a refusal naming make dvp-keygen", list, err)
		}
	}
}

// circuits, which -only is checked against, names exactly the circuits
// GenerationVkPk makes keys for (its want("<circuit>") calls): a circuit it
// lists but makes no keys for would let -only regenerate nothing again.
func TestCircuits_AreTheOnesGenerationVkPkMakes(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var made []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if fn, ok := call.Fun.(*ast.Ident); !ok || fn.Name != "want" {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			t.Fatalf("want is called with %T, not a circuit name, so the circuits it makes cannot be read", call.Args[0])
		}
		name, err := strconv.Unquote(lit.Value)
		if err != nil {
			t.Fatal(err)
		}
		made = append(made, name)
		return true
	})
	if len(made) == 0 {
		t.Fatal("found no want(\"<circuit>\") call in main.go")
	}
	slices.Sort(made)
	if listed := slices.Sorted(slices.Values(circuits)); !slices.Equal(made, listed) {
		t.Errorf("GenerationVkPk makes keys for %v, but circuits lists %v", made, listed)
	}
}
