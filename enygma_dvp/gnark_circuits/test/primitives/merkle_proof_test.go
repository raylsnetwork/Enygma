package test

import (
    "math/big"
    "testing"
	"gnark_server/primitives"
    "github.com/consensys/gnark/frontend"
    "github.com/consensys/gnark/frontend/cs/r1cs"
    "github.com/consensys/gnark/backend/groth16"
    "github.com/consensys/gnark-crypto/ecc"
    fr "github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/iden3/go-iden3-crypto/poseidon"
	
   
)

type merkleCircuit struct {
    Leaf         frontend.Variable
    PathIndices  frontend.Variable
    PathElements [2]frontend.Variable
    // the Merkle root we expect
    ExpectedRoot frontend.Variable `gnark:",public"`
}

func (c *merkleCircuit) Define(api frontend.API) error {
    // call the function under test
    root := primitives.MerkleProof(api, c.Leaf, c.PathIndices, c.PathElements[:])
    api.AssertIsEqual(root, c.ExpectedRoot)
    return nil
}

func computeExpectedRoot(leaf uint64, pathIndices uint64, siblings []uint64) *big.Int {
    // start from the leaf
    var current fr.Element
    current.SetUint64(leaf)

    // ascend the tree
    for i, sibVal := range siblings {
        // unpack the bit i of pathIndices
        bit := (pathIndices >> uint(i)) & 1

        // choose left / right
        var left, right fr.Element
        var sib fr.Element
        sib.SetUint64(sibVal)
        if bit == 0 {
            left = current
            right = sib
        } else {
            left = sib
            right = current
        }
		var LeftBi big.Int
    	left.BigInt(&LeftBi)

		var RighBi big.Int
    	right.BigInt(&RighBi)

        // hash [left, right]
		rootBig, _ := poseidon.Hash([]*big.Int{&LeftBi,&RighBi})

		var root fr.Element
    	root.SetBigInt(rootBig)
        current = root
	}	
    // return as *big.Int
    var bi big.Int
    current.BigInt(&bi)
    return &bi	
	
}

func TestMerkleProof(t *testing.T) {
    // ── test data: leaf, two siblings, and index bits ──
    const leafVal = uint64(123)
    siblings := []uint64{456, 789}
    // pathIndices = 0b01 (little‐endian):
    //   level 0 bit = 1 → swap at first level
    //   level 1 bit = 0 → keep at second level
    const pathIdx = uint64(1) 

    // compute offline “correct” root
    expectedRoot := computeExpectedRoot(leafVal, pathIdx, siblings)

    // ── compile the circuit ──
    var circuit merkleCircuit
    cc, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
    if err != nil {
        t.Fatalf("compile error: %v", err)
    }

    // ── setup proving & verifying keys ──
    pk, vk, err := groth16.Setup(cc)
    if err != nil {
        t.Fatalf("setup error: %v", err)
    }

    // ── create witness with both private inputs and the public root ──
    witness := merkleCircuit{
        Leaf:         leafVal,
        PathIndices:  pathIdx,
        PathElements: [2]frontend.Variable{siblings[0], siblings[1]},
        ExpectedRoot: expectedRoot,
    }
    fullWit, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
    if err != nil {
        t.Fatalf("new witness error: %v", err)
    }
    publicWit, err := fullWit.Public()
    if err != nil {
        t.Fatalf("public witness error: %v", err)
    }

    // ── generate proof ──
    proof, err := groth16.Prove(cc, pk, fullWit)
    if err != nil {
        t.Fatalf("prove error: %v", err)
    }

    // ── verify proof ──
    if err := groth16.Verify(proof, vk, publicWit); err != nil {
        t.Fatalf("verify error: %v", err)
    }
}
