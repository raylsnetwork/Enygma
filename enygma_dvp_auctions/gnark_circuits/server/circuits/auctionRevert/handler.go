package auctionRevert

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
// AuctionRevertCircuit. Keys are loaded once at startup.
func NewHandler(pkPath, vkPath string) gin.HandlerFunc {
	curve := ecc.BN254
	pk, _ := utils.LoadProvingKey(curve, pkPath)
	vk, _ := utils.LoadVerifyingKey(curve, vkPath)

	return func(c *gin.Context) {
		var req AuctionRevertRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		fmt.Println(req)

		newCircuit := func() templates.AuctionRevertCircuit {
			return templates.AuctionRevertCircuit{}
		}

		circuit := newCircuit()
		witness := newCircuit()

		// public inputs
		witness.StAuctionId = frontend.Variable(req.StAuctionId)
		witness.StCommitLocked = frontend.Variable(req.StCommitLocked)
		witness.StNftTokenId = frontend.Variable(req.StNftTokenId)
		witness.StRevertedCommit = frontend.Variable(req.StRevertedCommit)

		// private witnesses
		witness.WtSpendKey = frontend.Variable(req.WtSpendKey)
		witness.WtTokenId = frontend.Variable(req.WtTokenId)
		witness.WtSaltLocked = frontend.Variable(req.WtSaltLocked)
		witness.WtSaltOut = frontend.Variable(req.WtSaltOut)

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
		fmt.Println("AuctionRevert proof verified successfully!")

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

		// public signal: [StAuctionId, StCommitLocked, StNftTokenId, StRevertedCommit]
		publicSignal := []*big.Int{
			utils.ParseBigInt(req.StAuctionId),
			utils.ParseBigInt(req.StCommitLocked),
			utils.ParseBigInt(req.StNftTokenId),
			utils.ParseBigInt(req.StRevertedCommit),
		}

		c.JSON(http.StatusOK, AuctionRevertOutput{Proof: proofRemix, PublicSignal: publicSignal})
	}
}
