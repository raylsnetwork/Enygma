package paymentFee

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

	"gnark_server/primitives"
	"gnark_server/templates"
	utils "gnark_server/utils"
)

// NewHandler returns a gin.HandlerFunc for POST /proof/paymentFee.
//
// Circuit: 1 input / 2 outputs / Merkle depth 8.
//
// Conservation: Σ(valuesIn) == valOut[0] + valOut[1] + StFee
//
// Public signal layout (8 elements):
//
//	[StMessage, StTreeNumbers[0], StMerkleRoots[0], StNullifiers[0],
//	 StCommitmentsOut[0], StCommitmentsOut[1], StContractAddress, StFee]
func NewHandler(pkPath, vkPath string) gin.HandlerFunc {
	curve := ecc.BN254

	pk, _ := utils.LoadProvingKey(curve, pkPath)
	vk, _ := utils.LoadVerifyingKey(curve, vkPath)

	cfg := templates.PaymentCircuitConfig{
		TmNInputs:         1,
		TmMOutputs:        2,
		TmMerkleTreeDepth: 8,
		TmRange:           frontend.Variable("1000000000000000000000000000000000000"),
	}

	newCircuit := func() templates.PaymentFeeCircuit {
		cir := templates.PaymentFeeCircuit{
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
	witness := newCircuit()

	solver.RegisterHint(primitives.ModHint)
	solver.RegisterHint(primitives.PoseidonNative)
	solver.RegisterHint(primitives.PoseidonPrivateKeyNative)

	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		panic(fmt.Sprintf("compile paymentFee circuit: %v", err))
	}

	return func(c *gin.Context) {
		var request PaymentFeeRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		witness = newCircuit()

		witness.StMessage = frontend.Variable(request.StMessage)
		witness.StContractAddress = frontend.Variable(request.StContractAddress)
		witness.StFee = frontend.Variable(request.StFee)
		witness.WtTokenId = frontend.Variable(request.WtTokenId)

		for i := 0; i < cfg.TmNInputs; i++ {
			witness.StTreeNumbers[i] = frontend.Variable(request.StTreeNumbers[i])
			witness.StMerkleRoots[i] = frontend.Variable(request.StMerkleRoots[i])
			witness.StNullifiers[i] = frontend.Variable(request.StNullifiers[i])
			witness.WtPrivateKeysIn[i] = frontend.Variable(request.WtPrivateKeysIn[i])
			witness.WtValuesIn[i] = frontend.Variable(request.WtValuesIn[i])
			witness.WtSaltsIn[i] = frontend.Variable(request.WtSaltsIn[i])
			witness.WtPathIndices[i] = frontend.Variable(request.WtPathIndices[i])
			for j := 0; j < cfg.TmMerkleTreeDepth; j++ {
				witness.WtPathElements[i][j] = frontend.Variable(request.WtPathElements[i][j])
			}
		}

		for i := 0; i < cfg.TmMOutputs; i++ {
			witness.StCommitmentsOut[i] = frontend.Variable(request.StCommitmentsOut[i])
			witness.WtSpendPublicKeysOut[i] = frontend.Variable(request.WtSpendPublicKeysOut[i])
			witness.WtValuesOut[i] = frontend.Variable(request.WtValuesOut[i])
			witness.WtSaltsOut[i] = frontend.Variable(request.WtSaltsOut[i])
		}

		witness.Config = cfg

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

		// public signal (8 elements):
		// [msg, treeNum[0], root[0], nf[0], cmt_bob, cmt_change, contractAddr, fee]
		var publicSignal []*big.Int
		publicSignal = append(publicSignal, utils.ParseBigInt(request.StMessage))
		for i := 0; i < cfg.TmNInputs; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(request.StTreeNumbers[i]))
			publicSignal = append(publicSignal, utils.ParseBigInt(request.StMerkleRoots[i]))
			publicSignal = append(publicSignal, utils.ParseBigInt(request.StNullifiers[i]))
		}
		for i := 0; i < cfg.TmMOutputs; i++ {
			publicSignal = append(publicSignal, utils.ParseBigInt(request.StCommitmentsOut[i]))
		}
		publicSignal = append(publicSignal, utils.ParseBigInt(request.StContractAddress))
		publicSignal = append(publicSignal, utils.ParseBigInt(request.StFee))

		c.JSON(http.StatusOK, PaymentFeeOutput{
			Proof:        proofRemix,
			PublicSignal: publicSignal,
		})
	}
}
