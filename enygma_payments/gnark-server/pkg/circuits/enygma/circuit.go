package enygma

import (
	"math/big"

	pos "enygma-server/poseidon"
	utils "enygma-server/utils"
	
    "github.com/consensys/gnark/frontend"
	cmp "github.com/consensys/gnark/std/math/cmp"
	"github.com/consensys/gnark/std/algebra/native/twistededwards"
)


const nCommitments = 6;
const bitWidth = 256

type EnygmaCircuit struct {

	ArrayHashSecret [nCommitments][nCommitments]frontend.Variable   `gnark:",public"`  // Array of hash of shared secrets 
	PublicKey   	[nCommitments]frontend.Variable  				`gnark:",public"`  // Public keys from all other PLs (public)
	PreviousCommit  [nCommitments][2]frontend.Variable  			`gnark:",public"`  // Array of previous balances (Pedersen commitments)  
	BlockNumber     frontend.Variable 								`gnark:",public"`  // Previous block_number to ensure that random factors are well-generated
	KIndex 		    [nCommitments]frontend.Variable  				`gnark:",public"`  // Array with indices of the banks that are in the tx ("k"-anonymity)

	SenderId      	frontend.Variable                               // Identifier of the sender of the tx
	Secrets       	[nCommitments][nCommitments]frontend.Variable 	// Array of shared secrets with all the other PLs
	TagMessage      [nCommitments] frontend.Variable 				// Array of tag messages to ensure unique transactions when parties transact in the same block
	Sk 				frontend.Variable								// Secret key of the sender of the tx 
	PreviousV  		frontend.Variable 								// Previous balance in the last Pedersen commitment    
	PreviousR   	frontend.Variable 								// Previous random factor in the last Pedersen commitment
	TxCommit 		[nCommitments][2]frontend.Variable 				// Array containing the commitments for this new tx
	TxValue 		[nCommitments]frontend.Variable					// Array of balances debited/credited in this transaction
	TxRandom 		[nCommitments]frontend.Variable 				// Array of random factors for the pedersen commitments in this tx
	V				frontend.Variable 								// Balance to be spent in this tx
	Nullifier       frontend.Variable 								// Nullifier to ensure transaction is not a double spend
	

}


func (circuit *EnygmaCircuit) Define(api frontend.API) error {	

	//Subgroup order
	r:= frontend.Variable("2736030358979909402780800718157159386076813972158567259200215660948447373041")

	//////////////////////////////////**///////////////////////////////////
	//Check if SenderId is in K
	sumIsInK :=frontend.Variable(0)
	for i:=0; i< nCommitments;i++{
		isEqual := api.IsZero(api.Sub(circuit.KIndex[i], circuit.SenderId))  // KIndex  = Senderid? IsEQqual =1 /0
		sumIsInK = api.Add(isEqual,sumIsInK )
	}

	api.AssertIsEqual(sumIsInK,1)

	///////////////////////////////////**///////////////////////////////////
	// Check if V is in TxValue
	selected_v :=frontend.Variable(0)
	
	for i:=0; i< nCommitments;i++{
		diff := api.Sub(circuit.SenderId, i)
		eq := api.IsZero(diff)

		selected_v = api.Add(selected_v, api.Mul(eq, circuit.TxValue[i]))
	}
	selectedVBits := api.ToBinary(selected_v, bitWidth)
	vBits := api.ToBinary(circuit.V, bitWidth)
	pDiffBits := api.ToBinary(r, bitWidth)

	selectedVConstrained := api.FromBinary(selectedVBits...)
	vConstrained := api.FromBinary(vBits...)
	pDiffConstrained := api.FromBinary(pDiffBits...)

	api.AssertIsEqual(selectedVConstrained, api.Sub(pDiffConstrained,vConstrained))
	
	///////////////////////////////////**///////////////////////////////////
	//Check knowledge of secret 


	///TODO
   

	///////////////////////////////////**///////////////////////////////////
	// Check if Hash Array of Secret is well formed

	for i := 0; i < nCommitments; i++ { // for each secret perform hash calculation and sees if is equal to correspondent Array of hash secret
		for j:=0; j< nCommitments; j++{
			calculatedHash := pos.Poseidon(api, []frontend.Variable{circuit.Secrets[i][j],circuit.Secrets[i][j]})
			api.AssertIsEqual(calculatedHash, circuit.ArrayHashSecret[i][j])
		}
	}


	///////////////////////////////////**///////////////////////////////////
	// Knowledge of Sk - Perform public key generation and check if Sk generate senderId's PublicKey
	selectedPK := frontend.Variable(0)

	for i:=0; i< nCommitments; i++{ // loop through array and selected senderId's PublicKey in selectedPK variable
		
		diff := api.Sub(circuit.SenderId, i)
		eq := api.IsZero(diff)
		selectedPK = api.Add(selectedPK, api.Mul(eq, circuit.PublicKey[i]))
		
	}	
	pk := pos.Poseidon(api, []frontend.Variable{circuit.Sk, circuit.Sk}) // Pk = PoseidonHash (sk , sk)

	api.AssertIsEqual(selectedPK, pk) // check if calculated Publickey is equal to  selectedPK

	///////////////////////////////////**///////////////////////////////////
	//Knowledge of Previous Commitment
	selectedPreviousCommitmentX := frontend.Variable(0)
	selectedPreviousCommitmentY := frontend.Variable(0)

	for i:=0; i< nCommitments; i++{ //Store in selectedPreviousCommitmentX and selectedPreviousCommitmentX if 
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
		rNegate := api.Sub(r,circuit.TxRandom[i])
		random :=  api.Add( api.Mul(api.Sub(1,isEqual),rNegate), api.Mul(isEqual, circuit.TxRandom[i]))

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

	previousVBits := api.ToBinary(circuit.PreviousV, bitWidth)
	previousVConstrained := api.FromBinary(previousVBits...)
	

	api.AssertIsEqual(api.Cmp(previousVConstrained, vConstrained), frontend.Variable(1))   // Previous V > V
	api.AssertIsEqual(api.Cmp(vConstrained, frontend.Variable(0)), frontend.Variable(1))   // V > 0


	///////////////////////////////////**//////////////////////////////////////
	// Knoweldge of Nullifier

	computedNullifier := pos.Poseidon(api, []frontend.Variable{circuit.Sk, circuit.BlockNumber})
	api.AssertIsEqual(computedNullifier, circuit.Nullifier)

	///////////////////////////////////**//////////////////////////////////////
	//Check if Pedersen Commitment is well formed
	for i := 0; i < nCommitments; i++ {

		isEqual := api.IsZero(api.Sub(circuit.KIndex[i], circuit.SenderId))// If sender Id isEqual =1
		
		rNegate := api.Sub(r,circuit.TxRandom[i])
		random :=  api.Add( api.Mul(api.Sub(1,isEqual),rNegate), api.Mul(isEqual, circuit.TxRandom[i]))
		
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
		remainder := randomInter[0] // remaninder
		q := randomInter[1] // quotient

		api.AssertIsEqual(api.Add(api.Mul(q, r),remainder,),RandomFactor)

		isValid := cmp.IsLess(api, remainder, r)
		api.AssertIsEqual(isValid, 1)
		
		calculatedRandomFactor[i] = remainder
		
		sumFactor = api.Add(api.Mul(api.Sub(1,isEqual), remainder),sumFactor)
		sumInter,_ := api.NewHint(utils.ModHint, 2,sumFactor)

		
		sumQ := sumInter[1] //quotient
		sumR := sumInter[0] //remainder

		//Enforce api New Hint
		api.AssertIsEqual(api.Add(api.Mul(sumQ, r),sumR,),sumFactor)
		isSumValid := cmp.IsLess(api, sumR, r)
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

type EnygmaRequest struct {
	SenderID       string                  `json:"senderId" binding:"required"`
	Secrets        [nCommitments]string    `json:"secrets" binding:"required,len=6"`
	PublicKey      [nCommitments][2]string `json:"publicKey" binding:"required,len=6,dive,len=2"`
	Sk             string     			   `json:"sk" binding:"required"`
	PreviousV      string     			   `json:"previousV" binding:"required"`
	PreviousR      string     			   `json:"previousR" binding:"required"`
	PreviousCommit [nCommitments][2]string `json:"previousCommit" binding:"required,len=6,dive,len=2"`
	TxCommit       [nCommitments][2]string `json:"txCommit" binding:"required,len=6,dive,len=2"`
	TxValue        [nCommitments]string    `json:"txValue" binding:"required,len=6"`
	TxRandom       [nCommitments]string    `json:"txRandom" binding:"required,len=6"`
	V              string     			   `json:"v" binding:"required"`
	Nullifier      string     			   `json:"nullifier" binding:"required"`
	BlockNumber    string     			   `json:"blockNumber" binding:"required"`
	KIndex         [nCommitments]string    `json:"kIndex" binding:"required,len=6"`
}

type EnygmaOutput struct{
	Proof 			[]*big.Int `json:"proof"`
	PublicSignal    []*big.Int `json:"publicSignal"`

}