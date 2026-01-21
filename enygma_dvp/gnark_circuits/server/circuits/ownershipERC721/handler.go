package ownershipERC721

import (
	"log"
	//"fmt"
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
        var request OwnershipERC721Request
		
        if err := c.ShouldBindJSON(&request); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        } 
		// fmt.Println(request)
	
		var publicSignal []*big.Int

		ownership_erc721_config := templates.Erc721CircuitConfig{
			TmNumOfTokens: 1,
			TmMerkleTreeDepth: 8,
		}

		circuitOwnershipERC721:=templates.Erc721Circuit{
			Config: ownership_erc721_config,
			StTreeNumbers:			    make([]frontend.Variable,ownership_erc721_config.TmNumOfTokens),
			StNullifiers:				make([]frontend.Variable, ownership_erc721_config.TmNumOfTokens),
			StMerkleRoots:				make([]frontend.Variable, ownership_erc721_config.TmNumOfTokens),
			StCommitmentOut:			make([]frontend.Variable, ownership_erc721_config.TmNumOfTokens),
			WtPrivateKeysIn:			make([]frontend.Variable, ownership_erc721_config.TmNumOfTokens),
			WtValues:					make([]frontend.Variable, ownership_erc721_config.TmNumOfTokens),
			
			WtPathElements:             make([][]frontend.Variable, ownership_erc721_config.TmNumOfTokens),
			WtPathIndices:				make([]frontend.Variable, ownership_erc721_config.TmNumOfTokens),
			WtPublicKeysOut:			make([]frontend.Variable, ownership_erc721_config.TmNumOfTokens),
		}
		
		for i := range circuitOwnershipERC721.WtPathElements {

			circuitOwnershipERC721.WtPathElements[i] = make([]frontend.Variable, ownership_erc721_config.TmMerkleTreeDepth)
		}
		
		witness:=templates.Erc721Circuit{
			Config: ownership_erc721_config,
			StTreeNumbers:			    make([]frontend.Variable,ownership_erc721_config.TmNumOfTokens),
			StNullifiers:				make([]frontend.Variable, ownership_erc721_config.TmNumOfTokens),
			StMerkleRoots:				make([]frontend.Variable, ownership_erc721_config.TmNumOfTokens),
			StCommitmentOut:			make([]frontend.Variable, ownership_erc721_config.TmNumOfTokens),
			WtPrivateKeysIn:			make([]frontend.Variable, ownership_erc721_config.TmNumOfTokens),
			WtValues:					make([]frontend.Variable, ownership_erc721_config.TmNumOfTokens),
			
			WtPathElements:             make([][]frontend.Variable, ownership_erc721_config.TmNumOfTokens),
			WtPathIndices:				make([]frontend.Variable, ownership_erc721_config.TmNumOfTokens),
			WtPublicKeysOut:			make([]frontend.Variable, ownership_erc721_config.TmNumOfTokens),
		}
		
		for i := range witness.WtPathElements {

			witness.WtPathElements[i] = make([]frontend.Variable, ownership_erc721_config.TmMerkleTreeDepth)
		}
		

		witness.StMessage = frontend.Variable(request.StMessage)
		
		
		for i:=0; i < ownership_erc721_config.TmNumOfTokens ;i++{
			witness.StTreeNumbers[i] = frontend.Variable(request.StTreeNumbers[i])
			witness.StMerkleRoots[i] = frontend.Variable(request.StMerkleRoots[i])
			witness.StNullifiers[i]  =  frontend.Variable(request.StNullifiers[i])
			witness.StCommitmentOut[i] = frontend.Variable(request.StCommitmentOut[i])

			witness.WtPrivateKeysIn[i] =  frontend.Variable(request.WtPrivateKeysIn[i])
			witness.WtValues[i] = frontend.Variable(request.WtValues[i])
			
			witness.WtPathIndices[i] = frontend.Variable(request.WtPathIndices[i])
			witness.WtPublicKeysOut[i] = frontend.Variable(request.WtPublicKeysOut[i])

			for j:=0 ; j < ownership_erc721_config.TmMerkleTreeDepth; j++{
				witness.WtPathElements[i][j] = frontend.Variable(request.WtPathElements[i][j])
			}
		}

		witness.Config = ownership_erc721_config

		solver.RegisterHint(primitives.ModHint)
		solver.RegisterHint(primitives.ERC155UniqueIdNative)
		solver.RegisterHint(primitives.PoseidonNative)

		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitOwnershipERC721)

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

		//Generate public signal
		
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StMessage))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StTreeNumbers[0]))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StMerkleRoots[0]))
		
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StNullifiers[0]))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StCommitmentOut[0]))
		
		c.JSON(http.StatusOK, OwnershipERC721Output{
            Proof:  proofRemix,
            PublicSignal:publicSignal,
        })


	}
}	