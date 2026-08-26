package enygma

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

func createCircuitTemplate(config EnygmaCircuitConfig) EnygmaCircuit {
	fp := make([][]frontend.Variable, config.NCommitment)
	for i := range fp {
		fp[i] = make([]frontend.Variable, config.NCommitment)
	}
	circuit := EnygmaCircuit{
		Config:                     config,
		FingerPrintofSharedSecrets: fp,
		PublicKey:                  make([]frontend.Variable, config.NCommitment),
		PreviousCommit:             make([][2]frontend.Variable, config.NCommitment),
		TxCommit:                   make([][2]frontend.Variable, config.NCommitment),
		AnonymitySet:               make([]frontend.Variable, config.NCommitment),
		SharedSecrets:              make([]frontend.Variable, config.NCommitment),
		MessageTags:                make([]frontend.Variable, config.NCommitment),
		TxValues:                   make([]frontend.Variable, config.NCommitment),
		TxRandomValues:             make([]frontend.Variable, config.NCommitment),
	}
	return circuit
}

func NewHandler(pkPath, vkPath string) gin.HandlerFunc {

	curve := ecc.BN254
	pk, vk := utils.MustLoadKeys(curve, pkPath, vkPath) // Fix L-04 part 1

	config := EnygmaCircuitConfig{NCommitment: 6}
	circuitTemplate := createCircuitTemplate(config)
	solver.RegisterHint(utils.ModHint)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitTemplate)
	if err != nil {
		log.Fatalf("failed to compile enygma circuit: %v", err)
	}

	return func(c *gin.Context) {
		var request EnygmaRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		witness := createCircuitTemplate(config)

		var publicSignal []*big.Int

		// Fix M-08: bp.Parse accumulates the first parse failure instead
		// of silently turning a malformed decimal into nil — the exact
		// value that used to reach frontend.NewWitness and trigger
		// gnark's witness.Fill goroutine leak. bp.Err() is checked below,
		// before frontend.NewWitness is ever called.
		bp := &utils.BigIntParser{}

		witness.SenderId = frontend.Variable(request.SenderID)
		witness.SenderTxValue = frontend.Variable(request.SenderTxValue)
		witness.SecretKey = frontend.Variable(request.SecretKey)

		for i := 0; i < config.NCommitment; i++ {
			witness.SharedSecrets[i] = bp.Parse(request.SharedSecrets[i])
			for j := 0; j < config.NCommitment; j++ {
				witness.FingerPrintofSharedSecrets[i][j] = bp.Parse(request.FingerPrintofSharedSecrets[i][j])
			}
			witness.PublicKey[i] = bp.Parse(request.PublicKey[i])

			witness.PreviousCommit[i][0] = bp.Parse(request.PreviousCommit[i][0])
			witness.PreviousCommit[i][1] = bp.Parse(request.PreviousCommit[i][1])

			witness.TxCommit[i][0] = bp.Parse(request.TxCommit[i][0])
			witness.TxCommit[i][1] = bp.Parse(request.TxCommit[i][1])
			witness.TxValues[i] = bp.Parse(request.TxValues[i])
			witness.TxRandomValues[i] = bp.Parse(request.TxRandomValues[i])
			witness.AnonymitySet[i] = bp.Parse(request.AnonymitySet[i])
			witness.MessageTags[i] = bp.Parse(request.MessageTags[i])
		}

		witness.PreviousSenderBalance = bp.Parse(request.PreviousSenderBalance)
		witness.PreviousSenderRandomValue = bp.Parse(request.PreviousSenderRandomValue)
		witness.Nullifier = bp.Parse(request.Nullifier)
		witness.BlockNumber = frontend.Variable(request.BlockNumber)
		witness.DomainId = bp.Parse(request.DomainId) // Fix L-01

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
		// see utils.ProveLimiter's doc. 503 rather than an unbounded queue
		// if every slot is busy past ProveAcquireTimeout.
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

		// For G2 point B (handling Fp² coordinates)
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

		// Generate public signal - order must match circuit public signal order
		// FingerPrintofSharedSecrets is k×k, flattened row-major (36 values for k=6)
		for i := 0; i < config.NCommitment; i++ {
			for j := 0; j < config.NCommitment; j++ {
				publicSignal = append(publicSignal, bp.Parse(request.FingerPrintofSharedSecrets[i][j]))
			}
		}

		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, bp.Parse(request.PublicKey[i]))
		}

		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, bp.Parse(request.PreviousCommit[i][0]))
			publicSignal = append(publicSignal, bp.Parse(request.PreviousCommit[i][1]))
		}

		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, bp.Parse(request.TxCommit[i][0]))
			publicSignal = append(publicSignal, bp.Parse(request.TxCommit[i][1]))
		}
		publicSignal = append(publicSignal, bp.Parse(request.BlockNumber))
		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, bp.Parse(request.AnonymitySet[i]))
		}

		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, bp.Parse(request.MessageTags[i]))
		}
		publicSignal = append(publicSignal, bp.Parse(request.Nullifier))
		publicSignal = append(publicSignal, bp.Parse(request.DomainId)) // Fix L-01

		c.JSON(http.StatusOK, EnygmaOutput{
			Proof:        proofRemix,
			PublicSignal: publicSignal,
		})

	}
}
