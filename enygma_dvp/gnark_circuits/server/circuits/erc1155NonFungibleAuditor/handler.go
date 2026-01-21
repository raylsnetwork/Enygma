package erc1155NonFungibleAuditor

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
        var request ERC1155NonFungibleAuditorRequest
		
        if err := c.ShouldBindJSON(&request); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        } 
		fmt.Println(request)
	
		var publicSignal []*big.Int

		ownership_erc1155_non_fungible_auditor_config := templates.ERC1155NonFungibleWithAuditorCircuitConfig{
			TmNumOfTokens:1,
			TmMerkleTreeDepth:8,
			TmAssetGroupMerkleTree:8,
		}

		plainLength:= ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens*2+1
		decLength:=plainLength
		for decLength%3 != 0 {
			decLength++
		}
		encLength := decLength + 1

		circuitNonFungibleAuditorERC1155:=templates.ERC1155NonFungibleWithAuditorCircuit{
			Config: ownership_erc1155_non_fungible_auditor_config,
			StTreeNumbers:    			make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			StMerkleRoots:    			make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			StNullifiers:  				make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			StCommitmentOut:  			make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			StAssetGroupTreeNumber: 	make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			StAssetGroupMerkleRoot: 	make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			WtPrivateKeysIn: 			make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			WtValues: 					make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			WtPathElements:				make([][]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			WtPathIndices:				make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			WtErc1155TokenIds:			make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			WtPublicKeysOut:    		make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			WtAssetGroupPathElements:   make([][]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			WtAssetGroupPathIndices:    make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			
			StAuditorEncryptedValues: make([]frontend.Variable, encLength),
		}

		for i := range circuitNonFungibleAuditorERC1155.WtPathElements {

			circuitNonFungibleAuditorERC1155.WtPathElements[i] = make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmMerkleTreeDepth)
		}

		for i := range circuitNonFungibleAuditorERC1155.WtAssetGroupPathElements {

			circuitNonFungibleAuditorERC1155.WtAssetGroupPathElements[i] = make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmAssetGroupMerkleTree)
		}
		
		witness:=templates.ERC1155NonFungibleWithAuditorCircuit{
			Config: ownership_erc1155_non_fungible_auditor_config,
			StTreeNumbers:    			make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			StMerkleRoots:    			make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			StNullifiers:  				make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			StCommitmentOut:  			make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			StAssetGroupTreeNumber: 	make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			StAssetGroupMerkleRoot: 	make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			WtPrivateKeysIn: 			make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			WtValues: 					make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			WtPathElements:				make([][]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			WtPathIndices:				make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			WtErc1155TokenIds:			make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			WtPublicKeysOut:    		make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			WtAssetGroupPathElements:   make([][]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),
			WtAssetGroupPathIndices:    make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens),

			StAuditorEncryptedValues: make([]frontend.Variable, encLength),
		}

		for i := range witness.WtPathElements {

			witness.WtPathElements[i] = make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmMerkleTreeDepth)
		}

		for i := range witness.WtAssetGroupPathElements {

			witness.WtAssetGroupPathElements[i] = make([]frontend.Variable, ownership_erc1155_non_fungible_auditor_config.TmAssetGroupMerkleTree)
		}
			

		witness.StMessage = frontend.Variable(request.StMessage)
		witness.WtErc1155ContractAddress = frontend.Variable(request.WtErc1155ContractAddress)
		witness.StAuditorNonce  = frontend.Variable(request.StAuditorNonce)
		witness.WtAuditorRandom  = frontend.Variable(request.WtAuditorRandom)


		for i:=0; i < ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens ;i++{
			witness.StTreeNumbers[i] = frontend.Variable(request.StTreeNumbers[i])
			witness.StMerkleRoots[i] = frontend.Variable(request.StMerkleRoots[i])
			witness.StNullifiers[i] =  frontend.Variable(request.StNullifiers[i])
			witness.StCommitmentOut[i] =  frontend.Variable(request.StCommitmentOut[i])
			witness.StAssetGroupTreeNumber[i] = frontend.Variable(request.StAssetGroupTreeNumber[i])
			witness.StAssetGroupMerkleRoot[i] = frontend.Variable(request.StAssetGroupMerkleRoot[i])
			witness.WtPrivateKeysIn[i] = frontend.Variable(request.WtPrivateKeysIn[i])
			witness.WtValues[i] = frontend.Variable(request.WtValues[i])
			witness.WtPathIndices[i] = frontend.Variable(request.WtPathIndices[i])
			witness.WtErc1155TokenIds[i] = frontend.Variable(request.WtErc1155TokenIds[i])
			witness.WtPublicKeysOut[i] = frontend.Variable(request.WtPublicKeysOut[i])
			witness.WtAssetGroupPathIndices[i] = frontend.Variable(request.WtAssetGroupPathIndices[i])
			
			for j:=0 ; j < ownership_erc1155_non_fungible_auditor_config.TmMerkleTreeDepth; j++{
				witness.WtPathElements[i][j] = frontend.Variable(request.WtPathElements[i][j])
			}

			for j:=0 ; j < ownership_erc1155_non_fungible_auditor_config.TmAssetGroupMerkleTree; j++{
				witness.WtAssetGroupPathElements[i][j] = frontend.Variable(request.WtAssetGroupPathElements[i][j])
			}

		}
		
		for i:=0; i< 2; i++{
			witness.StAuditorPublickey[i] = frontend.Variable(request.StAuditorPublickey[i])
			witness.StAuditorAuthKey[i] = frontend.Variable(request.StAuditorAuthKey[i])
		}

		for i:=0; i< encLength; i++{
			witness.StAuditorEncryptedValues[i] = frontend.Variable(request.StAuditorEncryptedValues[i])
		}

		witness.Config = ownership_erc1155_non_fungible_auditor_config

		solver.RegisterHint(primitives.ModHint)
		solver.RegisterHint(primitives.ERC155UniqueIdNative)
		solver.RegisterHint(primitives.PoseidonNative)
		solver.RegisterHint(primitives.PoseidonPrivateKeyNative)

		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitNonFungibleAuditorERC1155)

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

		for i:=0; i < ownership_erc1155_non_fungible_auditor_config.TmNumOfTokens ;i++{
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StTreeNumbers[i]))
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StMerkleRoots[i]))
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StNullifiers[i]))
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StCommitmentOut[i]))
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAssetGroupTreeNumber[i]))
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAssetGroupMerkleRoot[i]))
			
		}
		
		c.JSON(http.StatusOK, ERC1155NonFungibleAuditorOutput{
            Proof:  proofRemix,
            PublicSignal:publicSignal,
        })


	}
}	


