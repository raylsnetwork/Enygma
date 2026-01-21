package auctionBidAuditor

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
		var request AuctionBidAuditorRequest

		if err := c.ShouldBindJSON(&request); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        } 
		fmt.Println(request)
	
		var publicSignal []*big.Int
		
		auctionbidAuditorConfig := templates.TmAuctionBidAuditorCircuitConfig{
			TmInputs:    2,
			TmOutputs:   2,
			TmNumOfIdParms: 5,
			TmMerkleTreeDepth: 8,
			TmRange:     frontend.Variable("1000000000000000000000000000000000000"),
			TmAssetGroupMerkleTreeDepth: 8,
		}

		plainLength:= auctionbidAuditorConfig.TmNumOfIdParms * (auctionbidAuditorConfig.TmInputs+auctionbidAuditorConfig.TmOutputs) + 4

		decLength:=plainLength
		for decLength%3 != 0 {
			decLength++
		}
		encLength := decLength + 1

		// Auctioneer encryption: 3 values (bidAmount, bidRandom, 0) -> encLength = 4
		auctioneerEncLength := 4

		circuitAuctionBid :=templates.TmAuctionBidAuditorCircuit{
			Config: auctionbidAuditorConfig,
			StTreeNumbers:    			make([]frontend.Variable, auctionbidAuditorConfig.TmInputs),
			StMerkleRoots:    			make([]frontend.Variable, auctionbidAuditorConfig.TmInputs),
			StNullifiers:  				make([]frontend.Variable, auctionbidAuditorConfig.TmInputs),
			StCommitmentsOuts:  		make([]frontend.Variable, auctionbidAuditorConfig.TmOutputs),

			StAuctioneeEncryptedValues: make([]frontend.Variable, auctioneerEncLength),
			StAuditorEncryptedValues: 	make([]frontend.Variable, encLength),

			WtPathElements: 			make([][]frontend.Variable, auctionbidAuditorConfig.TmInputs),

			WtPrivateKeysIn:			make([]frontend.Variable, auctionbidAuditorConfig.TmInputs),

			WtPathIndices:				make([]frontend.Variable, auctionbidAuditorConfig.TmInputs),
			WtPublicKeysOut:			make([]frontend.Variable, auctionbidAuditorConfig.TmOutputs),
			WtAssetGroupPathElements:	make([]frontend.Variable, auctionbidAuditorConfig.TmMerkleTreeDepth),


			WtIdParamsIn:    			make([][]frontend.Variable, auctionbidAuditorConfig.TmInputs),
			WtIdParamsOut:   			make([][]frontend.Variable, auctionbidAuditorConfig.TmOutputs),
		}
		for i := range circuitAuctionBid.WtPathElements {

			circuitAuctionBid.WtPathElements[i] = make([]frontend.Variable, auctionbidAuditorConfig.TmMerkleTreeDepth)
		}

		for i := range circuitAuctionBid.WtIdParamsIn {

			circuitAuctionBid.WtIdParamsIn[i] = make([]frontend.Variable,auctionbidAuditorConfig.TmNumOfIdParms)
			circuitAuctionBid.WtIdParamsOut[i] = make([]frontend.Variable,auctionbidAuditorConfig.TmNumOfIdParms)
		}


		witness:=templates.TmAuctionBidAuditorCircuit{
			Config: auctionbidAuditorConfig,
			StTreeNumbers:    			make([]frontend.Variable, auctionbidAuditorConfig.TmInputs),
			StMerkleRoots:    			make([]frontend.Variable, auctionbidAuditorConfig.TmInputs),
			StNullifiers:  				make([]frontend.Variable, auctionbidAuditorConfig.TmInputs),
			StCommitmentsOuts:  		make([]frontend.Variable, auctionbidAuditorConfig.TmOutputs),

			StAuctioneeEncryptedValues: make([]frontend.Variable, auctioneerEncLength),
			StAuditorEncryptedValues: 	make([]frontend.Variable, encLength),

			WtPathElements: 			make([][]frontend.Variable, auctionbidAuditorConfig.TmInputs),

			WtPrivateKeysIn:			make([]frontend.Variable, auctionbidAuditorConfig.TmInputs),

			WtPathIndices:				make([]frontend.Variable, auctionbidAuditorConfig.TmInputs),
			WtPublicKeysOut:			make([]frontend.Variable, auctionbidAuditorConfig.TmOutputs),
			WtAssetGroupPathElements:	make([]frontend.Variable, auctionbidAuditorConfig.TmMerkleTreeDepth),


			WtIdParamsIn:    			make([][]frontend.Variable, auctionbidAuditorConfig.TmInputs),
			WtIdParamsOut:   			make([][]frontend.Variable, auctionbidAuditorConfig.TmOutputs),
		}
		for i := range witness.WtPathElements {

			witness.WtPathElements[i] = make([]frontend.Variable, auctionbidAuditorConfig.TmMerkleTreeDepth)
		}

		for i := range witness.WtIdParamsIn {

			witness.WtIdParamsIn[i] = make([]frontend.Variable,auctionbidAuditorConfig.TmNumOfIdParms)
			witness.WtIdParamsOut[i] = make([]frontend.Variable,auctionbidAuditorConfig.TmNumOfIdParms)
		}

		witness.StBeacon = frontend.Variable(request.StBeacon)
		witness.StAuctionId = frontend.Variable(request.StAuctionId)
		witness.StBlindedBid = frontend.Variable(request.StBlindedBid)
		witness.StVaultId = frontend.Variable(request.StVaultId)
		witness.StAssetGroupTreeNumber = frontend.Variable(request.StAssetGroupTreeNumber)
		witness.StAssetGrupoMerkleRoot = frontend.Variable(request.StAssetGrupoMerkleRoot)
		witness.StAuditorNonce = frontend.Variable(request.StAuditorNonce)
		witness.WtAuditoRandom = frontend.Variable(request.WtAuditoRandom)

		witness.WtBidAmount = frontend.Variable(request.WtBidAmount)
		witness.WtBidRandom = frontend.Variable(request.WtBidRandom)

		witness.WtContractAddress = frontend.Variable(request.WtContractAddress)
		witness.WtAssetGroupPathIndices = frontend.Variable(request.WtAssetGroupPathIndices)

		for i:=0; i < auctionbidAuditorConfig.TmInputs;i++{
			witness.StTreeNumbers[i] = frontend.Variable(request.StTreeNumbers[i])
			witness.StMerkleRoots[i] = frontend.Variable(request.StMerkleRoots[i])
			witness.StNullifiers[i] = frontend.Variable(request.StNullifiers[i])
			

			witness.WtPrivateKeysIn[i] = frontend.Variable(request.WtPrivateKeysIn[i])
			witness.WtPathIndices[i] = frontend.Variable(request.WtPathIndices[i])

			for j:=0; j<auctionbidAuditorConfig.TmMerkleTreeDepth; j++{
				witness.WtPathElements[i][j] = frontend.Variable(request.WtPathElements[i][j])
			}
			for j:=0; j<auctionbidAuditorConfig.TmNumOfIdParms; j++{
				witness.WtIdParamsIn[i][j] = frontend.Variable(request.WtIdParamsIn[i][j])
			}
		}

		for i:=0; i<auctionbidAuditorConfig.TmOutputs;i++{
			witness.StCommitmentsOuts[i] = frontend.Variable(request.StCommitmentsOuts[i])
			witness.WtPublicKeysOut[i] = frontend.Variable(request.WtPublicKeysOut[i])
			for j:=0; j<auctionbidAuditorConfig.TmNumOfIdParms; j++{
				witness.WtIdParamsOut[i][j] = frontend.Variable(request.WtIdParamsOut[i][j])
			}

		}

		for i:=0; i<auctionbidAuditorConfig.TmMerkleTreeDepth;i++{
			witness.WtAssetGroupPathElements[i] = frontend.Variable(request.WtAssetGroupPathElements[i])
		}

		// Auctioneer fields
		witness.StAuctioneerNonce = frontend.Variable(request.StAuctioneerNonce)
		witness.WtAuctioneerRandom = frontend.Variable(request.WtAuctioneerRandom)

		for i:=0; i<2;i++{
			witness.StAuctioneerPublicKey[i] = frontend.Variable(request.StAuctioneerPublicKey[i])
			witness.StAuctioneerAuthKey[i] = frontend.Variable(request.StAuctioneerAuthKey[i])
		}

		for i:=0; i<auctioneerEncLength;i++{
			witness.StAuctioneeEncryptedValues[i] = frontend.Variable(request.StAuctioneerEncryptedValues[i])
		}

		// Auditor fields
		witness.StAuditorNonce = frontend.Variable(request.StAuditorNonce)
		witness.WtAuditoRandom = frontend.Variable(request.WtAuditoRandom)

		for i:=0; i<2;i++{
			witness.StAuditorPublicKey[i] = frontend.Variable(request.StAuditorPublicKey[i])
			witness.StAuditorAuthKey[i] = frontend.Variable(request.StAuditorAuthKey[i])
		}

		for i:=0; i<encLength;i++{
			witness.StAuditorEncryptedValues[i] = frontend.Variable(request.StAuditorEncryptedValues[i])
		}


		witness.Config = auctionbidAuditorConfig

		solver.RegisterHint(primitives.ModHint)
		solver.RegisterHint(primitives.ERC155UniqueIdNative)
		solver.RegisterHint(primitives.PoseidonNative)

		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitAuctionBid)

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


		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StBeacon))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAuctionId))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StBlindedBid))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StVaultId))
		
		for i:=0; i < auctionbidAuditorConfig.TmInputs;i++{
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StTreeNumbers[i]))
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StMerkleRoots[i]))
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StNullifiers[i]))	
		}
		for i:=0; i < auctionbidAuditorConfig.TmOutputs;i++{
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StCommitmentsOuts[i]))
		}

		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAssetGroupTreeNumber))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAssetGrupoMerkleRoot))

		// Auctioneer public signals
		for i:=0; i < 2;i++{
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAuctioneerPublicKey[i]))
		}
		for i:=0; i < 2;i++{
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAuctioneerAuthKey[i]))
		}
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAuctioneerNonce))
		for i:=0; i < auctioneerEncLength;i++{
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAuctioneerEncryptedValues[i]))
		}

		// Auditor public signals
		for i:=0; i < 2;i++{
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAuditorPublicKey[i]))
		}
		for i:=0; i < 2;i++{
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAuditorAuthKey[i]))
		}
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAuditorNonce))
		for i:=0; i < encLength;i++{
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAuditorEncryptedValues[i]))
		}

		c.JSON(http.StatusOK, AuctionBidAuditorOutput{
            Proof:  proofRemix,
            PublicSignal:publicSignal,
        })


	
	}
}