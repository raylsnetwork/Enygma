package core

import (
	"fmt"
	"math/big"

	"enygma_dvp/layout"
)

// DvPInitiatorResult holds the output of DvPInitiatorProof.
type DvPInitiatorResult struct {
	Proof []string // 8-element Groth16 proof
	// Statement is the public signals exactly as the gnark server returned
	// them, in the DvpInitiator statement layout's order.
	Statement       []*big.Int
	NumberOfInputs  int      // always 1; the contracts never read a leg's counts (GLOSSARY: DvP leg)
	NumberOfOutputs int      // always 1; settlement inserts the leg's one commitment, CommitB
	CipherText      []byte   // ML-KEM capsule for Bob (1088 bytes)
	EncTxData       []byte   // AES-256-GCM ciphertext of (tokenIdIn || valueIn)
	RevertSalt      *big.Int // opens RevertCommitA() after claimSwapTimeout; SwapRevertSalt(sk, nf_A)
	SaltA           *big.Int // passed to Bob's DvPDestinationProof
}

// ContractStatement returns the statement for on-chain submission: Statement,
// as the server returned it.
func (r *DvPInitiatorResult) ContractStatement() []*big.Int { return r.Statement }

// Message equals CommitA, linking the two legs of the swap.
func (r *DvPInitiatorResult) Message() *big.Int {
	return r.Statement[layout.DvpInitiatorMessageOffset]
}

// TreeNumber is the tree the initiator's input note lives in.
func (r *DvPInitiatorResult) TreeNumber() *big.Int {
	return r.Statement[layout.DvpInitiatorTreeNumberOffset]
}

// MerkleRoot is the root the initiator's input note is proved against.
func (r *DvPInitiatorResult) MerkleRoot() *big.Int {
	return r.Statement[layout.DvpInitiatorMerkleRootOffset]
}

// Nullifier is the initiator's input note's nullifier.
func (r *DvPInitiatorResult) Nullifier() *big.Int {
	return r.Statement[layout.DvpInitiatorNullifierOffset]
}

// CommitB is the note the counterparty receives: the initiator's asset.
func (r *DvPInitiatorResult) CommitB() *big.Int {
	return r.Statement[layout.DvpInitiatorCommitBOffset]
}

// CommitA is the note the initiator receives: the counterparty's asset.
func (r *DvPInitiatorResult) CommitA() *big.Int {
	return r.Statement[layout.DvpInitiatorCommitAOffset]
}

// RevertCommitA is the initiator's refund note if the swap times out.
func (r *DvPInitiatorResult) RevertCommitA() *big.Int {
	return r.Statement[layout.DvpInitiatorRevertCommitAOffset]
}

// CounterVault is the vault CommitA must be paid from.
func (r *DvPInitiatorResult) CounterVault() *big.Int {
	return r.Statement[layout.DvpInitiatorCounterVaultOffset]
}

// DvPInitiatorProof generates Alice's side of the DvP proof.
//
// stMessage is automatically set to commitA (Alice's expected output from Bob) so that
// the on-chain submitPartialSettlement cross-reference check works:
//
//	_pendingTransactions[commitB].targetReceiptId = commitA = Message
//
// Settlement inserts only CommitB into the ERC20 vault, whatever counts the
// receipt carries (a leg is read by name). commitA goes into the ERC721 vault via
// Bob's DvPDestinationProof; revertCommitA is only inserted on swap timeout.
//
// counterVault is the address (as an integer) of the vault Alice expects to be
// paid from, i.e. the counterparty's vault. It is the statement's CounterVault
// field and EnygmaDvp rejects a settlement from any other vault (CounterVaultMismatch).
// The same applies to DvPInitiatorProofFromSalts and DvPDestinationProof.
func (c *GnarkClient) DvPInitiatorProof(
	aliceKey KeyPair,
	aliceSaltIn *big.Int,
	valueIn *big.Int,
	tokenIdIn *big.Int,
	bobSpendPk *big.Int,
	bobViewEncapKey []byte,
	valueBob *big.Int,
	tokenIdBob *big.Int,
	stTreeNumber *big.Int,
	merkleProof *MerkleProof,
	merkleDepth int,
	counterVault *big.Int,
) (*DvPInitiatorResult, error) {
	if err := checkCounterVault(counterVault); err != nil {
		return nil, err
	}
	ss, cipherText, err := Encapsulate(bobViewEncapKey)
	if err != nil {
		return nil, fmt.Errorf("Encapsulate: %w", err)
	}
	saltBBytes, err := DerivePaymentSalt(ss)
	if err != nil {
		return nil, fmt.Errorf("DerivePaymentSalt: %w", err)
	}
	saltABytes, err := DeriveDvpSaltInit(ss)
	if err != nil {
		return nil, fmt.Errorf("DeriveDvpSaltInit: %w", err)
	}
	encKey, err := DerivePaymentKey(ss)
	if err != nil {
		return nil, fmt.Errorf("DerivePaymentKey: %w", err)
	}
	saltB := SaltBToField(saltBBytes)
	saltA := SaltBToField(saltABytes)

	encTxData, err := EncryptPayload(encKey, tokenIdIn, valueIn)
	if err != nil {
		return nil, fmt.Errorf("EncryptPayload: %w", err)
	}

	pathIndex := merkleProof.Indices
	// HIGH-1 fix: incorporate tree number into the nullifier.
	nf, err := GetNullifierWithTree(aliceKey.PrivateKey, stTreeNumber, pathIndex, merkleDepth)
	if err != nil {
		return nil, fmt.Errorf("GetNullifier: %w", err)
	}
	revertSalt, err := SwapRevertSalt(aliceKey.PrivateKey, nf)
	if err != nil {
		return nil, fmt.Errorf("SwapRevertSalt: %w", err)
	}

	commitB, err := Erc20CommitmentV2(bobSpendPk, saltB, valueIn, tokenIdIn)
	if err != nil {
		return nil, fmt.Errorf("Erc20CommitmentV2 (commitB): %w", err)
	}
	commitA, err := Erc20CommitmentV2(aliceKey.PublicKey, saltA, valueBob, tokenIdBob)
	if err != nil {
		return nil, fmt.Errorf("Erc20CommitmentV2 (commitA): %w", err)
	}
	revertCommitA, err := Erc20CommitmentV2(aliceKey.PublicKey, revertSalt, valueIn, tokenIdIn)
	if err != nil {
		return nil, fmt.Errorf("Erc20CommitmentV2 (revertCommitA): %w", err)
	}

	pathElems := make([]*big.Int, merkleDepth)
	copy(pathElems, merkleProof.Elements[:merkleDepth])

	// stMessage = commitA for on-chain cross-referencing:
	//   submitPartialSettlement stores _pendingTransactions[commitB].targetReceiptId = Message = commitA
	//   Bob's DvPDestinationProof uses stMessage=commitB, output=commitA, so the check resolves correctly.
	stMessage := commitA

	payload := map[string]interface{}{
		"stMessage":       stMessage.String(),
		"stTreeNumber":    stTreeNumber.String(),
		"stMerkleRoot":    merkleProof.Root.String(),
		"stNullifier":     nf.String(),
		"stCommitB":       commitB.String(),
		"stCommitA":       commitA.String(),
		"stRevertCommitA": revertCommitA.String(),
		"stCounterVault":  counterVault.String(),
		"wtSpendKeyIn":    aliceKey.PrivateKey.String(),
		"wtValueIn":       valueIn.String(),
		"wtSaltIn":        aliceSaltIn.String(),
		"wtTokenIdIn":     tokenIdIn.String(),
		"wtPathElements":  bigIntSliceToStrings(pathElems),
		"wtPathIndex":     pathIndex.String(),
		"wtSpendPkBob":    bobSpendPk.String(),
		"wtSaltB":         saltB.String(),
		"wtValueBob":      valueBob.String(),
		"wtTokenIdBob":    tokenIdBob.String(),
		"wtSaltA":         saltA.String(),
		"wtRevertSalt":    revertSalt.String(),
	}

	body, err := c.PostProof("/proof/dvpInitiator", payload)
	if err != nil {
		return nil, fmt.Errorf("dvpInitiator proof request failed: %w", err)
	}

	proofStrs, statement, err := proofResponse(body, layout.DvpInitiatorLen)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dvpInitiator proof response: %w", err)
	}

	return &DvPInitiatorResult{
		Proof:           proofStrs,
		Statement:       statement,
		NumberOfInputs:  1,
		NumberOfOutputs: 1,
		CipherText:      cipherText,
		EncTxData:       encTxData,
		RevertSalt:      revertSalt,
		SaltA:           saltA,
	}, nil
}

// DvPInitiatorProofFromSalts is like DvPInitiatorProof but accepts pre-computed
// saltB and saltA instead of deriving them from a KEM shared secret.  This
// allows both parties of an ERC-20 ↔ ERC-20 exchange to independently produce
// DvP Initiator proofs that cross-reference each other: each party picks the
// same "commitForSelf" and "commitForCounterparty" by sharing salt values
// out-of-band before generating their proofs.
//
// No KEM encapsulation is performed; CipherText and EncTxData in the returned
// result are nil.
func (c *GnarkClient) DvPInitiatorProofFromSalts(
	aliceKey KeyPair,
	aliceSaltIn *big.Int,
	valueIn *big.Int,
	tokenIdIn *big.Int,
	bobSpendPk *big.Int,
	saltB *big.Int,
	valueBob *big.Int,
	tokenIdBob *big.Int,
	saltA *big.Int,
	stTreeNumber *big.Int,
	merkleProof *MerkleProof,
	merkleDepth int,
	counterVault *big.Int,
) (*DvPInitiatorResult, error) {
	if err := checkCounterVault(counterVault); err != nil {
		return nil, err
	}
	pathIndex := merkleProof.Indices
	// HIGH-1 fix: incorporate tree number into the nullifier.
	nf, err := GetNullifierWithTree(aliceKey.PrivateKey, stTreeNumber, pathIndex, merkleDepth)
	if err != nil {
		return nil, fmt.Errorf("GetNullifier: %w", err)
	}

	commitB, err := Erc20CommitmentV2(bobSpendPk, saltB, valueIn, tokenIdIn)
	if err != nil {
		return nil, fmt.Errorf("Erc20CommitmentV2 (commitB): %w", err)
	}
	commitA, err := Erc20CommitmentV2(aliceKey.PublicKey, saltA, valueBob, tokenIdBob)
	if err != nil {
		return nil, fmt.Errorf("Erc20CommitmentV2 (commitA): %w", err)
	}

	revertSalt, err := SwapRevertSalt(aliceKey.PrivateKey, nf)
	if err != nil {
		return nil, fmt.Errorf("SwapRevertSalt: %w", err)
	}
	revertCommitA, err := Erc20CommitmentV2(aliceKey.PublicKey, revertSalt, valueIn, tokenIdIn)
	if err != nil {
		return nil, fmt.Errorf("Erc20CommitmentV2 (revertCommitA): %w", err)
	}

	stMessage := commitA

	pathElems := make([]*big.Int, merkleDepth)
	copy(pathElems, merkleProof.Elements[:merkleDepth])

	payload := map[string]interface{}{
		"stMessage":       stMessage.String(),
		"stTreeNumber":    stTreeNumber.String(),
		"stMerkleRoot":    merkleProof.Root.String(),
		"stNullifier":     nf.String(),
		"stCommitB":       commitB.String(),
		"stCommitA":       commitA.String(),
		"stRevertCommitA": revertCommitA.String(),
		"stCounterVault":  counterVault.String(),
		"wtSpendKeyIn":    aliceKey.PrivateKey.String(),
		"wtValueIn":       valueIn.String(),
		"wtSaltIn":        aliceSaltIn.String(),
		"wtTokenIdIn":     tokenIdIn.String(),
		"wtPathElements":  bigIntSliceToStrings(pathElems),
		"wtPathIndex":     pathIndex.String(),
		"wtSpendPkBob":    bobSpendPk.String(),
		"wtSaltB":         saltB.String(),
		"wtValueBob":      valueBob.String(),
		"wtTokenIdBob":    tokenIdBob.String(),
		"wtSaltA":         saltA.String(),
		"wtRevertSalt":    revertSalt.String(),
	}

	body, err := c.PostProof("/proof/dvpInitiator", payload)
	if err != nil {
		return nil, fmt.Errorf("dvpInitiatorFromSalts proof request failed: %w", err)
	}

	proofStrs, statement, err := proofResponse(body, layout.DvpInitiatorLen)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dvpInitiatorFromSalts proof response: %w", err)
	}

	return &DvPInitiatorResult{
		Proof:           proofStrs,
		Statement:       statement,
		NumberOfInputs:  1,
		NumberOfOutputs: 1,
		SaltA:           saltA,
		RevertSalt:      revertSalt,
	}, nil
}

// DvPDestinationResult holds the output of DvPDestinationProof.
type DvPDestinationResult struct {
	Proof []string // 8-element Groth16 proof
	// Statement is the public signals exactly as the gnark server returned
	// them, in the DvpDestination statement layout's order.
	Statement       []*big.Int
	NumberOfInputs  int // always 1
	NumberOfOutputs int // always 1
}

// ContractStatement returns the statement for on-chain submission: Statement,
// as the server returned it.
func (r *DvPDestinationResult) ContractStatement() []*big.Int { return r.Statement }

// Message equals the initiator's CommitB.
func (r *DvPDestinationResult) Message() *big.Int {
	return r.Statement[layout.DvpDestinationMessageOffset]
}

// TreeNumber is the tree the counterparty's input note lives in.
func (r *DvPDestinationResult) TreeNumber() *big.Int {
	return r.Statement[layout.DvpDestinationTreeNumberOffset]
}

// MerkleRoot is the root the counterparty's input note is proved against.
func (r *DvPDestinationResult) MerkleRoot() *big.Int {
	return r.Statement[layout.DvpDestinationMerkleRootOffset]
}

// Nullifier is the counterparty's input note's nullifier.
func (r *DvPDestinationResult) Nullifier() *big.Int {
	return r.Statement[layout.DvpDestinationNullifierOffset]
}

// CommitA is the initiator's CommitA, recomputed.
func (r *DvPDestinationResult) CommitA() *big.Int {
	return r.Statement[layout.DvpDestinationCommitAOffset]
}

// CounterVault is the vault the initiator's CommitB must be paid from.
func (r *DvPDestinationResult) CounterVault() *big.Int {
	return r.Statement[layout.DvpDestinationCounterVaultOffset]
}

// DvPDestinationProof generates Bob's side of the DvP proof.
//
// HIGH-3 fix: the circuit now constrains StMessage == COMMIT_B (commitB).
// Bob must supply saltB, valueAlice, tokenIdAlice (from ENC_TX_DATA decryption)
// so the circuit can recompute commitB and verify stMessage equals it.
// stMessage is now automatically derived as commitB — the caller no longer
// passes an arbitrary stMessage.
func (c *GnarkClient) DvPDestinationProof(
	bobKey KeyPair,
	bobSaltIn *big.Int,
	valueIn *big.Int,
	tokenIdIn *big.Int,
	aliceSpendPk *big.Int,
	saltA *big.Int,
	saltB *big.Int, // HKDF(ss_B, "note salt") — used by Alice to build commitB for Bob
	valueAlice *big.Int, // Alice's delivered amount (from ENC_TX_DATA decryption)
	tokenIdAlice *big.Int, // Alice's delivered tokenId (from ENC_TX_DATA decryption)
	commitA *big.Int,
	stTreeNumber *big.Int,
	merkleProof *MerkleProof,
	merkleDepth int,
	counterVault *big.Int,
) (*DvPDestinationResult, error) {
	if err := checkCounterVault(counterVault); err != nil {
		return nil, err
	}
	pathIndex := merkleProof.Indices
	// HIGH-1 fix: incorporate tree number into the nullifier.
	nf, err := GetNullifierWithTree(bobKey.PrivateKey, stTreeNumber, pathIndex, merkleDepth)
	if err != nil {
		return nil, fmt.Errorf("GetNullifier: %w", err)
	}

	// HIGH-3 fix: commitB is the circuit-constrained value for stMessage.
	commitB, err := Erc20CommitmentV2(bobKey.PublicKey, saltB, valueAlice, tokenIdAlice)
	if err != nil {
		return nil, fmt.Errorf("Erc20CommitmentV2 (commitB): %w", err)
	}

	pathElems := make([]*big.Int, merkleDepth)
	copy(pathElems, merkleProof.Elements[:merkleDepth])

	payload := map[string]interface{}{
		"stMessage":      commitB.String(), // HIGH-3: stMessage == commitB (circuit-enforced)
		"stTreeNumber":   stTreeNumber.String(),
		"stMerkleRoot":   merkleProof.Root.String(),
		"stNullifier":    nf.String(),
		"stCommitA":      commitA.String(),
		"stCounterVault": counterVault.String(),
		"wtSpendKeyIn":   bobKey.PrivateKey.String(),
		"wtValueIn":      valueIn.String(),
		"wtSaltIn":       bobSaltIn.String(),
		"wtTokenIdIn":    tokenIdIn.String(),
		"wtPathElements": bigIntSliceToStrings(pathElems),
		"wtPathIndex":    pathIndex.String(),
		"wtSpendPkAlice": aliceSpendPk.String(),
		"wtSaltA":        saltA.String(),
		"wtSaltB":        saltB.String(),
		"wtValueAlice":   valueAlice.String(),
		"wtTokenIdAlice": tokenIdAlice.String(),
	}

	body, err := c.PostProof("/proof/dvpDestination", payload)
	if err != nil {
		return nil, fmt.Errorf("dvpDestination proof request failed: %w", err)
	}

	proofStrs, statement, err := proofResponse(body, layout.DvpDestinationLen)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dvpDestination proof response: %w", err)
	}

	return &DvPDestinationResult{
		Proof:           proofStrs,
		Statement:       statement,
		NumberOfInputs:  1,
		NumberOfOutputs: 1,
	}, nil
}

// checkCounterVault rejects a missing counterparty vault before a proof request
// (the circuits require it to be non-zero).
func checkCounterVault(counterVault *big.Int) error {
	if counterVault == nil || counterVault.Sign() <= 0 {
		return fmt.Errorf("counterVault must be the counterparty vault's address (non-zero)")
	}
	return nil
}
