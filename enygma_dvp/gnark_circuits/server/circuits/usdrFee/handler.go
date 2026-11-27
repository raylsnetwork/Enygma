package usdrFee

import (
	"fmt"
	"math/big"
	"net/http"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	groth16_bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/gin-gonic/gin"

	"enygma_dvp/gnark_circuits/templates"
	utils "enygma_dvp/gnark_circuits/utils"

	"enygma_gnark_shared/witnessinput"
)

// NewHandler returns a gin.HandlerFunc for POST /proof/usdrFee.
//
// Circuit: 1 input / 2 outputs / Merkle depth 8.
//
// Conservation: Σ(valuesIn) == valOut[0] + valOut[1]  (nothing burned)
// Fee binding:  valOut[0] == StFee
//
// The response's public signals are the proven witness's, in the UsdrFee
// statement layout (enygma_dvp/layout).
func NewHandler(pkPath, vkPath string) gin.HandlerFunc {
	curve := ecc.BN254

	pk, _ := utils.LoadProvingKey(curve, pkPath)
	vk, _ := utils.LoadVerifyingKey(curve, vkPath)

	cfg := templates.UsdrFeeConfig

	newCircuit := func() templates.UsdrFeeCircuit {
		cir := templates.UsdrFeeCircuit{
			Config:               cfg,
			StTreeNumbers:        make([]frontend.Variable, cfg.TmNInputs),
			StMerkleRoots:        make([]frontend.Variable, cfg.TmNInputs),
			StNullifiers:         make([]frontend.Variable, cfg.TmNInputs),
			StCommitmentsOut:     make([]frontend.Variable, cfg.TmMOutputs),
			WtPrivateKeysIn:      make([]frontend.Variable, cfg.TmNInputs),
			WtValuesIn:           make([]frontend.Variable, cfg.TmNInputs),
			WtSaltsIn:            make([]frontend.Variable, cfg.TmNInputs),
			WtPathIndices:        make([]frontend.Variable, cfg.TmNInputs),
			WtPathElements:       make([][]frontend.Variable, cfg.TmNInputs),
			WtSpendPublicKeysOut: make([]frontend.Variable, cfg.TmMOutputs),
			WtValuesOut:          make([]frontend.Variable, cfg.TmMOutputs),
			WtSaltsOut:           make([]frontend.Variable, cfg.TmMOutputs),
		}
		for i := range cir.WtPathElements {
			cir.WtPathElements[i] = make([]frontend.Variable, cfg.TmMerkleTreeDepth)
		}
		return cir
	}

	circuit := newCircuit()

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		panic(fmt.Sprintf("compile usdrFee circuit: %v", err))
	}

	return func(c *gin.Context) {
		var request UsdrFeeRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		witness := newCircuit()

		var numbers witnessinput.Parser
		witness.StMessage = numbers.Parse(request.StMessage)
		witness.StContractAddress = numbers.Parse(request.StContractAddress)
		witness.StCiphertextHash = numbers.Parse(request.StCiphertextHash)
		witness.StFee = numbers.Parse(request.StFee)
		witness.StTokenId = numbers.Parse(request.StTokenId)

		for i := 0; i < cfg.TmNInputs; i++ {
			witness.StTreeNumbers[i] = numbers.Parse(request.StTreeNumbers[i])
			witness.StMerkleRoots[i] = numbers.Parse(request.StMerkleRoots[i])
			witness.StNullifiers[i] = numbers.Parse(request.StNullifiers[i])
			witness.WtPrivateKeysIn[i] = numbers.Parse(request.WtPrivateKeysIn[i])
			witness.WtValuesIn[i] = numbers.Parse(request.WtValuesIn[i])
			witness.WtSaltsIn[i] = numbers.Parse(request.WtSaltsIn[i])
			witness.WtPathIndices[i] = numbers.Parse(request.WtPathIndices[i])
			for j := 0; j < cfg.TmMerkleTreeDepth; j++ {
				witness.WtPathElements[i][j] = numbers.Parse(request.WtPathElements[i][j])
			}
		}

		for i := 0; i < cfg.TmMOutputs; i++ {
			witness.StCommitmentsOut[i] = numbers.Parse(request.StCommitmentsOut[i])
			witness.WtSpendPublicKeysOut[i] = numbers.Parse(request.WtSpendPublicKeysOut[i])
			witness.WtValuesOut[i] = numbers.Parse(request.WtValuesOut[i])
			witness.WtSaltsOut[i] = numbers.Parse(request.WtSaltsOut[i])
		}

		witness.Config = cfg

		if err := numbers.Err(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		witnessFull, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("build witness: %v", err)})
			return
		}

		proof, err := groth16.Prove(ccs, pk, witnessFull)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("prove: %v", err)})
			return
		}

		witnessPublic, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField(), frontend.PublicOnly())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("build public witness: %v", err)})
			return
		}
		if err := groth16.Verify(proof, vk, witnessPublic); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("verify: %v", err)})
			return
		}

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

		publicSignal, err := utils.PublicSignals(witnessFull)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, UsdrFeeOutput{
			Proof:        proofRemix,
			PublicSignal: publicSignal,
		})
	}
}
