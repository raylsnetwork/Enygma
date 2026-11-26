package auctionFinal

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

	"enygma_dvp_auctions/gnark_circuits/primitives"
	"enygma_dvp_auctions/gnark_circuits/templates"
	"enygma_dvp_auctions/gnark_circuits/utils"

	"enygma_gnark_shared/witnessinput"
)

// NewHandler returns a gin.HandlerFunc that generates a Groth16 proof for the
// AuctionFinalCircuit (Phase 2). Keys are loaded once at startup.
func NewHandler(pkPath, vkPath string) gin.HandlerFunc {
	curve := ecc.BN254
	pk, _ := utils.LoadProvingKey(curve, pkPath)
	vk, _ := utils.LoadVerifyingKey(curve, vkPath)

	return func(c *gin.Context) {
		var req AuctionFinalRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		k := templates.AuctionFinalBatches

		newCircuit := func() templates.AuctionFinalCircuit {
			return templates.AuctionFinalCircuit{
				StBatchWinnerCommit: make([]frontend.Variable, k),
				StBatchWinnerPk:     make([]frontend.Variable, k),
				StBatchWinnerAmount: make([]frontend.Variable, k),
				WtBatchActive:       make([]frontend.Variable, k),
			}
		}

		circuit := newCircuit()
		witness := newCircuit()

		// public inputs — Phase-1 batch outputs
		var numbers witnessinput.Parser
		witness.StAuctionId = numbers.Parse(req.StAuctionId)
		for i := 0; i < k; i++ {
			witness.StBatchWinnerCommit[i] = numbers.Parse(req.StBatchWinnerCommit[i])
			witness.StBatchWinnerPk[i] = numbers.Parse(req.StBatchWinnerPk[i])
			witness.StBatchWinnerAmount[i] = numbers.Parse(req.StBatchWinnerAmount[i])
		}

		// public outputs — settlement
		witness.StOverallWinnerCommit = numbers.Parse(req.StOverallWinnerCommit)
		witness.StWinnerPk = numbers.Parse(req.StWinnerPk)
		witness.StWinnerCommitB = numbers.Parse(req.StWinnerCommitB)
		witness.StWinnerNftCommit = numbers.Parse(req.StWinnerNftCommit)
		witness.StNftTokenId = numbers.Parse(req.StNftTokenId)
		witness.StWinningAmount = numbers.Parse(req.StWinningAmount)
		witness.StFloorPrice = numbers.Parse(req.StFloorPrice)

		// private witnesses
		witness.WtAuctionId = numbers.Parse(req.WtAuctionId)
		witness.WtWinnerBatchIdx = numbers.Parse(req.WtWinnerBatchIdx)
		witness.WtPkBob = numbers.Parse(req.WtPkBob)
		witness.WtSaltB = numbers.Parse(req.WtSaltB)
		witness.WtUsdcTokenId = numbers.Parse(req.WtUsdcTokenId)
		witness.WtSaltAlice = numbers.Parse(req.WtSaltAlice)
		witness.WtNftTokenId = numbers.Parse(req.WtNftTokenId)
		for i := 0; i < k; i++ {
			witness.WtBatchActive[i] = numbers.Parse(req.WtBatchActive[i])
		}

		solver.RegisterHint(primitives.PoseidonNative)
		solver.RegisterHint(primitives.PoseidonPrivateKeyNative)

		if err := numbers.Err(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

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
		fmt.Println("AuctionFinal proof verified successfully!")

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

		// public signal order matches gnark's witness serialization:
		// [StAuctionId,
		//  StBatchWinnerCommit[0..9], StBatchWinnerPk[0..9], StBatchWinnerAmount[0..9],
		//  StOverallWinnerCommit, StWinnerPk, StWinnerCommitB,
		//  StWinnerNftCommit, StNftTokenId, StWinningAmount, StFloorPrice]
		publicSignal := make([]*big.Int, 0, 3*k+8)
		publicSignal = append(publicSignal, utils.ParseBigInt(req.StAuctionId))
		for i := 0; i < k; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(req.StBatchWinnerCommit[i]))
		}
		for i := 0; i < k; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(req.StBatchWinnerPk[i]))
		}
		for i := 0; i < k; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(req.StBatchWinnerAmount[i]))
		}
		publicSignal = append(publicSignal, utils.ParseBigInt(req.StOverallWinnerCommit))
		publicSignal = append(publicSignal, utils.ParseBigInt(req.StWinnerPk))
		publicSignal = append(publicSignal, utils.ParseBigInt(req.StWinnerCommitB))
		publicSignal = append(publicSignal, utils.ParseBigInt(req.StWinnerNftCommit))
		publicSignal = append(publicSignal, utils.ParseBigInt(req.StNftTokenId))
		publicSignal = append(publicSignal, utils.ParseBigInt(req.StWinningAmount))
		publicSignal = append(publicSignal, utils.ParseBigInt(req.StFloorPrice))

		c.JSON(http.StatusOK, AuctionFinalOutput{Proof: proofRemix, PublicSignal: publicSignal})
	}
}
