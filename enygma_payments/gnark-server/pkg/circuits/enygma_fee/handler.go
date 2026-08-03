package enygma_fee

import (
	"fmt"
	"log"
	"math/big"
	"net/http"

	utils "enygma-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/backend/groth16"
	groth16_bn254 "github.com/consensys/gnark/backend/groth16/bn254"
)

func createCircuitTemplate(config EnygmaFeeCircuitConfig) EnygmaFeeCircuit {
	return EnygmaFeeCircuit{
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
}

// NewHandler returns a gin handler for POST /proof/enygma_fee.
// pkPath and vkPath are the file paths to the Groth16 proving and verifying keys.
func NewHandler(pkPath, vkPath string) gin.HandlerFunc {
	curve := ecc.BN254
	pk, _ := utils.LoadProvingKey(curve, pkPath)

	config := EnygmaFeeCircuitConfig{NCommitment: 6}
	circuitTemplate := createCircuitTemplate(config)
	solver.RegisterHint(utils.ModHint)
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitTemplate)
	if err != nil {
		log.Fatalf("failed to compile enygma_fee circuit: %v", err)
	}

	return func(c *gin.Context) {
		var request EnygmaFeeRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		witness := createCircuitTemplate(config)

		witness.SenderId = frontend.Variable(request.SenderID)
		witness.SenderTxValue = frontend.Variable(request.SenderTxValue)
		witness.SecretKey = frontend.Variable(request.SecretKey)
		witness.Fee = utils.ParseBigInt(request.Fee)

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

		// ── Compute aggregate public outputs ─────────────────────────────────
		// SumTxValuesWithFee = (Σ TxValues + fee) mod P_bjj
		sumTxVal := new(big.Int)
		for i := 0; i < config.NCommitment; i++ {
			sumTxVal.Add(sumTxVal, utils.ParseBigInt(request.TxValues[i]))
		}
		sumTxVal.Add(sumTxVal, utils.ParseBigInt(request.Fee))
		sumTxVal.Mod(sumTxVal, utils.P)
		witness.SumTxValuesWithFee = sumTxVal

		// SumTxCommit = Σ(TxCommit[i]) + fee·G as a Baby JubJub point.
		// Sender's TxValue = -(amount+fee), so Σ(TxCommit) = (-fee)·G.
		// Adding fee·G gives (0)·G = identity (0,1) — the conservation invariant.
		// Start from TxCommit[0] to mirror the circuit's chain exactly.
		sumCommit := &babyjub.Point{
			X: utils.ParseBigInt(request.TxCommit[0][0]),
			Y: utils.ParseBigInt(request.TxCommit[0][1]),
		}
		for i := 1; i < config.NCommitment; i++ {
			pt := &babyjub.Point{
				X: utils.ParseBigInt(request.TxCommit[i][0]),
				Y: utils.ParseBigInt(request.TxCommit[i][1]),
			}
			sumCommit = utils.AddPks(sumCommit, pt)
		}
		// Add fee·G using the same generator G as the gnark circuit
		feeInt := utils.ParseBigInt(request.Fee)
		feeGPoint := babyjub.NewPoint().Mul(feeInt, utils.CircuitGBabyJub)
		sumCommit = utils.AddPks(sumCommit, feeGPoint)
		witness.SumTxCommit[0] = sumCommit.X
		witness.SumTxCommit[1] = sumCommit.Y

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

		// Public signal order matches circuit struct field declaration order (51 slots total).
		var publicSignal []*big.Int

		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(request.HashedSharedSecrets[i])) // 0–5
		}
		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(request.PublicKey[i])) // 6–11
		}
		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(request.PreviousCommit[i][0])) // 12–23
			publicSignal = append(publicSignal, utils.ParseBigInt(request.PreviousCommit[i][1]))
		}
		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(request.TxCommit[i][0])) // 24–35
			publicSignal = append(publicSignal, utils.ParseBigInt(request.TxCommit[i][1]))
		}
		publicSignal = append(publicSignal, utils.ParseBigInt(request.BlockNumber)) // 36
		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(request.AnonymitySet[i])) // 37–42
		}
		for i := 0; i < config.NCommitment; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(request.MessageTags[i])) // 43–48
		}
		publicSignal = append(publicSignal, utils.ParseBigInt(request.Nullifier)) // 49
		publicSignal = append(publicSignal, utils.ParseBigInt(request.Fee))       // 50
		publicSignal = append(publicSignal, sumCommit.X)                          // 51 SumTxCommit.X
		publicSignal = append(publicSignal, sumCommit.Y)                          // 52 SumTxCommit.Y
		publicSignal = append(publicSignal, sumTxVal)                             // 53 SumTxValuesWithFee

		c.JSON(http.StatusOK, EnygmaFeeOutput{
			Proof:        proofRemix,
			PublicSignal: publicSignal,
		})
	}
}
