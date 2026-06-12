package main

// Key generation for the three auction circuits.
//
// Run with:
//
//	go run generation.go
//
// Keys are written to ./scripts/keys/.

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
	if err := os.MkdirAll("./scripts/keys", 0755); err != nil {
		log.Fatalf("mkdir keys: %v", err)
	}

	solver.RegisterHint(primitives.PoseidonNative)
	solver.RegisterHint(primitives.PoseidonPrivateKeyNative)

	generateAuctionLock()
	generateAuctionBid()
	generateAuctionSettle()
	generateAuctionNotWinning()

	fmt.Println("all keys generated → ./scripts/keys/")
}

func generateAuctionLock() {
	const depth = 8
	cfg := templates.AuctionLockCircuitConfig{TmMerkleTreeDepth: depth}
	circuit := templates.AuctionLockCircuit{
		Config:         cfg,
		WtPathElements: make([]frontend.Variable, depth),
	}

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		log.Fatalf("AuctionLock compile: %v", err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		log.Fatalf("AuctionLock setup: %v", err)
	}
	saveKeys("AuctionLock", pk, vk)
}

func generateAuctionBid() {
	const depth = 8
	cfg := templates.AuctionBidCircuitConfig{
		TmMerkleTreeDepth: depth,
		TmRange:           frontend.Variable("1000000000000000000000000000000000000"),
	}
	circuit := templates.AuctionBidCircuit{
		Config:         cfg,
		WtPathElements: make([]frontend.Variable, depth),
	}

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		log.Fatalf("AuctionBid compile: %v", err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		log.Fatalf("AuctionBid setup: %v", err)
	}
	saveKeys("AuctionBid", pk, vk)
}

func generateAuctionSettle() {
	cfg := templates.AuctionSettleCircuitConfig{
		TmRange: frontend.Variable("1000000000000000000000000000000000000"),
	}
	circuit := templates.AuctionSettleCircuit{Config: cfg}

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		log.Fatalf("AuctionSettle compile: %v", err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		log.Fatalf("AuctionSettle setup: %v", err)
	}
	saveKeys("AuctionSettle", pk, vk)
}

func generateAuctionNotWinning() {
	cfg := templates.AuctionNotWinningCircuit{}
	circuit := cfg

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		log.Fatalf("AuctionNotWinning compile: %v", err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		log.Fatalf("AuctionNotWinning setup: %v", err)
	}
	saveKeys("AuctionNotWinning", pk, vk)
}

func saveKeys(name string, pk groth16.ProvingKey, vk groth16.VerifyingKey) {
	pkPath := fmt.Sprintf("./scripts/keys/%s.pk", name)
	vkPath := fmt.Sprintf("./scripts/keys/%s.vk", name)

	fpk, err := os.Create(pkPath)
	if err != nil {
		log.Fatalf("%s create pk: %v", name, err)
	}
	defer fpk.Close()
	if _, err := pk.WriteTo(fpk); err != nil {
		log.Fatalf("%s write pk: %v", name, err)
	}

	fvk, err := os.Create(vkPath)
	if err != nil {
		log.Fatalf("%s create vk: %v", name, err)
	}
	defer fvk.Close()
	if _, err := vk.WriteTo(fvk); err != nil {
		log.Fatalf("%s write vk: %v", name, err)
	}

	fmt.Printf("  %s: pk=%s  vk=%s\n", name, pkPath, vkPath)
}
