package withdraw

import (
	"math/big"

	pos "enygma-server/poseidon"
	utils "enygma-server/utils"
	
    "github.com/consensys/gnark/frontend"
)

const nCommitments = 6;

type WithdrawEnygmaCircuitConfig struct{

	NSplit int
}
// const nSplit =6
type WithdrawEnygmaCircuit struct {
	Config 			WithdrawEnygmaCircuitConfig
	SenderId      	frontend.Variable 
	Address       	frontend.Variable   `gnark:",public"` 
	HashArray 		[]frontend.Variable 
	VArray          []frontend.Variable
	V   			frontend.Variable  
	Pk 				[]frontend.Variable   
	TxCommit  	    [nCommitments][2]frontend.Variable 	
	TxRandom 		[nCommitments]frontend.Variable 
}


func (circuit *WithdrawEnygmaCircuit) Define(api frontend.API) error {	
	PDiff:= frontend.Variable("2736030358979909402780800718157159386076813972158567259200215660948447373041")

	//Knowledge of Pedersen Commitment
	NegValue:= api.Sub(PDiff,circuit.V) // P - v

	for i:=0; i< nCommitments;i++{
		isEqual := api.IsZero(api.Sub(i, circuit.SenderId))

		Value := api.Mul(isEqual,NegValue)
		PedersenCommitmentCalculated := utils.PedersenCommitment(api,Value, circuit.TxRandom[i])

		api.AssertIsEqual(PedersenCommitmentCalculated.X,circuit.TxCommit[i][0] )
		api.AssertIsEqual(PedersenCommitmentCalculated.Y,circuit.TxCommit[i][1] )
	}

	//Check if sum(VArray) = V
	sumCheckV :=frontend.Variable(0)
	for i:=0; i< circuit.Config.NSplit;i++{
		sumCheckV = api.Add(sumCheckV, circuit.VArray[i])
	}
	api.AssertIsEqual(sumCheckV,circuit.V)
	
	for i:=0; i < circuit.Config.NSplit; i++{
		//Knowledge of Hash
		uid := pos.Poseidon(api, []frontend.Variable{circuit.Address,circuit.VArray[i] })
		CalculatedHash := pos.Poseidon(api, []frontend.Variable{uid, circuit.Pk[i]})
		api.AssertIsEqual(CalculatedHash,circuit.HashArray[i])
	}
	
	return nil
}



type WithdrawRequest struct {
	SenderID       string                  `json:"senderId" binding:"required"`
	Address        string				   `json:"address" binding:"required"`
	HashArray      []string                `json:"hashArray" binding:"required,min=1,max=6"`
	VArray         []string                `json:"vArray" binding:"required,min=1,max=6"`
	V              string                  `json:"v" binding:"required"`
	Pk             []string     		   `json:"pk" binding:"required,min=1,max=6"`
	TxCommit       [nCommitments][2]string `json:"txCommit" binding:"required,len=6,dive,len=2"`
	TxRandom       [nCommitments]string    `json:"txRandom" binding:"required,len=6"`
}

type WithdrawOutput struct{
	Proof 			[]*big.Int `json:"proof"`
	PublicSignal    []*big.Int `json:"publicSignal"`

}