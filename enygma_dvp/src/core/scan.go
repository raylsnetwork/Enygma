package core

import (
	"crypto/mlkem"
	"fmt"
	"math/big"
)

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
//     checks it matches the on-chain commitment. A mismatch is an error (tampered).
//
// dk is the recipient's ML-KEM decapsulation key (sk_view).
// pkSpend is the recipient's spend public key (pk_spend = Poseidon(sk_spend)).
func ScanForErc20Notes(
	dk *mlkem.DecapsulationKey768,
	pkSpend *big.Int,
	events []OnChainErc20Event,
) ([]OwnedErc20Note, error) {
	var owned []OwnedErc20Note

	for i, ev := range events {
		// Step 1: recover raw shared secret from the ML-KEM capsule.
		ss, err := Decapsulate(dk, ev.CipherText)
		if err != nil {
			return nil, fmt.Errorf("event %d: decapsulation failed: %w", i, err)
		}

		// Step 2: HKDF-derive salt and encryption key.
		saltB, err := DerivePaymentSalt(ss)
		if err != nil {
			return nil, fmt.Errorf("event %d: DerivePaymentSalt failed: %w", i, err)
		}
		encKey, err := DerivePaymentKey(ss)
		if err != nil {
			return nil, fmt.Errorf("event %d: DerivePaymentKey failed: %w", i, err)
		}

		// Step 3: try to decrypt the payload. AES-GCM auth failure = not for us.
		tokenId, amount, err := DecryptPayload(encKey, ev.EncTxData)
		if err != nil {
			continue
		}

		// Step 4: recompute commitment and verify it matches the chain.
		saltBField := SaltBToField(saltB)
		expected, err := Erc20CommitmentV2(pkSpend, saltBField, amount, tokenId)
		if err != nil {
			return nil, fmt.Errorf("event %d: failed to compute commitment: %w", i, err)
		}
		if expected.Cmp(ev.Commitment) != 0 {
			return nil, fmt.Errorf(
				"event %d: commitment mismatch — decrypted payload does not match on-chain commitment"+
					" (got %s, want %s)", i, expected.String(), ev.Commitment.String(),
			)
		}

		owned = append(owned, OwnedErc20Note{
			Commitment: new(big.Int).Set(ev.Commitment),
			TokenId:    tokenId,
			Amount:     amount,
			SaltBField: saltBField,
		})
	}

	return owned, nil
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

// ScanForErc721Notes scans on-chain ERC721 events for notes owned by the given key pair.
//
// EncTxData was created with EncryptPayload(encKey, contractAddress, tokenId)
// so DecryptPayload returns (contractAddress, tokenId).
func ScanForErc721Notes(
	dk *mlkem.DecapsulationKey768,
	pkSpend *big.Int,
	events []OnChainErc721Event,
) ([]OwnedErc721Note, error) {
	var owned []OwnedErc721Note

	for i, ev := range events {
		ss, err := Decapsulate(dk, ev.CipherText)
		if err != nil {
			return nil, fmt.Errorf("event %d: decapsulation failed: %w", i, err)
		}

		saltB, err := DerivePaymentSalt(ss)
		if err != nil {
			return nil, fmt.Errorf("event %d: DerivePaymentSalt failed: %w", i, err)
		}
		encKey, err := DerivePaymentKey(ss)
		if err != nil {
			return nil, fmt.Errorf("event %d: DerivePaymentKey failed: %w", i, err)
		}

		// DecryptPayload returns (contractAddress, tokenId) for ERC721
		contractAddress, tokenId, err := DecryptPayload(encKey, ev.EncTxData)
		if err != nil {
			continue // not addressed to this key
		}

		saltBField := SaltBToField(saltB)
		expected, err := Erc721Commitment(tokenId, pkSpend, saltBField)
		if err != nil {
			return nil, fmt.Errorf("event %d: failed to compute commitment: %w", i, err)
		}
		if expected.Cmp(ev.Commitment) != 0 {
			return nil, fmt.Errorf(
				"event %d: commitment mismatch — decrypted payload does not match on-chain commitment"+
					" (got %s, want %s)", i, expected.String(), ev.Commitment.String(),
			)
		}

		owned = append(owned, OwnedErc721Note{
			Commitment:      new(big.Int).Set(ev.Commitment),
			ContractAddress: contractAddress,
			TokenId:         tokenId,
			SaltBField:      saltBField,
		})
	}

	return owned, nil
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

// ScanForZkDvpSwap scans on-chain ZkDvp pending events for swaps addressed to Bob.
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
	var swaps []ZkDvpSwapInfo

	for i, ev := range events {
		// Step 1: recover raw shared secret from the ML-KEM capsule.
		// ZkDvP uses ss directly (not HKDF-derived) for commitment and encryption.
		ss, err := Decapsulate(dk, ev.CipherText)
		if err != nil {
			return nil, fmt.Errorf("event %d: decapsulation failed: %w", i, err)
		}

		// Step 2: decrypt payload; failure = not addressed to Bob.
		tokenIdOut, amountOut, saltStar, err := DecryptSwapPayload(ss, ev.EncTxData)
		if err != nil {
			continue // not for Bob
		}

		// Step 3: verify CommitmentA is well-formed.
		saltStarField := SaltBToField(saltStar)
		expectedCommA, err := Erc20CommitmentV2(aliceSpendPk, saltStarField, amountOut, tokenIdOut)
		if err != nil {
			return nil, fmt.Errorf("event %d: failed to compute CommitmentA: %w", i, err)
		}
		if expectedCommA.Cmp(ev.CommitmentA) != 0 {
			return nil, fmt.Errorf(
				"event %d: CommitmentA mismatch (computed %s, on-chain %s)",
				i, expectedCommA, ev.CommitmentA,
			)
		}

		// Step 4: verify CommitmentB is well-formed using agreed swap terms.
		saltBField := SaltBToField(ss)
		expectedCommB, err := Erc20CommitmentV2(bobSpendPk, saltBField, amountIn, tokenIdIn)
		if err != nil {
			return nil, fmt.Errorf("event %d: failed to compute CommitmentB: %w", i, err)
		}
		if expectedCommB.Cmp(ev.CommitmentB) != 0 {
			return nil, fmt.Errorf(
				"event %d: CommitmentB mismatch (computed %s, on-chain %s)",
				i, expectedCommB, ev.CommitmentB,
			)
		}

		swaps = append(swaps, ZkDvpSwapInfo{
			CommitmentA: new(big.Int).Set(ev.CommitmentA),
			CommitmentB: new(big.Int).Set(ev.CommitmentB),
			TokenIdOut:  tokenIdOut,
			AmountOut:   amountOut,
			SaltStar:    saltStar,
			SaltBField:  saltBField,
		})
	}

	return swaps, nil
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

// ScanForErc1155Notes scans on-chain ERC1155 events for notes owned by the given key pair.
//
// EncTxData was created with EncryptPayload(encKey, tokenId, amount).
// contractAddress comes from the on-chain event (it is public).
func ScanForErc1155Notes(
	dk *mlkem.DecapsulationKey768,
	pkSpend *big.Int,
	events []OnChainErc1155Event,
) ([]OwnedErc1155Note, error) {
	var owned []OwnedErc1155Note

	for i, ev := range events {
		ss, err := Decapsulate(dk, ev.CipherText)
		if err != nil {
			return nil, fmt.Errorf("event %d: decapsulation failed: %w", i, err)
		}

		saltB, err := DerivePaymentSalt(ss)
		if err != nil {
			return nil, fmt.Errorf("event %d: DerivePaymentSalt failed: %w", i, err)
		}
		encKey, err := DerivePaymentKey(ss)
		if err != nil {
			return nil, fmt.Errorf("event %d: DerivePaymentKey failed: %w", i, err)
		}

		tokenId, amount, err := DecryptPayload(encKey, ev.EncTxData)
		if err != nil {
			continue // not addressed to this key
		}

		saltBField := SaltBToField(saltB)
		expected, err := Erc1155Commitment(tokenId, amount, pkSpend, saltBField)
		if err != nil {
			return nil, fmt.Errorf("event %d: failed to compute commitment: %w", i, err)
		}
		if expected.Cmp(ev.Commitment) != 0 {
			return nil, fmt.Errorf(
				"event %d: commitment mismatch — decrypted payload does not match on-chain commitment"+
					" (got %s, want %s)", i, expected.String(), ev.Commitment.String(),
			)
		}

		owned = append(owned, OwnedErc1155Note{
			Commitment:      new(big.Int).Set(ev.Commitment),
			ContractAddress: new(big.Int).Set(ev.ContractAddress),
			TokenId:         tokenId,
			Amount:          amount,
			SaltBField:      saltBField,
		})
	}

	return owned, nil
}
