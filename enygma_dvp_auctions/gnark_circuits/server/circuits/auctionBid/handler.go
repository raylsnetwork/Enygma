package auctionBid

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

const (
	bidMerkleDepth = 8
	// bidRange is the upper bound for amount range checks (same as dvp convention).
	bidRange = "1000000000000000000000000000000000000"
)

// NewHandler returns a gin.HandlerFunc that generates a Groth16 proof for the
// AuctionBidCircuit (Merkle depth 8, all-in bid). Keys are loaded once at startup.
func NewHandler(pkPath, vkPath string) gin.HandlerFunc {
	curve := ecc.BN254
	pk, _ := utils.LoadProvingKey(curve, pkPath)
	vk, _ := utils.LoadVerifyingKey(curve, vkPath)

	return func(c *gin.Context) {
		var req AuctionBidRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		fmt.Println(req)

		cfg := templates.AuctionBidCircuitConfig{
			TmMerkleTreeDepth: bidMerkleDepth,
			TmRange:           frontend.Variable(bidRange),
		}

		newCircuit := func() templates.AuctionBidCircuit {
			return templates.AuctionBidCircuit{
				Config:         cfg,
				WtPathElements: make([]frontend.Variable, bidMerkleDepth),
			}
		}

		circuit := newCircuit()
		witness := newCircuit()

		// populate public inputs
		witness.StAuctionId    = frontend.Variable(req.StAuctionId)
		witness.StTreeNumber   = frontend.Variable(req.StTreeNumber)
		witness.StMerkleRoot   = frontend.Variable(req.StMerkleRoot)
		witness.StNullifier    = frontend.Variable(req.StNullifier)
		witness.StCommitA      = frontend.Variable(req.StCommitA)
		witness.StCommitB      = frontend.Variable(req.StCommitB)
		witness.StRevertCommit = frontend.Variable(req.StRevertCommit)

		// populate private witnesses
		witness.WtAuctionId  = frontend.Variable(req.WtAuctionId)
		witness.WtTreeNumber = frontend.Variable(req.WtTreeNumber)
		witness.WtSpendKey   = frontend.Variable(req.WtSpendKey)
		witness.WtAmount     = frontend.Variable(req.WtAmount)
		witness.WtTokenId    = frontend.Variable(req.WtTokenId)
		witness.WtSaltIn     = frontend.Variable(req.WtSaltIn)
		witness.WtPathIndex  = frontend.Variable(req.WtPathIndex)
		witness.WtSpendPkBob = frontend.Variable(req.WtSpendPkBob)
		witness.WtSaltA      = frontend.Variable(req.WtSaltA)
		witness.WtSaltB      = frontend.Variable(req.WtSaltB)
		witness.WtBidAmount  = frontend.Variable(req.WtBidAmount)
		witness.WtSaltRevert = frontend.Variable(req.WtSaltRevert)
		for j := 0; j < bidMerkleDepth; j++ {
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
		fmt.Println("AuctionBid proof verified successfully!")

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

		// public signal: [stAuctionId, stTreeNumber, stMerkleRoot, stNullifier, stCommitA, stCommitB, stRevertCommit]
		publicSignal := []*big.Int{
			utils.ParseBigInt(req.StAuctionId),
			utils.ParseBigInt(req.StTreeNumber),
			utils.ParseBigInt(req.StMerkleRoot),
			utils.ParseBigInt(req.StNullifier),
			utils.ParseBigInt(req.StCommitA),
			utils.ParseBigInt(req.StCommitB),
			utils.ParseBigInt(req.StRevertCommit),
		}

		c.JSON(http.StatusOK, AuctionBidOutput{Proof: proofRemix, PublicSignal: publicSignal})
	}
}
