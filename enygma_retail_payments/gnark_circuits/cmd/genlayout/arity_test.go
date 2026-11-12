package main

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"enygma_retail_payments/gnark_circuits/templates"
)

const erc20CoinVaultSol = "../../../../enygma_dvp/contracts/core/contracts/vaults/Erc20CoinVault.sol"

var (
	solLen         = regexp.MustCompile(`uint256 constant (\w+_LEN) = (\d+);`)
	solLengthCheck = regexp.MustCompile(`statement\.length\s*[!=]=\s*(\w+)`)
	solLengthLen   = regexp.MustCompile(`\?\s*(\w+_LEN)\s*:\s*(\w+_LEN)`)
	// One _receiptShape clause: the exact receipt shape that selects a key.
	solShape = regexp.MustCompile(`nIn\s*==\s*(\d+)\s*&&\s*nOut\s*==\s*(\d+)\s*&&\s*len\s*==\s*(\w+)\s*\)\s*\{\s*return\s*\(\s*(VK_ID_\w+)`)
)

// Erc20CoinVault picks a Payment-family verifying key from a receipt's exact
// (numberOfInputs, numberOfOutputs, statement length), and checks a receipt's
// statement length before settling it, but nothing ties a verifying key id or
// a function to the shape of the circuit whose key it is. This pins each such
// check to its circuit's shape, whether the length is written as a generated
// *_LEN constant or as a literal.
func TestVaultLengthChecksMatchStatementLengths(t *testing.T) {
	src := readFile(t, erc20CoinVaultSol)
	lens := generatedLens(t)
	resolve := func(operand string) int {
		if n, err := strconv.Atoi(operand); err == nil {
			return n
		}
		n, ok := lens[operand]
		if !ok {
			t.Fatalf("length operand %s is neither a literal nor a generated *_LEN constant", operand)
		}
		return n
	}

	// The shape each Payment-family key is selected by in _receiptShape, which
	// checkReceiptConditions verifies every receipt with: the deployed config's
	// input and output counts and the layout's statement length.
	type receiptShape struct{ in, out, len int }
	circuitShape := func(c templates.PaymentCircuitConfig, l int) receiptShape {
		return receiptShape{c.TmNInputs, c.TmMOutputs, l}
	}
	dispatch := map[string]receiptShape{
		"VK_ID_ERC20_JOINSPLIT":         circuitShape(templates.PaymentConfig, templates.PaymentStatementLayout().Len),
		"VK_ID_ERC20_JOINSPLIT_2INPUT":  circuitShape(templates.Payment2inConfig, templates.Payment2inStatementLayout().Len),
		"VK_ID_ERC20_JOINSPLIT_FEE":     circuitShape(templates.PaymentFeeConfig, templates.PaymentFeeStatementLayout().Len),
		"VK_ID_ERC20_JOINSPLIT_RELAYER": circuitShape(templates.PaymentRelayerFeePublicConfig, templates.PaymentRelayerFeeStatementLayout().Len),
		"VK_ID_ERC20_USDR":              circuitShape(templates.UsdrFeeConfig, templates.UsdrFeeStatementLayout().Len),
	}
	seen := map[string]int{}
	for _, m := range solShape.FindAllStringSubmatch(functionBody(t, src, "_receiptShape"), -1) {
		vk := m[4]
		want, ok := dispatch[vk]
		if !ok {
			continue
		}
		in, _ := strconv.Atoi(m[1])
		out, _ := strconv.Atoi(m[2])
		if got := (receiptShape{in, out, resolve(m[3])}); got != want {
			t.Errorf("_receiptShape selects %s for %d in, %d out, %d signals; its circuit is %d in, %d out, %d signals",
				vk, got.in, got.out, got.len, want.in, want.out, want.len)
		}
		seen[vk]++
	}
	for vk := range dispatch {
		if seen[vk] != 1 {
			t.Errorf("_receiptShape selects %s in %d clauses, want exactly 1", vk, seen[vk])
		}
	}
	if !strings.Contains(functionBody(t, src, "checkReceiptConditions"), "_receiptShape(receipt)") {
		t.Error("checkReceiptConditions does not choose its verifying key with _receiptShape")
	}

	// partialWithdrawWithRelayerFee settles only PaymentRelayerFee receipts.
	relayer := solLengthCheck.FindAllStringSubmatch(functionBody(t, src, "partialWithdrawWithRelayerFee"), -1)
	if len(relayer) != 1 {
		t.Fatalf("partialWithdrawWithRelayerFee: %d statement length checks, want 1", len(relayer))
	}
	if got, want := resolve(relayer[0][1]), templates.PaymentRelayerFeeStatementLayout().Len; got != want {
		t.Errorf("partialWithdrawWithRelayerFee checks statement length %d, PaymentRelayerFee's is %d", got, want)
	}

	// _requirePlainPaymentShape settles only Payment (1 input) and
	// Payment2in receipts, choosing the length by the number of inputs.
	shape := solLengthLen.FindStringSubmatch(functionBody(t, src, "_requirePlainPaymentShape"))
	if shape == nil {
		t.Fatal("_requirePlainPaymentShape: no `numberOfInputs == 1 ? X_LEN : Y_LEN` length choice")
	}
	if got, want := resolve(shape[1]), templates.PaymentStatementLayout().Len; got != want {
		t.Errorf("_requirePlainPaymentShape: 1-input length %d, Payment's is %d", got, want)
	}
	if got, want := resolve(shape[2]), templates.Payment2inStatementLayout().Len; got != want {
		t.Errorf("_requirePlainPaymentShape: 2-input length %d, Payment2in's is %d", got, want)
	}
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
