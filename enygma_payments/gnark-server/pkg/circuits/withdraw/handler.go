package withdraw

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
	pk, vk := utils.MustLoadKeys(curve, pkPath, vkPath) // Fix L-04 part 1

	config := WithdrawEnygmaCircuitConfig{NCommitment: 6}
	circuitTemplate := createWithdrawCircuitTemplate(config)
	solver.RegisterHint(utils.ModHint)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitTemplate)
	if err != nil {
		log.Fatalf("failed to compile withdraw circuit: %v", err)
	}

	return func(c *gin.Context) {
		var request WithdrawRequest

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		witness := createWithdrawCircuitTemplate(config)

		var publicSignal []*big.Int

		// Fix M-08: bp.Parse accumulates the first parse failure instead
		// of silently turning a malformed decimal into nil. bp.Err() is
		// checked below, before frontend.NewWitness is ever called — see
		// utils.ParseBigInt's doc for why that ordering closes the
		// witness.Fill goroutine leak.
		bp := &utils.BigIntParser{}

		witness.SenderId = frontend.Variable(request.SenderID)
		witness.Address = frontend.Variable(request.Address)

		witness.SenderTxValue = frontend.Variable(request.SenderTxValue)
		witness.SecretKey = frontend.Variable(request.SecretKey)

		for i := 0; i < config.NCommitment; i++ {
			witness.SharedSecrets[i] = bp.Parse(request.SharedSecrets[i])
			witness.HashedSharedSecrets[i] = bp.Parse(request.HashedSharedSecrets[i])
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

		// Fix M-03: BlockNumber, Nullifier, PreviousSenderBalance and
		// PreviousSenderRandomValue were never assigned on the witness at
		// all — every real HTTP request panicked in frontend.NewWitness
		// (missing required circuit fields), so the withdraw circuit's
		// prover route was completely unreachable. Mirrors
		// deposit/handler.go's identical assignment for the same four
		// fields (BlockNumber via frontend.Variable directly, matching
		// deposit's own convention; the rest via bp.Parse like every
		// other field here).
		witness.BlockNumber = frontend.Variable(request.BlockNumber)
		witness.Nullifier = bp.Parse(request.Nullifier)
		witness.PreviousSenderBalance = bp.Parse(request.PreviousSenderBalance)
		witness.PreviousSenderRandomValue = bp.Parse(request.PreviousSenderRandomValue)

		// Fix M-08 (residual): these three previously used
		// frontend.Variable(request.X[i]) directly — wrapping the raw
		// request string rather than going through bp.Parse — which
		// skipped the error-return / BigIntParser gating every other
		// field already has, leaving the same witness.Fill goroutine-leak
		// trigger reachable through Hashes/SkDeposits/VPerDeposit alone.
		// Parsing them here also yields real *big.Int values, which the
		// Fix C-09 sum below needs anyway.
		vPerDeposit := make([]*big.Int, 10)
		for i := 0; i < 10; i++ {
			witness.Hashes[i] = bp.Parse(request.Hashes[i])
			witness.SkDeposits[i] = bp.Parse(request.SkDeposits[i])
			vPerDeposit[i] = bp.Parse(request.VPerDeposit[i])
			witness.VPerDeposit[i] = vPerDeposit[i]
		}

		// Fix C-09: TotalDepositValue is fully determined by the request's
		// own VPerDeposit entries — the caller does not supply it
		// separately, so there is no way to submit a mismatched value
		// through this endpoint (the circuit's own AssertIsEqual against
		// SenderTxValue is what actually enforces the binding; this is
		// just deriving the one witness value that assertion needs).
		totalDepositValue := new(big.Int)
		for _, v := range vPerDeposit {
			totalDepositValue.Add(totalDepositValue, v)
		}
		witness.TotalDepositValue = totalDepositValue
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

		// Generate public signal - order must match circuit public signal order
		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, bp.Parse(request.HashedSharedSecrets[i]))
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
		// Fix C-09: TotalDepositValue — see the AssertIsEqual in
		// circuit.go and Enygma.sol's withdraw() for how this gets bound
		// against Σ depositParams[i].amount on chain.
		publicSignal = append(publicSignal, new(big.Int).Set(totalDepositValue))
		publicSignal = append(publicSignal, bp.Parse(request.DomainId)) // Fix L-01

		c.JSON(http.StatusOK, WithdrawOutput{
			Proof:        proofRemix,
			PublicSignal: publicSignal,
		})

	}
}
