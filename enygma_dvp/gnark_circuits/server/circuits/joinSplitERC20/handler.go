package joinSplitERC20

import (
	"log"
	"fmt"
	"math/big"
    "net/http"

	utils "gnark_server/utils"

    "github.com/gin-gonic/gin"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
    "github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/constraint/solver"
    
	groth16_bn254 "github.com/consensys/gnark/backend/groth16/bn254"

	"gnark_server/templates"
	"gnark_server/primitives"
 
)

func NewHandler(pkPath, vkPath string) gin.HandlerFunc {

	curve := ecc.BN254 
	
	pk, _ := utils.LoadProvingKey(curve, pkPath)
	vk, _ := utils.LoadVerifyingKey(curve, vkPath)
	return func(c *gin.Context) {
        var request JoinSplitERC20Request
		
        if err := c.ShouldBindJSON(&request); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        } 
		fmt.Println(request)
	
		var publicSignal []*big.Int

		joinsplit_erc20_config := templates.Erc20CircuitConfig{
			TmNInputs: 2,
			TmMOutputs: 2,
			TmMerkleTreeDepth:8,
			TmRange: frontend.Variable("1000000000000000000000000000000000000"),
		}

		circuitJoinSplitERC20:=templates.Erc20Circuit{
			Config: joinsplit_erc20_config,
			StTreeNumber:    			make([]frontend.Variable, joinsplit_erc20_config.TmNInputs),
			StMerkleRoots:    			make([]frontend.Variable, joinsplit_erc20_config.TmNInputs),
			StNullifiers:  				make([]frontend.Variable, joinsplit_erc20_config.TmNInputs),
			StCommitmentOut:  			make([]frontend.Variable, joinsplit_erc20_config.TmMOutputs),
			WtPrivateKeysIn: 			make([]frontend.Variable, joinsplit_erc20_config.TmNInputs),
			WtValuesIn: 				make([]frontend.Variable, joinsplit_erc20_config.TmNInputs),
			WtPathElements:				make([][]frontend.Variable, joinsplit_erc20_config.TmNInputs),
			WtPathIndices:				make([]frontend.Variable, joinsplit_erc20_config.TmNInputs),
			WtPublicKeysOut:			make([]frontend.Variable, joinsplit_erc20_config.TmMOutputs),
			WtValuesOut:    			make([]frontend.Variable, joinsplit_erc20_config.TmMOutputs),
    
		}
		
		for i := range circuitJoinSplitERC20.WtPathElements {

			circuitJoinSplitERC20.WtPathElements[i] = make([]frontend.Variable, joinsplit_erc20_config.TmMerkleTreeDepth)
		}
		
		witness:=templates.Erc20Circuit{
			Config: joinsplit_erc20_config,
			StTreeNumber:    			make([]frontend.Variable, joinsplit_erc20_config.TmNInputs),
			StMerkleRoots:    			make([]frontend.Variable, joinsplit_erc20_config.TmNInputs),
			StNullifiers:  				make([]frontend.Variable, joinsplit_erc20_config.TmNInputs),
			StCommitmentOut:  			make([]frontend.Variable, joinsplit_erc20_config.TmMOutputs),
			WtPrivateKeysIn: 			make([]frontend.Variable, joinsplit_erc20_config.TmNInputs),
			WtValuesIn: 				make([]frontend.Variable, joinsplit_erc20_config.TmNInputs),
			WtPathElements:				make([][]frontend.Variable, joinsplit_erc20_config.TmNInputs),
			WtPathIndices:				make([]frontend.Variable, joinsplit_erc20_config.TmNInputs),
			WtPublicKeysOut:			make([]frontend.Variable, joinsplit_erc20_config.TmMOutputs),
			WtValuesOut:    			make([]frontend.Variable, joinsplit_erc20_config.TmMOutputs),
		}
		
		for i := range witness.WtPathElements {

			witness.WtPathElements[i] = make([]frontend.Variable, joinsplit_erc20_config.TmMerkleTreeDepth)
		}
		

		witness.StMessage = frontend.Variable(request.StMessage)
		witness.WtErc20ContractAddress = frontend.Variable(request.WtErc20ContractAddress)
		

		for i:=0; i < joinsplit_erc20_config.TmNInputs ;i++{
			witness.StTreeNumber[i] = frontend.Variable(request.StTreeNumber[i])
			witness.StMerkleRoots[i] = frontend.Variable(request.StMerkleRoots[i])
			witness.StNullifiers[i] =  frontend.Variable(request.StNullifiers[i])
			
			witness.WtPrivateKeysIn[i] =  frontend.Variable(request.WtPrivateKeysIn[i])
			witness.WtValuesIn[i] = frontend.Variable(request.WtValuesIn[i])
			witness.WtPathIndices[i] = frontend.Variable(request.WtPathIndices[i])

			for j:=0 ; j < joinsplit_erc20_config.TmMerkleTreeDepth; j++{
				witness.WtPathElements[i][j] = frontend.Variable(request.WtPathElements[i][j])
			}
		}

		for i:=0; i < joinsplit_erc20_config.TmMOutputs; i++{
			witness.StCommitmentOut[i] =  frontend.Variable(request.StCommitmentOut[i])
			witness.WtPublicKeysOut[i] =  frontend.Variable(request.WtPublicKeysOut[i])
			witness.WtValuesOut[i] =  frontend.Variable(request.WtValuesOut[i])
		}

		
		
		witness.Config = joinsplit_erc20_config

		solver.RegisterHint(primitives.ModHint)
		solver.RegisterHint(primitives.ERC155UniqueIdNative)
		solver.RegisterHint(primitives.PoseidonNative)

		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitJoinSplitERC20)

		witnessFull, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
		if err != nil {
			log.Fatal(err)
		}
		
		proof, err := groth16.Prove(ccs, pk, witnessFull)
		witnessPublic, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField(), frontend.PublicOnly())
		err = groth16.Verify(proof, vk, witnessPublic)
		if err != nil {
			panic(err)
		}

		println("Proof verified successfully!")
		fmt.Println("proof",proof)
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
		p.Bs.X.A0.BigInt(BX01) // Convert first part of B.X

		BX11 := new(big.Int)
		p.Bs.X.A1.BigInt(BX11) // Convert second part of B.X

		BY01 := new(big.Int)
		p.Bs.Y.A0.BigInt(BY01) // Convert first part of B.Y

		BY11 := new(big.Int)
		p.Bs.Y.A1.BigInt(BY11) // Convert second part of B.Y

		//Proof in Remix format (order matters!)
		proofRemix := []*big.Int{
			A_x1, A_y1,     // G1 point Ar
			BX11, BX01,     // G2 point Bs.X (Fp²)
			BY11, BY01,     // G2 point Bs.Y (Fp²)
			C_x1, C_y1,     // G1 point Krs
		}

		//Generate public signal
		
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StMessage))

		for i:=0; i < joinsplit_erc20_config.TmNInputs ;i++{
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StTreeNumber[i]))
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StMerkleRoots[i]))
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StNullifiers[i]))
			
		}
		for i:=0; i < joinsplit_erc20_config.TmMOutputs ;i++{
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StCommitmentOut[i]))
		}
		
		
		c.JSON(http.StatusOK, JoinSplitERC20Output{
            Proof:  proofRemix,
            PublicSignal:publicSignal,
        })


	}
}	