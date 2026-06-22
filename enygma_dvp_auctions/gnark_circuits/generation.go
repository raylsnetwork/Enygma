package main

// Key generation for the auction circuits.
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
	generateAuctionWithdraw()
	generateAuctionBatch()
	generateAuctionFinal()
	generateAuctionRevert()

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

func generateAuctionBatch() {
	n := templates.AuctionBatchSize
	circuit := templates.AuctionBatchCircuit{
		StBidCommitA: make([]frontend.Variable, n),
		WtActive:     make([]frontend.Variable, n),
		WtPkBidder:   make([]frontend.Variable, n),
		WtSaltA:      make([]frontend.Variable, n),
		WtAmount:     make([]frontend.Variable, n),
		WtTokenId:    make([]frontend.Variable, n),
	}

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		log.Fatalf("AuctionBatch compile: %v", err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		log.Fatalf("AuctionBatch setup: %v", err)
	}
	saveKeys("AuctionBatch", pk, vk)
}

func generateAuctionFinal() {
	k := templates.AuctionFinalBatches
	circuit := templates.AuctionFinalCircuit{
		StBatchWinnerCommit: make([]frontend.Variable, k),
		StBatchWinnerPk:     make([]frontend.Variable, k),
		StBatchWinnerAmount: make([]frontend.Variable, k),
		WtBatchActive:       make([]frontend.Variable, k),
	}

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		log.Fatalf("AuctionFinal compile: %v", err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		log.Fatalf("AuctionFinal setup: %v", err)
	}
	saveKeys("AuctionFinal", pk, vk)
}

func generateAuctionWithdraw() {
	circuit := templates.AuctionWithdrawCircuit{}

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		log.Fatalf("AuctionWithdraw compile: %v", err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		log.Fatalf("AuctionWithdraw setup: %v", err)
	}
	saveKeys("AuctionWithdraw", pk, vk)
}

func generateAuctionRevert() {
	circuit := templates.AuctionRevertCircuit{}

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		log.Fatalf("AuctionRevert compile: %v", err)
	}
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		log.Fatalf("AuctionRevert setup: %v", err)
	}
	saveKeys("AuctionRevert", pk, vk)
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
