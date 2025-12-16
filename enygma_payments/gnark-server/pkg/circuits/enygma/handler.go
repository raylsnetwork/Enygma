package enygma

import (
	"log"
	"math/big"
    "net/http"
	"strconv"
	"fmt"
	utils "enygma-server/utils"

    "github.com/gin-gonic/gin"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark-crypto/ecc"
    "github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/constraint/solver"
    "github.com/consensys/gnark/backend/groth16"
	groth16_bn254 "github.com/consensys/gnark/backend/groth16/bn254"
 
)

func createCircuitTemplate(config EnygmaCircuitConfig) EnygmaCircuit {
    circuit :=EnygmaCircuit{
        Config:          config,
        ArrayHashSecret: make([][]frontend.Variable, config.NCommitment),
        PublicKey:       make([]frontend.Variable, config.NCommitment),
        PreviousCommit:  make([][2]frontend.Variable, config.NCommitment),
        KIndex:          make([]frontend.Variable, config.NCommitment),
        Secrets:         make([][]frontend.Variable, config.NCommitment),
        TagMessage:      make([]frontend.Variable, config.NCommitment),
        TxCommit:        make([][2]frontend.Variable, config.NCommitment),
        TxValue:         make([]frontend.Variable, config.NCommitment),
        TxRandom:        make([]frontend.Variable, config.NCommitment),
    }
    
    for i := 0; i < config.NCommitment; i++ {
        circuit.ArrayHashSecret[i] = make([]frontend.Variable, config.NCommitment)
        circuit.Secrets[i] = make([]frontend.Variable, config.NCommitment)
    }
    
    return circuit
}



func NewHandler(pkPath, vkPath string) gin.HandlerFunc {

	curve := ecc.BN254 
	pk, _ := utils.LoadProvingKey(curve, pkPath)

	return func(c *gin.Context) {
        var request EnygmaRequest
        if err := c.ShouldBindJSON(&request); err != nil {
			fmt.Println(request)
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        } 
		config:= EnygmaCircuitConfig{
			BitWith:256,
			NCommitment:6,
		}
		witness := createCircuitTemplate(config)
        circuit := createCircuitTemplate(config)
	
		var publicSignal []*big.Int
		solver.RegisterHint(utils.ModHint)
			 
		witness.SenderId,_ = strconv.Atoi(request.SenderId)
		witness.V = frontend.Variable(request.V)
		witness.Sk = frontend.Variable(request.Sk)

		
		for i := 0; i < config.NCommitment; i++ { 
			for j:=0;j< config.NCommitment; j++{
				witness.Secrets[i][j] = utils.ParseBigInt(request.Secrets[i][j])
				witness.ArrayHashSecret[i][j] = utils.ParseBigInt(request.ArrayHashSecret[i][j])
			}

			witness.PublicKey[i] =  utils.ParseBigInt(request.PublicKey[i])
			

			witness.PreviousCommit[i][0] = utils.ParseBigInt(request.PreviousCommit[i][0])
			witness.PreviousCommit[i][1] = utils.ParseBigInt(request.PreviousCommit[i][1])

			witness.TxCommit[i][0] = utils.ParseBigInt(request.TxCommit[i][0])
			witness.TxCommit[i][1] = utils.ParseBigInt(request.TxCommit[i][1])
			witness.TxValue[i] = utils.ParseBigInt(request.TxValue[i])
			witness.TxRandom[i] = utils.ParseBigInt(request.TxRandom[i])
			witness.KIndex[i] = utils.ParseBigInt(request.KIndex[i])
			witness.TagMessage[i] = utils.ParseBigInt(request.TagMessage[i])
			
			
		}

		witness.PreviousV = utils.ParseBigInt(request.PreviousV)
		witness.PreviousR =  utils.ParseBigInt(request.PreviousR)
		witness.Nullifier =  utils.ParseBigInt(request.Nullifier)
		witness.BlockNumber = frontend.Variable(request.BlockNumber)

		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)

		witnessFull, err := frontend.NewWitness(&witness, ecc.BN254.ScalarField())
		if err != nil {
			log.Fatal(err)
		}
		proof, err := groth16.Prove(ccs, pk, witnessFull)

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
		for i := 0; i < config.NCommitment; i++ { 
			for j := 0; j < config.NCommitment; j++ {
				publicSignal =  append(publicSignal,  utils.ParseBigInt(request.ArrayHashSecret[i][j]))
			}
		}
		for i := 0; i < config.NCommitment; i++ { 
			publicSignal =  append(publicSignal,  utils.ParseBigInt(request.PublicKey[i]))
		}
		for i := 0; i < config.NCommitment; i++ { 
			publicSignal =  append(publicSignal,  utils.ParseBigInt(request.PreviousCommit[i][0]))
			publicSignal =  append(publicSignal,  utils.ParseBigInt(request.PreviousCommit[i][1]))
		}

	
		publicSignal =  append(publicSignal,  utils.ParseBigInt(request.BlockNumber))
		for i := 0; i < config.NCommitment; i++ { 
			publicSignal =  append(publicSignal, utils.ParseBigInt(request.KIndex[i]))
			
		}
		publicSignal =  append(publicSignal,  utils.ParseBigInt(request.Nullifier))
		fmt.Println(len(publicSignal))
		c.JSON(http.StatusOK, EnygmaOutput{
            Proof:  proofRemix,
            PublicSignal:publicSignal,
        })


	}
}	