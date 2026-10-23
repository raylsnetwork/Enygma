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
	solTableRow    = regexp.MustCompile(`PaymentCircuit\(\{([^}]*)\}\)`)
	solRowVk       = regexp.MustCompile(`vkId:\s*(VK_ID_\w+)`)
	solPslName     = regexp.MustCompile(`Psl\.([A-Z][A-Z0-9_]*)_(?:OFFSET|SIZE)\b`)
	solIndexRead   = regexp.MustCompile(`statement\[([^\]]*)\]`)
	solTableIndex  = regexp.MustCompile(`^\s*((Psl|Dsl)\.[A-Z][A-Z0-9_]*_OFFSET|circuit\.\w+(\s*\+\s*(1|i))*)\s*$`)
)

// Erc20CoinVault picks a Payment-family circuit from a table by shape and
// statement length, and checks a receipt's statement length before settling
// it, but nothing ties a table row or a function to the length of the
// circuit it reads. This pins each such check to its circuit's statement
// length, whether it is written as a generated *_LEN constant or as a
// literal, and each row's offsets to its own circuit.
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

	// _paymentCircuit is the Payment-family table: each row names a
	// verifying key and the offsets the vault reads, and is returned after a
	// statement length check. Pin each row to one circuit: the length
	// compared last before it must be that circuit's, and every layout
	// constant in it must be that circuit's own.
	circuits := map[string]struct {
		prefix string
		len    int
	}{
		"VK_ID_ERC20_JOINSPLIT":         {"PAYMENT", templates.PaymentStatementLayout().Len},
		"VK_ID_ERC20_JOINSPLIT_2INPUT":  {"PAYMENT2IN", templates.Payment2inStatementLayout().Len},
		"VK_ID_ERC20_JOINSPLIT_FEE":     {"PAYMENT_FEE", templates.PaymentFeeStatementLayout().Len},
		"VK_ID_ERC20_JOINSPLIT_RELAYER": {"PAYMENT_RELAYER_FEE", templates.PaymentRelayerFeeStatementLayout().Len},
		"VK_ID_ERC20_USDR":              {"USDR_FEE", templates.UsdrFeeStatementLayout().Len},
	}
	prefixes := []string{}
	for _, c := range circuits {
		prefixes = append(prefixes, c.prefix)
	}
	// circuitOf returns the circuit a layout constant name belongs to, by
	// its longest matching circuit prefix.
	circuitOf := func(constant string) string {
		best := ""
		for _, p := range prefixes {
			if strings.HasPrefix(constant, p+"_") && len(p) > len(best) {
				best = p
			}
		}
		return best
	}
	table := functionBody(t, src, "_paymentCircuit")
	seen := map[string]bool{}
	for _, m := range solTableRow.FindAllStringSubmatchIndex(table, -1) {
		row := table[m[2]:m[3]]
		vk := solRowVk.FindStringSubmatch(row)
		if vk == nil {
			t.Errorf("_paymentCircuit row has no vkId: %s", row)
			continue
		}
		c, ok := circuits[vk[1]]
		if !ok {
			t.Errorf("_paymentCircuit row names %s, not a Payment-family verifying key", vk[1])
			continue
		}
		if seen[vk[1]] {
			t.Errorf("_paymentCircuit has two rows for %s", vk[1])
		}
		seen[vk[1]] = true
		checks := solLengthCheck.FindAllStringSubmatch(table[:m[0]], -1)
		if len(checks) == 0 {
			t.Errorf("_paymentCircuit row %s: no statement length check guards it", vk[1])
		} else if got := resolve(checks[len(checks)-1][1]); got != c.len {
			t.Errorf("_paymentCircuit row %s is guarded by statement length %d, its circuit's is %d", vk[1], got, c.len)
		}
		for _, k := range solPslName.FindAllStringSubmatch(row, -1) {
			if got := circuitOf(k[1]); got != c.prefix {
				t.Errorf("_paymentCircuit row %s reads Psl.%s, a %s constant, not %s's", vk[1], k[1], got, c.prefix)
			}
		}
	}
	for vk := range circuits {
		if !seen[vk] {
			t.Errorf("_paymentCircuit has no row for %s", vk)
		}
	}

	// Every read of a receipt's statement in the vault is at a named layout
	// offset or at a table row's offset: none is counted from the
	// caller-supplied numberOfInputs/numberOfOutputs.
	for _, m := range solIndexRead.FindAllStringSubmatch(src, -1) {
		if !solTableIndex.MatchString(m[1]) {
			t.Errorf("Erc20CoinVault reads statement[%s], not a named layout offset or a table row's", m[1])
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
