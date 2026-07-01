// bench_prover measures the Groth16 prover time for the three active circuits
// using real proving keys and a valid satisfying witness for each circuit.
//
// Run from gnark_circuits/:
//
//	go run ./cmd/bench_prover/
package main

import (
	"fmt"
	"math/big"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/iden3/go-iden3-crypto/poseidon"

	"gnark_server/primitives"
	"gnark_server/templates"
	"gnark_server/utils"
)

var BN254Order, _ = new(big.Int).SetString(
	"21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

func poseidon2(a, b *big.Int) *big.Int {
	h, _ := poseidon.Hash([]*big.Int{a, b})
	return h.Mod(h, BN254Order)
}

func poseidon4(a, b, c, d *big.Int) *big.Int {
	h, _ := poseidon.Hash([]*big.Int{a, b, c, d})
	return h.Mod(h, BN254Order)
}

func bench(label, pkPath string, circuit, witness frontend.Circuit) {
	pk, err := utils.LoadProvingKey(ecc.BN254, pkPath)
	if err != nil {
		fmt.Printf("%-20s ERROR loading PK: %v\n", label, err)
		return
	}

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		fmt.Printf("%-20s ERROR compiling: %v\n", label, err)
		return
	}

	witnessFull, err := frontend.NewWitness(witness, ecc.BN254.ScalarField())
	if err != nil {
		fmt.Printf("%-20s ERROR building witness: %v\n", label, err)
		return
	}

	// warm-up run (first run includes JIT overhead)
	if _, err = groth16.Prove(ccs, pk, witnessFull); err != nil {
		fmt.Printf("%-20s ERROR proving: %v\n", label, err)
		return
	}

	const runs = 3
	var total time.Duration
	for i := 0; i < runs; i++ {
		start := time.Now()
		if _, err = groth16.Prove(ccs, pk, witnessFull); err != nil {
			fmt.Printf("%-20s ERROR proving run %d: %v\n", label, i, err)
			return
		}
		total += time.Since(start)
	}
	fmt.Printf("%-20s avg %s  (over %d runs)\n", label, total/runs, runs)
}

func main() {
	solver.RegisterHint(primitives.ModHint)

	range_ := frontend.Variable("1000000000000000000000000000000000000")
	depth := 8

	// ── PrivateMint ──────────────────────────────────────────────────────────
	{
		spendPk := big.NewInt(42)
		salt := big.NewInt(7)
		amount := big.NewInt(100)
		tokenId := big.NewInt(1)
		contractAddr := big.NewInt(999)

		commitment := poseidon4(spendPk, salt, amount, tokenId)
		cipherTextRaw, _ := poseidon.Hash([]*big.Int{spendPk, salt, contractAddr})
		cipherText := cipherTextRaw.Mod(cipherTextRaw, BN254Order)

		cfg := templates.PrivateMintConfig{TmRange: range_}
		circuit := &templates.PrivateMintCircuit{Config: cfg}
		witness := &templates.PrivateMintCircuit{
			Config:          cfg,
			Commitment:      frontend.Variable(commitment),
			ContractAddress: frontend.Variable(contractAddr),
			TokenId:         frontend.Variable(tokenId),
			CipherText:      frontend.Variable(cipherText),
			Salt:            frontend.Variable(salt),
			Amount:          frontend.Variable(amount),
			PublicKey:       frontend.Variable(spendPk),
		}
		bench("PrivateMint", "scripts/keys/PrivateMintPK.key", circuit, witness)
	}

	// ── DvPInitiator ─────────────────────────────────────────────────────────
	{
		cfg := templates.DvPInitiatorCircuitConfig{TmMerkleTreeDepth: depth, TmRange: range_}

		// build a single-leaf Merkle tree so the path is trivially valid
		spendSkA := big.NewInt(12345)
		spendPkA, _ := poseidon.Hash([]*big.Int{spendSkA})
		saltIn := big.NewInt(11)
		valueIn := big.NewInt(50)
		tokenIdIn := big.NewInt(1)

		leafIndex := big.NewInt(0)
		nfA := poseidon2(spendSkA, leafIndex)

		// zero-value used in Merkle tree (keccak256("ZkDvp") % BN254Order)
		zeroValue, _ := new(big.Int).SetString(
			"21786163348386038604960770631540184405176086071424084281943739401806009032442", 10)

		// leaf = Poseidon4(spendPkA, saltIn, valueIn, tokenIdIn)
		leaf := poseidon4(spendPkA, saltIn, valueIn, tokenIdIn)

		// path: leaf is index 0, so all siblings are zero-value nodes
		pathElements := make([]*big.Int, depth)
		for i := range pathElements {
			pathElements[i] = zeroValue
		}

		// compute root by hashing leaf with sibling at each level
		root := new(big.Int).Set(leaf)
		for i := 0; i < depth; i++ {
			root = poseidon2(root, zeroValue)
		}

		spendPkBob := big.NewInt(67890)
		saltB := big.NewInt(22)
		saltA := big.NewInt(33)
		revertSalt := big.NewInt(44)
		valueBob := big.NewInt(80)
		tokenIdBob := big.NewInt(2)

		commitB := poseidon4(spendPkBob, saltB, valueIn, tokenIdIn)
		commitA := poseidon4(spendPkA, saltA, valueBob, tokenIdBob)
		revertCommitA := poseidon4(spendPkA, revertSalt, valueIn, tokenIdIn)

		wtPath := make([]frontend.Variable, depth)
		for i, p := range pathElements {
			wtPath[i] = frontend.Variable(p)
		}

		circuit := &templates.DvPInitiatorCircuit{
			Config:         cfg,
			WtPathElements: make([]frontend.Variable, depth),
		}
		witness := &templates.DvPInitiatorCircuit{
			Config:          cfg,
			StMessage:       frontend.Variable(commitA), // HIGH-5: StMessage must equal StCommitA
			StTreeNumber:    frontend.Variable(big.NewInt(0)),
			StMerkleRoot:    frontend.Variable(root),
			StNullifier:     frontend.Variable(nfA),
			StCommitB:       frontend.Variable(commitB),
			StCommitA:       frontend.Variable(commitA),
			StRevertCommitA: frontend.Variable(revertCommitA),
			WtSpendKeyIn:    frontend.Variable(spendSkA),
			WtValueIn:       frontend.Variable(valueIn),
			WtSaltIn:        frontend.Variable(saltIn),
			WtTokenIdIn:     frontend.Variable(tokenIdIn),
			WtPathElements:  wtPath,
			WtPathIndex:     frontend.Variable(leafIndex),
			WtSpendPkBob:    frontend.Variable(spendPkBob),
			WtSaltB:         frontend.Variable(saltB),
			WtValueBob:      frontend.Variable(valueBob),
			WtTokenIdBob:    frontend.Variable(tokenIdBob),
			WtSaltA:         frontend.Variable(saltA),
			WtRevertSalt:    frontend.Variable(revertSalt),
		}
		bench("DvPInitiator", "scripts/keys/DvPInitiatorPK.key", circuit, witness)
	}

	// ── DvPDestination ───────────────────────────────────────────────────────
	{
		cfg := templates.DvPDestinationCircuitConfig{TmMerkleTreeDepth: depth, TmRange: range_}

		spendSkB := big.NewInt(99999)
		spendPkB, _ := poseidon.Hash([]*big.Int{spendSkB})
		saltIn := big.NewInt(55)
		valueIn := big.NewInt(80)
		tokenIdIn := big.NewInt(2)

		leafIndex := big.NewInt(0)
		nfB := poseidon2(spendSkB, leafIndex)

		zeroValue, _ := new(big.Int).SetString(
			"21786163348386038604960770631540184405176086071424084281943739401806009032442", 10)
		leaf := poseidon4(spendPkB, saltIn, valueIn, tokenIdIn)
		root := new(big.Int).Set(leaf)
		pathElements := make([]*big.Int, depth)
		for i := range pathElements {
			pathElements[i] = zeroValue
			root = poseidon2(root, zeroValue)
		}

		spendPkA := big.NewInt(42)
		saltA := big.NewInt(33)
		saltB := big.NewInt(22)
		valueAlice := big.NewInt(50)
		tokenIdAlice := big.NewInt(1)

		commitA := poseidon4(spendPkA, saltA, valueIn, tokenIdIn)
		commitB := poseidon4(spendPkB, saltB, valueAlice, tokenIdAlice)

		wtPath := make([]frontend.Variable, depth)
		for i, p := range pathElements {
			wtPath[i] = frontend.Variable(p)
		}

		circuit := &templates.DvPDestinationCircuit{
			Config:         cfg,
			WtPathElements: make([]frontend.Variable, depth),
		}
		witness := &templates.DvPDestinationCircuit{
			Config:         cfg,
			StMessage:      frontend.Variable(commitB),
			StTreeNumber:   frontend.Variable(big.NewInt(0)),
			StMerkleRoot:   frontend.Variable(root),
			StNullifier:    frontend.Variable(nfB),
			StCommitA:      frontend.Variable(commitA),
			WtSpendKeyIn:   frontend.Variable(spendSkB),
			WtValueIn:      frontend.Variable(valueIn),
			WtSaltIn:       frontend.Variable(saltIn),
			WtTokenIdIn:    frontend.Variable(tokenIdIn),
			WtPathElements: wtPath,
			WtPathIndex:    frontend.Variable(leafIndex),
			WtSpendPkAlice: frontend.Variable(spendPkA),
			WtSaltA:        frontend.Variable(saltA),
			WtSaltB:        frontend.Variable(saltB),
			WtValueAlice:   frontend.Variable(valueAlice),
			WtTokenIdAlice: frontend.Variable(tokenIdAlice),
		}
		bench("DvPDestination", "scripts/keys/DvPDestinationPK.key", circuit, witness)
	}
}
