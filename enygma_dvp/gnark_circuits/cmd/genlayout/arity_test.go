package main

import (
	"maps"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"enygma_dvp/gnark_circuits/templates"
)

const (
	enygmaDvpSol            = "../../../contracts/core/contracts/EnygmaDvp.sol"
	dvpReceiptSol           = "../../../contracts/core/contracts/DvpReceipt.sol"
	iEnygmaDvpSol           = "../../../contracts/core/interfaces/IEnygmaDvp.sol"
	iPrivateMintVerifierSol = "../../../contracts/core/interfaces/IPrivateMintVerifier.sol"
	privateMintVerifierSol  = "../../../contracts/core/contracts/PrivateMintVerifier.sol"
)

var (
	solLen         = regexp.MustCompile(`uint256 constant (\w+_LEN) = (\d+);`)
	solLengthCheck = regexp.MustCompile(`\.length\s*[!=]=\s*(\w+)`)
	solFunction    = regexp.MustCompile(`function (\w+)\(`)
	solOffsetRead  = regexp.MustCompile(`\b([A-Z][A-Z0-9_]*)_OFFSET\b`)
	solMintSignals = regexp.MustCompile(`uint256\[(\w+)\]\s+public_signal;`)
	solMintInput   = regexp.MustCompile(`uint256\[(\w+)\]\s+calldata\s+input`)
)

// A DvP receipt's statement is a dynamic array, so the compiler cannot tie a
// read at a DvpStatementLayout offset to the circuit that offset belongs to:
// a DvpInitiator offset read from a DvpDestination statement compiles and
// reads the wrong field, or past its end. This pins every such read in
// EnygmaDvp and DvpReceipt to a statement length check for its own circuit:
// the length compared last before the read, in the same function, must be
// that circuit's statement length.
func TestDvpReadsAreGuardedByTheirCircuitsLength(t *testing.T) {
	lens := generatedLens(t)
	circuits := map[string]int{
		"DVP_INITIATOR":   templates.DvpInitiatorStatementLayout().Len,
		"DVP_DESTINATION": templates.DvpDestinationStatementLayout().Len,
	}
	// Each must read DvP fields by name, itself or through DvpReceipt.read
	// (whose reads this checks): without one, the check passes on a file that
	// still reads by hand-written index.
	mustRead := map[string]bool{
		"DvpReceipt.read":                   false,
		"EnygmaDvp.submitPartialSettlement": false,
		"EnygmaDvp._settleOnGroupPair":      false,
	}
	for contract, path := range map[string]string{"EnygmaDvp": enygmaDvpSol, "DvpReceipt": dvpReceiptSol} {
		src := readFile(t, path)
		for _, fn := range solFunction.FindAllStringSubmatch(src, -1) {
			name := contract + "." + fn[1]
			body := functionBody(t, src, fn[1])
			if _, ok := mustRead[name]; ok && strings.Contains(body, "DvpReceipt.read(") {
				mustRead[name] = true
			}
			for _, m := range solOffsetRead.FindAllStringSubmatchIndex(body, -1) {
				circuit, ok := circuitOf(body[m[2]:m[3]], circuits)
				if !ok {
					continue
				}
				if _, ok := mustRead[name]; ok {
					mustRead[name] = true
				}
				checks := solLengthCheck.FindAllStringSubmatch(body[:m[0]], -1)
				if len(checks) == 0 {
					t.Errorf("%s reads %s with no statement length check before it", name, body[m[2]:m[3]])
					continue
				}
				if got, want := resolve(t, lens, checks[len(checks)-1][1]), circuits[circuit]; got != want {
					t.Errorf("%s reads %s guarded by statement length %d, %s's is %d", name, body[m[2]:m[3]], got, circuit, want)
				}
			}
		}
	}
	for name, read := range mustRead {
		if !read {
			t.Errorf("%s reads no DvP statement field by name", name)
		}
	}

	// DvpReceipt.isLeg reads no field, but decides which receipts reach
	// DvpReceipt.read: it must accept exactly the DvP circuits' lengths.
	accepted := map[int]bool{}
	for _, m := range solLengthCheck.FindAllStringSubmatch(functionBody(t, readFile(t, dvpReceiptSol), "isLeg"), -1) {
		accepted[resolve(t, lens, m[1])] = true
	}
	want := map[int]bool{}
	for _, n := range circuits {
		want[n] = true
	}
	if !maps.Equal(accepted, want) {
		t.Errorf("DvpReceipt.isLeg accepts statement lengths %v, the DvP circuits' are %v", accepted, want)
	}
}

// A private-mint proof's public signals are a fixed-size array, so its length
// is checked by the array type: EnygmaDvp's proof struct, the verifier
// interface it calls and the generated verifier must all size it to
// PrivateMint's statement length.
func TestPrivateMintSignalArraysMatchStatementLength(t *testing.T) {
	lens := generatedLens(t)
	want := templates.PrivateMintStatementLayout().Len
	for _, c := range []struct {
		path, function string
		re             *regexp.Regexp
	}{
		{iEnygmaDvpSol, "", solMintSignals},
		{iPrivateMintVerifierSol, "verifyProof", solMintInput},
		{privateMintVerifierSol, "verifyProof", solMintInput},
	} {
		src := readFile(t, c.path)
		if c.function != "" {
			src = functionSignature(t, src, c.function)
		}
		m := c.re.FindStringSubmatch(src)
		if m == nil {
			t.Errorf("%s: no private-mint public-signal array", c.path)
			continue
		}
		if got := resolve(t, lens, m[1]); got != want {
			t.Errorf("%s sizes private-mint public signals %d, PrivateMint's statement length is %d", c.path, got, want)
		}
	}
}

// circuitOf returns the circuit an offset constant name (without _OFFSET)
// belongs to, by its longest matching circuit prefix.
func circuitOf(constant string, circuits map[string]int) (string, bool) {
	best := ""
	for c := range circuits {
		if strings.HasPrefix(constant, c+"_") && len(c) > len(best) {
			best = c
		}
	}
	return best, best != ""
}

// resolve reads a length operand: a literal or a generated *_LEN constant.
func resolve(t *testing.T, lens map[string]int, operand string) int {
	t.Helper()
	if n, err := strconv.Atoi(operand); err == nil {
		return n
	}
	n, ok := lens[operand]
	if !ok {
		t.Fatalf("length operand %s is neither a literal nor a generated *_LEN constant", operand)
	}
	return n
}

// generatedLens reads the *_LEN constants from the committed Solidity
// library (which main_test.go keeps equal to the layouts).
func generatedLens(t *testing.T) map[string]int {
	t.Helper()
	lens := map[string]int{}
	for _, m := range solLen.FindAllStringSubmatch(readFile(t, solOut), -1) {
		n, _ := strconv.Atoi(m[2])
		lens[m[1]] = n
	}
	return lens
}

// functionSignature returns the parameter list of the Solidity function name.
func functionSignature(t *testing.T, src, name string) string {
	t.Helper()
	start := strings.Index(src, "function "+name+"(")
	if start < 0 {
		t.Fatalf("no function %s", name)
	}
	end := strings.Index(src[start:], ")")
	return src[start : start+end+1]
}

// functionBody returns the body of the Solidity function name, braces
// included.
func functionBody(t *testing.T, src, name string) string {
	t.Helper()
	start := strings.Index(src, "function "+name+"(")
	if start < 0 {
		t.Fatalf("no function %s", name)
	}
	open := strings.Index(src[start:], "{")
	if open < 0 {
		t.Fatalf("function %s has no body", name)
	}
	depth := 0
	for i := start + open; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[start+open : i+1]
			}
		}
	}
	t.Fatalf("function %s: unbalanced braces", name)
	return ""
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
