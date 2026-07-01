package dvpDestination

import (
	"fmt"
	"math/big"
	"net/http"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	groth16_bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/gin-gonic/gin"

	"gnark_server/primitives"
	"gnark_server/templates"
	utils "gnark_server/utils"
)

const merkleDepth = 8

// NewHandler returns a gin.HandlerFunc that generates a Groth16 proof for the
// DvPDestinationCircuit (Merkle depth 8).
//
// Keys are loaded once at startup from pkPath / vkPath and reused for every request.
func NewHandler(pkPath, vkPath string) gin.HandlerFunc {
	curve := ecc.BN254

	pk, _ := utils.LoadProvingKey(curve, pkPath)
	vk, _ := utils.LoadVerifyingKey(curve, vkPath)

	return func(c *gin.Context) {
		var req DvPDestinationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		fmt.Println(req)

		cfg := templates.DvPDestinationCircuitConfig{
			TmMerkleTreeDepth: merkleDepth,
			TmRange:           frontend.Variable("1000000000000000000000000000000000000"),
		}

		newCircuit := func() templates.DvPDestinationCircuit {
			return templates.DvPDestinationCircuit{
				Config:         cfg,
				WtPathElements: make([]frontend.Variable, merkleDepth),
			}
		}

		circuit := newCircuit()
		witness := newCircuit()

		// --- populate witness ---
		witness.StMessage    = frontend.Variable(req.StMessage)
		witness.StTreeNumber = frontend.Variable(req.StTreeNumber)
		witness.StMerkleRoot = frontend.Variable(req.StMerkleRoot)
		witness.StNullifier  = frontend.Variable(req.StNullifier)
		witness.StCommitA    = frontend.Variable(req.StCommitA)

		witness.WtSpendKeyIn = frontend.Variable(req.WtSpendKeyIn)
		witness.WtValueIn    = frontend.Variable(req.WtValueIn)
		witness.WtSaltIn     = frontend.Variable(req.WtSaltIn)
		witness.WtTokenIdIn  = frontend.Variable(req.WtTokenIdIn)
		witness.WtPathIndex  = frontend.Variable(req.WtPathIndex)
		for j := 0; j < merkleDepth; j++ {
			witness.WtPathElements[j] = frontend.Variable(req.WtPathElements[j])
		}

		witness.WtSpendPkAlice = frontend.Variable(req.WtSpendPkAlice)
		witness.WtSaltA        = frontend.Variable(req.WtSaltA)
		witness.WtSaltB        = frontend.Variable(req.WtSaltB)
		witness.WtValueAlice   = frontend.Variable(req.WtValueAlice)
		witness.WtTokenIdAlice = frontend.Variable(req.WtTokenIdAlice)

		// --- compile, prove, verify ---
		solver.RegisterHint(primitives.ModHint)
	

		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("compile circuit: %v", err)})
			return
		}

		witnessFull, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("build witness: %v", err)})
			return
		}

		proof, err := groth16.Prove(ccs, pk, witnessFull)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("prove: %v", err)})
			return
		}

		witnessPublic, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField(), frontend.PublicOnly())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("build public witness: %v", err)})
			return
		}
		if err := groth16.Verify(proof, vk, witnessPublic); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("verify: %v", err)})
			return
		}
		fmt.Println("DvPDestination proof verified successfully!")

		// --- extract G1/G2 coordinates ---
		p := proof.(*groth16_bn254.Proof)
		ax, ay := new(big.Int), new(big.Int)
		p.Ar.X.BigInt(ax)
		p.Ar.Y.BigInt(ay)
		cx, cy := new(big.Int), new(big.Int)
		p.Krs.X.BigInt(cx)
		p.Krs.Y.BigInt(cy)
		bx0, bx1 := new(big.Int), new(big.Int)
		p.Bs.X.A0.BigInt(bx0)
		p.Bs.X.A1.BigInt(bx1)
		by0, by1 := new(big.Int), new(big.Int)
		p.Bs.Y.A0.BigInt(by0)
		p.Bs.Y.A1.BigInt(by1)

		proofRemix := []*big.Int{ax, ay, bx1, bx0, by1, by0, cx, cy}

		// --- public signal: [msg, treeNum, root, nf_B, commitA] ---
		publicSignal := []*big.Int{
			utils.ParseBigInt(req.StMessage),
			utils.ParseBigInt(req.StTreeNumber),
			utils.ParseBigInt(req.StMerkleRoot),
			utils.ParseBigInt(req.StNullifier),
			utils.ParseBigInt(req.StCommitA),
		}

		c.JSON(http.StatusOK, DvPDestinationOutput{
			Proof:        proofRemix,
			PublicSignal: publicSignal,
		})
	}
}
