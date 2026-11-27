package dvpInit

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

	"enygma_dvp/gnark_circuits/primitives"
	"enygma_dvp/gnark_circuits/templates"
	utils "enygma_dvp/gnark_circuits/utils"

	"enygma_gnark_shared/witnessinput"
)

const merkleDepth = 8

// NewHandler returns a gin.HandlerFunc that generates a Groth16 proof for the
// DvPInitiatorCircuit (Merkle depth 8).
//
// Keys are loaded once at startup from pkPath / vkPath and reused for every request.
func NewHandler(pkPath, vkPath string) gin.HandlerFunc {
	curve := ecc.BN254

	pk, _ := utils.LoadProvingKey(curve, pkPath)
	vk, _ := utils.LoadVerifyingKey(curve, vkPath)

	return func(c *gin.Context) {
		var req DvPInitiatorRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		cfg := templates.DvPInitiatorCircuitConfig{
			TmMerkleTreeDepth: merkleDepth,
		}

		newCircuit := func() templates.DvPInitiatorCircuit {
			return templates.DvPInitiatorCircuit{
				Config:         cfg,
				WtPathElements: make([]frontend.Variable, merkleDepth),
			}
		}

		circuit := newCircuit()
		witness := newCircuit()

		// --- populate witness ---
		var numbers witnessinput.Parser
		witness.StMessage = numbers.Parse(req.StMessage)
		witness.StTreeNumber = numbers.Parse(req.StTreeNumber)
		witness.StMerkleRoot = numbers.Parse(req.StMerkleRoot)
		witness.StNullifier = numbers.Parse(req.StNullifier)
		witness.StCommitB = numbers.Parse(req.StCommitB)
		witness.StCommitA = numbers.Parse(req.StCommitA)
		witness.StRevertCommitA = numbers.Parse(req.StRevertCommitA)
		witness.StCounterVault = numbers.Parse(req.StCounterVault)
		witness.StDeadline = numbers.Parse(req.StDeadline)
		witness.StGroupID = numbers.Parse(req.StGroupID)

		witness.WtSpendKeyIn = numbers.Parse(req.WtSpendKeyIn)
		witness.WtValueIn = numbers.Parse(req.WtValueIn)
		witness.WtSaltIn = numbers.Parse(req.WtSaltIn)
		witness.WtTokenIdIn = numbers.Parse(req.WtTokenIdIn)
		witness.WtPathIndex = numbers.Parse(req.WtPathIndex)
		for j := 0; j < merkleDepth; j++ {
			witness.WtPathElements[j] = numbers.Parse(req.WtPathElements[j])
		}

		witness.WtSpendPkBob = numbers.Parse(req.WtSpendPkBob)
		witness.WtSaltB = numbers.Parse(req.WtSaltB)
		witness.WtValueBob = numbers.Parse(req.WtValueBob)
		witness.WtTokenIdBob = numbers.Parse(req.WtTokenIdBob)
		witness.WtSaltA = numbers.Parse(req.WtSaltA)
		witness.WtRevertSalt = numbers.Parse(req.WtRevertSalt)

		// --- compile, prove, verify ---
		solver.RegisterHint(primitives.ModHint)

		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("compile circuit: %v", err)})
			return
		}

		if err := numbers.Err(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		fmt.Println("DvPInitiator proof verified successfully!")

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

		publicSignal, err := utils.PublicSignals(witnessFull)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, DvPInitiatorOutput{
			Proof:        proofRemix,
			PublicSignal: publicSignal,
		})
	}
}
