// export_verifier exports the PrivateMint gnark BN254 Groth16 verifying key
// as a Solidity contract file that can be compiled with solc/Hardhat.
//
// Usage (run from gnark_circuits/ directory):
//
//	go run ./cmd/export_verifier/ <output.sol>
//
// Example:
//
//	go run ./cmd/export_verifier/ /tmp/PrivateMintVerifier.sol
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run ./cmd/export_verifier/ <output.sol>")
		os.Exit(1)
	}
	outPath := os.Args[1]

	f, err := os.Open("scripts/keys/PrivateMintVK.key")
	if err != nil {
		log.Fatalf("open VK: %v", err)
	}
	defer f.Close()

	vk := groth16.NewVerifyingKey(ecc.BN254)
	if _, err := vk.ReadFrom(f); err != nil {
		log.Fatalf("read VK: %v", err)
	}

	out, err := os.Create(outPath)
	if err != nil {
		log.Fatalf("create output: %v", err)
	}
	defer out.Close()

	if err := vk.ExportSolidity(out); err != nil {
		log.Fatalf("ExportSolidity: %v", err)
	}
	fmt.Printf("exported PrivateMintVK.key → %s\n", outPath)
}
