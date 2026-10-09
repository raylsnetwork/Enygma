package server

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"
)

// feeNoteStore keeps the opening of every fee note the relayer is paid with.
//
// A fee note is Poseidon(feeSpendPubKey, salt, amount, tokenId), and the client
// chooses the salt: the relayer only learns it from the request. Nothing on
// chain carries it to the relayer (the ciphertexts published with the payment
// are for its recipient), so a salt not kept here leaves the note unspendable:
// the relayer would hold the spend key but not the opening.
//
// Records are appended as JSON lines and synced before the transaction is sent,
// so the opening survives even if the transaction lands after the relayer gave
// up waiting. A record whose transaction never lands is harmless: its
// commitment simply never appears on chain. The salt alone does not let anyone
// spend the note; that also takes the relayer's fee spend key.
type feeNoteStore struct {
	mu   sync.Mutex
	path string
}

// feeNoteRecord is one line of the store. Status is "submitted" when written
// before sending, "mined" once the transaction is confirmed (with TxHash).
type feeNoteRecord struct {
	Status     string `json:"status"`
	Route      string `json:"route"`
	VaultId    string `json:"vaultId"`
	Commitment string `json:"commitment"`
	Salt       string `json:"salt,omitempty"`
	Amount     string `json:"amount,omitempty"`
	TokenId    string `json:"tokenId,omitempty"`
	TxHash     string `json:"txHash,omitempty"`
	Block      uint64 `json:"block,omitempty"`
	RecordedAt string `json:"recordedAt"`
}

func newFeeNoteStore(path string) *feeNoteStore { return &feeNoteStore{path: path} }

func (s *feeNoteStore) append(r feeNoteRecord) error {
	r.RecordedAt = time.Now().UTC().Format(time.RFC3339)
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open fee note store %s: %w", s.path, err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write fee note store: %w", err)
	}
	return f.Sync()
}

// recordSubmitted keeps a fee note's opening before its transaction is sent.
func (s *feeNoteStore) recordSubmitted(route string, vaultId, commitment, salt, amount, tokenId *big.Int) error {
	return s.append(feeNoteRecord{
		Status: "submitted", Route: route, VaultId: vaultId.String(), Commitment: commitment.String(),
		Salt: salt.String(), Amount: amount.String(), TokenId: tokenId.String(),
	})
}

// recordMined notes the transaction that inserted the fee note. Best effort:
// the opening is already kept by recordSubmitted.
func (s *feeNoteStore) recordMined(route string, vaultId, commitment *big.Int, txHash string, block uint64) error {
	return s.append(feeNoteRecord{
		Status: "mined", Route: route, VaultId: vaultId.String(), Commitment: commitment.String(),
		TxHash: txHash, Block: block,
	})
}
