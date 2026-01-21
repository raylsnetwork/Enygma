package brokerRegistration

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
		var request BrokerRegistrationRequest

		if err := c.ShouldBindJSON(&request); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        } 
		fmt.Println(request)
	
		var publicSignal []*big.Int
		
		brokerRegistrationConfig := templates.BrokerageRegistrationConfig{
			TmNumOfInputs :		      2,
			TmMerkleTreeDepth: 	  8,
			TmGroupMerkleTreeDepth:  8,
			TmMaxPermittedCommissionRate:10,
			TmComissionRateDecimals:2,
			TmRange: "1000000000000000000000000000000000000",
		}

		circuitBrokerRegistration:=templates.BrokerageRegistrationCircuit{
			Config: brokerRegistrationConfig,
			StDelegatorTreeNumbers: make([]frontend.Variable, brokerRegistrationConfig.TmNumOfInputs),
			StDelegatorMerkleRoots: make([]frontend.Variable, brokerRegistrationConfig.TmNumOfInputs),
			StDelegatorNullifier:   make([]frontend.Variable, brokerRegistrationConfig.TmNumOfInputs),
			WtDelegatorPrivatekeys: make([]frontend.Variable, brokerRegistrationConfig.TmNumOfInputs),
			WtDelegatorPathElements:make([][]frontend.Variable, brokerRegistrationConfig.TmNumOfInputs),
			WtDelegatorPathIndices: make([]frontend.Variable, brokerRegistrationConfig.TmNumOfInputs),
			WtDelegatorIdParams:	make([][5]frontend.Variable, brokerRegistrationConfig.TmNumOfInputs),
			WtAssetGroupPathElements: make([]frontend.Variable, brokerRegistrationConfig.TmGroupMerkleTreeDepth),

		}

		for i := range circuitBrokerRegistration.WtDelegatorPathElements {

			circuitBrokerRegistration.WtDelegatorPathElements[i] = make([]frontend.Variable, brokerRegistrationConfig.TmMerkleTreeDepth)
		}


		witness:=templates.BrokerageRegistrationCircuit{
			Config: brokerRegistrationConfig,
			StDelegatorTreeNumbers: make([]frontend.Variable, brokerRegistrationConfig.TmNumOfInputs),
			StDelegatorMerkleRoots: make([]frontend.Variable, brokerRegistrationConfig.TmNumOfInputs),
			StDelegatorNullifier:   make([]frontend.Variable, brokerRegistrationConfig.TmNumOfInputs),
			WtDelegatorPrivatekeys: make([]frontend.Variable, brokerRegistrationConfig.TmNumOfInputs),
			WtDelegatorPathElements:make([][]frontend.Variable, brokerRegistrationConfig.TmNumOfInputs),
			WtDelegatorPathIndices: make([]frontend.Variable, brokerRegistrationConfig.TmNumOfInputs),
			WtDelegatorIdParams:	make([][5]frontend.Variable, brokerRegistrationConfig.TmNumOfInputs),
			WtAssetGroupPathElements: make([]frontend.Variable, brokerRegistrationConfig.TmGroupMerkleTreeDepth),

		}

		for i := range witness.WtDelegatorPathElements {

			witness.WtDelegatorPathElements[i] = make([]frontend.Variable, brokerRegistrationConfig.TmMerkleTreeDepth)
		}

		witness.StBeacon = frontend.Variable(request.StBeacon)
		witness.StVaultId = frontend.Variable(request.StVaultId)
		witness.StGroupId = frontend.Variable(request.StGroupId)

		witness.StBrokerBlindedPublicKey = frontend.Variable(request.StBrokerBlindedPublicKey)
		witness.StBrokerMinComissionRate = frontend.Variable(request.StBrokerMinComissionRate)
		witness.StBrokerMaxComissionRate = frontend.Variable(request.StBrokerMaxComissionRate)

		witness.StAssetGroupTreeNumber = frontend.Variable(request.StAssetGroupTreeNumber)
		witness.StAssetGroupMerkleRoot = frontend.Variable(request.StAssetGroupMerkleRoot)

		witness.WtContractAddress = frontend.Variable(request.WtContractAddress)
		witness.WtBrokerPublickey = frontend.Variable(request.WtBrokerPublickey)
		witness.WtAssetGroupPathIndices = frontend.Variable(request.WtAssetGroupPathIndices)

		for i:=0; i < brokerRegistrationConfig.TmNumOfInputs;i++{
			witness.StDelegatorTreeNumbers[i] = frontend.Variable(request.StDelegatorTreeNumbers[i])
			witness.StDelegatorMerkleRoots[i] = frontend.Variable(request.StDelegatorMerkleRoots[i])
			witness.StDelegatorNullifier[i] = frontend.Variable(request.StDelegatorNullifier[i])

			witness.WtDelegatorPrivatekeys[i] = frontend.Variable(request.WtDelegatorPrivatekeys[i])
			witness.WtDelegatorPathIndices[i] = frontend.Variable(request.WtDelegatorPathIndices[i])

	
			for j:=0; j<brokerRegistrationConfig.TmMerkleTreeDepth; j++{
				witness.WtDelegatorPathElements[i][j] = frontend.Variable(request.WtDelegatorPathElements[i][j])
			}
			for j:=0; j<5; j++{
				witness.WtDelegatorIdParams[i][j] = frontend.Variable(request.WtDelegatorIdParams[i][j])
			}
		}

		for i:=0; i<brokerRegistrationConfig.TmGroupMerkleTreeDepth;i++{
			witness.WtAssetGroupPathElements[i] = frontend.Variable(request.WtAssetGroupPathElements[i])	

		}


		witness.Config = brokerRegistrationConfig

		solver.RegisterHint(primitives.ModHint)
		solver.RegisterHint(primitives.ERC155UniqueIdNative)
		solver.RegisterHint(primitives.PoseidonNative)

		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitBrokerRegistration)

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
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StGroupId))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StBrokerBlindedPublicKey))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StBrokerMinComissionRate))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StBrokerMaxComissionRate))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAssetGroupTreeNumber))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.StAssetGroupMerkleRoot))
		
		for i:=0; i < brokerRegistrationConfig.TmNumOfInputs;i++{
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StDelegatorTreeNumbers[i]))
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StDelegatorMerkleRoots[i]))
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.StDelegatorNullifier[i]))
		}
		

		c.JSON(http.StatusOK, BrokerRegistrationOutput{
            Proof:  proofRemix,
            PublicSignal:publicSignal,
        })


	
	}
}