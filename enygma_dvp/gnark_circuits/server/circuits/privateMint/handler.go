package privateMint

import (
	"log"

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
        var request PrivateMintRequest
		
        if err := c.ShouldBindJSON(&request); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        } 
		// fmt.Println(request)
	
		var publicSignal []*big.Int

		circuitPrivateMint:=templates.PrivateMintCircuit{
		
		}
			
		witness:=templates.PrivateMintCircuit{
		
		}
	

		witness.Commitment = frontend.Variable(request.Commitment)
		witness.ContractAddress = frontend.Variable(request.ContractAddress)
		witness.TokenId = frontend.Variable(request.TokenId)
		witness.Salt = frontend.Variable(request.Salt)
		witness.Amount = frontend.Variable(request.Amount)
		witness.PublicKey = frontend.Variable(request.PublicKey)
		witness.CipherText = frontend.Variable(request.CipherText)

		solver.RegisterHint(primitives.ModHint)
		solver.RegisterHint(primitives.ERC155UniqueIdNative)
		solver.RegisterHint(primitives.PoseidonNative)

		ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuitPrivateMint)

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

		
		proofStr := make([]string, len(proofRemix))
		for i, v := range proofRemix {
			proofStr[i] = v.String() // full decimal, no exponent
		}

		publicSignal=[]*big.Int{
			utils.ParseBigInt(request.Commitment),
			utils.ParseBigInt(request.ContractAddress),
			utils.ParseBigInt(request.TokenId),
			utils.ParseBigInt(request.CipherText),
		}

		pubStr := make([]string, len(publicSignal))
		for i, v := range publicSignal {
			pubStr[i] = v.String()
		}

		
		c.JSON(http.StatusOK, PrivateMintOutput{
			Proof:        proofStr,
			PublicSignal: pubStr,
		})

	


	}
}