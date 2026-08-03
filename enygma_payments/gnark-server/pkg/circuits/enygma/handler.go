package enygma

import (
	"fmt"
	"log"
	"math/big"
	"net/http"

	utils "enygma-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/backend/groth16"
	groth16_bn254 "github.com/consensys/gnark/backend/groth16/bn254"
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
	pk, _ := utils.LoadProvingKey(curve, pkPath)

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

		witness.SenderId = frontend.Variable(request.SenderID)
		witness.SenderTxValue = frontend.Variable(request.SenderTxValue)
		witness.SecretKey = frontend.Variable(request.SecretKey)

		for i := 0; i < config.NCommitment; i++ {
			witness.SharedSecrets[i] = utils.ParseBigInt(request.SharedSecrets[i])
			for j := 0; j < config.NCommitment; j++ {
				witness.FingerPrintofSharedSecrets[i][j] = utils.ParseBigInt(request.FingerPrintofSharedSecrets[i][j])
			}
			witness.PublicKey[i] = utils.ParseBigInt(request.PublicKey[i])

			witness.PreviousCommit[i][0] = utils.ParseBigInt(request.PreviousCommit[i][0])
			witness.PreviousCommit[i][1] = utils.ParseBigInt(request.PreviousCommit[i][1])

			witness.TxCommit[i][0] = utils.ParseBigInt(request.TxCommit[i][0])
			witness.TxCommit[i][1] = utils.ParseBigInt(request.TxCommit[i][1])
			witness.TxValues[i] = utils.ParseBigInt(request.TxValues[i])
			witness.TxRandomValues[i] = utils.ParseBigInt(request.TxRandomValues[i])
			witness.AnonymitySet[i] = utils.ParseBigInt(request.AnonymitySet[i])
			witness.MessageTags[i] = utils.ParseBigInt(request.MessageTags[i])
		}

		witness.PreviousSenderBalance = utils.ParseBigInt(request.PreviousSenderBalance)
		witness.PreviousSenderRandomValue = utils.ParseBigInt(request.PreviousSenderRandomValue)
		witness.Nullifier = utils.ParseBigInt(request.Nullifier)
		witness.BlockNumber = frontend.Variable(request.BlockNumber)


		witnessFull, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid witness: %v", err)})
			return
		}

		proof, err := groth16.Prove(ccs, pk, witnessFull)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("proof generation failed: %v", err)})
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
				publicSignal = append(publicSignal, utils.ParseBigInt(request.FingerPrintofSharedSecrets[i][j]))
			}
		}

		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(request.PublicKey[i]))
		}

		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(request.PreviousCommit[i][0]))
			publicSignal = append(publicSignal, utils.ParseBigInt(request.PreviousCommit[i][1]))
		}

		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(request.TxCommit[i][0]))
			publicSignal = append(publicSignal, utils.ParseBigInt(request.TxCommit[i][1]))
		}
		publicSignal = append(publicSignal, utils.ParseBigInt(request.BlockNumber))
		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(request.AnonymitySet[i]))
		}

		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(request.MessageTags[i]))
		}
		publicSignal = append(publicSignal, utils.ParseBigInt(request.Nullifier))
		
		c.JSON(http.StatusOK, EnygmaOutput{
            Proof:  proofRemix,
            PublicSignal:publicSignal,
        })


	}
}	