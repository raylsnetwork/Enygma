// Regenerates the PrivateMint proving and verifying keys after a circuit change.
// Run from the gnark_circuits directory:
//
//	go run ./cmd/setup_private_mint/
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"

	"gnark_server/primitives"
	"gnark_server/templates"
)

func main() {
	solver.RegisterHint(primitives.ModHint)
	

	circuit := templates.PrivateMintCircuit{}

	fmt.Println("Compiling PrivateMint circuit...")
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		log.Fatalf("compile failed: %v", err)
	}

	fmt.Println("Running Groth16 setup...")
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		log.Fatalf("setup failed: %v", err)
	}

	pkPath := "scripts/keys/PrivateMintPK.key"
	vkPath := "scripts/keys/PrivateMintVK.key"

	fpk, err := os.Create(pkPath)
	if err != nil {
		log.Fatalf("create PK file: %v", err)
	}
	defer fpk.Close()
	if _, err := pk.WriteTo(fpk); err != nil {
		log.Fatalf("write PK: %v", err)
	}

	fvk, err := os.Create(vkPath)
	if err != nil {
		log.Fatalf("create VK file: %v", err)
	}
	defer fvk.Close()
	if _, err := vk.WriteTo(fvk); err != nil {
		log.Fatalf("write VK: %v", err)
	}

	fmt.Printf("Done. Keys written to:\n  %s\n  %s\n", pkPath, vkPath)
}
