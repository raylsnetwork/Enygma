package withdraw

import (
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

func createWithdrawCircuitTemplate(config WithdrawEnygmaCircuitConfig) WithdrawEnygmaCircuit {
	circuit := WithdrawEnygmaCircuit{
		Config:              config,
		HashedSharedSecrets: make([]frontend.Variable, config.NCommitment),
		PublicKey:           make([]frontend.Variable, config.NCommitment),
		PreviousCommit:      make([][2]frontend.Variable, config.NCommitment),
		TxCommit:            make([][2]frontend.Variable, config.NCommitment),
		AnonymitySet:        make([]frontend.Variable, config.NCommitment),
		SharedSecrets:       make([]frontend.Variable, config.NCommitment),
		MessageTags:         make([]frontend.Variable, config.NCommitment),
		TxValues:            make([]frontend.Variable, config.NCommitment),
		TxRandomValues:      make([]frontend.Variable, config.NCommitment),
	}
	return circuit
}

func NewHandler(pkPath, vkPath string) gin.HandlerFunc {

	curve := ecc.BN254
	pk, _ := utils.LoadProvingKey(curve, pkPath)

	return func(c *gin.Context) {
		var request WithdrawRequest

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		config := WithdrawEnygmaCircuitConfig{
			NCommitment: 6,
		}

		solver.RegisterHint(utils.ModHint)

		witness := createWithdrawCircuitTemplate(config)
		circuit := createWithdrawCircuitTemplate(config)

		var publicSignal []*big.Int

		witness.SenderId = frontend.Variable(request.SenderID)
		witness.Address = frontend.Variable(request.Address)
		witness.SenderTxValue = frontend.Variable(request.SenderTxValue)
		witness.SecretKey = frontend.Variable(request.SecretKey)

		for i := 0; i < config.NCommitment; i++ {
			witness.SharedSecrets[i] = utils.ParseBigInt(request.SharedSecrets[i])
			witness.HashedSharedSecrets[i] = utils.ParseBigInt(request.HashedSharedSecrets[i])
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

		// Set deposit-specific fields (always 10)
		for i := 0; i < 10; i++ {
			witness.Hashes[i] = frontend.Variable(request.Hashes[i])
			witness.SkDeposits[i] = frontend.Variable(request.SkDeposits[i])
			witness.VPerDeposit[i] = frontend.Variable(request.VPerDeposit[i])
		}

		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
		if err != nil {
			log.Fatal(err)
		}

		witnessFull, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
		if err != nil {
			log.Fatal(err)
		}

		proof, err := groth16.Prove(ccs, pk, witnessFull)
		if err != nil {
			log.Fatal(err)
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

		// Generate public signal - order must match circuit public signal order
		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(request.HashedSharedSecrets[i]))
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

		c.JSON(http.StatusOK, WithdrawOutput{
			Proof:        proofRemix,
			PublicSignal: publicSignal,
		})
	}
}
