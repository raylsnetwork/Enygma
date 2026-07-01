package auctionBatch

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

// NewHandler returns a gin.HandlerFunc that generates a Groth16 proof for the
// AuctionBatchCircuit (Phase 1). Keys are loaded once at startup.
func NewHandler(pkPath, vkPath string) gin.HandlerFunc {
	curve := ecc.BN254
	pk, _ := utils.LoadProvingKey(curve, pkPath)
	vk, _ := utils.LoadVerifyingKey(curve, vkPath)

	return func(c *gin.Context) {
		var req AuctionBatchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		fmt.Println(req)

		n := templates.AuctionBatchSize

		newCircuit := func() templates.AuctionBatchCircuit {
			return templates.AuctionBatchCircuit{
				StBidCommitA: make([]frontend.Variable, n),
				WtActive:     make([]frontend.Variable, n),
				WtPkBidder:   make([]frontend.Variable, n),
				WtSaltA:      make([]frontend.Variable, n),
				WtAmount:     make([]frontend.Variable, n),
				WtTokenId:    make([]frontend.Variable, n),
			}
		}

		circuit := newCircuit()
		witness := newCircuit()

		// public inputs
		witness.StAuctionId         = frontend.Variable(req.StAuctionId)
		witness.StBatchWinnerCommit = frontend.Variable(req.StBatchWinnerCommit)
		witness.StBatchWinnerPk     = frontend.Variable(req.StBatchWinnerPk)
		witness.StBatchWinnerAmount = frontend.Variable(req.StBatchWinnerAmount)
		for i := 0; i < n; i++ {
			witness.StBidCommitA[i] = frontend.Variable(req.StBidCommitA[i])
		}

		// private witnesses
		witness.WtAuctionId = frontend.Variable(req.WtAuctionId)
		witness.WtWinnerIdx = frontend.Variable(req.WtWinnerIdx)
		for i := 0; i < n; i++ {
			witness.WtActive[i]   = frontend.Variable(req.WtActive[i])
			witness.WtPkBidder[i] = frontend.Variable(req.WtPkBidder[i])
			witness.WtSaltA[i]    = frontend.Variable(req.WtSaltA[i])
			witness.WtAmount[i]   = frontend.Variable(req.WtAmount[i])
			witness.WtTokenId[i]  = frontend.Variable(req.WtTokenId[i])
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
		fmt.Println("AuctionBatch proof verified successfully!")

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

		// public signal order matches gnark's witness serialization:
		// [StAuctionId, StBidCommitA[0..99], StBatchWinnerCommit, StBatchWinnerPk, StBatchWinnerAmount]
		publicSignal := make([]*big.Int, 0, n+4)
		publicSignal = append(publicSignal, utils.ParseBigInt(req.StAuctionId))
		for i := 0; i < n; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(req.StBidCommitA[i]))
		}
		publicSignal = append(publicSignal, utils.ParseBigInt(req.StBatchWinnerCommit))
		publicSignal = append(publicSignal, utils.ParseBigInt(req.StBatchWinnerPk))
		publicSignal = append(publicSignal, utils.ParseBigInt(req.StBatchWinnerAmount))

		c.JSON(http.StatusOK, AuctionBatchOutput{Proof: proofRemix, PublicSignal: publicSignal})
	}
}
