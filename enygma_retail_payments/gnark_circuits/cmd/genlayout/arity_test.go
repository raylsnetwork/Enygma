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
	solVerifyProof = regexp.MustCompile(`verifyProof\(\s*(VK_ID_\w+)`)
)

// Erc20CoinVault picks a Payment-family verifying key by statement length,
// and checks a receipt's statement length before settling it, but nothing
// ties a verifying key id or a function to the length of the circuit whose
// key it is. This pins each such check to its circuit's statement length,
// whether it is written as a generated *_LEN constant or as a literal.
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

	// The verifier each length dispatches to in checkReceiptConditions: the
	// length compared last before a verifyProof call is the one guarding it.
	dispatch := map[string]int{
		"VK_ID_ERC20_JOINSPLIT_FEE": templates.PaymentFeeStatementLayout().Len,
		"VK_ID_ERC20_USDR":          templates.UsdrFeeStatementLayout().Len,
	}
	body := functionBody(t, src, "checkReceiptConditions")
	seen := map[string]bool{}
	for _, m := range solVerifyProof.FindAllStringSubmatchIndex(body, -1) {
		vk := body[m[2]:m[3]]
		want, ok := dispatch[vk]
		if !ok {
			continue
		}
		checks := solLengthCheck.FindAllStringSubmatch(body[:m[0]], -1)
		if len(checks) == 0 {
			t.Errorf("verifyProof(%s): no statement length check guards it", vk)
			continue
		}
		if got := resolve(checks[len(checks)-1][1]); got != want {
			t.Errorf("verifyProof(%s) is guarded by statement length %d, its circuit's is %d", vk, got, want)
		}
		seen[vk] = true
	}
	for vk := range dispatch {
		if !seen[vk] {
			t.Errorf("checkReceiptConditions has no verifyProof(%s)", vk)
		}
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
