package deposit

import (
	"log"
	// "fmt"
	"math/big"
    "net/http"

	utils "enygma-server/utils"

    "github.com/gin-gonic/gin"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark-crypto/ecc"
    "github.com/consensys/gnark/frontend/cs/r1cs"
	
    "github.com/consensys/gnark/backend/groth16"
	groth16_bn254 "github.com/consensys/gnark/backend/groth16/bn254"
 
)

func NewHandler(pkPath, vkPath string) gin.HandlerFunc {

	curve := ecc.BN254 
	pk, _ := utils.LoadProvingKey(curve, pkPath)
	
	return func(c *gin.Context) {
        var request DepositRequest
		
        if err := c.ShouldBindJSON(&request); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        } 
		
		var witness DepositEnygmaCircuit
		var publicSignal []*big.Int
	
		witness.SenderId = frontend.Variable(request.SenderID)
		witness.Address = frontend.Variable(request.Address)

		witness.Hash = frontend.Variable(request.Hash)
		
		witness.VInit = frontend.Variable(request.VInit)
		witness.VDeposit = frontend.Variable(request.VDeposit)
		
		witness.Pk = frontend.Variable(request.Pk)
		witness.Secret = frontend.Variable(request.Secret)

		for i := 0; i < nCommitments; i++ { 
			witness.TxCommit[i][0] = utils.ParseBigInt(request.TxCommit[i][0])
			witness.TxCommit[i][1] = utils.ParseBigInt(request.TxCommit[i][1])
			witness.TxRandom[i]    = utils.ParseBigInt(request.TxRandom[i])
	
		}

		var circuit DepositEnygmaCircuit
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
		
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.Address))
		publicSignal =  append(publicSignal, utils.ParseBigInt(request.Pk))


		
		c.JSON(http.StatusOK, DepositOutput{
            Proof:  proofRemix,
            PublicSignal:publicSignal,
        })


	}
}	