package auctionLock

import (
	"fmt"
	"math/big"
	"net/http"

	groth16_bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/gin-gonic/gin"

	"gnark_server/primitives"
	"gnark_server/templates"
	"gnark_server/utils"
)

const merkleDepth = 8

// NewHandler returns a gin.HandlerFunc that generates a Groth16 proof for the
// AuctionLockCircuit (Merkle depth 8). Keys are loaded once at startup.
func NewHandler(pkPath, vkPath string) gin.HandlerFunc {
	curve := ecc.BN254
	pk, _ := utils.LoadProvingKey(curve, pkPath)
	vk, _ := utils.LoadVerifyingKey(curve, vkPath)

	return func(c *gin.Context) {
		var req AuctionLockRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		fmt.Println(req)

		cfg := templates.AuctionLockCircuitConfig{TmMerkleTreeDepth: merkleDepth}

		newCircuit := func() templates.AuctionLockCircuit {
			return templates.AuctionLockCircuit{
				Config:         cfg,
				WtPathElements: make([]frontend.Variable, merkleDepth),
			}
		}

		circuit := newCircuit()
		witness := newCircuit()

		// populate public inputs
		witness.StAuctionId    = frontend.Variable(req.StAuctionId)
		witness.StTreeNumber   = frontend.Variable(req.StTreeNumber)
		witness.StMerkleRoot   = frontend.Variable(req.StMerkleRoot)
		witness.StNullifier    = frontend.Variable(req.StNullifier)
		witness.StCommitLocked = frontend.Variable(req.StCommitLocked)
		witness.StNftTokenId   = frontend.Variable(req.StNftTokenId)

		// populate private witnesses
		witness.WtSpendKey   = frontend.Variable(req.WtSpendKey)
		witness.WtTokenId    = frontend.Variable(req.WtTokenId)
		witness.WtSaltIn     = frontend.Variable(req.WtSaltIn)
		witness.WtPathIndex  = frontend.Variable(req.WtPathIndex)
		witness.WtSaltLocked = frontend.Variable(req.WtSaltLocked)
		for j := 0; j < merkleDepth; j++ {
			witness.WtPathElements[j] = frontend.Variable(req.WtPathElements[j])
		}

		solver.RegisterHint(primitives.PoseidonNative)
		solver.RegisterHint(primitives.PoseidonPrivateKeyNative)

		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("compile: %v", err)})
			return
		}

		witnessFull, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("witness: %v", err)})
			return
		}

		proof, err := groth16.Prove(ccs, pk, witnessFull)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("prove: %v", err)})
			return
		}

		witnessPublic, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField(), frontend.PublicOnly())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("public witness: %v", err)})
			return
		}
		if err := groth16.Verify(proof, vk, witnessPublic); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("verify: %v", err)})
			return
		}
		fmt.Println("AuctionLock proof verified successfully!")

		p := proof.(*groth16_bn254.Proof)
		ax, ay := new(big.Int), new(big.Int)
		p.Ar.X.BigInt(ax); p.Ar.Y.BigInt(ay)
		cx, cy := new(big.Int), new(big.Int)
		p.Krs.X.BigInt(cx); p.Krs.Y.BigInt(cy)
		bx0, bx1 := new(big.Int), new(big.Int)
		p.Bs.X.A0.BigInt(bx0); p.Bs.X.A1.BigInt(bx1)
		by0, by1 := new(big.Int), new(big.Int)
		p.Bs.Y.A0.BigInt(by0); p.Bs.Y.A1.BigInt(by1)

		proofRemix := []*big.Int{ax, ay, bx1, bx0, by1, by0, cx, cy}

		// public signal: [stAuctionId, stTreeNumber, stMerkleRoot, stNullifier, stCommitLocked, stNftTokenId]
		publicSignal := []*big.Int{
			utils.ParseBigInt(req.StAuctionId),
			utils.ParseBigInt(req.StTreeNumber),
			utils.ParseBigInt(req.StMerkleRoot),
			utils.ParseBigInt(req.StNullifier),
			utils.ParseBigInt(req.StCommitLocked),
			utils.ParseBigInt(req.StNftTokenId),
		}

		c.JSON(http.StatusOK, AuctionLockOutput{Proof: proofRemix, PublicSignal: publicSignal})
	}
}
