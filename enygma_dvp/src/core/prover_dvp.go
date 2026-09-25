package core

import (
	"encoding/json"
	"fmt"
	"math/big"
)

// ZkDvpSwapInitResult holds everything Alice produces when initiating a ZkDvp swap.
type ZkDvpSwapInitResult struct {
	// AliceNullifier is the nullifier that burns Alice's input note.
	AliceNullifier *big.Int

	// CommitmentB = Poseidon(bobSpendPk, SaltBToField(saltB), amountIn, tokenIdIn).
	// This is the commitment Bob receives (Alice's asset going to Bob).
	CommitmentB *big.Int

	// CommitmentA is C' = Poseidon(aliceSpendPk, SaltBToField(saltStar), amountOut, tokenIdOut).
	// This is the commitment Alice receives (Bob's asset coming to Alice).
	CommitmentA *big.Int

	// CipherText is the ML-KEM capsule (1088 bytes) from Encapsulate(bobViewEncapKey).
	// Bob decapsulates to recover saltB.
	CipherText []byte

	// EncTxData is the AEAD ciphertext of (tokenIdOut || amountOut || saltStar)
	// keyed by saltB. Bob decrypts to verify CommitmentA is well-formed.
	EncTxData []byte

	// SaltStar is the raw random salt used in CommitmentA. Alice keeps this to
	// spend CommitmentA in a future proof (used as WtSaltsIn).
	SaltStar []byte

	// SaltStarField is SaltBToField(saltStar) — the field element embedded in CommitmentA.
	SaltStarField *big.Int

	// Proof is the gnark proof bytes (populated when the gnark server is called).
	// Empty when used in off-chain-only mode.
	Proof []string
}

// ZkDvpInitiateSwap produces all of Alice's ZkDvp swap artefacts in one call:
//
//  1. Calls Encapsulate(bobViewEncapKey) to derive (saltB, cipherText).
//  2. Computes CommitmentB = Poseidon(bobSpendPk, SaltBToField(saltB), amountIn, tokenIdIn).
//  3. Generates saltStar and computes CommitmentA = C' = Poseidon(aliceSpendPk, SaltBToField(saltStar), amountOut, tokenIdOut).
//  4. Encrypts (tokenIdOut || amountOut || saltStar) with saltB → encTxData.
//  5. Computes Alice's nullifier.
//  6. Generates the JoinSplit ZK proof for Alice's input note with StMessage = CommitmentA (C').
//
// The StMessage is set to CommitmentA so that the on-chain cross-commitment check passes:
//
//	stMessage(Alice) = C'         must equal firstOutput(Bob) = C'
//	stMessage(Bob)   = CommitmentB must equal firstOutput(Alice) = CommitmentB
//
// Parameters:
//   - aliceKey     — Alice's spend key pair (input note ownership)
//   - aliceSaltIn  — the saltBField Alice received when she got her input note
//   - amountIn     — amount of Alice's input note (e.g. 5 USDT)
//   - tokenIdIn    — token ID of Alice's input note (e.g. 10)
//   - bobSpendPk   — Bob's spend public key
//   - bobViewEncapKey — Bob's ML-KEM encapsulation key (1184 bytes)
//   - tokenIdOut   — token ID Alice will receive (e.g. 25, concert ticket)
//   - amountOut    — amount Alice will receive (e.g. 1)
//   - merkleProof  — Merkle proof for Alice's input note (nil = dummy/no proof)
//   - stTreeNumber — tree number for Alice's input note
func (c *GnarkClient) ZkDvpInitiateSwap(
	aliceKey KeyPair,
	aliceSaltIn *big.Int,
	amountIn *big.Int,
	tokenIdIn *big.Int,
	bobSpendPk *big.Int,
	bobViewEncapKey []byte,
	tokenIdOut *big.Int,
	amountOut *big.Int,
	merkleDepth int,
	merkleProof *MerkleProof,
	stTreeNumber *big.Int,
) (*ZkDvpSwapInitResult, error) {
	// Step 1: Alice encapsulates Bob's view key → raw shared secret ss + cipherText.
	// HIGH-8 fix: use HKDF-derived values instead of raw ss for both the commitment
	// salt and the AEAD encryption key. Using raw ss for both violates key separation —
	// recovering the salt from the commitment would immediately expose the AEAD key.
	ss, cipherText, err := Encapsulate(bobViewEncapKey)
	if err != nil {
		return nil, fmt.Errorf("encapsulate failed: %w", err)
	}
	saltBBytes, err := DerivePaymentSalt(ss)
	if err != nil {
		return nil, fmt.Errorf("DerivePaymentSalt failed: %w", err)
	}
	saltBField := SaltBToField(saltBBytes)

	// Step 2: CommitmentB — Bob receives Alice's asset.
	commitmentB, err := Erc20CommitmentV2(bobSpendPk, saltBField, amountIn, tokenIdIn)
	if err != nil {
		return nil, fmt.Errorf("CommitmentB computation failed: %w", err)
	}

	// Step 3: C' (CommitmentA) — Alice receives Bob's asset.
	// saltStar is freshly generated with the same byte-length as ss.
	saltStar, err := GenerateRandomValue(len(ss))
	if err != nil {
		return nil, fmt.Errorf("GenerateRandomValue failed: %w", err)
	}
	saltStarField := SaltBToField(saltStar)
	commitmentA, err := Erc20CommitmentV2(aliceKey.PublicKey, saltStarField, amountOut, tokenIdOut)
	if err != nil {
		return nil, fmt.Errorf("CommitmentA (C') computation failed: %w", err)
	}

	// Step 4: Encrypt (tokenIdOut || amountOut || saltStar) for Bob.
	// HIGH-8 fix: use HKDF-derived encryption key, not raw ss.
	encKey, err := DerivePaymentKey(ss)
	if err != nil {
		return nil, fmt.Errorf("DerivePaymentKey failed: %w", err)
	}
	encTxData, err := EncryptSwapPayload(encKey, tokenIdOut, amountOut, saltStar)
	if err != nil {
		return nil, fmt.Errorf("EncryptSwapPayload failed: %w", err)
	}

	// Step 5: Compute Alice's nullifier.
	var pathIndices *big.Int
	var pathElements []*big.Int
	var merkleRoot *big.Int
	if merkleProof != nil {
		pathIndices = merkleProof.Indices
		pathElements = merkleProof.Elements
		merkleRoot = merkleProof.Root
	} else {
		pathIndices = big.NewInt(0)
		pathElements = make([]*big.Int, merkleDepth)
		for j := range pathElements {
			pathElements[j] = big.NewInt(0)
		}
		merkleRoot = big.NewInt(0)
	}
	// HIGH-1 fix: incorporate tree number to prevent cross-tree double-spend.
	nullifier, err := GetNullifierWithTree(aliceKey.PrivateKey, stTreeNumber, pathIndices, merkleDepth)
	if err != nil {
		return nil, fmt.Errorf("GetNullifier failed: %w", err)
	}

	// Step 6: Generate DvP Initiator ZK proof via the dedicated /proof/dvpInitiator endpoint.
	// HIGH-9 fix: was incorrectly calling /proof/joinSplitERC20 (JoinSplit circuit) with a
	// payload shaped for the DvP Initiator circuit. The two circuits have different VKs
	// and statement layouts — using the wrong endpoint either always reverts on-chain or
	// silently accepts a semantically incorrect proof.
	revertSalt, err := RandomInField()
	if err != nil {
		return nil, fmt.Errorf("RandomInField revertSalt failed: %w", err)
	}
	revertCommit, err := Erc20CommitmentV2(aliceKey.PublicKey, revertSalt, amountIn, tokenIdIn)
	if err != nil {
		return nil, fmt.Errorf("revertCommitA computation failed: %w", err)
	}

	// saltA: HKDF-derived from ss using "Init Salt" (same derivation Bob uses after decapsulation).
	saltABytes, err := DeriveDvpSaltInit(ss)
	if err != nil {
		return nil, fmt.Errorf("DeriveDvpSaltInit failed: %w", err)
	}
	saltAField := SaltBToField(saltABytes)

	// Convert path elements to [8]string for the dvpInit circuit (single-input, depth 8).
	var pathElemsFixed [8]string
	for j := 0; j < merkleDepth && j < 8; j++ {
		pathElemsFixed[j] = pathElements[j].String()
	}

	payload := map[string]interface{}{
		"stMessage":       commitmentB.String(), // HIGH-5: StMessage must equal StCommitB
		"stTreeNumber":    stTreeNumber.String(),
		"stMerkleRoot":    merkleRoot.String(),
		"stNullifier":     nullifier.String(),
		"stCommitB":       commitmentB.String(),
		"stCommitA":       commitmentA.String(),
		"stRevertCommitA": revertCommit.String(),
		"wtSpendKeyIn":    aliceKey.PrivateKey.String(),
		"wtValueIn":       amountIn.String(),
		"wtSaltIn":        aliceSaltIn.String(),
		"wtTokenIdIn":     tokenIdIn.String(),
		"wtPathElements":  pathElemsFixed,
		"wtPathIndex":     pathIndices.String(),
		"wtSpendPkBob":    bobSpendPk.String(),
		"wtSaltB":         saltBField.String(),
		"wtValueBob":      amountOut.String(),
		"wtTokenIdBob":    tokenIdOut.String(),
		"wtSaltA":         saltAField.String(),
		"wtRevertSalt":    revertSalt.String(),
	}

	body, err := c.PostProof("/proof/dvpInitiator", payload)
	if err != nil {
		return nil, fmt.Errorf("ZkDvp Initiator proof request failed: %w", err)
	}

	var gnarkResp struct {
		Proof []json.Number `json:"proof"`
	}
	if parseErr := json.Unmarshal(body, &gnarkResp); parseErr != nil {
		return nil, fmt.Errorf("failed to parse ZkDvp proof response: %w", parseErr)
	}
	proofStrs := make([]string, len(gnarkResp.Proof))
	for i, n := range gnarkResp.Proof {
		proofStrs[i] = n.String()
	}

	return &ZkDvpSwapInitResult{
		AliceNullifier: nullifier,
		CommitmentB:    commitmentB,
		CommitmentA:    commitmentA,
		CipherText:     cipherText,
		EncTxData:      encTxData,
		SaltStar:       saltStar,
		SaltStarField:  saltStarField,
		Proof:          proofStrs,
	}, nil
}

// DvPInitiatorResult holds the output of DvPInitiatorProof.
type DvPInitiatorResult struct {
	Proof           []string   // 8-element Groth16 proof
	Statement       []*big.Int // [commitA, treeNum, root, nf_A, commitB, commitA, revertCommitA] (7 elements)
	NumberOfInputs  int        // always 1
	NumberOfOutputs int        // reported as 1 on-chain (only commitB inserted; commitA goes to ERC721 vault via Bob's proof)
	CipherText      []byte     // ML-KEM capsule for Bob (1088 bytes)
	EncTxData       []byte     // AES-256-GCM ciphertext of (tokenIdIn || valueIn)
	CommitB         *big.Int
	CommitA         *big.Int
	RevertCommitA   *big.Int
	SaltA           *big.Int // passed to Bob's DvPDestinationProof
}

// DvPInitiatorProof generates Alice's side of the DvP proof.
//
// stMessage is automatically set to commitA (Alice's expected output from Bob) so that
// the on-chain submitPartialSettlement cross-reference check works:
//
//	_pendingTransactions[commitB].targetReceiptId = commitA = statement[0]
//
// On-chain receipt should use NumberOfOutputs=1 so only commitB (statement[4])
// is inserted into the ERC20 vault. commitA goes into the ERC721 vault via
// Bob's DvPDestinationProof; revertCommitA is only inserted on swap timeout.
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
) (*DvPInitiatorResult, error) {
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

	revertSaltBytes, err := GenerateRandomValue(32)
	if err != nil {
		return nil, fmt.Errorf("RandomBytes (revert salt): %w", err)
	}
	revertSalt := SaltBToField(revertSaltBytes)

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
	revertCommitA, err := Erc20CommitmentV2(aliceKey.PublicKey, revertSalt, valueIn, tokenIdIn)
	if err != nil {
		return nil, fmt.Errorf("Erc20CommitmentV2 (revertCommitA): %w", err)
	}

	pathElems := make([]*big.Int, merkleDepth)
	copy(pathElems, merkleProof.Elements[:merkleDepth])

	// stMessage = commitA for on-chain cross-referencing:
	//   submitPartialSettlement stores _pendingTransactions[commitB].targetReceiptId = statement[0] = commitA
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

	var gnarkResp struct {
		Proof        []json.Number `json:"proof"`
		PublicSignal []json.Number `json:"publicSignal"`
	}
	if err := json.Unmarshal(body, &gnarkResp); err != nil {
		return nil, fmt.Errorf("failed to parse dvpInitiator proof response: %w", err)
	}
	proofStrs := make([]string, len(gnarkResp.Proof))
	for i, n := range gnarkResp.Proof {
		proofStrs[i] = n.String()
	}

	// Full 7-element statement for VK verification (DvP Initiator VK has IC[8]).
	// On-chain NumberOfOutputs is reported as 1 so only commitB (statement[4])
	// is inserted into the ERC20 vault during settlement.
	statement := []*big.Int{stMessage, stTreeNumber, merkleProof.Root, nf, commitB, commitA, revertCommitA}

	return &DvPInitiatorResult{
		Proof:           proofStrs,
		Statement:       statement,
		NumberOfInputs:  1,
		NumberOfOutputs: 1, // only commitB counts as payment output; commitA goes to ERC721 via Bob's proof
		CipherText:      cipherText,
		EncTxData:       encTxData,
		CommitB:         commitB,
		CommitA:         commitA,
		RevertCommitA:   revertCommitA,
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
) (*DvPInitiatorResult, error) {
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

	revertSaltBytes, err := GenerateRandomValue(32)
	if err != nil {
		return nil, fmt.Errorf("RandomBytes (revert salt): %w", err)
	}
	revertSalt := SaltBToField(revertSaltBytes)
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

	var gnarkResp struct {
		Proof        []json.Number `json:"proof"`
		PublicSignal []json.Number `json:"publicSignal"`
	}
	if err := json.Unmarshal(body, &gnarkResp); err != nil {
		return nil, fmt.Errorf("failed to parse dvpInitiatorFromSalts proof response: %w", err)
	}
	proofStrs := make([]string, len(gnarkResp.Proof))
	for i, n := range gnarkResp.Proof {
		proofStrs[i] = n.String()
	}

	statement := []*big.Int{stMessage, stTreeNumber, merkleProof.Root, nf, commitB, commitA, revertCommitA}
	return &DvPInitiatorResult{
		Proof:           proofStrs,
		Statement:       statement,
		NumberOfInputs:  1,
		NumberOfOutputs: 1,
		SaltA:           saltA,
		CommitB:         commitB,
		CommitA:         commitA,
		RevertCommitA:   revertCommitA,
	}, nil
}

// DvPDestinationResult holds the output of DvPDestinationProof.
type DvPDestinationResult struct {
	Proof           []string   // 8-element Groth16 proof
	Statement       []*big.Int // [commitB, treeNum, root, nf_B, commitA]
	NumberOfInputs  int        // always 1
	NumberOfOutputs int        // always 1
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
) (*DvPDestinationResult, error) {
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

	var gnarkResp struct {
		Proof        []json.Number `json:"proof"`
		PublicSignal []json.Number `json:"publicSignal"`
	}
	if err := json.Unmarshal(body, &gnarkResp); err != nil {
		return nil, fmt.Errorf("failed to parse dvpDestination proof response: %w", err)
	}
	proofStrs := make([]string, len(gnarkResp.Proof))
	for i, n := range gnarkResp.Proof {
		proofStrs[i] = n.String()
	}

	statement := []*big.Int{commitB, stTreeNumber, merkleProof.Root, nf, commitA}
	return &DvPDestinationResult{
		Proof:           proofStrs,
		Statement:       statement,
		NumberOfInputs:  1,
		NumberOfOutputs: 1,
	}, nil
}
