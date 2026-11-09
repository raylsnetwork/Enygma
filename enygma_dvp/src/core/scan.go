package core

import (
	"crypto/mlkem"
	"errors"
	"fmt"
	"math/big"
)

// RejectReason says why a scan refused an event. It is an error so the
// per-event step can report it to scanEvents.
type RejectReason string

const (
	// RejectBadCapsule: the capsule is not a valid ML-KEM-768 ciphertext.
	RejectBadCapsule RejectReason = "bad capsule"
	// RejectCommitmentMismatch: the payload opened under this wallet's key,
	// but its values don't hash to the published commitment.
	RejectCommitmentMismatch RejectReason = "commitment mismatch"
	// RejectCommitmentNotComputable: the payload opened under this wallet's
	// key, but a decrypted value can't be hashed (it is outside the field).
	RejectCommitmentNotComputable RejectReason = "commitment not computable"
)

func (r RejectReason) Error() string { return string(r) }

// RejectedEvent is an event a scan refused: Index is its position in the
// scanned events. A wrong-length capsule is reported by every wallet's scan.
// The other reasons are reported only by the wallet the payload was sealed
// to, and mean someone sent it a note that doesn't open.
type RejectedEvent struct {
	Index  int
	Reason RejectReason
}

// errNotAddressed is the per-event step's report that the payload does not
// open under this wallet's key: someone else's note, not a bad event.
var errNotAddressed = errors.New("not addressed to this key")

// scanEvents is the per-event path every scanner shares. It decapsulates each
// event's capsule and hands the shared secret to open. An event that open
// reports as errNotAddressed or a RejectReason is skipped; any other error
// isn't the event's fault, and fails the scan.
func scanEvents[E, N any](
	dk *mlkem.DecapsulationKey768,
	events []E,
	capsule func(E) []byte,
	open func(ev E, ss []byte) (N, error),
) ([]N, []RejectedEvent, error) {
	var found []N
	var rejected []RejectedEvent
	for i, ev := range events {
		ss, err := Decapsulate(dk, capsule(ev))
		if err != nil {
			rejected = append(rejected, RejectedEvent{Index: i, Reason: RejectBadCapsule})
			continue
		}
		note, err := open(ev, ss)
		var reason RejectReason
		switch {
		case err == nil:
			found = append(found, note)
		case errors.Is(err, errNotAddressed):
		case errors.As(err, &reason):
			rejected = append(rejected, RejectedEvent{Index: i, Reason: reason})
		default:
			return nil, nil, fmt.Errorf("event %d: %w", i, err)
		}
	}
	return found, rejected, nil
}

// checkWalletValues fails a scan on a value the wallet itself supplied that
// can't be hashed. Checking these before the events means a commitment that
// can't be computed during the scan is always the event's fault.
func checkWalletValues(values ...*big.Int) error {
	for _, v := range values {
		if v == nil || v.Sign() < 0 || v.Cmp(SNARK_SCALAR_FIELD) >= 0 {
			return errors.New("scan input is not a field element")
		}
	}
	return nil
}

// openNote derives a note's salt and key from the shared secret and decrypts
// its payload, returning the payload's two values and the salt as a field
// element.
func openNote(ss, encTxData []byte) (first, second, saltBField *big.Int, err error) {
	saltB, err := DerivePaymentSalt(ss)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("DerivePaymentSalt failed: %w", err)
	}
	encKey, err := DerivePaymentKey(ss)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("DerivePaymentKey failed: %w", err)
	}
	first, second, err = DecryptPayload(encKey, encTxData)
	if err != nil {
		return nil, nil, nil, errNotAddressed
	}
	return first, second, SaltBToField(saltB), nil
}

// checkCommitment reports the rejection, if any, for a commitment recomputed
// from an event's decrypted values. computeErr is the recomputation's error.
func checkCommitment(expected *big.Int, computeErr error, onChain *big.Int) error {
	if computeErr != nil {
		return RejectCommitmentNotComputable
	}
	if expected.Cmp(onChain) != 0 {
		return RejectCommitmentMismatch
	}
	return nil
}

// OnChainErc20Event represents a single on-chain note-creation event published
// by the ERC20 vault after a successful JoinSplit proof.
//
// The sender posts (Commitment, CipherText, EncTxData) for each output note.
// Everything else (token address, tree number, Merkle root) comes from the chain
// but is not needed for note scanning — the recipient only needs these three fields
// to determine ownership.
type OnChainErc20Event struct {
	// Commitment is the output note commitment:
	//   Poseidon(pk_spendRecipient, SaltBToField(HKDF(ss,"Bob salt")), amount, tokenId)
	Commitment *big.Int

	// CipherText is the ML-KEM capsule (1088 bytes) produced by the sender via
	// Encapsulate(pk_viewRecipient). The recipient decapsulates it with sk_view to
	// recover the raw shared secret ss.
	CipherText []byte

	// EncTxData is the AES-256-GCM ciphertext of (tokenId || amount), keyed by
	// HKDF(ss, "encryption key"). A decryption failure means this note is not
	// addressed to the scanning view key.
	EncTxData []byte
}

// OwnedErc20Note is a note that Bob has confirmed belongs to him after scanning.
// It contains everything Bob needs to later spend the note (build a JoinSplit proof).
type OwnedErc20Note struct {
	// Commitment is the on-chain commitment value.
	Commitment *big.Int

	// TokenId recovered from encTxData.
	TokenId *big.Int

	// Amount recovered from encTxData.
	Amount *big.Int

	// SaltBField is saltB reduced mod SNARK_SCALAR_FIELD.
	// This is the witness value (WtSaltsIn) needed when spending this note.
	SaltBField *big.Int
}

// ScanForErc20Notes scans a list of on-chain ERC20 events and returns all notes
// addressed to the given view/spend key pair.
//
// For each event it:
//  1. Decapsulates cipherText to recover the raw ML-KEM shared secret ss.
//  2. Derives salt_B = HKDF(ss, "Bob salt") and encKey = HKDF(ss, "encryption key").
//  3. Attempts to decrypt encTxData with encKey (AES-GCM); failure = not for us.
//  4. Recomputes Poseidon(pk_spend, SaltBToField(salt_B), amount, tokenId) and
//     checks it matches the on-chain commitment.
//
// Events are untrusted: one that fails any of these steps is skipped, and the
// scan continues. An error means the scan itself failed, for example because
// pkSpend is not a field element.
//
// dk is the recipient's ML-KEM decapsulation key (sk_view).
// pkSpend is the recipient's spend public key (pk_spend = Poseidon(sk_spend)).
func ScanForErc20Notes(
	dk *mlkem.DecapsulationKey768,
	pkSpend *big.Int,
	events []OnChainErc20Event,
) ([]OwnedErc20Note, error) {
	owned, _, err := ScanForErc20NotesWithRejected(dk, pkSpend, events)
	return owned, err
}

// ScanForErc20NotesWithRejected is ScanForErc20Notes, also returning the events
// it refused and why.
func ScanForErc20NotesWithRejected(
	dk *mlkem.DecapsulationKey768,
	pkSpend *big.Int,
	events []OnChainErc20Event,
) ([]OwnedErc20Note, []RejectedEvent, error) {
	if err := checkWalletValues(pkSpend); err != nil {
		return nil, nil, err
	}
	return scanEvents(dk, events,
		func(ev OnChainErc20Event) []byte { return ev.CipherText },
		func(ev OnChainErc20Event, ss []byte) (OwnedErc20Note, error) {
			tokenId, amount, saltBField, err := openNote(ss, ev.EncTxData)
			if err != nil {
				return OwnedErc20Note{}, err
			}
			expected, err := Erc20CommitmentV2(pkSpend, saltBField, amount, tokenId)
			if err := checkCommitment(expected, err, ev.Commitment); err != nil {
				return OwnedErc20Note{}, err
			}
			return OwnedErc20Note{
				Commitment: new(big.Int).Set(ev.Commitment),
				TokenId:    tokenId,
				Amount:     amount,
				SaltBField: saltBField,
			}, nil
		},
	)
}

// OnChainErc721Event represents a single on-chain note-creation event published
// by the ERC721 vault after a successful ownership proof.
type OnChainErc721Event struct {
	Commitment *big.Int
	CipherText []byte
	EncTxData  []byte
}

// OwnedErc721Note is a note confirmed to belong to the scanning key pair.
type OwnedErc721Note struct {
	Commitment      *big.Int
	ContractAddress *big.Int
	TokenId         *big.Int
	SaltBField      *big.Int
}

// ScanForErc721Notes scans on-chain ERC721 events for notes owned by the given key
// pair, skipping bad events as ScanForErc20Notes does.
//
// EncTxData was created with EncryptPayload(encKey, contractAddress, tokenId)
// so DecryptPayload returns (contractAddress, tokenId).
func ScanForErc721Notes(
	dk *mlkem.DecapsulationKey768,
	pkSpend *big.Int,
	events []OnChainErc721Event,
) ([]OwnedErc721Note, error) {
	owned, _, err := ScanForErc721NotesWithRejected(dk, pkSpend, events)
	return owned, err
}

// ScanForErc721NotesWithRejected is ScanForErc721Notes, also returning the
// events it refused and why.
func ScanForErc721NotesWithRejected(
	dk *mlkem.DecapsulationKey768,
	pkSpend *big.Int,
	events []OnChainErc721Event,
) ([]OwnedErc721Note, []RejectedEvent, error) {
	if err := checkWalletValues(pkSpend); err != nil {
		return nil, nil, err
	}
	return scanEvents(dk, events,
		func(ev OnChainErc721Event) []byte { return ev.CipherText },
		func(ev OnChainErc721Event, ss []byte) (OwnedErc721Note, error) {
			contractAddress, tokenId, saltBField, err := openNote(ss, ev.EncTxData)
			if err != nil {
				return OwnedErc721Note{}, err
			}
			expected, err := Erc721Commitment(tokenId, pkSpend, saltBField)
			if err := checkCommitment(expected, err, ev.Commitment); err != nil {
				return OwnedErc721Note{}, err
			}
			return OwnedErc721Note{
				Commitment:      new(big.Int).Set(ev.Commitment),
				ContractAddress: contractAddress,
				TokenId:         tokenId,
				SaltBField:      saltBField,
			}, nil
		},
	)
}

// OnChainZkDvpEvent represents a pending-swap event emitted by the EnygmaDvp contract
// after Alice submits her side of a ZkDvp swap.
//
// Alice has spent her input note and proposed:
//   - CommitmentA (C') — the new note Alice will receive (the asset Bob is sending)
//   - CommitmentB      — the new note Bob will receive (the asset Alice is sending)
//
// To complete the swap Bob must decapsulate CipherText, decrypt EncTxData,
// verify both commitments, and submit his own nullifier + ZK proof.
type OnChainZkDvpEvent struct {
	// CommitmentA is Alice's output commitment C':
	//   Poseidon(aliceSpendPk, SaltBToField(saltStar), amountOut, tokenIdOut)
	CommitmentA *big.Int

	// CommitmentB is the commitment Bob will receive for Alice's payment:
	//   Poseidon(bobSpendPk, SaltBToField(saltB), amountIn, tokenIdIn)
	CommitmentB *big.Int

	// CipherText is the ML-KEM capsule (1088 bytes) Alice produced via
	// Encapsulate(bobViewEncapKey). Bob decapsulates to recover saltB.
	CipherText []byte

	// EncTxData is the AEAD ciphertext of (tokenIdOut || amountOut || saltStar)
	// keyed by saltB. Bob decrypts to verify CommitmentA is well-formed.
	EncTxData []byte
}

// ZkDvpSwapInfo is the result of Bob successfully scanning a ZkDvp pending event.
// It contains everything Bob needs to verify and complete the swap.
type ZkDvpSwapInfo struct {
	// CommitmentA is Alice's output commitment (verified well-formed).
	CommitmentA *big.Int

	// CommitmentB is the commitment Bob will receive (verified against saltB).
	CommitmentB *big.Int

	// TokenIdOut is the token ID Alice will receive (from encTxData).
	TokenIdOut *big.Int

	// AmountOut is the amount Alice will receive (from encTxData).
	AmountOut *big.Int

	// SaltStar is the salt Alice used in CommitmentA. Bob needs this to know
	// what salt is embedded in CommitmentA (it is NOT Bob's saltB).
	SaltStar []byte

	// SaltBField is saltB reduced mod SNARK_SCALAR_FIELD — the witness value
	// Bob passes as WtSaltsIn when spending CommitmentB in a future JoinSplit.
	SaltBField *big.Int
}

// ScanForZkDvpSwap scans on-chain ZkDvp pending events for swaps addressed to Bob,
// skipping bad events as ScanForErc20Notes does.
//
// For each event it:
//  1. Decapsulates CipherText with dk (Bob's view decapsulation key) to recover saltB.
//  2. Decrypts EncTxData with saltB to get (tokenIdOut, amountOut, saltStar).
//     An AEAD failure silently skips the event (not addressed to Bob).
//  3. Verifies CommitmentA = Poseidon(aliceSpendPk, SaltBToField(saltStar), amountOut, tokenIdOut).
//  4. Verifies CommitmentB = Poseidon(bobSpendPk, SaltBToField(saltB), amountIn, tokenIdIn).
//     amountIn/tokenIdIn are the agreed swap terms (what Bob contributes).
//
// Parameters:
//   - dk           — Bob's ML-KEM decapsulation key (sk_view)
//   - aliceSpendPk — Alice's spend public key (exchanged during setup)
//   - bobSpendPk   — Bob's own spend public key
//   - amountIn     — amount Bob contributes (the value in CommitmentB)
//   - tokenIdIn    — token ID Bob contributes (the tokenId in CommitmentB)
func ScanForZkDvpSwap(
	dk *mlkem.DecapsulationKey768,
	aliceSpendPk *big.Int,
	bobSpendPk *big.Int,
	amountIn, tokenIdIn *big.Int,
	events []OnChainZkDvpEvent,
) ([]ZkDvpSwapInfo, error) {
	swaps, _, err := ScanForZkDvpSwapWithRejected(dk, aliceSpendPk, bobSpendPk, amountIn, tokenIdIn, events)
	return swaps, err
}

// ScanForZkDvpSwapWithRejected is ScanForZkDvpSwap, also returning the events
// it refused and why.
func ScanForZkDvpSwapWithRejected(
	dk *mlkem.DecapsulationKey768,
	aliceSpendPk *big.Int,
	bobSpendPk *big.Int,
	amountIn, tokenIdIn *big.Int,
	events []OnChainZkDvpEvent,
) ([]ZkDvpSwapInfo, []RejectedEvent, error) {
	if err := checkWalletValues(aliceSpendPk, bobSpendPk, amountIn, tokenIdIn); err != nil {
		return nil, nil, err
	}
	return scanEvents(dk, events,
		func(ev OnChainZkDvpEvent) []byte { return ev.CipherText },
		func(ev OnChainZkDvpEvent, ss []byte) (ZkDvpSwapInfo, error) {
			// ZkDvP uses ss directly (not HKDF-derived) for commitment and encryption.
			tokenIdOut, amountOut, saltStar, err := DecryptSwapPayload(ss, ev.EncTxData)
			if err != nil {
				return ZkDvpSwapInfo{}, errNotAddressed
			}

			expectedCommA, err := Erc20CommitmentV2(aliceSpendPk, SaltBToField(saltStar), amountOut, tokenIdOut)
			if err := checkCommitment(expectedCommA, err, ev.CommitmentA); err != nil {
				return ZkDvpSwapInfo{}, err
			}

			saltBField := SaltBToField(ss)
			expectedCommB, err := Erc20CommitmentV2(bobSpendPk, saltBField, amountIn, tokenIdIn)
			if err := checkCommitment(expectedCommB, err, ev.CommitmentB); err != nil {
				return ZkDvpSwapInfo{}, err
			}

			return ZkDvpSwapInfo{
				CommitmentA: new(big.Int).Set(ev.CommitmentA),
				CommitmentB: new(big.Int).Set(ev.CommitmentB),
				TokenIdOut:  tokenIdOut,
				AmountOut:   amountOut,
				SaltStar:    saltStar,
				SaltBField:  saltBField,
			}, nil
		},
	)
}

// OnChainErc1155Event represents a single on-chain note-creation event published
// by the ERC1155 vault after a successful JoinSplit or ownership proof.
type OnChainErc1155Event struct {
	Commitment      *big.Int
	ContractAddress *big.Int // public info from on-chain event
	CipherText      []byte
	EncTxData       []byte
}

// OwnedErc1155Note is an ERC1155 note confirmed to belong to the scanning key pair.
type OwnedErc1155Note struct {
	Commitment      *big.Int
	ContractAddress *big.Int
	TokenId         *big.Int
	Amount          *big.Int
	SaltBField      *big.Int
}

// ScanForErc1155Notes scans on-chain ERC1155 events for notes owned by the given
// key pair, skipping bad events as ScanForErc20Notes does.
//
// EncTxData was created with EncryptPayload(encKey, tokenId, amount).
// contractAddress comes from the on-chain event (it is public).
func ScanForErc1155Notes(
	dk *mlkem.DecapsulationKey768,
	pkSpend *big.Int,
	events []OnChainErc1155Event,
) ([]OwnedErc1155Note, error) {
	owned, _, err := ScanForErc1155NotesWithRejected(dk, pkSpend, events)
	return owned, err
}

// ScanForErc1155NotesWithRejected is ScanForErc1155Notes, also returning the
// events it refused and why.
func ScanForErc1155NotesWithRejected(
	dk *mlkem.DecapsulationKey768,
	pkSpend *big.Int,
	events []OnChainErc1155Event,
) ([]OwnedErc1155Note, []RejectedEvent, error) {
	if err := checkWalletValues(pkSpend); err != nil {
		return nil, nil, err
	}
	return scanEvents(dk, events,
		func(ev OnChainErc1155Event) []byte { return ev.CipherText },
		func(ev OnChainErc1155Event, ss []byte) (OwnedErc1155Note, error) {
			tokenId, amount, saltBField, err := openNote(ss, ev.EncTxData)
			if err != nil {
				return OwnedErc1155Note{}, err
			}
			expected, err := Erc1155Commitment(tokenId, amount, pkSpend, saltBField)
			if err := checkCommitment(expected, err, ev.Commitment); err != nil {
				return OwnedErc1155Note{}, err
			}
			return OwnedErc1155Note{
				Commitment:      new(big.Int).Set(ev.Commitment),
				ContractAddress: new(big.Int).Set(ev.ContractAddress),
				TokenId:         tokenId,
				Amount:          amount,
				SaltBField:      saltBField,
			}, nil
		},
	)
}
