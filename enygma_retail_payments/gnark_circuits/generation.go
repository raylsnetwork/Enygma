// Key generation entry point. Run with: go run generation.go
// Note: main.go also declares func main (the gnark server). Both files share
// package main intentionally — run them individually, never with go build ./...
package main

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/constraint/solver"

	"gnark_server/templates"
	"gnark_server/primitives"
	script "gnark_server/scripts"
)

func GenerationVkPk() {
	solver.RegisterHint(primitives.ModHint)
	solver.RegisterHint(primitives.PoseidonNative)
	solver.RegisterHint(primitives.PoseidonPrivateKeyNative)

	payment1inConfig := templates.PaymentCircuitConfig{
		TmNInputs:         1,
		TmMOutputs:        2,
		TmMerkleTreeDepth: 8,
		TmRange:           frontend.Variable("1000000000000000000000000000000000000"),
	}
	script.SetupPayment(payment1inConfig, "Payment")

	// 2-input/2-output circuit: separate VK slot (VK_ID_ERC20_JOINSPLIT_2INPUT = 1).
	// Fixes VULN-2: 1-in and 2-in circuits have different R1CS and must use distinct VKs.
	payment2inConfig := templates.PaymentCircuitConfig{
		TmNInputs:         2,
		TmMOutputs:        2,
		TmMerkleTreeDepth: 8,
		TmRange:           frontend.Variable("1000000000000000000000000000000000000"),
	}
	script.SetupPayment(payment2inConfig, "Payment2in")

	paymentRelayerFeePublicConfig := templates.PaymentCircuitConfig{
		TmNInputs:         1,
		TmMOutputs:        3,
		TmMerkleTreeDepth: 8,
		TmRange:           frontend.Variable("1000000000000000000000000000000000000"),
	}
	script.SetupPaymentRelayerFeePublic(paymentRelayerFeePublicConfig, "PaymentRelayerFeePublic")

	script.SetupPrivateMint("PrivateMint")
}

func main() {
	GenerationVkPk()
}
