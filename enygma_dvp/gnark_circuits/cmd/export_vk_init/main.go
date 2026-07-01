// export_vk_init exports all gnark BN254 Groth16 verifying keys to circom-format
// JSON files that scripts/init.go can read via getVerificationKeys().
//
// Usage (run from gnark_circuits/ directory):
//
//	go run ./cmd/export_vk_init/ <build_dir>
//
// Example:
//
//	go run ./cmd/export_vk_init/ ../build
//
// Output: one <CircuitName>.json per circuit in <build_dir>, matching the
// VerificationKeyJSON struct used by scripts/init.go:
//
//	{
//	  "protocol": "groth16",
//	  "curve": "bn128",
//	  "vk_alpha_1": ["alpha_x", "alpha_y", "1"],
//	  "vk_beta_2": [["beta_x_im","beta_x_re"],["beta_y_im","beta_y_re"],["0","1"]],
//	  "vk_gamma_2": [...],
//	  "vk_delta_2": [...],
//	  "IC": [["K0_x","K0_y","1"], ...]
//	}
//
// G2 coordinate convention (EIP-197):
//
//	x[0] = imaginary (A1), x[1] = real (A0)   ← same as export_vk tool
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

// circomVK mirrors the VerificationKeyJSON struct in scripts/init.go.
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

// circuitMapping maps circuit names (used in enygmadvp.config.json) to VK key files
// (relative to the gnark_circuits/ directory).
//
// All 21 entries must be present because the vault contracts use hardcoded VK
// indices: VK_ID_ERC20_JOINSPLIT=0, VK_ID_DVP_INITIATOR=23, VK_ID_DVP_DESTINATION=24.
// Entries at unused positions (1–22) use pre-existing keys so the index numbering
// is maintained; those VKs are never exercised by integration tests (test/01–04).
var circuitMapping = map[string]string{
	// index 0 — covered by test/02 (Payment uses VK_ID_ERC20_JOINSPLIT=0)
	"Payment": "scripts/keys/PaymentVK.key",

	// indices 1–22 — NOT covered by integration tests; old keys kept for index alignment
	"OwnershipErc721":                        "scripts/keys/OwnershipERC721VK.key",
	"OwnershipErc1155NonFungible":            "scripts/keys/OwnershipERC1155NonFungibleVK.key",
	"OwnershipErc1155Fungible":               "scripts/keys/OwnershipERC1155FungibleVK.key",
	"JoinSplitErc1155":                       "scripts/keys/JoiSplitERC1155VK.key",
	"BatchErc1155":                           "scripts/keys/ERC1155BatchVK.key",
	"JoinSplitErc20_10_2":                    "scripts/keys/JoinErc20_10_2VK.key",
	"AuctionInit":                            "scripts/keys/AuctionInitVK.key",
	"AuctionBid":                             "scripts/keys/AuctionBidVK.key",
	"AuctionNotWinningBid":                   "scripts/keys/AuctionNotWinningVK.key",
	"AuctionPrivateOpening":                  "scripts/keys/AuctionPrivateOpeningVK.key",
"JoinSplitErc1155WithAuditor":            "scripts/keys/JoinSplitERC1155AuditorVK.key",
	"OwnershipErc1155NonFungibleWithAuditor": "scripts/keys/OwnershipERC1155NonFungibleAuditorVK.key",
	"BatchErc1155NonFungibleWithAuditor":     "scripts/keys/ERC1155BatchAuditorVK.key",
	"OwnershipErc721WithAuditor":             "scripts/keys/OwnershipERC721AuditorVK.key",
	"JoinSplitErc20WithAuditor":              "scripts/keys/JoinERC20AuditorVK.key",
	"JoinSplitErc20_10_2_WithAuditor":        "scripts/keys/JoinERC20_10_2AuditorVK.key",
	"AuctionInit_Auditor":                    "scripts/keys/AuctionInitAuditorVK.key",
	"AuctionBid_Auditor":                     "scripts/keys/AuctionBidAuditorVK.key",

	// index 23 — covered by test/03 and test/04 (VK_ID_DVP_INITIATOR=23)
	"DvPInitiator": "scripts/keys/DvPInitiatorVK.key",
	// index 24 — covered by test/03 and test/04 (VK_ID_DVP_DESTINATION=24)
	"DvPDestination": "scripts/keys/DvPDestinationVK.key",
}

func g1Fields(x, y *big.Int) []string {
	return []string{x.String(), y.String(), "1"}
}

// g2Fields returns [[x_im, x_re], [y_im, y_re], ["0", "1"]]
// matching the circom/snarkjs format where index 0 = imaginary (A1), 1 = real (A0).
func g2Fields(xA0, xA1, yA0, yA1 *big.Int) [][]string {
	return [][]string{
		{xA1.String(), xA0.String()}, // [imaginary, real]
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

	// alpha1
	var ax, ay big.Int
	vk.G1.Alpha.X.BigInt(&ax)
	vk.G1.Alpha.Y.BigInt(&ay)

	// beta2
	var bxA0, bxA1, byA0, byA1 big.Int
	vk.G2.Beta.X.A0.BigInt(&bxA0)
	vk.G2.Beta.X.A1.BigInt(&bxA1)
	vk.G2.Beta.Y.A0.BigInt(&byA0)
	vk.G2.Beta.Y.A1.BigInt(&byA1)

	// gamma2
	var gxA0, gxA1, gyA0, gyA1 big.Int
	vk.G2.Gamma.X.A0.BigInt(&gxA0)
	vk.G2.Gamma.X.A1.BigInt(&gxA1)
	vk.G2.Gamma.Y.A0.BigInt(&gyA0)
	vk.G2.Gamma.Y.A1.BigInt(&gyA1)

	// delta2
	var dxA0, dxA1, dyA0, dyA1 big.Int
	vk.G2.Delta.X.A0.BigInt(&dxA0)
	vk.G2.Delta.X.A1.BigInt(&dxA1)
	vk.G2.Delta.Y.A0.BigInt(&dyA0)
	vk.G2.Delta.Y.A1.BigInt(&dyA1)

	// IC points
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

	if err := os.WriteFile(outPath, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: export_vk_init <build_dir>")
		os.Exit(1)
	}
	buildDir := os.Args[1]

	if err := os.MkdirAll(buildDir, 0755); err != nil {
		log.Fatalf("create build dir: %v", err)
	}

	ok := 0
	for name, vkRelPath := range circuitMapping {
		outPath := filepath.Join(buildDir, name+".json")
		if err := exportVK(vkRelPath, outPath); err != nil {
			log.Printf("WARN: skipping %s: %v", name, err)
			continue
		}
		fmt.Printf("  exported %s → %s\n", name, outPath)
		ok++
	}
	fmt.Printf("Done: %d/%d VKs exported to %s\n", ok, len(circuitMapping), buildDir)
}
