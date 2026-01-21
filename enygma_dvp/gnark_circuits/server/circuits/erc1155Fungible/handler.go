package erc1155Fungible

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
		var request ERC1155FungibleRequest

		if err := c.ShouldBindJSON(&request); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        } 
		fmt.Println(request)
	
		var publicSignal []*big.Int
		
		erc1155_join_split := templates.ERC1155FungibleCircuitConfig{
			TmNInputs: 2,
			TmMOutputs: 2,
			TmMerkleTreeDepth:8,
			TmAssetGroupMerkleTree: 8,
			TmRange: frontend.Variable("1000000000000000000000000000000000000"),
		}


		circuitJoinSplitERC1155:=templates.Erc1155FungibleCircuit{
			Config: erc1155_join_split,
			StTreeNumbers:    			make([]frontend.Variable, erc1155_join_split.TmNInputs),
			StMerkleRoots:    			make([]frontend.Variable, erc1155_join_split.TmNInputs),
			StNullifiers:  				make([]frontend.Variable, erc1155_join_split.TmNInputs),
			StCommitmentOut:  			make([]frontend.Variable, erc1155_join_split.TmMOutputs),
		
			WtPrivateKeysIn: 			make([]frontend.Variable, erc1155_join_split.TmNInputs),
			WtValuesIn:					make([]frontend.Variable, erc1155_join_split.TmNInputs),
			WtPathElements:				make([][]frontend.Variable, erc1155_join_split.TmNInputs),
			
			WtPathIndices:				make([]frontend.Variable, erc1155_join_split.TmNInputs),
			WtPublicKeysOut:    		make([]frontend.Variable, erc1155_join_split.TmMOutputs),
			WtValuesOut:    			make([]frontend.Variable, erc1155_join_split.TmMOutputs),
			WtAssetGroupPathElements: 	make([]frontend.Variable, erc1155_join_split.TmAssetGroupMerkleTree),
		}

		for i := range circuitJoinSplitERC1155.WtPathElements {

			circuitJoinSplitERC1155.WtPathElements[i] = make([]frontend.Variable, erc1155_join_split.TmMerkleTreeDepth)
		}



		witness:=templates.Erc1155FungibleCircuit{
			Config: erc1155_join_split,
			StTreeNumbers:    			make([]frontend.Variable, erc1155_join_split.TmNInputs),
			StMerkleRoots:    			make([]frontend.Variable, erc1155_join_split.TmNInputs),
			StNullifiers:  				make([]frontend.Variable, erc1155_join_split.TmNInputs),
			StCommitmentOut:  			make([]frontend.Variable, erc1155_join_split.TmMOutputs),
		
			WtPrivateKeysIn: 			make([]frontend.Variable, erc1155_join_split.TmNInputs),
			WtValuesIn:					make([]frontend.Variable, erc1155_join_split.TmNInputs),
			WtPathElements:				make([][]frontend.Variable, erc1155_join_split.TmNInputs),
			
			WtPathIndices:				make([]frontend.Variable, erc1155_join_split.TmNInputs),
			WtPublicKeysOut:    		make([]frontend.Variable, erc1155_join_split.TmMOutputs),
			WtValuesOut:    			make([]frontend.Variable, erc1155_join_split.TmMOutputs),
			WtAssetGroupPathElements: 	make([]frontend.Variable, erc1155_join_split.TmAssetGroupMerkleTree),
		}

		for i := range witness.WtPathElements {

			witness.WtPathElements[i] = make([]frontend.Variable, erc1155_join_split.TmMerkleTreeDepth)
		}


		witness.StMessage = frontend.Variable(request.StMessage)
		witness.StAssetGroupMerkleRoot = frontend.Variable(request.StAssetGroupMerkleRoot)
		witness.StAssetGroupTreeNumber = frontend.Variable(request.StAssetGroupTreeNumber)
		witness.WtErc1155ContractAddress = frontend.Variable(request.WtErc1155ContractAddress)
		witness.WtErc1155TokenId = frontend.Variable(request.WtErc1155TokenId)
		witness.WtAssetGroupPathIndices = frontend.Variable(request.WtAssetGroupPathIndices)
		

		for i:=0; i < erc1155_join_split.TmNInputs;i++{
			witness.StTreeNumbers[i] = frontend.Variable(request.StTreeNumbers[i])
			witness.StMerkleRoots[i] = frontend.Variable(request.StMerkleRoots[i])
			witness.StNullifiers[i] = frontend.Variable(request.StNullifiers[i])

			witness.WtPrivateKeysIn[i] = frontend.Variable(request.WtPrivateKeysIn[i])
			witness.WtValuesIn[i] = frontend.Variable(request.WtValuesIn[i])
			witness.WtPathIndices[i] = frontend.Variable(request.WtPathIndices[i])
			
	
			for j:=0; j<erc1155_join_split.TmMerkleTreeDepth; j++{
				witness.WtPathElements[i][j] = frontend.Variable(request.WtPathElements[i][j])
			}
		}

		for i:=0; i<erc1155_join_split.TmMOutputs;i++{
			witness.StCommitmentOut[i] = frontend.Variable(request.StCommitmentOut[i])
			witness.WtPublicKeysOut[i] = frontend.Variable(request.WtPublicKeysOut[i])	
			witness.WtValuesOut[i] = frontend.Variable(request.WtValuesOut[i])		

		}

		for i:=0; i<erc1155_join_split.TmAssetGroupMerkleTree;i++{
			witness.WtAssetGroupPathElements[i] = frontend.Variable(request.WtAssetGroupPathElements[i])
		}


		witness.Config = erc1155_join_split

		solver.RegisterHint(primitives.ModHint)
		solver.RegisterHint(primitives.ERC155UniqueIdNative)
		solver.RegisterHint(primitives.PoseidonNative)
		solver.RegisterHint(primitives.PoseidonPrivateKeyNative)


		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitJoinSplitERC1155)

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


		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StMessage))
		
		for i:=0; i < erc1155_join_split.TmNInputs;i++{
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StTreeNumbers[i]))
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StMerkleRoots[i]))
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StNullifiers[i]))
		}
		for i:=0; i < erc1155_join_split.TmMOutputs;i++{
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StCommitmentOut[i]))
		}
		

		c.JSON(http.StatusOK, ERC1155FungibleWithBrokerOutput{
            Proof:  proofRemix,
            PublicSignal:publicSignal,
        })


	
	}
}