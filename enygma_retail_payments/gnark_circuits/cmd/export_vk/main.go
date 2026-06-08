// export_vk exports the Payment gnark BN254 Groth16 verifying key to a
// circom-format JSON file that scripts/init.go reads via getVerificationKeys().
//
// Usage (run from gnark_circuits/ directory):
//
//	go run ./cmd/export_vk/ <build_dir>
//
// Example:
//
//	go run ./cmd/export_vk/ ../build
//
// Output: build/Payment.json in circom/snarkjs format:
//
//	{
//	  "protocol": "groth16",
//	  "curve": "bn128",
//	  "vk_alpha_1": ["alpha_x", "alpha_y", "1"],
//	  "vk_beta_2":  [["beta_x_im","beta_x_re"],["beta_y_im","beta_y_re"],["0","1"]],
//	  ...
//	  "IC": [["K0_x","K0_y","1"], ...]
//	}
//
// G2 coordinate convention (EIP-197): index 0 = imaginary (A1), index 1 = real (A0).
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	groth16bn254 "github.com/consensys/gnark/backend/groth16/bn254"
)

type circomVK struct {
	Protocol    string       `json:"protocol"`
	Curve       string       `json:"curve"`
	VkAlpha1    []string     `json:"vk_alpha_1"`
	VkBeta2     [][]string   `json:"vk_beta_2"`
	VkGamma2    [][]string   `json:"vk_gamma_2"`
	VkDelta2    [][]string   `json:"vk_delta_2"`
	VkAlphabeta [][][]string `json:"vk_alphabeta_12"`
	IC          [][]string   `json:"IC"`
}

func g1Fields(x, y *big.Int) []string {
	return []string{x.String(), y.String(), "1"}
}

// g2Fields returns [[x_im, x_re], [y_im, y_re], ["0", "1"]]
// where index 0 = imaginary (A1), index 1 = real (A0).
func g2Fields(xA0, xA1, yA0, yA1 *big.Int) [][]string {
	return [][]string{
		{xA1.String(), xA0.String()},
		{yA1.String(), yA0.String()},
		{"0", "1"},
	}
}

func exportVK(keyPath, outPath string) error {
	f, err := os.Open(keyPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", keyPath, err)
	}
	defer f.Close()

	vkIface := groth16.NewVerifyingKey(ecc.BN254)
	if _, err := vkIface.ReadFrom(f); err != nil {
		return fmt.Errorf("read vk from %s: %w", keyPath, err)
	}

	vk, ok := vkIface.(*groth16bn254.VerifyingKey)
	if !ok {
		return fmt.Errorf("unexpected VK type %T", vkIface)
	}

	var ax, ay big.Int
	vk.G1.Alpha.X.BigInt(&ax)
	vk.G1.Alpha.Y.BigInt(&ay)

	var bxA0, bxA1, byA0, byA1 big.Int
	vk.G2.Beta.X.A0.BigInt(&bxA0)
	vk.G2.Beta.X.A1.BigInt(&bxA1)
	vk.G2.Beta.Y.A0.BigInt(&byA0)
	vk.G2.Beta.Y.A1.BigInt(&byA1)

	var gxA0, gxA1, gyA0, gyA1 big.Int
	vk.G2.Gamma.X.A0.BigInt(&gxA0)
	vk.G2.Gamma.X.A1.BigInt(&gxA1)
	vk.G2.Gamma.Y.A0.BigInt(&gyA0)
	vk.G2.Gamma.Y.A1.BigInt(&gyA1)

	var dxA0, dxA1, dyA0, dyA1 big.Int
	vk.G2.Delta.X.A0.BigInt(&dxA0)
	vk.G2.Delta.X.A1.BigInt(&dxA1)
	vk.G2.Delta.Y.A0.BigInt(&dyA0)
	vk.G2.Delta.Y.A1.BigInt(&dyA1)

	ic := make([][]string, len(vk.G1.K))
	for i, pt := range vk.G1.K {
		var kx, ky big.Int
		pt.X.BigInt(&kx)
		pt.Y.BigInt(&ky)
		ic[i] = g1Fields(&kx, &ky)
	}

	out := circomVK{
		Protocol:    "groth16",
		Curve:       "bn128",
		VkAlpha1:    g1Fields(&ax, &ay),
		VkBeta2:     g2Fields(&bxA0, &bxA1, &byA0, &byA1),
		VkGamma2:    g2Fields(&gxA0, &gxA1, &gyA0, &gyA1),
		VkDelta2:    g2Fields(&dxA0, &dxA1, &dyA0, &dyA1),
		VkAlphabeta: [][][]string{},
		IC:          ic,
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(outPath, data, 0644)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run ./cmd/export_vk/ <build_dir>")
		os.Exit(1)
	}
	buildDir := os.Args[1]

	if err := os.MkdirAll(buildDir, 0755); err != nil {
		log.Fatalf("create build dir: %v", err)
	}

	exports := []struct{ keyFile, outName string }{
		{"scripts/keys/PaymentVK.key", "Payment.json"},
		{"scripts/keys/Payment2inVK.key", "Payment2in.json"},
	}
	for _, e := range exports {
		outPath := filepath.Join(buildDir, e.outName)
		if err := exportVK(e.keyFile, outPath); err != nil {
			log.Fatalf("export %s: %v", e.keyFile, err)
		}
		fmt.Printf("exported %s → %s\n", e.keyFile, outPath)
	}
}
