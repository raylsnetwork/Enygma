package types

import(
	"math/big"

)

type TransactionArgs struct {
	QtyBanks  int
	Value     *big.Int
	SenderId  int
	Sk        *big.Int
	PreviousV *big.Int
	PreviousR *big.Int

}

type Response struct {
	Message        string     `json:"message"`
	Proof          []*big.Int   `json:"proof"`
	PublicSignal   []*big.Int   `json:"publicSignal"`
}

type Proof struct {
	
	ArrayHashSecret [][]string `json:"arrayHashSecret`
	PublicKeys      []string   `json:"publicKey"`
	PreviousCommit [][]string `json:"previousCommit"`
	BlockNumber     string     `json:"blockNumber"`
	K               []string   `json:"kIndex"`

	SenderId        string     `json:"senderId"`
	Secrets         [][]string `json:"secrets"`
	TagMessage		[]string   `json:"tagMessage"`
	Sk              string     `json:"sk"`
	PreviousV       string     `json:"previousV"`
	PreviousR       string     `json:"previousR"`
	TxCommit        [][]string `json:"txCommit"`
	TxValue         []string   `json:"txValue"`
	TxRandom        []string   `json:"txRandom"`
	V               string     `json:"v"`
	Nullifier       string     `json:"nullifier"`
	
}

