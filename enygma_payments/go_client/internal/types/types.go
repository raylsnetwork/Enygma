package types

import (
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
	Message      string     `json:"message"`
	Proof        []*big.Int `json:"proof"`
	PublicSignal []*big.Int `json:"publicSignal"`
}

// Fix L-08 (blocker 1): the enygma circuit moved to a k×k FingerPrint
// matrix (Fix C-04 — recipients' shared secrets are checked against a
// mutually-confirmed on-chain registry, not a flat per-sender hash array)
// tagged "fingerprint_shared_secrets", not the old flat
// "hashed_shared_secrets" []string this struct used to declare. Every
// *working* caller (demo/main.go, go_client/enygma_test/*.go) already
// sends this shape; this struct — the one the documented CLI actually
// uses — was the one left behind.
type Proof struct {
	FingerPrintofSharedSecrets [][]string `json:"fingerprint_shared_secrets"`
	PublicKey                  []string   `json:"public_keys"`
	PreviousCommit             [][]string `json:"previous_commits"`
	TxCommit                   [][]string `json:"tx_commits"`
	BlockNumber                string     `json:"block_number"`
	AnonymitySet               []string   `json:"anonymity_set"`
	MessageTags                []string   `json:"message_tags"`
	Nullifier                  string     `json:"nullifier"`

	SenderID                  string   `json:"sender_id"`
	SharedSecrets             []string `json:"shared_secrets"`
	SecretKey                 string   `json:"secret_key"`
	PreviousSenderBalance     string   `json:"previous_sender_balance"`
	PreviousSenderRandomValue string   `json:"previous_sender_random_value"`
	TxValues                  []string `json:"tx_values"`
	TxRandomValues            []string `json:"tx_random_values"`
	SenderTxValue             string   `json:"sender_tx_value"`
	// Fix L-01: chainId<<160|contractAddress — see Enygma.sol's
	// _expectedDomainId() doc comment. The gnark server has no chain
	// connection of its own, so the client (which does) supplies it.
	DomainId string `json:"domain_id"`
}
