package burn

import (
	"fmt"
	"log"
	"math/big"
	"net/http"

	utils "enygma-server/utils"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	groth16_bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/gin-gonic/gin"
)

func NewHandler(pkPath, vkPath string) gin.HandlerFunc {
	curve := ecc.BN254
	pk, vk := utils.MustLoadKeys(curve, pkPath, vkPath) // Fix L-04 part 1

	circuitTemplate := BurnCircuit{}
	solver.RegisterHint(utils.ModHint)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitTemplate)
	if err != nil {
		log.Fatalf("failed to compile burn circuit: %v", err)
	}

	return func(c *gin.Context) {
		var request BurnRequest

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Fix M-08: bp.Parse accumulates the first parse failure instead
		// of silently turning a malformed decimal into nil. bp.Err() is
		// checked below, before frontend.NewWitness is ever called — see
		// utils.ParseBigInt's doc for why that ordering closes the
		// witness.Fill goroutine leak.
		bp := &utils.BigIntParser{}

		witness := BurnCircuit{
			PublicKey:           bp.Parse(request.PublicKey),
			Amount:              bp.Parse(request.Amount),
			BlockNumber:         bp.Parse(request.BlockNumber),
			Nullifier:           bp.Parse(request.Nullifier),
			SecretKey:           bp.Parse(request.SecretKey),
			PreviousBalance:     bp.Parse(request.PreviousBalance),
			PreviousRandomValue: bp.Parse(request.PreviousRandomValue),
			DomainId:            bp.Parse(request.DomainId), // Fix L-01
		}
		witness.PreviousCommit[0] = bp.Parse(request.PreviousCommit[0])
		witness.PreviousCommit[1] = bp.Parse(request.PreviousCommit[1])
		witness.NewCommit[0] = bp.Parse(request.NewCommit[0])
		witness.NewCommit[1] = bp.Parse(request.NewCommit[1])

		if err := bp.Err(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request: %v", err)})
			return
		}

		witnessFull, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid witness: %v", err)})
			return
		}

		// Fix M-08: bound how many groth16.Prove calls run concurrently —
		// see utils.ProveLimiter's doc.
		if !utils.ProveLimiter.Acquire() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "proving server is busy, try again shortly"})
			return
		}
		proof, err := groth16.Prove(ccs, pk, witnessFull)
		utils.ProveLimiter.Release()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("proof generation failed: %v", err)})
			return
		}

		// Fix L-04 part 2: self-verify before returning.
		if err := utils.SelfVerify(proof, vk, witnessFull); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("server-side %v", err)})
			return
		}

		p := proof.(*groth16_bn254.Proof)
		A_x1 := new(big.Int)
		p.Ar.X.BigInt(A_x1)

		A_y1 := new(big.Int)
		p.Ar.Y.BigInt(A_y1)

		C_x1 := new(big.Int)
		p.Krs.X.BigInt(C_x1)

		C_y1 := new(big.Int)
		p.Krs.Y.BigInt(C_y1)

		BX01 := new(big.Int)
		p.Bs.X.A0.BigInt(BX01)

		BX11 := new(big.Int)
		p.Bs.X.A1.BigInt(BX11)

		BY01 := new(big.Int)
		p.Bs.Y.A0.BigInt(BY01)

		BY11 := new(big.Int)
		p.Bs.Y.A1.BigInt(BY11)

		proofRemix := []*big.Int{
			A_x1, A_y1,
			BX11, BX01,
			BY11, BY01,
			C_x1, C_y1,
		}

		// Public signal order must match circuit field declaration order:
		// PublicKey, PreviousCommit×2, NewCommit×2, Amount, BlockNumber, Nullifier.
		publicSignal := []*big.Int{
			bp.Parse(request.PublicKey),
			bp.Parse(request.PreviousCommit[0]),
			bp.Parse(request.PreviousCommit[1]),
			bp.Parse(request.NewCommit[0]),
			bp.Parse(request.NewCommit[1]),
			bp.Parse(request.Amount),
			bp.Parse(request.BlockNumber),
			bp.Parse(request.Nullifier),
			bp.Parse(request.DomainId), // Fix L-01
		}

		c.JSON(http.StatusOK, BurnOutput{
			Proof:        proofRemix,
			PublicSignal: publicSignal,
		})
	}
}
