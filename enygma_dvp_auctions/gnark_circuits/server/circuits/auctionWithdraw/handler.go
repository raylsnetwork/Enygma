package auctionWithdraw

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
// AuctionWithdrawCircuit. Keys are loaded once at startup.
func NewHandler(pkPath, vkPath string) gin.HandlerFunc {
	curve := ecc.BN254
	pk, _ := utils.LoadProvingKey(curve, pkPath)
	vk, _ := utils.LoadVerifyingKey(curve, vkPath)

	return func(c *gin.Context) {
		var req AuctionWithdrawRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		newCircuit := func() templates.AuctionWithdrawCircuit {
			return templates.AuctionWithdrawCircuit{}
		}

		circuit := newCircuit()
		witness := newCircuit()

		// public inputs
		var numbers witnessinput.Parser
		witness.StAuctionId = numbers.Parse(req.StAuctionId)
		witness.StCommitA = numbers.Parse(req.StCommitA)

		// private witnesses
		witness.WtSpendKey = numbers.Parse(req.WtSpendKey)
		witness.WtSaltA = numbers.Parse(req.WtSaltA)
		witness.WtAmount = numbers.Parse(req.WtAmount)
		witness.WtTokenId = numbers.Parse(req.WtTokenId)
		witness.WtAuctionId = numbers.Parse(req.WtAuctionId)

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
		fmt.Println("AuctionWithdraw proof verified successfully!")

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

		// public signal: [StAuctionId, StCommitA]
		publicSignal := []*big.Int{
			utils.ParseBigInt(req.StAuctionId),
			utils.ParseBigInt(req.StCommitA),
		}

		c.JSON(http.StatusOK, AuctionWithdrawOutput{Proof: proofRemix, PublicSignal: publicSignal})
	}
}
