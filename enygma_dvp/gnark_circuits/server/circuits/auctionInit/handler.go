package auctionInit

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
        var request AuctionInitRequest
		
        if err := c.ShouldBindJSON(&request); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        } 

		fmt.Println(request)
	
		var publicSignal []*big.Int

		auctionInitConfig := templates.AuctionInitCircuitConfig{
			TmNumOfIdParms:5,
			TmMerkleTreeDepth:8,
			TmGroupMerkleTreeDepth:8,

		}

		circuitAuctionInit :=templates.AuctionInitCircuit{
			Config: auctionInitConfig,
			WtIdParams:					make([]frontend.Variable, auctionInitConfig.TmNumOfIdParms),
			WtPathElements:    			make([]frontend.Variable, auctionInitConfig.TmMerkleTreeDepth),
			WtAssetGroupPathElements:   make([]frontend.Variable, auctionInitConfig.TmGroupMerkleTreeDepth),
			
		}

		witness:=templates.AuctionInitCircuit{
			Config: auctionInitConfig,
			WtIdParams:					make([]frontend.Variable, auctionInitConfig.TmNumOfIdParms),
			WtPathElements:    			make([]frontend.Variable, auctionInitConfig.TmMerkleTreeDepth),
			WtAssetGroupPathElements:   make([]frontend.Variable, auctionInitConfig.TmGroupMerkleTreeDepth),
		}


		witness.StBeacon  = frontend.Variable(request.StBeacon)
		witness.StVaultId  = frontend.Variable(request.StVaultId)
		witness.StAuctionId  = frontend.Variable(request.StAuctionId)
		witness.StTreeNumber  = frontend.Variable(request.StTreeNumber)
		witness.StMerkleRoot  = frontend.Variable(request.StMerkleRoot)
		witness.StNullifier  = frontend.Variable(request.StNullifier)
		witness.StAssetGroupMerkleRoot  = frontend.Variable(request.StAssetGroupMerkleRoot)
		witness.StAssetGroupMerkleRoot = frontend.Variable(request.StAssetGroupMerkleRoot)

		witness.WtCommiment = frontend.Variable(request.WtCommiment)
		witness.WtPathIndices  = frontend.Variable(request.WtPathIndices)
		witness.WtPrivateKey  = frontend.Variable(request.WtPrivateKey)
		witness.WtContractAddress = frontend.Variable(request.WtContractAddress)
		witness.WtAssetGroupPathIndices = frontend.Variable(request.WtAssetGroupPathIndices)


		for i:=0; i< auctionInitConfig.TmMerkleTreeDepth; i++{
			witness.WtPathElements[i] = frontend.Variable(request.WtPathElements[i])
		}

		for i:=0; i< auctionInitConfig.TmGroupMerkleTreeDepth; i++{
			witness.WtAssetGroupPathElements[i] = frontend.Variable(request.WtAssetGroupPathElements[i])
		}
		for i:=0; i< auctionInitConfig.TmNumOfIdParms; i++{
			witness.WtIdParams[i] = frontend.Variable(request.WtIdParams[i])
		}

		
		witness.Config = auctionInitConfig

		solver.RegisterHint(primitives.ModHint)
		solver.RegisterHint(primitives.ERC155UniqueIdNative)
		solver.RegisterHint(primitives.PoseidonNative)

		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitAuctionInit)

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
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StVaultId))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAuctionId))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StTreeNumber))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StMerkleRoot))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StNullifier))
		

		c.JSON(http.StatusOK, AuctionInitOutput{
            Proof:  proofRemix,
            PublicSignal:publicSignal,
        })


	}

}