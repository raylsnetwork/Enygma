// Key generation entry point. Run from gnark_circuits/ with: go run ./cmd/keygen
// (all circuits) or go run ./cmd/keygen -only Payment,PaymentFee (just those).
package main

import (
	"flag"
	"log"
	"strings"

	"github.com/consensys/gnark/constraint/solver"

	"enygma_retail_payments/gnark_circuits/primitives"
	script "enygma_retail_payments/gnark_circuits/scripts"
	"enygma_retail_payments/gnark_circuits/templates"
)

// GenerationVkPk regenerates every circuit named in only, or all of them if only
// is empty. Each run draws fresh random keys, so regenerate only the circuits
// whose constraints changed: a regenerated PrivateMint key, for one, also needs
// a regenerated PrivateMintVerifier contract.
func GenerationVkPk(only map[string]bool) {
	want := func(name string) bool { return len(only) == 0 || only[name] }
	solver.RegisterHint(primitives.ModHint)
	solver.RegisterHint(primitives.PoseidonNative)
	solver.RegisterHint(primitives.PoseidonPrivateKeyNative)

	if want("Payment") {
		script.SetupPayment(templates.PaymentConfig, "Payment")
	}

	if want("Payment2in") {
		script.SetupPayment(templates.Payment2inConfig, "Payment2in")
	}

	if want("PaymentFee") {
		script.SetupPaymentFee(templates.PaymentFeeConfig, "PaymentFee")
	}

	if want("PaymentRelayerFeePublic") {
		script.SetupPaymentRelayerFeePublic(templates.PaymentRelayerFeePublicConfig, "PaymentRelayerFeePublic")
	}

	if want("UsdrFee") {
		script.SetupUsdrFee(templates.UsdrFeeConfig, "UsdrFee")
	}

	if want("PrivateMint") {
		script.SetupPrivateMint("PrivateMint")
	}
}

var circuits = []string{"Payment", "Payment2in", "PaymentFee", "PaymentRelayerFeePublic", "UsdrFee", "PrivateMint"}

func main() {
	onlyFlag := flag.String("only", "", "comma-separated circuits to regenerate (default: all): "+strings.Join(circuits, ","))
	flag.Parse()
	only := map[string]bool{}
	for _, name := range strings.Split(*onlyFlag, ",") {
		if name = strings.TrimSpace(name); name == "" {
			continue
		}
		known := false
		for _, c := range circuits {
			known = known || c == name
		}
		if !known {
			log.Fatalf("unknown circuit %q", name)
		}
		only[name] = true
	}
	GenerationVkPk(only)
}
