package main

import (
	"net/http"
	"github.com/gin-gonic/gin"

	"fmt"
	"log"
	"math/big"

	pos "enygma-server/poseidon"
	utils "enygma-server/utils"
	

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	cmp "github.com/consensys/gnark/std/math/cmp"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/std/algebra/native/twistededwards"
	groth16_bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	 

)

// Request payload structure
type EnygmaRequest struct {
	SenderID      string     `json:"senderId" binding:"required"`
	Secrets       [6]string  `json:"secrets" binding:"required,len=6"`
	PublicKey     [6][2]string `json:"publicKey" binding:"required,len=6,dive,len=2"`
	Sk            string     `json:"sk" binding:"required"`
	PreviousV     string     `json:"previousV" binding:"required"`
	PreviousR     string     `json:"previousR" binding:"required"`
	PreviousCommit [6][2]string `json:"previousCommit" binding:"required,len=6,dive,len=2"`
	TxCommit      [6][2]string `json:"txCommit" binding:"required,len=6,dive,len=2"`
	TxValue       [6]string  `json:"txValue" binding:"required,len=6"`
	TxRandom      [6]string  `json:"txRandom" binding:"required,len=6"`
	V             string     `json:"v" binding:"required"`
	Nullifier     string     `json:"nullifier" binding:"required"`
	BlockNumber   string     `json:"blockNumber" binding:"required"`
	KIndex        [6]string  `json:"kIndex" binding:"required,len=6"`
}

const nCommitments = 6;
type EnygmaCircuit struct {
	SenderId      	frontend.Variable 
	Secrets       	[nCommitments]frontend.Variable 
	PublicKey   	[nCommitments][2]frontend.Variable  `gnark:",public"` 
	Sk 				frontend.Variable
	PreviousV  		frontend.Variable 
	PreviousR   	frontend.Variable 
	PreviousCommit  [nCommitments][2]frontend.Variable `gnark:",public"` 
	TxCommit 		[nCommitments][2]frontend.Variable 
	TxValue 		[nCommitments]frontend.Variable
	TxRandom 		[nCommitments]frontend.Variable 
	V				frontend.Variable 
	Nullifier       frontend.Variable 
	BlockNumber     frontend.Variable `gnark:",public"` 
	KIndex 		    [nCommitments]frontend.Variable  `gnark:",public"` 

}



func (circuit *EnygmaCircuit) Define(api frontend.API) error {	

	//Check if SenderId is in K
	sumIsInK :=frontend.Variable(0)
	for i:=0; i< nCommitments;i++{
		isEqual := api.IsZero(api.Sub(circuit.KIndex[i], circuit.SenderId))  // KIndex  = Senderid? IsEQqual =1 /0
		sumIsInK = api.Add(isEqual,sumIsInK )
	}

	api.AssertIsEqual(sumIsInK,1)

	PDiff:= frontend.Variable("2736030358979909402780800718157159386076813972158567259200215660948447373041")

	selected_v :=frontend.Variable(0)
	//Check if  getNegative(V) == TxValue[Sender_ID]
	for i:=0; i< nCommitments;i++{
		diff := api.Sub(circuit.SenderId, i)
		eq := api.IsZero(diff)

		selected_v = api.Add(selected_v, api.Mul(eq, circuit.TxValue[i]))
	}
	api.AssertIsEqual(selected_v, api.Sub(PDiff,circuit.V))

	//Check if Secret of SenderId is zero
	for i := 0; i < nCommitments; i++ {
		isEqual := api.IsZero(api.Sub(circuit.KIndex[i], circuit.SenderId))
		api.AssertIsEqual(api.Mul(isEqual, circuit.Secrets[i]), 0)
	}
	
	///////////////////////////////////**///////////////////////////////////
	//Check if PublicKey is on Curve
	for i := 0; i < nCommitments; i++ {
		X := circuit.PublicKey[i][0]
		Y := circuit.PublicKey[i][1]

		utils.AssertPointsIsOnCurve(api,X,Y)
	}

	///////////////////////////////////**///////////////////////////////////
	// Knowledge of Sk
	selectedPK0 := frontend.Variable(0)
	selectedPK1 := frontend.Variable(0)

	for i:=0; i< nCommitments; i++{
		diff := api.Sub(circuit.SenderId, i)
		eq := api.IsZero(diff)

		selectedPK0 = api.Add(selectedPK0, api.Mul(eq, circuit.PublicKey[i][0]))
		selectedPK1 = api.Add(selectedPK1, api.Mul(eq, circuit.PublicKey[i][1]))
	}
	
	// Perform scalar multiplication using custom logic
	pk := utils.ScalarMul(api, utils.G , circuit.Sk)             // v * G

	// Assert equality with the provided commitment

	api.AssertIsEqual(selectedPK0, pk.X)
	api.AssertIsEqual(selectedPK1, pk.Y)


	///////////////////////////////////**///////////////////////////////////
	//Knowledge of Previous Commitment
	selectedPreviousCommitmentX := frontend.Variable(0)
	selectedPreviousCommitmentY := frontend.Variable(0)

	for i:=0; i< nCommitments; i++{
		diff := api.Sub(circuit.SenderId, i)
		eq := api.IsZero(diff)

		selectedPreviousCommitmentX = api.Add(selectedPreviousCommitmentX, api.Mul(eq, circuit.PreviousCommit[i][0]))
		selectedPreviousCommitmentY = api.Add(selectedPreviousCommitmentY, api.Mul(eq, circuit.PreviousCommit[i][1]))
	}
	 
	computedPreviousCommitment := utils.PedersenCommitment(api, circuit.PreviousV, circuit.PreviousR)

	api.AssertIsEqual(selectedPreviousCommitmentX, computedPreviousCommitment.X)
	api.AssertIsEqual(selectedPreviousCommitmentY, computedPreviousCommitment.Y)

	// ///////////////////////////////////**///////////////////////////////////
	// SumCommitment  = [0,1]
	
	sumX :=frontend.Variable(0)
	sumY :=frontend.Variable(0)

	for i := 0; i < nCommitments; i++ {
		isEqual := api.IsZero(api.Sub(circuit.KIndex[i], circuit.SenderId))
		r := api.Sub(PDiff,circuit.TxRandom[i])
		random :=  api.Add( api.Mul(api.Sub(1,isEqual),r), api.Mul(isEqual, circuit.TxRandom[i]))

		sumX =  api.Add(sumX, circuit.TxValue[i])
		sumY =  api.Add(sumY, random)

	}  
	PedersenZero :=utils.PedersenCommitment(api, sumX, sumY)
	
	api.AssertIsEqual(PedersenZero.X,  frontend.Variable(0))
	api.AssertIsEqual(PedersenZero.Y,  frontend.Variable(1))


	// part 2 sum of pedersen
	sum := twistededwards.Point{
        X: circuit.TxCommit[0][0],
        Y: circuit.TxCommit[0][1],
    }

	for i := 1; i < nCommitments; i++ {
		point := twistededwards.Point{
			X: circuit.TxCommit[i][0],
			Y: circuit.TxCommit[i][1],
		}
		sum = utils.PointAdd(api, sum, point)
	}

	api.AssertIsEqual(sum.X,frontend.Variable(0))
	api.AssertIsEqual(sum.Y, frontend.Variable(1))


	///////////////////////////////////**///////////////////////////////////
	// Range Proof  Previous V > V & V>0

	api.AssertIsEqual(api.Cmp(circuit.PreviousV, circuit.V), frontend.Variable(1))   // Previous V > V
	api.AssertIsEqual(api.Cmp(circuit.V, frontend.Variable(0)),frontend.Variable(1)) // V>0


	///////////////////////////////////**//////////////////////////////////////
	// Knoweldge of Nullifier

	computedNullifier := pos.Poseidon(api, []frontend.Variable{circuit.Sk, circuit.BlockNumber})
	api.AssertIsEqual(computedNullifier, circuit.Nullifier)

	///////////////////////////////////**//////////////////////////////////////
	//Check if Pedersen Commitment is well formed
	for i := 0; i < nCommitments; i++ {

		isEqual := api.IsZero(api.Sub(circuit.KIndex[i], circuit.SenderId))// If sender Id isEqual =1
		
		r := api.Sub(PDiff,circuit.TxRandom[i])
		random :=  api.Add( api.Mul(api.Sub(1,isEqual),r), api.Mul(isEqual, circuit.TxRandom[i]))
		
		computedPedersenCommitment := utils.PedersenCommitment(api, circuit.TxValue[i], random)     
	
		api.AssertIsEqual(circuit.TxCommit[i][0], computedPedersenCommitment.X)
		api.AssertIsEqual(circuit.TxCommit[i][1], computedPedersenCommitment.Y)
	}

	// ///////////////////////////////////**//////////////////////////////////////
	// CHeck if random factor are well formed
	var calculatedRandomFactor [nCommitments]frontend.Variable
	
	sumFactor:= frontend.Variable(0)
	for i := 0; i < nCommitments; i++ {
		isEqual := api.IsZero(api.Sub(circuit.KIndex[i], circuit.SenderId))
		RandomFactor := pos.Poseidon(api, []frontend.Variable{circuit.Secrets[i], circuit.BlockNumber})
		
		randomInter,_ := api.NewHint(utils.ModHint, 2,RandomFactor)
		r := randomInter[0] // remaninder
		q := randomInter[1] // quotient

		api.AssertIsEqual(api.Add(api.Mul(q, PDiff),r,),RandomFactor)

		isValid := cmp.IsLess(api, r, PDiff)
		api.AssertIsEqual(isValid, 1)
		
		calculatedRandomFactor[i] = r
		
		sumFactor = api.Add(api.Mul(api.Sub(1,isEqual), r),sumFactor)
		sumInter,_ := api.NewHint(utils.ModHint, 2,sumFactor)

		
		sumQ := sumInter[1] //quotient
		sumR := sumInter[0] //remainder

		//Enforce api New Hint
		api.AssertIsEqual(api.Add(api.Mul(sumQ, PDiff),sumR,),sumFactor)
		isSumValid := cmp.IsLess(api, sumR, PDiff)
		api.AssertIsEqual(isSumValid, 1)
		sumFactor = sumR
	}

	for i := 0; i < nCommitments; i++ {
		// Check if the current index is equal to circuit.SenderId
		isSender := api.IsZero(api.Sub(circuit.SenderId, frontend.Variable(i)))
		// If isSender is true, select sumFactor; otherwise, keep the existing value
		calculatedRandomFactor[i] = api.Select(isSender, sumFactor, calculatedRandomFactor[i])		
	}
	
	
	for i := 0; i < nCommitments; i++ {
		
		diff:=api.Sub(calculatedRandomFactor[i], circuit.TxRandom[i])
		api.AssertIsEqual(diff,frontend.Variable(0))

	}

	return nil

}


func generateProof(request EnygmaRequest) ([]*big.Int){

	var circuit EnygmaCircuit
	solver.RegisterHint(utils.ModHint)
	
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	curve := ecc.BN254 
	pk, _ := utils.LoadProvingKey(curve, "proving.key")

	var witness EnygmaCircuit
	witness.SenderId = frontend.Variable(request.SenderID)
	witness.V = frontend.Variable(request.V)
	witness.Sk = frontend.Variable(request.Sk)


	for i := 0; i < nCommitments; i++ { 
		witness.Secrets[i] = utils.ParseBigInt(request.Secrets[i])

		witness.PublicKey[i][0] =  utils.ParseBigInt(request.PublicKey[i][0])
        witness.PublicKey[i][1] =  utils.ParseBigInt(request.PublicKey[i][1])

		witness.PreviousCommit[i][0] = utils.ParseBigInt(request.PreviousCommit[i][0])
        witness.PreviousCommit[i][1] = utils.ParseBigInt(request.PreviousCommit[i][1])

		witness.TxCommit[i][0] = utils.ParseBigInt(request.TxCommit[i][0])
        witness.TxCommit[i][1] = utils.ParseBigInt(request.TxCommit[i][1])
        witness.TxValue[i] = utils.ParseBigInt(request.TxValue[i])
        witness.TxRandom[i] = utils.ParseBigInt(request.TxRandom[i])
		witness.KIndex[i] = utils.ParseBigInt(request.KIndex[i])
	}

	witness.PreviousV = utils.ParseBigInt(request.PreviousV)
	witness.PreviousR =  utils.ParseBigInt(request.PreviousR)
	witness.Nullifier =  utils.ParseBigInt(request.Nullifier)
	witness.BlockNumber = frontend.Variable(request.BlockNumber)
	
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

	
	
	return proofRemix
	
}

func main() {
	// Create a new Gin router
	router := gin.Default()

	// Define POST endpoint
	router.POST("/generateProof", func(c *gin.Context) {
		// Parse request body
		var request EnygmaRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Process the message (example: reverse the text)
		fmt.Println("request",request)
		proof:=generateProof(request)

		// Return response
		c.JSON(http.StatusOK, gin.H{
			"message": "Proof generated successfully",
			"proof":proof,
		})
	})

	// Start server
	router.Run(":8080")
}

