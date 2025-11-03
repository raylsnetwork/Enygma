package interfacezkdvp

import (
	"math/big"
)


type Key  struct {
	PublicKey string  `json:"publicKey"`
	PrivateKey string `json:"privateKey"`
}

type EnygmaProofResponse struct {
	Pi_A          []string   `json:"pi_a"`
	Pi_B          [][]string `json:"pi_b"`
	Pi_C          []string   `json:"pi_c"`
	Public_Signal []string   `json:"public_signal"`
}
type MerkleTree struct {
	TreeNumber int  `json:"treeNumber"`
	PreviousTree  [][]string `json:"prevTrees"`
	Depth int        `json:"depth"`
	Zeros []string   `json:"zeros"`
	Tree  [][]string `json:"tree"`
}

type InsertLeaf struct{
	Commitment string `json:"commitment"`
}

type InsertLeafResponse struct{
	Sucess      bool      `json:"success"`
	Message    	string    `json:"message"`
	Leaves      string    `json:"leaves"`
}


type MerkleTreeResponse struct {
	Sucess        bool     `json:"success"`
	MerkleTree    MerkleTree    `json:"merkleTree"`
}

type ZkDvpProofToWithdrawRequest struct {
	PK             string     `json:"publicKey"`
	Amount         string     `json:"amount"`
	WithdrawKey    Key        `json:"withdrawKey"`
	EnygmaAddress  string     `json:"enygmaAddress"`
	MerkleTree     MerkleTree `json:"merkleTree"`
}

type ZkDvpProofWithdraw struct {
	Proof struct {
		A []string   `json:"a"`
		B [][]string `json:"b"`
		C []string   `json:"c"`
	} `json:"proof"`
	NumberOfInputs   	 int  `json:"numberOfInputs"`
	NumberOfOutputs      int   `json:"numberOfOutputs"`
	Statement   		[]string `json:"statement"`

}

type ZkDvpProofToWithdrawResponse struct {
	Success       bool               `json:"success"`
	Message       string             `json:"message"`
	ProofWithdraw ZkDvpProofWithdraw `json:"proofWithdraw"`
}


type SnarkWithdraw struct {
	SenderID      string     `json:"senderId"`
	Address       string     `json:"address"`
	HashArray     []string   `json:"hashArray"`
	VArray		  []string   `json:"vArray"`
	V             string     `json:"v"`
 	Pk            []string     `json:"pk"`
	TxCommit      [][]string `json:"txCommit"`
	TxRandom      []string   `json:"txRandom"`
	
}

type SnarkRespose struct{
	Message        string     `json:"message"`
	Proof          []*big.Int   `json:"proof"`
	PublicSignal   []*big.Int   `json:"publicSignal"`
}