package deposit

import (
	"math/big"

	pos "enygma-server/poseidon"
	utils "enygma-server/utils"
	
    "github.com/consensys/gnark/frontend"
)

const nCommitments = 6;
const nSplit = 3;

type DepositEnygmaCircuit struct {
	SenderId      	frontend.Variable 
	Address       	frontend.Variable               `gnark:",public"` 
	Hash 			frontend.Variable       		
	VInit			frontend.Variable
	VDeposit   		frontend.Variable 
	Secret          frontend.Variable
	Pk 				frontend.Variable   		     `gnark:",public"` 
	TxCommit  	    [nCommitments][2]frontend.Variable 	
	TxRandom 		[nCommitments]frontend.Variable 
}

func (circuit *DepositEnygmaCircuit) Define(api frontend.API) error {	
	
	
	for i:=0; i< nCommitments;i++{

		isEqual := api.IsZero(api.Sub(i, circuit.SenderId))

		Value := api.Mul(isEqual,circuit.VDeposit)
		PedersenCommitmentCalculated := utils.PedersenCommitment(api,Value, circuit.TxRandom[i])

		api.AssertIsEqual(PedersenCommitmentCalculated.X,circuit.TxCommit[i][0] )
		api.AssertIsEqual(PedersenCommitmentCalculated.Y,circuit.TxCommit[i][1] )

	}
	// Check VInit >VDeposit
	api.AssertIsEqual(api.Cmp(circuit.VInit, circuit.VDeposit), frontend.Variable(1))
    
	// Check if all hash Commitment (commitment in ZkDvp - MerkleTree) is well formed
	uid := pos.Poseidon(api, []frontend.Variable{circuit.Address,circuit.VDeposit })
	CalculatedHash := pos.Poseidon(api, []frontend.Variable{uid, circuit.Pk})
	api.AssertIsEqual(CalculatedHash,circuit.Hash)
	
	return nil
}


type DepositRequest struct {
	SenderID       string                  `json:"senderId" binding:"required"`
	Address        string				   `json:"address" binding:"required"`
	Hash	       string          			`json:"hash" binding:"required"`
	VInit          string		   			`json:"vInit" binding:"required"`	
	VDeposit       string                  `json:"vDeposit" binding:"required"`
	Secret         string                  `json:"secret" binding:"required"`
	Pk             string     			   `json:"pk" binding:"required"`
	TxCommit       [nCommitments][2]string `json:"txCommit" binding:"required,len=6,dive,len=2"`
	TxRandom       [nCommitments]string    `json:"txRandom" binding:"required,len=6"`
}

type DepositOutput struct{
	Proof 			[]*big.Int `json:"proof"`
	PublicSignal    []*big.Int `json:"publicSignal"`

}