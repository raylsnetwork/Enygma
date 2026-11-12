// export_vk_init exports all gnark BN254 Groth16 verifying keys to circom-format
// JSON files that scripts/cmd/init/main.go can read via getVerificationKeys().
//
// Usage (run from gnark_circuits/ directory):
//
//	go run ./cmd/export_vk_init/ [-placeholder] [-config ../enygmadvp.config.json] <build_dir>
//
// Example:
//
//	go run ./cmd/export_vk_init/ ../build
//
// init registers one VK per circuit in enygmadvp.config.json, in order, so
// every one needs a JSON. A circuit whose key file is missing (or that has no
// key file, see circuitMapping) keeps the JSON already in <build_dir>, if
// there is one. Otherwise it is an error naming it, unless -placeholder is
// given: then it gets an unprovable placeholder VK (see placeholderVK), which
// keeps the slot numbering and verifies no proof. make dvp-stack passes it for
// a local chain; slots 1-22 are never exercised by the integration tests.
//
// Output: one <CircuitName>.json per circuit in <build_dir>, matching the
// VerificationKeyJSON struct used by scripts/cmd/init/main.go:
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
	"errors"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	groth16bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// circomVK mirrors the VerificationKeyJSON struct in scripts/cmd/init/main.go.
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
// The vault contracts use hardcoded VK indices (VK_ID_ERC20_JOINSPLIT=0,
// VK_ID_DVP_INITIATOR=23, VK_ID_DVP_DESTINATION=24), and init registers a VK
// for every circuit in enygmadvp.config.json, in order. Positions 1-22 use
// pre-existing keys only to keep that numbering; no integration test
// exercises them, and keygen does not make them. BrokerRegistration,
// LegitBroker, JoinSplitErc20WithBrokerV1 and JoinSplitErc1155WithBrokerV1
// are in the config but have no key file here at all: -placeholder covers
// them and any missing key.
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

// readVK reads a gnark BN254 Groth16 verifying key file.
func readVK(keyPath string) (*groth16bn254.VerifyingKey, error) {
	f, err := os.Open(keyPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", keyPath, err)
	}
	defer f.Close()

	vkIface := groth16.NewVerifyingKey(ecc.BN254)
	if _, err := vkIface.ReadFrom(f); err != nil {
		return nil, fmt.Errorf("read vk from %s: %w", keyPath, err)
	}
	vk, ok := vkIface.(*groth16bn254.VerifyingKey)
	if !ok {
		return nil, fmt.Errorf("unexpected VK type %T", vkIface)
	}
	return vk, nil
}

// placeholderCircuit has one public input and one constraint.
type placeholderCircuit struct {
	X frontend.Variable `gnark:",public"`
}

func (c *placeholderCircuit) Define(api frontend.API) error {
	api.AssertIsEqual(c.X, c.X)
	return nil
}

// placeholderVK returns the VK of a fresh trusted setup of placeholderCircuit
// whose proving key is discarded: it is a well-formed key the verifier
// contract accepts, and no proof verifies against it.
func placeholderVK() (*groth16bn254.VerifyingKey, error) {
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &placeholderCircuit{})
	if err != nil {
		return nil, fmt.Errorf("compile placeholder circuit: %w", err)
	}
	_, vk, err := groth16.Setup(ccs)
	if err != nil {
		return nil, fmt.Errorf("set up placeholder circuit: %w", err)
	}
	return vk.(*groth16bn254.VerifyingKey), nil
}

func writeVK(vk *groth16bn254.VerifyingKey, outPath string) error {
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

// configCircuits returns the circuit names enygmadvp.config.json registers,
// in order.
func configCircuits(configPath string) ([]string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var cfg struct {
		Circom struct {
			Circuits []struct {
				Filename string `json:"filename"`
			} `json:"circuits"`
		} `json:"circom"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", configPath, err)
	}
	names := make([]string, len(cfg.Circom.Circuits))
	for i, c := range cfg.Circom.Circuits {
		names[i] = c.Filename
	}
	return names, nil
}

// exported is what exportAll did with each circuit.
type exported struct {
	Exported  []string // written from its key file
	Kept      []string // no key file: the JSON already in the build dir is kept
	Placeheld []string // no key file or JSON: an unprovable placeholder VK
}

// exportAll writes <buildDir>/<name>.json for each circuit, from its key file
// in mapping. A circuit with no key file keeps the JSON already in buildDir
// if there is one (an earlier setup's); without one it is a placeholder
// (placeholderVK) when placeholder is set, and missing otherwise. Missing
// circuits get no file and make exportAll return an error naming them.
func exportAll(circuits []string, mapping map[string]string, buildDir string, placeholder bool) (exported, error) {
	var out exported
	var missing []string
	var stand *groth16bn254.VerifyingKey
	for _, name := range circuits {
		outPath := filepath.Join(buildDir, name+".json")
		keyPath, mapped := mapping[name]
		var vk *groth16bn254.VerifyingKey
		var readErr error
		if mapped {
			vk, readErr = readVK(keyPath)
		}
		if mapped && readErr == nil {
			if err := writeVK(vk, outPath); err != nil {
				return out, err
			}
			out.Exported = append(out.Exported, name)
			continue
		}
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return out, readErr
		}
		if _, err := os.Stat(outPath); err == nil {
			out.Kept = append(out.Kept, name)
			continue
		}
		if !placeholder {
			missing = append(missing, name)
			continue
		}
		if stand == nil {
			var err error
			if stand, err = placeholderVK(); err != nil {
				return out, err
			}
		}
		if err := writeVK(stand, outPath); err != nil {
			return out, err
		}
		out.Placeheld = append(out.Placeheld, name)
	}
	if len(missing) > 0 {
		return out, fmt.Errorf("no key file and no %s/<name>.json for %s; run keygen, or pass -placeholder for a local chain (an unprovable VK in each such slot)",
			buildDir, strings.Join(missing, ", "))
	}
	return out, nil
}

func main() {
	placeholder := flag.Bool("placeholder", false, "give a circuit with no key file an unprovable placeholder VK")
	configPath := flag.String("config", "../enygmadvp.config.json", "enygmadvp.config.json, whose circuits init registers")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: export_vk_init [-placeholder] [-config <path>] <build_dir>")
		os.Exit(1)
	}
	buildDir := flag.Arg(0)

	if err := os.MkdirAll(buildDir, 0755); err != nil {
		log.Fatalf("create build dir: %v", err)
	}
	circuits, err := configCircuits(*configPath)
	if err != nil {
		log.Fatalf("read config: %v", err)
	}
	out, err := exportAll(circuits, circuitMapping, buildDir, *placeholder)
	for _, name := range out.Exported {
		fmt.Printf("  exported %s\n", name)
	}
	for _, name := range out.Kept {
		fmt.Printf("  kept %s (no key file; %s.json already in the build dir)\n", name, name)
	}
	for _, name := range out.Placeheld {
		fmt.Printf("  placeholder %s (unprovable VK)\n", name)
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Done: %d exported, %d kept, %d placeholders, of the config's %d circuits, to %s\n",
		len(out.Exported), len(out.Kept), len(out.Placeheld), len(circuits), buildDir)
}
