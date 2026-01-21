package acutionBid

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
		var request AuctionBidRequest

		if err := c.ShouldBindJSON(&request); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        } 
		fmt.Println(request)
	
		var publicSignal []*big.Int
		
		auctionbidConfig := templates.AuctionBidCircuitConfig{
			TmNInputs:    2,
			TmMOutputs:   2,
			TmNumOfIdParams: 3,
			TmDepthMerkle: 8,
			TmRange:     frontend.Variable("1000000000000000000000000000000000000"),
			TmGroupMerkleTreeDepth: 8,
		}
		circuitAuctionBid :=templates.AuctionBidCircuit{
			Config: auctionbidConfig,
			StTreeNumber:    			make([]frontend.Variable, auctionbidConfig.TmNInputs),
			StMerkleRoot:    			make([]frontend.Variable, auctionbidConfig.TmNInputs),
			StNullifier:  				make([]frontend.Variable, auctionbidConfig.TmNInputs),
			StCommitmentsOuts:  		make([]frontend.Variable, auctionbidConfig.TmNInputs),
			
			WtPrivateKeysIn: 				make([]frontend.Variable, auctionbidConfig.TmNInputs),
			
			WtPathElements: 			make([][]frontend.Variable, auctionbidConfig.TmNInputs),

			WtPathIndices: 				make([]frontend.Variable, auctionbidConfig.TmNInputs),
			WtPublicKeysOut:			make([]frontend.Variable, auctionbidConfig.TmMOutputs),
			WtAssetGroupPathElements:	make([]frontend.Variable, auctionbidConfig.TmDepthMerkle),
			
			WtIdParamsIn:    			make([][]frontend.Variable, auctionbidConfig.TmNInputs),
			WtIdParamsOut:   			make([][]frontend.Variable, auctionbidConfig.TmMOutputs),
		}
		for i := range circuitAuctionBid.WtPathElements {

			circuitAuctionBid.WtPathElements[i] = make([]frontend.Variable, auctionbidConfig.TmDepthMerkle)
		}

		for i := range circuitAuctionBid.WtIdParamsIn {

			circuitAuctionBid.WtIdParamsIn[i] = make([]frontend.Variable,5)
			circuitAuctionBid.WtIdParamsOut[i] = make([]frontend.Variable,5)
		}


		witness:=templates.AuctionBidCircuit{
			Config: auctionbidConfig,
			StTreeNumber:    			make([]frontend.Variable, auctionbidConfig.TmNInputs),
			StMerkleRoot:    			make([]frontend.Variable, auctionbidConfig.TmNInputs),
			StNullifier:  				make([]frontend.Variable, auctionbidConfig.TmNInputs),
			StCommitmentsOuts:  		make([]frontend.Variable, auctionbidConfig.TmNInputs),
			WtPrivateKeysIn: 			make([]frontend.Variable, auctionbidConfig.TmNInputs),

			WtPathElements: 			make([][]frontend.Variable, auctionbidConfig.TmNInputs),
			WtPathIndices: 				make([]frontend.Variable, auctionbidConfig.TmNInputs),
			WtPublicKeysOut:			make([]frontend.Variable, auctionbidConfig.TmMOutputs),

			WtAssetGroupPathElements:	make([]frontend.Variable, auctionbidConfig.TmDepthMerkle),
			WtIdParamsIn:    			make([][]frontend.Variable, auctionbidConfig.TmNInputs),
			WtIdParamsOut:   			make([][]frontend.Variable, auctionbidConfig.TmMOutputs),
		}
		
		for i := range witness.WtPathElements {

			witness.WtPathElements[i] = make([]frontend.Variable, auctionbidConfig.TmDepthMerkle)

		}
		for i := range witness.WtIdParamsIn {

			witness.WtIdParamsIn[i] = make([]frontend.Variable,5)
			witness.WtIdParamsOut[i] = make([]frontend.Variable,5)
		}

		
		witness.StAuctionId = frontend.Variable(request.StAuctionId)
		witness.StBlindedBid = frontend.Variable(request.StBlindedBid)
		witness.StVaultId = frontend.Variable(request.StVaultId)
		witness.StAssetGroupMerkleRoot = frontend.Variable(request.StAssetGroupMerkleRoot)
		witness.WtBidAmount = frontend.Variable(request.WtBidAmount)

		witness.WtBidRandom = frontend.Variable(request.WtBidRandom)

		witness.WtContractAddress = frontend.Variable(request.WtContractAddress)

		witness.WtAssetGroupPathIndices = frontend.Variable(request.WtAssetGroupPathIndices)

		for i:=0; i < auctionbidConfig.TmNInputs;i++{
			witness.StTreeNumber[i] = frontend.Variable(request.StTreeNumber[i])
			witness.StMerkleRoot[i] = frontend.Variable(request.StMerkleRoot[i])
			witness.StNullifier[i] = frontend.Variable(request.StNullifier[i])
			witness.StCommitmentsOuts[i] = frontend.Variable(request.StCommitmentsOuts[i])

			witness.WtPrivateKeysIn[i] = frontend.Variable(request.WtPrivateKeysIn[i])
			witness.WtPathIndices[i] = frontend.Variable(request.WtPathIndices[i])

			for j:=0; j<auctionbidConfig.TmDepthMerkle; j++{
				witness.WtPathElements[i][j] = frontend.Variable(request.WtPathElements[i][j])
			}
			for j:=0; j<5; j++{
				witness.WtIdParamsIn[i][j] = frontend.Variable(request.WtIdParamsIn[i][j])
			}
		}

		for i:=0; i<auctionbidConfig.TmMOutputs;i++{
			witness.WtPublicKeysOut[i] = frontend.Variable(request.WtPublicKeysOut[i])
			for j:=0; j<5; j++{
				witness.WtIdParamsOut[i][j] = frontend.Variable(request.WtIdParamsOut[i][j])
			}

		}

		for i:=0; i<auctionbidConfig.TmDepthMerkle;i++{
			witness.WtAssetGroupPathElements[i] = frontend.Variable(request.WtAssetGroupPathElements[i])
		}

		witness.Config = auctionbidConfig

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

		// publicSignal =  append(publicSignal, utils.ParseBigInt(request.StBeacon))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAuctionId))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StBlindedBid))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StVaultId))
		

		
		for i:=0; i < auctionbidConfig.TmNInputs;i++{
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StTreeNumber[i]))
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StMerkleRoot[i]))
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StNullifier[i]))
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StMerkleRoot[i]))
		}
		for i:=0; i < auctionbidConfig.TmMOutputs;i++{
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StCommitmentsOuts[i]))

		}

		// publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAssetGroupTreeNumber))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAssetGroupMerkleRoot))

		c.JSON(http.StatusOK, AuctionBidOutput{
            Proof:  proofRemix,
            PublicSignal:publicSignal,
        })


	
	}
}