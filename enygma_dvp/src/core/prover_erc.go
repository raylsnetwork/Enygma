package core

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/iden3/go-iden3-crypto/poseidon"
)

// Erc20JoinSplitProof generates an ERC20 JoinSplit proof for the non-interactive flow.
//
// For inputs the caller provides:
//   - keysIn       — spend key pairs (sk_spend used for nullifier + commitment check)
//   - wtSaltsIn    — saltB field elements from when each input note was received
//
// For outputs the caller provides:
//   - recipientSpendPks      — pk_spend of each recipient (goes into commitment)
//   - recipientViewEncapKeys — ML-KEM encapsulation keys of each recipient (1184 bytes each)
//   - wtTokenId              — token identifier shared across all notes
//
// The function runs Encapsulate per output to derive saltB, then:
//   - computes Erc20CommitmentV2(pk_spend, saltB_field, amount, tokenId)
//   - encrypts tokenId||amount with ChaCha20-Poly1305 keyed by saltB
//   - carries both ciphertexts in ProofResult for on-chain publication
func (c *GnarkClient) Erc20JoinSplitProof(
	stMessage *big.Int,
	wtValuesIn []*big.Int,
	keysIn []KeyPair,
	wtSaltsIn []*big.Int,
	wtValuesOut []*big.Int,
	recipientSpendPks []*big.Int,
	recipientViewEncapKeys [][]byte,
	merkleDepth int,
	merkleProofs []*MerkleProof,
	stTreeNumbers []*big.Int,
	wtTokenId *big.Int,
	use10_2 bool,
) (*ProofResult, error) {
	nIn := len(wtValuesIn)
	nOut := len(wtValuesOut)

	// --- inputs: nullifiers and Merkle paths ---
	stNullifiers := make([]*big.Int, nIn)
	wtPathIndices := make([]*big.Int, nIn)
	wtPathElements := make([]*big.Int, 0)

	for i := 0; i < nIn; i++ {
		// Verify local commitment matches what should be in the tree (V2 layout)
		_, err := Erc20CommitmentV2(keysIn[i].PublicKey, wtSaltsIn[i], wtValuesIn[i], wtTokenId)
		if err != nil {
			return nil, fmt.Errorf("failed to compute erc20CommitmentV2 for input %d: %w", i, err)
		}

		if wtValuesIn[i].Sign() == 0 {
			// dummy input — zero out path
			wtPathIndices[i] = big.NewInt(0)
			zeros := make([]*big.Int, merkleDepth)
			for j := range zeros {
				zeros[j] = big.NewInt(0)
			}
			wtPathElements = append(wtPathElements, zeros...)
		} else {
			wtPathIndices[i] = merkleProofs[i].Indices
			wtPathElements = append(wtPathElements, merkleProofs[i].Elements...)
		}

		// HIGH-1 fix: incorporate tree number to prevent cross-tree double-spend.
		nullifier, err := GetNullifierWithTree(keysIn[i].PrivateKey, stTreeNumbers[i], wtPathIndices[i], merkleDepth)
		if err != nil {
			return nil, fmt.Errorf("failed to compute nullifier for input %d: %w", i, err)
		}
		stNullifiers[i] = nullifier
	}

	// --- outputs: KEM encapsulation, AEAD encryption, V2 commitments ---
	wtSaltsOut := make([]*big.Int, nOut)
	stCommitmentsOut := make([]*big.Int, nOut)
	cipherText := make([][]byte, nOut)
	encTxData := make([][]byte, nOut)

	for i := 0; i < nOut; i++ {
		// Encapsulate using recipient's view public key → raw shared secret ss
		ss, ctI, err := Encapsulate(recipientViewEncapKeys[i])
		if err != nil {
			return nil, fmt.Errorf("failed to encapsulate for output %d: %w", i, err)
		}
		cipherText[i] = ctI

		// HKDF-derive commitment salt and AES-GCM encryption key from ss
		saltB, err := DerivePaymentSalt(ss)
		if err != nil {
			return nil, fmt.Errorf("failed to derive payment salt for output %d: %w", i, err)
		}
		encKey, err := DerivePaymentKey(ss)
		if err != nil {
			return nil, fmt.Errorf("failed to derive payment key for output %d: %w", i, err)
		}

		saltBField := SaltBToField(saltB)
		wtSaltsOut[i] = saltBField

		// Encrypt tokenId||amount with AES-GCM so the recipient can learn what was sent
		ctII, err := EncryptPayload(encKey, wtTokenId, wtValuesOut[i])
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt payload for output %d: %w", i, err)
		}
		encTxData[i] = ctII

		// V2 commitment: Poseidon(pk_spendRecipient, saltB_field, amount, tokenId)
		cmt, err := Erc20CommitmentV2(recipientSpendPks[i], saltBField, wtValuesOut[i], wtTokenId)
		if err != nil {
			return nil, fmt.Errorf("failed to compute erc20CommitmentV2 for output %d: %w", i, err)
		}
		stCommitmentsOut[i] = cmt
	}

	// --- Merkle roots (zero for dummy inputs) ---
	stMerkleRoots := make([]*big.Int, nIn)
	for i := range wtValuesIn {
		if wtValuesIn[i].Sign() == 0 {
			stMerkleRoots[i] = big.NewInt(0)
		} else {
			stMerkleRoots[i] = merkleProofs[i].Root
		}
	}

	pathElementChunks := chunkBigIntSlice(wtPathElements, merkleDepth)

	payload := map[string]interface{}{
		"StMessage":            stMessage.String(),
		"StTreeNumber":         bigIntSliceToStrings(stTreeNumbers),
		"StMerkleRoots":        bigIntSliceToStrings(stMerkleRoots),
		"StNullifiers":         bigIntSliceToStrings(stNullifiers),
		"StCommitmentOut":      bigIntSliceToStrings(stCommitmentsOut),
		"WtPrivateKeysIn":      bigIntSliceToStrings(extractPrivateKeys(keysIn)),
		"WtValuesIn":           bigIntSliceToStrings(wtValuesIn),
		"WtSaltsIn":            bigIntSliceToStrings(wtSaltsIn),
		"WtPathElements":       bigIntChunksToStringChunks(pathElementChunks),
		"WtPathIndices":        bigIntSliceToStrings(wtPathIndices),
		"WtTokenId":            wtTokenId.String(),
		"WtSpendPublicKeysOut": bigIntSliceToStrings(recipientSpendPks),
		"WtValuesOut":          bigIntSliceToStrings(wtValuesOut),
		"WtSaltsOut":           bigIntSliceToStrings(wtSaltsOut),
	}

	endpoint := "/proof/joinSplitERC20"
	if use10_2 {
		endpoint = "/proof/joinSplitERC20_10_2"
	}

	body, err := c.PostProof(endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("erc20JoinSplit proof request failed: %w", err)
	}

	// The gnark handler marshals []*big.Int as JSON numbers (big.Int.MarshalJSON returns
	// raw decimal bytes). Use json.Number to accept either JSON number or JSON string.
	var gnarkResp struct {
		Proof []json.Number `json:"proof"`
	}
	if parseErr := json.Unmarshal(body, &gnarkResp); parseErr != nil {
		return nil, fmt.Errorf("failed to parse joinSplit proof response: %w", parseErr)
	}
	proofStrs := make([]string, len(gnarkResp.Proof))
	for i, n := range gnarkResp.Proof {
		proofStrs[i] = n.String()
	}

	// Statement: [message, tree[0], root[0], null[0], ..., commit[0], commit[1]]
	statement := make([]*big.Int, 0, 1+3*nIn+nOut)
	statement = append(statement, stMessage)
	for i := 0; i < nIn; i++ {
		statement = append(statement, stTreeNumbers[i], stMerkleRoots[i], stNullifiers[i])
	}
	statement = append(statement, stCommitmentsOut...)

	return &ProofResult{
		Proof:           proofStrs,
		Statement:       statement,
		NumberOfInputs:  nIn,
		NumberOfOutputs: nOut,
		CipherText:     cipherText,
		EncTxData:    encTxData,
	}, nil
}

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
		CipherText:    cipherText,
		EncTxData:   encTxData,
		SaltStar:       saltStar,
		SaltStarField:  saltStarField,
		Proof:          proofStrs,
	}, nil
}

// Erc20WithdrawProof generates an ERC20 withdrawal proof for the V2 non-interactive flow.
//
// The withdrawal output note uses a fixed salt of 0 (no KEM needed — the commitment
// is public) and encodes the recipient address as pk_spend:
//
//	withdrawal commitment = Poseidon4(uint160(recipient), 0, withdrawAmount, tokenId)
//
// The dummy second output uses pk_spend=dummySpendPk with salt=0 and amount=0.
func (c *GnarkClient) Erc20WithdrawProof(
	stMessage *big.Int,
	wtValuesIn []*big.Int,
	keysIn []KeyPair,
	wtSaltsIn []*big.Int,
	withdrawAmount *big.Int,
	recipient *big.Int, // uint160(recipient address)
	dummySpendPk *big.Int, // dummy second output pk
	merkleDepth int,
	merkleProofs []*MerkleProof,
	stTreeNumbers []*big.Int,
	wtTokenId *big.Int,
	use10_2 bool,
) (*ProofResult, error) {
	nIn := len(wtValuesIn)

	// --- inputs: nullifiers and Merkle paths ---
	stNullifiers := make([]*big.Int, nIn)
	wtPathIndices := make([]*big.Int, nIn)
	wtPathElements := make([]*big.Int, 0)

	for i := 0; i < nIn; i++ {
		_, err := Erc20CommitmentV2(keysIn[i].PublicKey, wtSaltsIn[i], wtValuesIn[i], wtTokenId)
		if err != nil {
			return nil, fmt.Errorf("failed to compute erc20CommitmentV2 for input %d: %w", i, err)
		}

		if wtValuesIn[i].Sign() == 0 {
			wtPathIndices[i] = big.NewInt(0)
			zeros := make([]*big.Int, merkleDepth)
			for j := range zeros {
				zeros[j] = big.NewInt(0)
			}
			wtPathElements = append(wtPathElements, zeros...)
		} else {
			wtPathIndices[i] = merkleProofs[i].Indices
			wtPathElements = append(wtPathElements, merkleProofs[i].Elements...)
		}

		// HIGH-1 fix: incorporate tree number to prevent cross-tree double-spend.
		nullifier, err := GetNullifierWithTree(keysIn[i].PrivateKey, stTreeNumbers[i], wtPathIndices[i], merkleDepth)
		if err != nil {
			return nil, fmt.Errorf("failed to compute nullifier for input %d: %w", i, err)
		}
		stNullifiers[i] = nullifier
	}

	// --- outputs: fixed salt=0 for withdrawal (no KEM), dummy second output ---
	zero := big.NewInt(0)
	wtSaltsOut := []*big.Int{zero, zero}

	// Withdrawal output commitment: Poseidon4(uint160(recipient), 0, withdrawAmount, tokenId)
	withdrawCommitment, err := Erc20CommitmentV2(recipient, zero, withdrawAmount, wtTokenId)
	if err != nil {
		return nil, fmt.Errorf("failed to compute withdrawal commitment: %w", err)
	}

	// Dummy output commitment: Poseidon4(dummySpendPk, 0, 0, tokenId)
	dummyCommitment, err := Erc20CommitmentV2(dummySpendPk, zero, zero, wtTokenId)
	if err != nil {
		return nil, fmt.Errorf("failed to compute dummy commitment: %w", err)
	}

	stCommitmentsOut := []*big.Int{withdrawCommitment, dummyCommitment}
	wtSpendPublicKeysOut := []*big.Int{recipient, dummySpendPk}
	wtValuesOut := []*big.Int{withdrawAmount, zero}

	// --- Merkle roots (zero for dummy inputs) ---
	stMerkleRoots := make([]*big.Int, nIn)
	for i := range wtValuesIn {
		if wtValuesIn[i].Sign() == 0 {
			stMerkleRoots[i] = big.NewInt(0)
		} else {
			stMerkleRoots[i] = merkleProofs[i].Root
		}
	}

	pathElementChunks := chunkBigIntSlice(wtPathElements, merkleDepth)

	payload := map[string]interface{}{
		"StMessage":            stMessage.String(),
		"StTreeNumber":         bigIntSliceToStrings(stTreeNumbers),
		"StMerkleRoots":        bigIntSliceToStrings(stMerkleRoots),
		"StNullifiers":         bigIntSliceToStrings(stNullifiers),
		"StCommitmentOut":      bigIntSliceToStrings(stCommitmentsOut),
		"WtPrivateKeysIn":      bigIntSliceToStrings(extractPrivateKeys(keysIn)),
		"WtValuesIn":           bigIntSliceToStrings(wtValuesIn),
		"WtSaltsIn":            bigIntSliceToStrings(wtSaltsIn),
		"WtPathElements":       bigIntChunksToStringChunks(pathElementChunks),
		"WtPathIndices":        bigIntSliceToStrings(wtPathIndices),
		"WtTokenId":            wtTokenId.String(),
		"WtSpendPublicKeysOut": bigIntSliceToStrings(wtSpendPublicKeysOut),
		"WtValuesOut":          bigIntSliceToStrings(wtValuesOut),
		"WtSaltsOut":           bigIntSliceToStrings(wtSaltsOut),
	}

	endpoint := "/proof/joinSplitERC20"
	if use10_2 {
		endpoint = "/proof/joinSplitERC20_10_2"
	}

	body2, err := c.PostProof(endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("erc20Withdraw proof request failed: %w", err)
	}

	var gnarkResp2 struct {
		Proof []json.Number `json:"proof"`
	}
	if parseErr := json.Unmarshal(body2, &gnarkResp2); parseErr != nil {
		return nil, fmt.Errorf("failed to parse withdraw proof response: %w", parseErr)
	}
	proofStrs2 := make([]string, len(gnarkResp2.Proof))
	for i, n := range gnarkResp2.Proof {
		proofStrs2[i] = n.String()
	}

	// Statement: [message, tree[0], root[0], null[0], ..., commit[0], commit[1]]
	statement := make([]*big.Int, 0, 1+3*nIn+2)
	statement = append(statement, stMessage)
	for i := 0; i < nIn; i++ {
		statement = append(statement, stTreeNumbers[i], stMerkleRoots[i], stNullifiers[i])
	}
	statement = append(statement, stCommitmentsOut...)

	return &ProofResult{
		Proof:           proofStrs2,
		Statement:       statement,
		NumberOfInputs:  nIn,
		NumberOfOutputs: 2,
	}, nil
}

// Erc721OwnershipProof generates a strongly-typed ERC721 ownership proof.
// wtSaltIn is the salt used when the input note was originally created.
// A fresh salt for the output note is derived via ML-KEM encapsulation using recipientViewEncapKey.
func (c *GnarkClient) Erc721OwnershipProof(
	stMessage *big.Int,
	wtValue *big.Int, keyIn KeyPair, wtSaltIn *big.Int, keyOut KeyPair,
	recipientViewEncapKey []byte,
	merkleDepth int, merkleProof *MerkleProof,
	stTreeNumber *big.Int,
	wtErc721ContractAddress *big.Int,
) (*ProofResult, error) {
	_, err := Erc721Commitment(wtValue, keyIn.PublicKey, wtSaltIn)
	if err != nil {
		return nil, fmt.Errorf("failed to compute erc721Commitment for input: %w", err)
	}

	// HIGH-1 fix: incorporate tree number to prevent cross-tree double-spend.
	nullifier, err := GetNullifierWithTree(keyIn.PrivateKey, stTreeNumber, merkleProof.Indices, merkleDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to compute nullifier: %w", err)
	}

	// Generate fresh salt for output note via ML-KEM. recipientViewEncapKey is mandatory.
	if recipientViewEncapKey == nil {
		return nil, fmt.Errorf("recipientViewEncapKey is required for non-interactive note delivery")
	}
	ss, ctI, kemErr := Encapsulate(recipientViewEncapKey)
	if kemErr != nil {
		return nil, fmt.Errorf("failed to encapsulate for output: %w", kemErr)
	}
	saltB, err := DerivePaymentSalt(ss)
	if err != nil {
		return nil, fmt.Errorf("failed to derive payment salt for output: %w", err)
	}
	encKey, err := DerivePaymentKey(ss)
	if err != nil {
		return nil, fmt.Errorf("failed to derive payment key for output: %w", err)
	}
	wtSaltOut := SaltBToField(saltB)
	ctII, err := EncryptPayload(encKey, wtErc721ContractAddress, wtValue)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt payload for output: %w", err)
	}

	commitmentOut, err := Erc721Commitment(wtValue, keyOut.PublicKey, wtSaltOut)
	if err != nil {
		return nil, fmt.Errorf("failed to compute erc721Commitment for output: %w", err)
	}

	payload := map[string]interface{}{
		"StMessage":               stMessage.String(),
		"StTreeNumbers":           []string{stTreeNumber.String()},
		"StMerkleRoots":           []string{merkleProof.Root.String()},
		"StNullifiers":            []string{nullifier.String()},
		"StCommitmentOut":         []string{commitmentOut.String()},
		"WtPrivateKeysIn":         []string{keyIn.PrivateKey.String()},
		"WtValues":                []string{wtValue.String()},
		"WtErc721ContractAddress": wtErc721ContractAddress.String(),
		"WtPathElements":          [][]string{bigIntSliceToStrings(merkleProof.Elements)},
		"WtPathIndices":           []string{merkleProof.Indices.String()},
		"WtPublicKeysOut":         []string{keyOut.PublicKey.String()},
		"WtSaltsIn":               []string{wtSaltIn.String()},
		"WtSaltsOut":              []string{wtSaltOut.String()},
	}

	body721, err := c.PostProof("/proof/ownershipERC721", payload)
	if err != nil {
		return nil, fmt.Errorf("erc721Ownership proof request failed: %w", err)
	}

	var gnarkResp721 struct {
		Proof []json.Number `json:"proof"`
	}
	if parseErr := json.Unmarshal(body721, &gnarkResp721); parseErr != nil {
		return nil, fmt.Errorf("failed to parse erc721Ownership proof response: %w", parseErr)
	}
	proofStrs721 := make([]string, len(gnarkResp721.Proof))
	for i, n := range gnarkResp721.Proof {
		proofStrs721[i] = n.String()
	}

	// Statement: [message, treeNumber, merkleRoot, nullifier, commitmentOut]
	statement := []*big.Int{
		stMessage,
		stTreeNumber,
		merkleProof.Root,
		nullifier,
		commitmentOut,
	}

	return &ProofResult{
		Proof:           proofStrs721,
		Statement:       statement,
		NumberOfInputs:  1,
		NumberOfOutputs: 1,
		SaltsOut:        []*big.Int{wtSaltOut},
		CipherText:     [][]byte{ctI},
		EncTxData:    [][]byte{ctII},
	}, nil
}

// Erc721OwnershipProofFromSalt is like Erc721OwnershipProof but accepts a
// pre-computed output salt (wtSaltOut) and the corresponding ciphertexts instead of
// performing a fresh ML-KEM encapsulation.  Use this when the output commitment must
// be known before generating the proof — e.g. for atomic DVP swaps where the
// on-chain contract requires cross-commitment consistency between both parties' proofs.
func (c *GnarkClient) Erc721OwnershipProofFromSalt(
	stMessage *big.Int,
	wtValue *big.Int, keyIn KeyPair, wtSaltIn *big.Int, keyOut KeyPair,
	wtSaltOut *big.Int, ctI []byte, ctII []byte,
	merkleDepth int, merkleProof *MerkleProof,
	stTreeNumber *big.Int,
	wtErc721ContractAddress *big.Int,
) (*ProofResult, error) {
	_, err := Erc721Commitment(wtValue, keyIn.PublicKey, wtSaltIn)
	if err != nil {
		return nil, fmt.Errorf("failed to compute erc721Commitment for input: %w", err)
	}

	// HIGH-1 fix: incorporate tree number to prevent cross-tree double-spend.
	nullifier, err := GetNullifierWithTree(keyIn.PrivateKey, stTreeNumber, merkleProof.Indices, merkleDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to compute nullifier: %w", err)
	}

	commitmentOut, err := Erc721Commitment(wtValue, keyOut.PublicKey, wtSaltOut)
	if err != nil {
		return nil, fmt.Errorf("failed to compute erc721Commitment for output: %w", err)
	}

	payload := map[string]interface{}{
		"StMessage":               stMessage.String(),
		"StTreeNumbers":           []string{stTreeNumber.String()},
		"StMerkleRoots":           []string{merkleProof.Root.String()},
		"StNullifiers":            []string{nullifier.String()},
		"StCommitmentOut":         []string{commitmentOut.String()},
		"WtPrivateKeysIn":         []string{keyIn.PrivateKey.String()},
		"WtValues":                []string{wtValue.String()},
		"WtErc721ContractAddress": wtErc721ContractAddress.String(),
		"WtPathElements":          [][]string{bigIntSliceToStrings(merkleProof.Elements)},
		"WtPathIndices":           []string{merkleProof.Indices.String()},
		"WtPublicKeysOut":         []string{keyOut.PublicKey.String()},
		"WtSaltsIn":               []string{wtSaltIn.String()},
		"WtSaltsOut":              []string{wtSaltOut.String()},
	}

	body, err := c.PostProof("/proof/ownershipERC721", payload)
	if err != nil {
		return nil, fmt.Errorf("erc721OwnershipFromSalt proof request failed: %w", err)
	}

	var gnarkResp struct {
		Proof []json.Number `json:"proof"`
	}
	if parseErr := json.Unmarshal(body, &gnarkResp); parseErr != nil {
		return nil, fmt.Errorf("failed to parse erc721OwnershipFromSalt proof response: %w", parseErr)
	}
	proofStrs := make([]string, len(gnarkResp.Proof))
	for i, n := range gnarkResp.Proof {
		proofStrs[i] = n.String()
	}

	statement := []*big.Int{
		stMessage,
		stTreeNumber,
		merkleProof.Root,
		nullifier,
		commitmentOut,
	}

	return &ProofResult{
		Proof:           proofStrs,
		Statement:       statement,
		NumberOfInputs:  1,
		NumberOfOutputs: 1,
		SaltsOut:        []*big.Int{wtSaltOut},
		CipherText:     [][]byte{ctI},
		EncTxData:    [][]byte{ctII},
	}, nil
}

// Erc20JoinSplitProofFromSalts is like Erc20JoinSplitProof but accepts pre-computed
// output salts and ciphertexts instead of performing ML-KEM encapsulation.
// Use this when output commitments must be known before proof generation — e.g. for
// atomic DVP swaps where cross-commitment consistency is required.
func (c *GnarkClient) Erc20JoinSplitProofFromSalts(
	stMessage *big.Int,
	wtValuesIn []*big.Int,
	keysIn []KeyPair,
	wtSaltsIn []*big.Int,
	wtValuesOut []*big.Int,
	recipientSpendPks []*big.Int,
	wtSaltsOut []*big.Int,
	cipherText [][]byte,
	encTxData [][]byte,
	merkleDepth int,
	merkleProofs []*MerkleProof,
	stTreeNumbers []*big.Int,
	wtTokenId *big.Int,
	use10_2 bool,
) (*ProofResult, error) {
	nIn := len(wtValuesIn)
	nOut := len(wtValuesOut)

	stNullifiers := make([]*big.Int, nIn)
	wtPathIndices := make([]*big.Int, nIn)
	wtPathElements := make([]*big.Int, 0)

	for i := 0; i < nIn; i++ {
		if wtValuesIn[i].Sign() == 0 {
			wtPathIndices[i] = big.NewInt(0)
			zeros := make([]*big.Int, merkleDepth)
			for j := range zeros {
				zeros[j] = big.NewInt(0)
			}
			wtPathElements = append(wtPathElements, zeros...)
		} else {
			wtPathIndices[i] = merkleProofs[i].Indices
			wtPathElements = append(wtPathElements, merkleProofs[i].Elements...)
		}

		// HIGH-1 fix: incorporate tree number to prevent cross-tree double-spend.
		nullifier, err := GetNullifierWithTree(keysIn[i].PrivateKey, stTreeNumbers[i], wtPathIndices[i], merkleDepth)
		if err != nil {
			return nil, fmt.Errorf("failed to compute nullifier for input %d: %w", i, err)
		}
		stNullifiers[i] = nullifier
	}

	stCommitmentsOut := make([]*big.Int, nOut)
	for i := 0; i < nOut; i++ {
		cmt, err := Erc20CommitmentV2(recipientSpendPks[i], wtSaltsOut[i], wtValuesOut[i], wtTokenId)
		if err != nil {
			return nil, fmt.Errorf("failed to compute erc20CommitmentV2 for output %d: %w", i, err)
		}
		stCommitmentsOut[i] = cmt
	}

	stMerkleRoots := make([]*big.Int, nIn)
	for i := range wtValuesIn {
		if wtValuesIn[i].Sign() == 0 {
			stMerkleRoots[i] = big.NewInt(0)
		} else {
			stMerkleRoots[i] = merkleProofs[i].Root
		}
	}

	pathElementChunks := chunkBigIntSlice(wtPathElements, merkleDepth)

	payload := map[string]interface{}{
		"StMessage":            stMessage.String(),
		"StTreeNumber":         bigIntSliceToStrings(stTreeNumbers),
		"StMerkleRoots":        bigIntSliceToStrings(stMerkleRoots),
		"StNullifiers":         bigIntSliceToStrings(stNullifiers),
		"StCommitmentOut":      bigIntSliceToStrings(stCommitmentsOut),
		"WtPrivateKeysIn":      bigIntSliceToStrings(extractPrivateKeys(keysIn)),
		"WtValuesIn":           bigIntSliceToStrings(wtValuesIn),
		"WtSaltsIn":            bigIntSliceToStrings(wtSaltsIn),
		"WtPathElements":       bigIntChunksToStringChunks(pathElementChunks),
		"WtPathIndices":        bigIntSliceToStrings(wtPathIndices),
		"WtTokenId":            wtTokenId.String(),
		"WtSpendPublicKeysOut": bigIntSliceToStrings(recipientSpendPks),
		"WtValuesOut":          bigIntSliceToStrings(wtValuesOut),
		"WtSaltsOut":           bigIntSliceToStrings(wtSaltsOut),
	}

	endpoint := "/proof/joinSplitERC20"
	if use10_2 {
		endpoint = "/proof/joinSplitERC20_10_2"
	}

	body, err := c.PostProof(endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("erc20JoinSplitFromSalts proof request failed: %w", err)
	}

	var gnarkResp struct {
		Proof []json.Number `json:"proof"`
	}
	if parseErr := json.Unmarshal(body, &gnarkResp); parseErr != nil {
		return nil, fmt.Errorf("failed to parse erc20JoinSplitFromSalts proof response: %w", parseErr)
	}
	proofStrs := make([]string, len(gnarkResp.Proof))
	for i, n := range gnarkResp.Proof {
		proofStrs[i] = n.String()
	}

	statement := make([]*big.Int, 0, 1+3*nIn+nOut)
	statement = append(statement, stMessage)
	for i := 0; i < nIn; i++ {
		statement = append(statement, stTreeNumbers[i], stMerkleRoots[i], stNullifiers[i])
	}
	statement = append(statement, stCommitmentsOut...)

	return &ProofResult{
		Proof:           proofStrs,
		Statement:       statement,
		NumberOfInputs:  nIn,
		NumberOfOutputs: nOut,
		CipherText:     cipherText,
		EncTxData:    encTxData,
	}, nil
}

// Erc1155FungibleJoinSplitProof generates a strongly-typed ERC1155 fungible JoinSplit proof.
// wtSaltsIn must contain the salt used when each input note was originally created.
// Output salts are derived via ML-KEM encapsulation using recipientViewEncapKeys.
func (c *GnarkClient) Erc1155FungibleJoinSplitProof(
	stMessage *big.Int,
	wtValuesIn []*big.Int, keysIn []KeyPair, wtSaltsIn []*big.Int,
	wtValuesOut []*big.Int, keysOut []KeyPair,
	recipientViewEncapKeys [][]byte,
	merkleDepth int, merkleProofs []*MerkleProof,
	stTreeNumbers []*big.Int,
	wtErc1155ContractAddress *big.Int, wtErc1155TokenId *big.Int,
	stAssetGroupTreeNumber *big.Int, assetGroupMerkleProof *MerkleProof,
) (*ProofResult, error) {
	stCommitmentsOut, wtSaltsOut, ctI, ctII, stNullifiers, wtPathIndices, wtPathElements, err := prepareErc1155ProofParams(
		wtValuesIn, keysIn, wtSaltsIn, wtValuesOut, keysOut, recipientViewEncapKeys, merkleDepth, merkleProofs,
		wtErc1155ContractAddress, wtErc1155TokenId,
	)
	if err != nil {
		return nil, err
	}

	stMerkleRoots := make([]*big.Int, len(wtValuesIn))
	for i := range wtValuesIn {
		if wtValuesIn[i].Sign() == 0 {
			stMerkleRoots[i] = big.NewInt(0)
		} else {
			stMerkleRoots[i] = merkleProofs[i].Root
		}
	}

	stAssetGroupMerkleRoot := assetGroupMerkleProof.Root

	pathElementChunks := chunkBigIntSlice(wtPathElements, merkleDepth)

	payload := map[string]interface{}{
		"StMessage":                stMessage.String(),
		"StTreeNumbers":            bigIntSliceToStrings(stTreeNumbers),
		"StMerkleRoots":            bigIntSliceToStrings(stMerkleRoots),
		"StNullifiers":             bigIntSliceToStrings(stNullifiers),
		"StCommitmentOut":          bigIntSliceToStrings(stCommitmentsOut),
		"StAssetGroupMerkleRoot":   stAssetGroupMerkleRoot.String(),
		"StAssetGroupTreeNumber":   stAssetGroupTreeNumber.String(),
		"WtPrivateKeysIn":          bigIntSliceToStrings(extractPrivateKeys(keysIn)),
		"WtValuesIn":               bigIntSliceToStrings(wtValuesIn),
		"WtPathElements":           bigIntChunksToStringChunks(pathElementChunks),
		"WtPathIndices":            bigIntSliceToStrings(wtPathIndices),
		"WtErc1155ContractAddress": wtErc1155ContractAddress.String(),
		"WtErc1155TokenId":         wtErc1155TokenId.String(),
		"WtPublicKeysOut":          bigIntSliceToStrings(extractPublicKeys(keysOut)),
		"WtValuesOut":              bigIntSliceToStrings(wtValuesOut),
		"WtAssetGroupPathElements": bigIntSliceToStrings(assetGroupMerkleProof.Elements),
		"WtAssetGroupPathIndices":  assetGroupMerkleProof.Indices.String(),
		"WtSaltsIn":                bigIntSliceToStrings(wtSaltsIn),
		"WtSaltsOut":               bigIntSliceToStrings(wtSaltsOut),
	}

	bodyFung, err := c.PostProof("/proof/erc155Fungible", payload)
	if err != nil {
		return nil, fmt.Errorf("erc1155FungibleJoinSplit proof request failed: %w", err)
	}

	var gnarkRespFung struct {
		Proof []json.Number `json:"proof"`
	}
	if parseErr := json.Unmarshal(bodyFung, &gnarkRespFung); parseErr != nil {
		return nil, fmt.Errorf("failed to parse erc1155FungibleJoinSplit proof response: %w", parseErr)
	}
	proofStrsFung := make([]string, len(gnarkRespFung.Proof))
	for i, n := range gnarkRespFung.Proof {
		proofStrsFung[i] = n.String()
	}

	// Statement (interleaved per input, then commitments — no asset group in public signal):
	// [message, tree[0], root[0], null[0], tree[1], root[1], null[1], commit[0], commit[1]]
	statement := make([]*big.Int, 0, 1+3*len(wtValuesIn)+len(keysOut))
	statement = append(statement, stMessage)
	for i := 0; i < len(wtValuesIn); i++ {
		statement = append(statement, stTreeNumbers[i], stMerkleRoots[i], stNullifiers[i])
	}
	statement = append(statement, stCommitmentsOut...)

	return &ProofResult{
		Proof:           proofStrsFung,
		Statement:       statement,
		NumberOfInputs:  len(wtValuesIn),
		NumberOfOutputs: len(wtValuesOut),
		SaltsOut:        wtSaltsOut,
		CipherText:     ctI,
		EncTxData:    ctII,
	}, nil
}

// Erc1155NonFungibleOwnershipProof generates a strongly-typed ERC1155 non-fungible ownership proof.
// wtSaltIn is the salt used when the input note was originally created.
// A fresh salt for the output note is derived via ML-KEM encapsulation using recipientViewEncapKey.
func (c *GnarkClient) Erc1155NonFungibleOwnershipProof(
	stMessage *big.Int,
	wtValue *big.Int, keyIn KeyPair, wtSaltIn *big.Int, keyOut KeyPair,
	recipientViewEncapKey []byte,
	merkleDepth int, merkleProof *MerkleProof,
	stTreeNumber *big.Int,
	wtErc1155ContractAddress *big.Int, wtErc1155TokenId *big.Int,
	stAssetGroupTreeNumber *big.Int, assetGroupMerkleProof *MerkleProof,
) (*ProofResult, error) {
	// HIGH-1 fix: incorporate tree number to prevent cross-tree double-spend.
	nullifier, err := GetNullifierWithTree(keyIn.PrivateKey, stTreeNumber, merkleProof.Indices, merkleDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to compute nullifier: %w", err)
	}

	// Generate fresh salt for output note via ML-KEM. recipientViewEncapKey is mandatory.
	if recipientViewEncapKey == nil {
		return nil, fmt.Errorf("recipientViewEncapKey is required for non-interactive note delivery")
	}
	ss, ctI, kemErr := Encapsulate(recipientViewEncapKey)
	if kemErr != nil {
		return nil, fmt.Errorf("failed to encapsulate for output: %w", kemErr)
	}
	saltB, saltErr := DerivePaymentSalt(ss)
	if saltErr != nil {
		return nil, fmt.Errorf("failed to derive payment salt for output: %w", saltErr)
	}
	encKey, keyErr := DerivePaymentKey(ss)
	if keyErr != nil {
		return nil, fmt.Errorf("failed to derive payment key for output: %w", keyErr)
	}
	wtSaltOut := SaltBToField(saltB)
	ctII, encErr := EncryptPayload(encKey, wtErc1155TokenId, wtValue)
	if encErr != nil {
		return nil, fmt.Errorf("failed to encrypt payload for output: %w", encErr)
	}

	commitmentOut, err := Erc1155Commitment(wtErc1155TokenId, wtValue, keyOut.PublicKey, wtSaltOut)
	if err != nil {
		return nil, fmt.Errorf("failed to compute erc1155Commitment for output: %w", err)
	}

	stAssetGroupMerkleRoot := assetGroupMerkleProof.Root

	payload := map[string]interface{}{
		"StMessage":                stMessage.String(),
		"StTreeNumbers":            []string{stTreeNumber.String()},
		"StMerkleRoots":            []string{merkleProof.Root.String()},
		"StNullifiers":             []string{nullifier.String()},
		"StCommitmentOut":          []string{commitmentOut.String()},
		"StAssetGroupTreeNumber":   []string{stAssetGroupTreeNumber.String()},
		"StAssetGroupMerkleRoot":   []string{stAssetGroupMerkleRoot.String()},
		"WtPrivateKeysIn":          []string{keyIn.PrivateKey.String()},
		"WtValues":                 []string{wtValue.String()},
		"WtPathElements":           [][]string{bigIntSliceToStrings(merkleProof.Elements)},
		"WtPathIndices":            []string{merkleProof.Indices.String()},
		"WtErc1155TokenId":         []string{wtErc1155TokenId.String()},
		"WtPublicKeysOut":          []string{keyOut.PublicKey.String()},
		"WtErc1155ContractAddress": wtErc1155ContractAddress.String(),
		"WtAssetGroupPathElements": [][]string{bigIntSliceToStrings(assetGroupMerkleProof.Elements)},
		"WtAssetGroupPathIndices":  []string{assetGroupMerkleProof.Indices.String()},
		"WtSaltsIn":                []string{wtSaltIn.String()},
		"WtSaltsOut":               []string{wtSaltOut.String()},
	}

	bodyNF, err := c.PostProof("/proof/erc1155NonFungible", payload)
	if err != nil {
		return nil, fmt.Errorf("erc1155NonFungibleOwnership proof request failed: %w", err)
	}

	var gnarkRespNF struct {
		Proof []json.Number `json:"proof"`
	}
	if parseErr := json.Unmarshal(bodyNF, &gnarkRespNF); parseErr != nil {
		return nil, fmt.Errorf("failed to parse erc1155NonFungibleOwnership proof response: %w", parseErr)
	}
	proofStrsNF := make([]string, len(gnarkRespNF.Proof))
	for i, n := range gnarkRespNF.Proof {
		proofStrsNF[i] = n.String()
	}

	// Statement: [message, treeNumber, merkleRoot, nullifier, commitmentOut,
	//   assetGroupTreeNumber, assetGroupMerkleRoot]
	statement := []*big.Int{
		stMessage,
		stTreeNumber,
		merkleProof.Root,
		nullifier,
		commitmentOut,
		stAssetGroupTreeNumber,
		stAssetGroupMerkleRoot,
	}

	return &ProofResult{
		Proof:           proofStrsNF,
		Statement:       statement,
		NumberOfInputs:  1,
		NumberOfOutputs: 1,
		SaltsOut:        []*big.Int{wtSaltOut},
		CipherText:     [][]byte{ctI},
		EncTxData:    [][]byte{ctII},
	}, nil
}

// Erc1155NonFungibleOwnershipProofFromSalt is like Erc1155NonFungibleOwnershipProof but
// accepts a pre-computed output salt and ciphertexts instead of performing ML-KEM encapsulation.
// Use this when the output commitment must be known before proof generation — e.g. for
// atomic DVP swaps where cross-commitment consistency is required.
func (c *GnarkClient) Erc1155NonFungibleOwnershipProofFromSalt(
	stMessage *big.Int,
	wtValue *big.Int, keyIn KeyPair, wtSaltIn *big.Int, keyOut KeyPair,
	wtSaltOut *big.Int, ctI []byte, ctII []byte,
	merkleDepth int, merkleProof *MerkleProof,
	stTreeNumber *big.Int,
	wtErc1155ContractAddress *big.Int, wtErc1155TokenId *big.Int,
	stAssetGroupTreeNumber *big.Int, assetGroupMerkleProof *MerkleProof,
) (*ProofResult, error) {
	// HIGH-1 fix: incorporate tree number to prevent cross-tree double-spend.
	nullifier, err := GetNullifierWithTree(keyIn.PrivateKey, stTreeNumber, merkleProof.Indices, merkleDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to compute nullifier: %w", err)
	}

	commitmentOut, err := Erc1155Commitment(wtErc1155TokenId, wtValue, keyOut.PublicKey, wtSaltOut)
	if err != nil {
		return nil, fmt.Errorf("failed to compute erc1155Commitment for output: %w", err)
	}

	stAssetGroupMerkleRoot := assetGroupMerkleProof.Root

	payload := map[string]interface{}{
		"StMessage":                stMessage.String(),
		"StTreeNumbers":            []string{stTreeNumber.String()},
		"StMerkleRoots":            []string{merkleProof.Root.String()},
		"StNullifiers":             []string{nullifier.String()},
		"StCommitmentOut":          []string{commitmentOut.String()},
		"StAssetGroupTreeNumber":   []string{stAssetGroupTreeNumber.String()},
		"StAssetGroupMerkleRoot":   []string{stAssetGroupMerkleRoot.String()},
		"WtPrivateKeysIn":          []string{keyIn.PrivateKey.String()},
		"WtValues":                 []string{wtValue.String()},
		"WtPathElements":           [][]string{bigIntSliceToStrings(merkleProof.Elements)},
		"WtPathIndices":            []string{merkleProof.Indices.String()},
		"WtErc1155TokenId":         []string{wtErc1155TokenId.String()},
		"WtPublicKeysOut":          []string{keyOut.PublicKey.String()},
		"WtErc1155ContractAddress": wtErc1155ContractAddress.String(),
		"WtAssetGroupPathElements": [][]string{bigIntSliceToStrings(assetGroupMerkleProof.Elements)},
		"WtAssetGroupPathIndices":  []string{assetGroupMerkleProof.Indices.String()},
		"WtSaltsIn":                []string{wtSaltIn.String()},
		"WtSaltsOut":               []string{wtSaltOut.String()},
	}

	body, err := c.PostProof("/proof/erc1155NonFungible", payload)
	if err != nil {
		return nil, fmt.Errorf("erc1155NonFungibleOwnershipFromSalt proof request failed: %w", err)
	}

	var gnarkResp struct {
		Proof []json.Number `json:"proof"`
	}
	if parseErr := json.Unmarshal(body, &gnarkResp); parseErr != nil {
		return nil, fmt.Errorf("failed to parse erc1155NonFungibleOwnershipFromSalt proof response: %w", parseErr)
	}
	proofStrs := make([]string, len(gnarkResp.Proof))
	for i, n := range gnarkResp.Proof {
		proofStrs[i] = n.String()
	}

	// Statement: [message, treeNumber, merkleRoot, nullifier, commitmentOut,
	//   assetGroupTreeNumber, assetGroupMerkleRoot]
	statement := []*big.Int{
		stMessage,
		stTreeNumber,
		merkleProof.Root,
		nullifier,
		commitmentOut,
		stAssetGroupTreeNumber,
		stAssetGroupMerkleRoot,
	}

	return &ProofResult{
		Proof:           proofStrs,
		Statement:       statement,
		NumberOfInputs:  1,
		NumberOfOutputs: 1,
		SaltsOut:        []*big.Int{wtSaltOut},
		CipherText:     [][]byte{ctI},
		EncTxData:    [][]byte{ctII},
	}, nil
}

// PoseidonEncrypt calls the gnark server's /util/poseidonEncrypt endpoint to
// encrypt plaintext values using the Poseidon sponge cipher.
// key is [X, Y] of the BabyJubJub shared key (authKey = mulPointEscalar(auditorPubKey, random)).
// nonce must be < 2^128.
// realLength is the number of meaningful plaintext values.
// Returns the encrypted values including MAC (length = ceil(realLength/3)*3 + 1).
func (c *GnarkClient) PoseidonEncrypt(key [2]*big.Int, nonce *big.Int, realLength int, plaintext []*big.Int) ([]*big.Int, error) {
	plaintextStrs := make([]string, len(plaintext))
	for i, v := range plaintext {
		plaintextStrs[i] = v.String()
	}
	payload := map[string]interface{}{
		"key":        [2]string{key[0].String(), key[1].String()},
		"nonce":      nonce.String(),
		"realLength": realLength,
		"plaintext":  plaintextStrs,
	}
	body, err := c.PostProof("/util/poseidonEncrypt", payload)
	if err != nil {
		return nil, fmt.Errorf("poseidonEncrypt request failed: %w", err)
	}
	var resp struct {
		Encrypted []json.Number `json:"encrypted"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse poseidonEncrypt response: %w", err)
	}
	result := make([]*big.Int, len(resp.Encrypted))
	for i, n := range resp.Encrypted {
		v, ok := new(big.Int).SetString(n.String(), 10)
		if !ok {
			return nil, fmt.Errorf("invalid big int in encrypted response index %d", i)
		}
		result[i] = v
	}
	return result, nil
}

// AuditorDecrypt decrypts values encrypted by the auditor circuit using the
// Poseidon sponge cipher via the gnark server's /util/poseidonDecrypt endpoint.
//
// authKeyX, authKeyY is the shared encryption key: StAuditorAuthKey = mulPointEscalar(pubKey, random).
// nonce is StAuditorNonce from the proof data.
// encrypted is StAuditorEncryptedValues from the proof data.
// realLength is the number of plaintext values (6 for fungible, 3 for non-fungible).
//
// Returns an error if the MAC does not match (wrong key or tampered ciphertext).
func (c *GnarkClient) AuditorDecrypt(authKeyX, authKeyY, nonce *big.Int, encrypted []*big.Int, realLength int) ([]*big.Int, error) {
	encStrs := make([]string, len(encrypted))
	for i, v := range encrypted {
		encStrs[i] = v.String()
	}
	payload := map[string]interface{}{
		"key":        [2]string{authKeyX.String(), authKeyY.String()},
		"nonce":      nonce.String(),
		"realLength": realLength,
		"encrypted":  encStrs,
	}
	body, err := c.PostProof("/util/poseidonDecrypt", payload)
	if err != nil {
		return nil, fmt.Errorf("poseidonDecrypt request failed: %w", err)
	}
	var resp struct {
		Plaintext []json.Number `json:"plaintext"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse poseidonDecrypt response: %w", err)
	}
	result := make([]*big.Int, len(resp.Plaintext))
	for i, n := range resp.Plaintext {
		v, ok := new(big.Int).SetString(n.String(), 10)
		if !ok {
			return nil, fmt.Errorf("invalid big int in plaintext response index %d", i)
		}
		result[i] = v
	}
	return result, nil
}

// Erc1155FungibleAuditorProof generates an ERC1155 fungible JoinSplit proof with
// auditor encryption. The auditor's public key (BabyJubJub) is provided by the caller.
// The function generates a random blinding factor and nonce, computes the shared
// encryption key, encrypts the audit plaintext, and sends the full proof request.
//
// Audit plaintext (6 values): [valIn[0], valIn[1], valOut[0], valOut[1], tokenId, contractAddr]
func (c *GnarkClient) Erc1155FungibleAuditorProof(
	stMessage *big.Int,
	wtValuesIn []*big.Int, keysIn []KeyPair, wtSaltsIn []*big.Int,
	wtValuesOut []*big.Int, keysOut []KeyPair,
	recipientViewEncapKeys [][]byte,
	merkleDepth int, merkleProofs []*MerkleProof,
	stTreeNumbers []*big.Int,
	wtErc1155ContractAddress *big.Int, wtErc1155TokenId *big.Int,
	stAssetGroupTreeNumber *big.Int, assetGroupMerkleProof *MerkleProof,
	auditorPubKeyX, auditorPubKeyY *big.Int,
) (*ProofResult, error) {
	stCommitmentsOut, wtSaltsOut, ctI, ctII, stNullifiers, wtPathIndices, wtPathElements, err := prepareErc1155ProofParams(
		wtValuesIn, keysIn, wtSaltsIn, wtValuesOut, keysOut, recipientViewEncapKeys, merkleDepth, merkleProofs,
		wtErc1155ContractAddress, wtErc1155TokenId,
	)
	if err != nil {
		return nil, err
	}

	stMerkleRoots := make([]*big.Int, len(wtValuesIn))
	for i := range wtValuesIn {
		if wtValuesIn[i].Sign() == 0 {
			stMerkleRoots[i] = big.NewInt(0)
		} else {
			stMerkleRoots[i] = merkleProofs[i].Root
		}
	}

	// Auditor encryption
	auditorRandom, err := RandomAuditorScalar()
	if err != nil {
		return nil, fmt.Errorf("failed to generate auditor random: %w", err)
	}
	auditorNonce, err := RandomNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to generate auditor nonce: %w", err)
	}
	authKeyX, authKeyY := AuditorEncKey(auditorPubKeyX, auditorPubKeyY, auditorRandom)

	// plaintext: [valIn[0], valIn[1], valOut[0], valOut[1], tokenId, contractAddr]
	plaintext := make([]*big.Int, 0, 6)
	plaintext = append(plaintext, wtValuesIn...)
	plaintext = append(plaintext, wtValuesOut...)
	plaintext = append(plaintext, wtErc1155TokenId, wtErc1155ContractAddress)

	encrypted, err := c.PoseidonEncrypt([2]*big.Int{authKeyX, authKeyY}, auditorNonce, len(plaintext), plaintext)
	if err != nil {
		return nil, fmt.Errorf("auditor encryption failed: %w", err)
	}

	pathElementChunks := chunkBigIntSlice(wtPathElements, merkleDepth)

	payload := map[string]interface{}{
		"StMessage":                stMessage.String(),
		"StTreeNumbers":            bigIntSliceToStrings(stTreeNumbers),
		"StMerkleRoots":            bigIntSliceToStrings(stMerkleRoots),
		"StNullifiers":             bigIntSliceToStrings(stNullifiers),
		"StCommitmentOut":          bigIntSliceToStrings(stCommitmentsOut),
		"StAssetGroupMerkleRoot":   assetGroupMerkleProof.Root.String(),
		"StAssetGroupTreeNumber":   stAssetGroupTreeNumber.String(),
		"WtPrivateKeysIn":          bigIntSliceToStrings(extractPrivateKeys(keysIn)),
		"WtValuesIn":               bigIntSliceToStrings(wtValuesIn),
		"WtSaltsIn":                bigIntSliceToStrings(wtSaltsIn),
		"WtPathElements":           bigIntChunksToStringChunks(pathElementChunks),
		"WtPathIndices":            bigIntSliceToStrings(wtPathIndices),
		"WtErc1155ContractAddress": wtErc1155ContractAddress.String(),
		"WtErc1155TokenId":         wtErc1155TokenId.String(),
		"WtPublicKeysOut":          bigIntSliceToStrings(extractPublicKeys(keysOut)),
		"WtValuesOut":              bigIntSliceToStrings(wtValuesOut),
		"WtSaltsOut":               bigIntSliceToStrings(wtSaltsOut),
		"WtAssetGroupPathElements": bigIntSliceToStrings(assetGroupMerkleProof.Elements),
		"WtAssetGroupPathIndices":  assetGroupMerkleProof.Indices.String(),
		"StAuditorPublickey":       []string{auditorPubKeyX.String(), auditorPubKeyY.String()},
		"StAuditorAuthKey":         []string{authKeyX.String(), authKeyY.String()},
		"StAuditorNonce":           auditorNonce.String(),
		"StAuditorEncryptedValues": bigIntSliceToStrings(encrypted),
		"WtAuditorRandom":          auditorRandom.String(),
	}

	body, err := c.PostProof("/proof/erc1155FungibleAuditor", payload)
	if err != nil {
		return nil, fmt.Errorf("erc1155FungibleAuditor proof request failed: %w", err)
	}

	var gnarkResp struct {
		Proof []json.Number `json:"proof"`
	}
	if err := json.Unmarshal(body, &gnarkResp); err != nil {
		return nil, fmt.Errorf("failed to parse erc1155FungibleAuditor response: %w", err)
	}
	proofStrs := make([]string, len(gnarkResp.Proof))
	for i, n := range gnarkResp.Proof {
		proofStrs[i] = n.String()
	}

	statement := make([]*big.Int, 0, 1+3*len(wtValuesIn)+len(keysOut))
	statement = append(statement, stMessage)
	for i := 0; i < len(wtValuesIn); i++ {
		statement = append(statement, stTreeNumbers[i], stMerkleRoots[i], stNullifiers[i])
	}
	statement = append(statement, stCommitmentsOut...)

	return &ProofResult{
		Proof:           proofStrs,
		Statement:       statement,
		NumberOfInputs:  len(wtValuesIn),
		NumberOfOutputs: len(wtValuesOut),
		SaltsOut:        wtSaltsOut,
		CipherText:     ctI,
		EncTxData:    ctII,
		AuditData: &AuditEncryptionData{
			AuthKeyX:   authKeyX,
			AuthKeyY:   authKeyY,
			Nonce:      auditorNonce,
			Encrypted:  encrypted,
			RealLength: len(plaintext),
		},
	}, nil
}

// Erc1155NonFungibleAuditorProof generates an ERC1155 non-fungible ownership proof
// with auditor encryption. The auditor's public key (BabyJubJub) is provided by
// the caller.
//
// Audit plaintext (3 values): [value[0], tokenId[0], contractAddr]
func (c *GnarkClient) Erc1155NonFungibleAuditorProof(
	stMessage *big.Int,
	wtValue *big.Int, keyIn KeyPair, wtSaltIn *big.Int, keyOut KeyPair,
	recipientViewEncapKey []byte,
	merkleDepth int, merkleProof *MerkleProof,
	stTreeNumber *big.Int,
	wtErc1155ContractAddress *big.Int, wtErc1155TokenId *big.Int,
	stAssetGroupTreeNumber *big.Int, assetGroupMerkleProof *MerkleProof,
	auditorPubKeyX, auditorPubKeyY *big.Int,
) (*ProofResult, error) {
	// HIGH-1 fix: incorporate tree number to prevent cross-tree double-spend.
	nullifier, err := GetNullifierWithTree(keyIn.PrivateKey, stTreeNumber, merkleProof.Indices, merkleDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to compute nullifier: %w", err)
	}

	// Generate fresh salt for output note via ML-KEM. recipientViewEncapKey is mandatory.
	if recipientViewEncapKey == nil {
		return nil, fmt.Errorf("recipientViewEncapKey is required for non-interactive note delivery")
	}
	ss, ctI, kemErr := Encapsulate(recipientViewEncapKey)
	if kemErr != nil {
		return nil, fmt.Errorf("failed to encapsulate for output: %w", kemErr)
	}
	saltB, saltErr := DerivePaymentSalt(ss)
	if saltErr != nil {
		return nil, fmt.Errorf("failed to derive payment salt for output: %w", saltErr)
	}
	encKey, keyErr := DerivePaymentKey(ss)
	if keyErr != nil {
		return nil, fmt.Errorf("failed to derive payment key for output: %w", keyErr)
	}
	wtSaltOut := SaltBToField(saltB)
	ctII, encErr := EncryptPayload(encKey, wtErc1155TokenId, wtValue)
	if encErr != nil {
		return nil, fmt.Errorf("failed to encrypt payload for output: %w", encErr)
	}

	commitmentOut, err := Erc1155Commitment(wtErc1155TokenId, wtValue, keyOut.PublicKey, wtSaltOut)
	if err != nil {
		return nil, fmt.Errorf("failed to compute erc1155Commitment for output: %w", err)
	}

	// Auditor encryption
	auditorRandom, err := RandomAuditorScalar()
	if err != nil {
		return nil, fmt.Errorf("failed to generate auditor random: %w", err)
	}
	auditorNonce, err := RandomNonce()
	if err != nil {
		return nil, fmt.Errorf("failed to generate auditor nonce: %w", err)
	}
	authKeyX, authKeyY := AuditorEncKey(auditorPubKeyX, auditorPubKeyY, auditorRandom)

	// plaintext: [value[0], tokenId[0], contractAddr]
	plaintext := []*big.Int{wtValue, wtErc1155TokenId, wtErc1155ContractAddress}

	encrypted, err := c.PoseidonEncrypt([2]*big.Int{authKeyX, authKeyY}, auditorNonce, len(plaintext), plaintext)
	if err != nil {
		return nil, fmt.Errorf("auditor encryption failed: %w", err)
	}

	payload := map[string]interface{}{
		"StMessage":                stMessage.String(),
		"StTreeNumbers":            []string{stTreeNumber.String()},
		"StMerkleRoots":            []string{merkleProof.Root.String()},
		"StNullifiers":             []string{nullifier.String()},
		"StCommitmentOut":          []string{commitmentOut.String()},
		"StAssetGroupTreeNumber":   []string{stAssetGroupTreeNumber.String()},
		"StAssetGroupMerkleRoot":   []string{assetGroupMerkleProof.Root.String()},
		"WtPrivateKeysIn":          []string{keyIn.PrivateKey.String()},
		"WtValues":                 []string{wtValue.String()},
		"WtSaltsIn":                []string{wtSaltIn.String()},
		"WtPathElements":           [][]string{bigIntSliceToStrings(merkleProof.Elements)},
		"WtPathIndices":            []string{merkleProof.Indices.String()},
		"WtErc1155TokenIds":        []string{wtErc1155TokenId.String()},
		"WtErc1155ContractAddress": wtErc1155ContractAddress.String(),
		"WtPublicKeysOut":          []string{keyOut.PublicKey.String()},
		"WtSaltsOut":               []string{wtSaltOut.String()},
		"WtAssetGroupPathElements": [][]string{bigIntSliceToStrings(assetGroupMerkleProof.Elements)},
		"WtAssetGroupPathIndices":  []string{assetGroupMerkleProof.Indices.String()},
		"StAuditorPublickey":       []string{auditorPubKeyX.String(), auditorPubKeyY.String()},
		"StAuditorAuthKey":         []string{authKeyX.String(), authKeyY.String()},
		"StAuditorNonce":           auditorNonce.String(),
		"StAuditorEncryptedValues": bigIntSliceToStrings(encrypted),
		"WtAuditorRandom":          auditorRandom.String(),
	}

	body, err := c.PostProof("/proof/erc1155NonFungibleAuditor", payload)
	if err != nil {
		return nil, fmt.Errorf("erc1155NonFungibleAuditor proof request failed: %w", err)
	}

	var gnarkResp struct {
		Proof []json.Number `json:"proof"`
	}
	if err := json.Unmarshal(body, &gnarkResp); err != nil {
		return nil, fmt.Errorf("failed to parse erc1155NonFungibleAuditor response: %w", err)
	}
	proofStrs := make([]string, len(gnarkResp.Proof))
	for i, n := range gnarkResp.Proof {
		proofStrs[i] = n.String()
	}

	statement := []*big.Int{
		stMessage,
		stTreeNumber,
		merkleProof.Root,
		nullifier,
		commitmentOut,
		stAssetGroupTreeNumber,
		assetGroupMerkleProof.Root,
	}

	return &ProofResult{
		Proof:           proofStrs,
		Statement:       statement,
		NumberOfInputs:  1,
		NumberOfOutputs: 1,
		SaltsOut:        []*big.Int{wtSaltOut},
		CipherText:     [][]byte{ctI},
		EncTxData:    [][]byte{ctII},
		AuditData: &AuditEncryptionData{
			AuthKeyX:   authKeyX,
			AuthKeyY:   authKeyY,
			Nonce:      auditorNonce,
			Encrypted:  encrypted,
			RealLength: len(plaintext),
		},
	}, nil
}

// Erc20PrivateMintResult holds the outputs of a successful V2 PrivateMint proof.
type Erc20PrivateMintResult struct {
	// Commitment is the V2 note commitment: Poseidon(pkSpend, salt, amount, tokenId).
	// This is what gets inserted into the Merkle tree on-chain.
	Commitment *big.Int

	// CipherText is the note tag: Poseidon(pkSpend, salt).
	// Published on-chain as a public signal; Alice uses it to confirm a mint is hers.
	CipherText *big.Int

	// Salt is the random field element Alice chose. She must keep this as her
	// WtSaltsIn witness when she later spends the note in a JoinSplit proof.
	Salt *big.Int

	// ProofResponse carries the Groth16 proof and public signals from the gnark server.
	ProofResponse *PrivateMintProofResponse
}

// Erc20PrivateMintProof generates a V2 PrivateMint proof for an ERC20 deposit.
//
// Alice picks a random salt, then the function:
//  1. Computes commitment = Poseidon(pkSpend, salt, amount, tokenId) — the V2 format
//     expected by the JoinSplit circuit on the input side.
//  2. Computes cipherText = Poseidon(pkSpend, salt) — a note tag Alice can use when
//     scanning the chain to confirm this mint is hers.
//  3. Sends both to the gnark server's /proof/privateMint endpoint.
//
// The returned Salt must be stored by Alice — it is her WtSaltsIn when spending.
func (c *GnarkClient) Erc20PrivateMintProof(
	pkSpend *big.Int,
	salt *big.Int,
	amount *big.Int,
	tokenId *big.Int,
	contractAddress *big.Int,
) (*Erc20PrivateMintResult, error) {
	commitment, err := Erc20CommitmentV2(pkSpend, salt, amount, tokenId)
	if err != nil {
		return nil, fmt.Errorf("failed to compute V2 commitment: %w", err)
	}

	cipherText, err := poseidon.Hash([]*big.Int{pkSpend, salt, contractAddress})
	if err != nil {
		return nil, fmt.Errorf("failed to compute cipherText: %w", err)
	}

	payload := map[string]interface{}{
		"commitment":      commitment.String(),
		"contractAddress": contractAddress.String(),
		"tokenId":         tokenId.String(),
		"salt":            salt.String(),
		"amount":          amount.String(),
		"publicKey":       pkSpend.String(),
		"cipherText":      cipherText.String(),
	}

	body, err := c.PostProof("/proof/privateMint", payload)
	if err != nil {
		return nil, fmt.Errorf("privateMint proof request failed: %w", err)
	}

	var resp PrivateMintProofResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse privateMint response: %w", err)
	}

	return &Erc20PrivateMintResult{
		Commitment:    commitment,
		CipherText:    cipherText,
		Salt:          salt,
		ProofResponse: &resp,
	}, nil
}

// PaymentResult holds the output of PaymentProof — everything Alice needs to submit
// the transaction on-chain and that each recipient needs to scan their note.
type PaymentResult struct {
	Proof           []string    // 8-element Groth16 proof
	Statement       []*big.Int  // interleaved: [msg, tree0, root0, null0, ..., cmt0, cmt1, contractAddr?]
	NumberOfInputs  int
	NumberOfOutputs int
	// Bob's note discovery data (output 0 only — published on-chain).
	CipherText []byte // ML-KEM capsule for Bob (1088 bytes)
	EncTxData  []byte // AES-256-GCM ciphertext of (tokenId || amount) for Bob
	// Bob's salt — needed by the tag-based notification path (private tag layer).
	SaltB *big.Int // saltB field element used in Bob's output commitment
	// Alice's change note data (private — not published on-chain).
	SaltA *big.Int // random change salt; Alice must store this locally to open Commitment_A
	// ContractAddress is non-nil when the proof was generated via BoundPaymentProof (VULN-5 fix).
	// ContractStatement() appends it as the last statement element for on-chain verification.
	ContractAddress *big.Int
	// Fee is non-nil when generated via BoundPaymentFeeProof.
	// ContractStatement() appends it after ContractAddress so the on-chain verifier can
	// check statement[last] == PROTOCOL_FEE.
	Fee *big.Int
	// SaltRelayer is the random salt used for the relayer fee note (output 2) in
	// BoundPaymentRelayerProof. Alice must communicate this to the relayer off-chain.
	SaltRelayer *big.Int
}

// ContractStatement de-interleaves the statement for on-chain submission.
// When ContractAddress is set (BoundPaymentProof), it is appended as the last element.
// When Fee is also set (BoundPaymentFeeProof), it is appended after ContractAddress.
func (r *PaymentResult) ContractStatement() []*big.Int {
	nIn := r.NumberOfInputs
	nOut := r.NumberOfOutputs
	size := 1 + 3*nIn + nOut
	if r.ContractAddress != nil {
		size++
	}
	if r.Fee != nil {
		size++
	}
	out := make([]*big.Int, size)
	out[0] = r.Statement[0]
	for i := 0; i < nIn; i++ {
		base := 1 + i*3
		out[1+i]       = r.Statement[base]
		out[1+nIn+i]   = r.Statement[base+1]
		out[1+2*nIn+i] = r.Statement[base+2]
	}
	for i := 0; i < nOut; i++ {
		out[1+3*nIn+i] = r.Statement[1+3*nIn+i]
	}
	idx := 1 + 3*nIn + nOut
	if r.ContractAddress != nil {
		out[idx] = r.ContractAddress
		idx++
	}
	if r.Fee != nil {
		out[idx] = r.Fee
	}
	return out
}

// PaymentProof generates a Payment circuit proof for Alice paying some amount to Bob
// with optional change back to herself (or any other recipients).
//
// Circuit config: 1 input / 2 outputs / Merkle depth 8.
//   - Input 0: Alice's real note.
//   - Output 0: payment to Bob; Output 1: change back to Alice.
//
// Salt derivation follows the protocol:
//   - Output 0 (destination): salt derived via HKDF from the ML-KEM shared secret
//     so Bob can recover it by decapsulating the ML-KEM ciphertext.
//   - Output 1+ (change): salt is random (protocol §"Deriving a change salt");
//     the caller must store PaymentResult.SaltA locally to later open the commitment.
// The caller submits the resulting ctxts/encTxDatas alongside the proof on-chain.
func (c *GnarkClient) PaymentProof(
	stMessage *big.Int,
	wtValuesIn []*big.Int,           // [aliceAmount]
	keysIn []KeyPair,                // [aliceKey]
	wtSaltsIn []*big.Int,            // [aliceSaltBField]
	wtValuesOut []*big.Int,          // [paymentAmount, changeAmount]
	recipientSpendPks []*big.Int,    // [bobPk, alicePk]
	recipientViewEncapKeys [][]byte, // [bobViewEncapKey, aliceViewEncapKey]
	merkleDepth int,
	merkleProofs []*MerkleProof,     // [aliceProof]
	stTreeNumbers []*big.Int,        // [0]
	wtTokenId *big.Int,
) (*PaymentResult, error) {
	nIn := len(wtValuesIn)
	nOut := len(wtValuesOut)

	// --- inputs: nullifiers and Merkle paths ---
	stNullifiers := make([]*big.Int, nIn)
	wtPathIndices := make([]*big.Int, nIn)
	wtPathElements := make([]*big.Int, 0, nIn*merkleDepth)

	for i := 0; i < nIn; i++ {
		if wtValuesIn[i].Sign() == 0 {
			wtPathIndices[i] = big.NewInt(0)
			zeros := make([]*big.Int, merkleDepth)
			for j := range zeros {
				zeros[j] = big.NewInt(0)
			}
			wtPathElements = append(wtPathElements, zeros...)
		} else {
			wtPathIndices[i] = merkleProofs[i].Indices
			wtPathElements = append(wtPathElements, merkleProofs[i].Elements...)
		}
		// HIGH-1 fix: incorporate tree number to prevent cross-tree double-spend.
		nf, err := GetNullifierWithTree(keysIn[i].PrivateKey, stTreeNumbers[i], wtPathIndices[i], merkleDepth)
		if err != nil {
			return nil, fmt.Errorf("GetNullifier input %d: %w", i, err)
		}
		stNullifiers[i] = nf
	}

	// --- outputs: KEM + HKDF + AES-GCM + V2 commitments ---
	//
	// Output 0 (Bob): KEM-derived salt + on-chain ciphertext so Bob can scan.
	// Output 1+ (change): random salt + no on-chain ciphertext; Alice stores saltA locally.
	wtSaltsOut := make([]*big.Int, nOut)
	stCommitmentsOut := make([]*big.Int, nOut)
	var cipherText []byte
	var encTxData []byte
	var saltA *big.Int

	// Output 0 (Bob): KEM encapsulation → HKDF salt + HKDF key + AES-GCM ciphertext.
	ss0, ctxt0, err := Encapsulate(recipientViewEncapKeys[0])
	if err != nil {
		return nil, fmt.Errorf("Encapsulate Bob output: %w", err)
	}
	saltB0, err := DerivePaymentSalt(ss0)
	if err != nil {
		return nil, fmt.Errorf("DerivePaymentSalt Bob output: %w", err)
	}
	encKey0, err := DerivePaymentKey(ss0)
	if err != nil {
		return nil, fmt.Errorf("DerivePaymentKey Bob output: %w", err)
	}
	ctxtII0, err := EncryptPayload(encKey0, wtTokenId, wtValuesOut[0])
	if err != nil {
		return nil, fmt.Errorf("EncryptPayload Bob output: %w", err)
	}
	cipherText = ctxt0
	encTxData = ctxtII0
	wtSaltsOut[0] = SaltBToField(saltB0)

	cmt0, err := Erc20CommitmentV2(recipientSpendPks[0], wtSaltsOut[0], wtValuesOut[0], wtTokenId)
	if err != nil {
		return nil, fmt.Errorf("Erc20CommitmentV2 Bob output: %w", err)
	}
	stCommitmentsOut[0] = cmt0

	// Output 1+ (change): random salt, no KEM, no on-chain ciphertext.
	for i := 1; i < nOut; i++ {
		randSalt, err := RandomInField()
		if err != nil {
			return nil, fmt.Errorf("RandomInField change output %d: %w", i, err)
		}
		if i == 1 {
			saltA = randSalt
		}
		wtSaltsOut[i] = randSalt

		cmt, err := Erc20CommitmentV2(recipientSpendPks[i], randSalt, wtValuesOut[i], wtTokenId)
		if err != nil {
			return nil, fmt.Errorf("Erc20CommitmentV2 change output %d: %w", i, err)
		}
		stCommitmentsOut[i] = cmt
	}

	// --- Merkle roots ---
	stMerkleRoots := make([]*big.Int, nIn)
	for i := range wtValuesIn {
		if wtValuesIn[i].Sign() == 0 {
			stMerkleRoots[i] = big.NewInt(0)
		} else {
			stMerkleRoots[i] = merkleProofs[i].Root
		}
	}

	pathElementChunks := chunkBigIntSlice(wtPathElements, merkleDepth)

	payload := map[string]interface{}{
		"stMessage":            stMessage.String(),
		"stTreeNumbers":        bigIntSliceToStrings(stTreeNumbers),
		"stMerkleRoots":        bigIntSliceToStrings(stMerkleRoots),
		"stNullifiers":         bigIntSliceToStrings(stNullifiers),
		"stCommitmentsOut":     bigIntSliceToStrings(stCommitmentsOut),
		"wtPrivateKeysIn":      bigIntSliceToStrings(extractPrivateKeys(keysIn)),
		"wtValuesIn":           bigIntSliceToStrings(wtValuesIn),
		"wtSaltsIn":            bigIntSliceToStrings(wtSaltsIn),
		"wtPathElements":       bigIntChunksToStringChunks(pathElementChunks),
		"wtPathIndices":        bigIntSliceToStrings(wtPathIndices),
		"wtTokenId":            wtTokenId.String(),
		"wtSpendPublicKeysOut": bigIntSliceToStrings(recipientSpendPks),
		"wtValuesOut":          bigIntSliceToStrings(wtValuesOut),
		"wtSaltsOut":           bigIntSliceToStrings(wtSaltsOut),
	}

	body, err := c.PostProof("/proof/payment", payload)
	if err != nil {
		return nil, fmt.Errorf("payment proof request failed: %w", err)
	}

	var gnarkResp struct {
		Proof        []json.Number  `json:"proof"`
		PublicSignal []json.Number  `json:"publicSignal"`
	}
	if err := json.Unmarshal(body, &gnarkResp); err != nil {
		return nil, fmt.Errorf("failed to parse payment proof response: %w", err)
	}
	proofStrs := make([]string, len(gnarkResp.Proof))
	for i, n := range gnarkResp.Proof {
		proofStrs[i] = n.String()
	}

	// Build interleaved statement from public signal returned by the server.
	// Server returns: [msg, tree0, root0, null0, tree1, root1, null1, cmt0, cmt1]
	statement := make([]*big.Int, 0, 1+3*nIn+nOut)
	statement = append(statement, stMessage)
	for i := 0; i < nIn; i++ {
		statement = append(statement, stTreeNumbers[i], stMerkleRoots[i], stNullifiers[i])
	}
	statement = append(statement, stCommitmentsOut...)

	return &PaymentResult{
		Proof:           proofStrs,
		Statement:       statement,
		NumberOfInputs:  nIn,
		NumberOfOutputs: nOut,
		CipherText:      cipherText,
		EncTxData:       encTxData,
		SaltB:           wtSaltsOut[0],
		SaltA:           saltA,
	}, nil
}

// BoundPaymentProof is identical to PaymentProof but binds the proof to a specific vault
// contract address (VULN-5 fix).
//
// contractAddress must be the uint160 representation of the Erc20CoinVault address
// (i.e., new(big.Int).SetBytes(vaultAddr.Bytes())).
//
// Differences from PaymentProof:
//   - Nullifier: Poseidon3(sk, leafIndex, contractAddress) instead of Poseidon2(sk, leafIndex).
//   - Payload: includes "stContractAddress" in the gnark server request.
//   - Result:  ContractAddress field is set; ContractStatement() appends it to the statement.
func (c *GnarkClient) BoundPaymentProof(
	contractAddress *big.Int,
	stMessage *big.Int,
	wtValuesIn []*big.Int,
	keysIn []KeyPair,
	wtSaltsIn []*big.Int,
	wtValuesOut []*big.Int,
	recipientSpendPks []*big.Int,
	recipientViewEncapKeys [][]byte,
	merkleDepth int,
	merkleProofs []*MerkleProof,
	stTreeNumbers []*big.Int,
	wtTokenId *big.Int,
) (*PaymentResult, error) {
	nIn := len(wtValuesIn)
	nOut := len(wtValuesOut)

	stNullifiers := make([]*big.Int, nIn)
	wtPathIndices := make([]*big.Int, nIn)
	wtPathElements := make([]*big.Int, 0, nIn*merkleDepth)

	for i := 0; i < nIn; i++ {
		if wtValuesIn[i].Sign() == 0 {
			stNullifiers[i] = big.NewInt(0)
			wtPathIndices[i] = big.NewInt(0)
			zeros := make([]*big.Int, merkleDepth)
			for j := range zeros {
				zeros[j] = big.NewInt(0)
			}
			wtPathElements = append(wtPathElements, zeros...)
		} else {
			wtPathIndices[i] = merkleProofs[i].Indices
			wtPathElements = append(wtPathElements, merkleProofs[i].Elements...)
			// Bound nullifier: Poseidon3(sk, leafIndex, contractAddress)
			nf, err := GetNullifierBound(keysIn[i].PrivateKey, wtPathIndices[i], contractAddress)
			if err != nil {
				return nil, fmt.Errorf("GetNullifierBound input %d: %w", i, err)
			}
			stNullifiers[i] = nf
		}
	}

	wtSaltsOut := make([]*big.Int, nOut)
	stCommitmentsOut := make([]*big.Int, nOut)
	var cipherText, encTxData []byte
	var saltA *big.Int

	ss0, ctxt0, err := Encapsulate(recipientViewEncapKeys[0])
	if err != nil {
		return nil, fmt.Errorf("Encapsulate Bob output: %w", err)
	}
	saltB0, err := DerivePaymentSalt(ss0)
	if err != nil {
		return nil, fmt.Errorf("DerivePaymentSalt Bob output: %w", err)
	}
	encKey0, err := DerivePaymentKey(ss0)
	if err != nil {
		return nil, fmt.Errorf("DerivePaymentKey Bob output: %w", err)
	}
	ctxtII0, err := EncryptPayload(encKey0, wtTokenId, wtValuesOut[0])
	if err != nil {
		return nil, fmt.Errorf("EncryptPayload Bob output: %w", err)
	}
	cipherText = ctxt0
	encTxData = ctxtII0
	wtSaltsOut[0] = SaltBToField(saltB0)
	cmt0, err := Erc20CommitmentV2(recipientSpendPks[0], wtSaltsOut[0], wtValuesOut[0], wtTokenId)
	if err != nil {
		return nil, fmt.Errorf("Erc20CommitmentV2 Bob output: %w", err)
	}
	stCommitmentsOut[0] = cmt0

	for i := 1; i < nOut; i++ {
		randSalt, err := RandomInField()
		if err != nil {
			return nil, fmt.Errorf("RandomInField change output %d: %w", i, err)
		}
		if i == 1 {
			saltA = randSalt
		}
		wtSaltsOut[i] = randSalt
		cmt, err := Erc20CommitmentV2(recipientSpendPks[i], randSalt, wtValuesOut[i], wtTokenId)
		if err != nil {
			return nil, fmt.Errorf("Erc20CommitmentV2 change output %d: %w", i, err)
		}
		stCommitmentsOut[i] = cmt
	}

	stMerkleRoots := make([]*big.Int, nIn)
	for i := range wtValuesIn {
		if wtValuesIn[i].Sign() == 0 {
			stMerkleRoots[i] = big.NewInt(0)
		} else {
			stMerkleRoots[i] = merkleProofs[i].Root
		}
	}

	pathElementChunks := chunkBigIntSlice(wtPathElements, merkleDepth)

	payload := map[string]interface{}{
		"stMessage":            stMessage.String(),
		"stTreeNumbers":        bigIntSliceToStrings(stTreeNumbers),
		"stMerkleRoots":        bigIntSliceToStrings(stMerkleRoots),
		"stNullifiers":         bigIntSliceToStrings(stNullifiers),
		"stCommitmentsOut":     bigIntSliceToStrings(stCommitmentsOut),
		"stContractAddress":    contractAddress.String(),
		"wtPrivateKeysIn":      bigIntSliceToStrings(extractPrivateKeys(keysIn)),
		"wtValuesIn":           bigIntSliceToStrings(wtValuesIn),
		"wtSaltsIn":            bigIntSliceToStrings(wtSaltsIn),
		"wtPathElements":       bigIntChunksToStringChunks(pathElementChunks),
		"wtPathIndices":        bigIntSliceToStrings(wtPathIndices),
		"wtTokenId":            wtTokenId.String(),
		"wtSpendPublicKeysOut": bigIntSliceToStrings(recipientSpendPks),
		"wtValuesOut":          bigIntSliceToStrings(wtValuesOut),
		"wtSaltsOut":           bigIntSliceToStrings(wtSaltsOut),
	}

	body, err := c.PostProof("/proof/payment", payload)
	if err != nil {
		return nil, fmt.Errorf("bound payment proof request failed: %w", err)
	}

	var gnarkResp struct {
		Proof        []json.Number `json:"proof"`
		PublicSignal []json.Number `json:"publicSignal"`
	}
	if err := json.Unmarshal(body, &gnarkResp); err != nil {
		return nil, fmt.Errorf("parse bound payment proof response: %w", err)
	}
	proofStrs := make([]string, len(gnarkResp.Proof))
	for i, n := range gnarkResp.Proof {
		proofStrs[i] = n.String()
	}

	// Statement: interleaved [msg, tree0, root0, nf0, ..., cmt0, cmt1, contractAddr]
	statement := make([]*big.Int, 0, 1+3*nIn+nOut+1)
	statement = append(statement, stMessage)
	for i := 0; i < nIn; i++ {
		statement = append(statement, stTreeNumbers[i], stMerkleRoots[i], stNullifiers[i])
	}
	statement = append(statement, stCommitmentsOut...)
	statement = append(statement, contractAddress)

	return &PaymentResult{
		Proof:           proofStrs,
		Statement:       statement,
		NumberOfInputs:  nIn,
		NumberOfOutputs: nOut,
		CipherText:      cipherText,
		EncTxData:       encTxData,
		SaltB:           wtSaltsOut[0],
		SaltA:           saltA,
		ContractAddress: contractAddress,
	}, nil
}

// BoundPaymentFeeProof generates a PaymentFee proof where the sender's note absorbs
// the protocol fee: Σ(valuesIn) == Σ(valuesOut) + stFee.
//
// stFee must equal PROTOCOL_FEE; the relayer checks this before submitting on-chain.
// The resulting ContractStatement() appends [contractAddress, stFee] at the end.
//
// The circuit posts to /proof/paymentFee on the gnark server.
func (c *GnarkClient) BoundPaymentFeeProof(
	contractAddress *big.Int,
	stFee *big.Int,
	stMessage *big.Int,
	wtValuesIn []*big.Int,
	keysIn []KeyPair,
	wtSaltsIn []*big.Int,
	wtValuesOut []*big.Int,
	recipientSpendPks []*big.Int,
	recipientViewEncapKeys [][]byte,
	merkleDepth int,
	merkleProofs []*MerkleProof,
	stTreeNumbers []*big.Int,
	wtTokenId *big.Int,
) (*PaymentResult, error) {
	nIn := len(wtValuesIn)
	nOut := len(wtValuesOut)

	stNullifiers := make([]*big.Int, nIn)
	wtPathIndices := make([]*big.Int, nIn)
	wtPathElements := make([]*big.Int, 0, nIn*merkleDepth)

	for i := 0; i < nIn; i++ {
		if wtValuesIn[i].Sign() == 0 {
			stNullifiers[i] = big.NewInt(0)
			wtPathIndices[i] = big.NewInt(0)
			zeros := make([]*big.Int, merkleDepth)
			for j := range zeros {
				zeros[j] = big.NewInt(0)
			}
			wtPathElements = append(wtPathElements, zeros...)
		} else {
			wtPathIndices[i] = merkleProofs[i].Indices
			wtPathElements = append(wtPathElements, merkleProofs[i].Elements...)
			nf, err := GetNullifierBound(keysIn[i].PrivateKey, wtPathIndices[i], contractAddress)
			if err != nil {
				return nil, fmt.Errorf("GetNullifierBound input %d: %w", i, err)
			}
			stNullifiers[i] = nf
		}
	}

	wtSaltsOut := make([]*big.Int, nOut)
	stCommitmentsOut := make([]*big.Int, nOut)
	var cipherText, encTxData []byte
	var saltA *big.Int

	ss0, ctxt0, err := Encapsulate(recipientViewEncapKeys[0])
	if err != nil {
		return nil, fmt.Errorf("Encapsulate Bob output: %w", err)
	}
	saltB0, err := DerivePaymentSalt(ss0)
	if err != nil {
		return nil, fmt.Errorf("DerivePaymentSalt Bob output: %w", err)
	}
	encKey0, err := DerivePaymentKey(ss0)
	if err != nil {
		return nil, fmt.Errorf("DerivePaymentKey Bob output: %w", err)
	}
	ctxtII0, err := EncryptPayload(encKey0, wtTokenId, wtValuesOut[0])
	if err != nil {
		return nil, fmt.Errorf("EncryptPayload Bob output: %w", err)
	}
	cipherText = ctxt0
	encTxData = ctxtII0
	wtSaltsOut[0] = SaltBToField(saltB0)
	cmt0, err := Erc20CommitmentV2(recipientSpendPks[0], wtSaltsOut[0], wtValuesOut[0], wtTokenId)
	if err != nil {
		return nil, fmt.Errorf("Erc20CommitmentV2 Bob output: %w", err)
	}
	stCommitmentsOut[0] = cmt0

	for i := 1; i < nOut; i++ {
		randSalt, err := RandomInField()
		if err != nil {
			return nil, fmt.Errorf("RandomInField change output %d: %w", i, err)
		}
		if i == 1 {
			saltA = randSalt
		}
		wtSaltsOut[i] = randSalt
		cmt, err := Erc20CommitmentV2(recipientSpendPks[i], randSalt, wtValuesOut[i], wtTokenId)
		if err != nil {
			return nil, fmt.Errorf("Erc20CommitmentV2 change output %d: %w", i, err)
		}
		stCommitmentsOut[i] = cmt
	}

	stMerkleRoots := make([]*big.Int, nIn)
	for i := range wtValuesIn {
		if wtValuesIn[i].Sign() == 0 {
			stMerkleRoots[i] = big.NewInt(0)
		} else {
			stMerkleRoots[i] = merkleProofs[i].Root
		}
	}

	pathElementChunks := chunkBigIntSlice(wtPathElements, merkleDepth)

	payload := map[string]interface{}{
		"stMessage":            stMessage.String(),
		"stTreeNumbers":        bigIntSliceToStrings(stTreeNumbers),
		"stMerkleRoots":        bigIntSliceToStrings(stMerkleRoots),
		"stNullifiers":         bigIntSliceToStrings(stNullifiers),
		"stCommitmentsOut":     bigIntSliceToStrings(stCommitmentsOut),
		"stContractAddress":    contractAddress.String(),
		"stFee":                stFee.String(),
		"wtPrivateKeysIn":      bigIntSliceToStrings(extractPrivateKeys(keysIn)),
		"wtValuesIn":           bigIntSliceToStrings(wtValuesIn),
		"wtSaltsIn":            bigIntSliceToStrings(wtSaltsIn),
		"wtPathElements":       bigIntChunksToStringChunks(pathElementChunks),
		"wtPathIndices":        bigIntSliceToStrings(wtPathIndices),
		"wtTokenId":            wtTokenId.String(),
		"wtSpendPublicKeysOut": bigIntSliceToStrings(recipientSpendPks),
		"wtValuesOut":          bigIntSliceToStrings(wtValuesOut),
		"wtSaltsOut":           bigIntSliceToStrings(wtSaltsOut),
	}

	body, err := c.PostProof("/proof/paymentFee", payload)
	if err != nil {
		return nil, fmt.Errorf("payment fee proof request failed: %w", err)
	}

	var gnarkResp struct {
		Proof        []json.Number `json:"proof"`
		PublicSignal []json.Number `json:"publicSignal"`
	}
	if err := json.Unmarshal(body, &gnarkResp); err != nil {
		return nil, fmt.Errorf("parse payment fee proof response: %w", err)
	}
	proofStrs := make([]string, len(gnarkResp.Proof))
	for i, n := range gnarkResp.Proof {
		proofStrs[i] = n.String()
	}

	// Statement: interleaved [msg, tree0, root0, nf0, ..., cmt0, cmt1, contractAddr, fee]
	statement := make([]*big.Int, 0, 1+3*nIn+nOut+2)
	statement = append(statement, stMessage)
	for i := 0; i < nIn; i++ {
		statement = append(statement, stTreeNumbers[i], stMerkleRoots[i], stNullifiers[i])
	}
	statement = append(statement, stCommitmentsOut...)
	statement = append(statement, contractAddress)
	statement = append(statement, stFee)

	return &PaymentResult{
		Proof:           proofStrs,
		Statement:       statement,
		NumberOfInputs:  nIn,
		NumberOfOutputs: nOut,
		CipherText:      cipherText,
		EncTxData:       encTxData,
		SaltB:           wtSaltsOut[0],
		SaltA:           saltA,
		ContractAddress: contractAddress,
		Fee:             stFee,
	}, nil
}

// BoundPaymentRelayerProof generates a 1-in/3-out payment proof where the third
// output is a spendable fee note for the relayer.
//
// Output layout:
//
//	output[0] = Bob's payment  — any pk_spend (ML-KEM delivery via bobViewEncapKey)
//	output[1] = Alice's change — constrained to senderPk in-circuit (random salt, SaltA)
//	output[2] = Relayer fee    — any pk_spend (random salt, SaltRelayer)
//
// Conservation: Σ(valuesIn) == valOut[0] + valOut[1] + valOut[2]   (no value burned)
//
// Contract statement (8 elements):
//
//	[msg, treeNum, root, nf, cmt_bob, cmt_change, cmt_relayer, contractAddr]
func (c *GnarkClient) BoundPaymentRelayerProof(
	contractAddress *big.Int,
	stMessage *big.Int,
	wtValuesIn []*big.Int,
	keysIn []KeyPair,
	wtSaltsIn []*big.Int,
	wtValuesOut []*big.Int, // [bobAmt, changeAmt, relayerFeeAmt]
	relayerSpendPk *big.Int, // relayer's BabyJubJub spend public key
	recipientSpendPks []*big.Int, // [bobSpendPk, aliceSpendPk] — relayer is appended internally
	bobViewEncapKey []byte, // ML-KEM encapsulation key for Bob only
	merkleDepth int,
	merkleProofs []*MerkleProof,
	stTreeNumbers []*big.Int,
	wtTokenId *big.Int,
) (*PaymentResult, error) {
	nIn := len(wtValuesIn)
	// nOut is always 3 for this circuit
	nOut := 3
	if len(wtValuesOut) != 3 {
		return nil, fmt.Errorf("BoundPaymentRelayerProof: want 3 output values, got %d", len(wtValuesOut))
	}

	stNullifiers := make([]*big.Int, nIn)
	wtPathIndices := make([]*big.Int, nIn)
	wtPathElements := make([]*big.Int, 0, nIn*merkleDepth)

	for i := 0; i < nIn; i++ {
		if wtValuesIn[i].Sign() == 0 {
			stNullifiers[i] = big.NewInt(0)
			wtPathIndices[i] = big.NewInt(0)
			zeros := make([]*big.Int, merkleDepth)
			for j := range zeros {
				zeros[j] = big.NewInt(0)
			}
			wtPathElements = append(wtPathElements, zeros...)
		} else {
			wtPathIndices[i] = merkleProofs[i].Indices
			wtPathElements = append(wtPathElements, merkleProofs[i].Elements...)
			nf, err := GetNullifierBound(keysIn[i].PrivateKey, wtPathIndices[i], contractAddress)
			if err != nil {
				return nil, fmt.Errorf("GetNullifierBound input %d: %w", i, err)
			}
			stNullifiers[i] = nf
		}
	}

	wtSaltsOut := make([]*big.Int, nOut)
	stCommitmentsOut := make([]*big.Int, nOut)

	// Output 0: Bob's payment — ML-KEM delivery
	ss0, ctxt0, err := Encapsulate(bobViewEncapKey)
	if err != nil {
		return nil, fmt.Errorf("Encapsulate Bob output: %w", err)
	}
	saltB0, err := DerivePaymentSalt(ss0)
	if err != nil {
		return nil, fmt.Errorf("DerivePaymentSalt Bob output: %w", err)
	}
	encKey0, err := DerivePaymentKey(ss0)
	if err != nil {
		return nil, fmt.Errorf("DerivePaymentKey Bob output: %w", err)
	}
	ctxtII0, err := EncryptPayload(encKey0, wtTokenId, wtValuesOut[0])
	if err != nil {
		return nil, fmt.Errorf("EncryptPayload Bob output: %w", err)
	}
	bobSpendPk := recipientSpendPks[0]
	wtSaltsOut[0] = SaltBToField(saltB0)
	cmt0, err := Erc20CommitmentV2(bobSpendPk, wtSaltsOut[0], wtValuesOut[0], wtTokenId)
	if err != nil {
		return nil, fmt.Errorf("Erc20CommitmentV2 Bob output: %w", err)
	}
	stCommitmentsOut[0] = cmt0

	// Output 1: Alice's change — random salt (SaltA)
	saltA, err := RandomInField()
	if err != nil {
		return nil, fmt.Errorf("RandomInField change output: %w", err)
	}
	aliceSpendPk := recipientSpendPks[1]
	wtSaltsOut[1] = saltA
	cmt1, err := Erc20CommitmentV2(aliceSpendPk, saltA, wtValuesOut[1], wtTokenId)
	if err != nil {
		return nil, fmt.Errorf("Erc20CommitmentV2 change output: %w", err)
	}
	stCommitmentsOut[1] = cmt1

	// Output 2: Relayer fee note — random salt (SaltRelayer)
	saltRelayer, err := RandomInField()
	if err != nil {
		return nil, fmt.Errorf("RandomInField relayer output: %w", err)
	}
	wtSaltsOut[2] = saltRelayer
	cmt2, err := Erc20CommitmentV2(relayerSpendPk, saltRelayer, wtValuesOut[2], wtTokenId)
	if err != nil {
		return nil, fmt.Errorf("Erc20CommitmentV2 relayer output: %w", err)
	}
	stCommitmentsOut[2] = cmt2

	// Build all spend public keys in order [bob, alice, relayer]
	allSpendPks := []*big.Int{bobSpendPk, aliceSpendPk, relayerSpendPk}

	stMerkleRoots := make([]*big.Int, nIn)
	for i := range wtValuesIn {
		if wtValuesIn[i].Sign() == 0 {
			stMerkleRoots[i] = big.NewInt(0)
		} else {
			stMerkleRoots[i] = merkleProofs[i].Root
		}
	}

	pathElementChunks := chunkBigIntSlice(wtPathElements, merkleDepth)

	payload := map[string]interface{}{
		"stMessage":            stMessage.String(),
		"stTreeNumbers":        bigIntSliceToStrings(stTreeNumbers),
		"stMerkleRoots":        bigIntSliceToStrings(stMerkleRoots),
		"stNullifiers":         bigIntSliceToStrings(stNullifiers),
		"stCommitmentsOut":     bigIntSliceToStrings(stCommitmentsOut),
		"stContractAddress":    contractAddress.String(),
		"wtPrivateKeysIn":      bigIntSliceToStrings(extractPrivateKeys(keysIn)),
		"wtValuesIn":           bigIntSliceToStrings(wtValuesIn),
		"wtSaltsIn":            bigIntSliceToStrings(wtSaltsIn),
		"wtPathElements":       bigIntChunksToStringChunks(pathElementChunks),
		"wtPathIndices":        bigIntSliceToStrings(wtPathIndices),
		"wtTokenId":            wtTokenId.String(),
		"wtSpendPublicKeysOut": bigIntSliceToStrings(allSpendPks),
		"wtValuesOut":          bigIntSliceToStrings(wtValuesOut),
		"wtSaltsOut":           bigIntSliceToStrings(wtSaltsOut),
	}

	body, err := c.PostProof("/proof/paymentRelayer", payload)
	if err != nil {
		return nil, fmt.Errorf("payment relayer proof request failed: %w", err)
	}

	var gnarkResp struct {
		Proof        []json.Number `json:"proof"`
		PublicSignal []json.Number `json:"publicSignal"`
	}
	if err := json.Unmarshal(body, &gnarkResp); err != nil {
		return nil, fmt.Errorf("parse payment relayer proof response: %w", err)
	}
	proofStrs := make([]string, len(gnarkResp.Proof))
	for i, n := range gnarkResp.Proof {
		proofStrs[i] = n.String()
	}

	// Statement (interleaved): [msg, tree0, root0, nf0, cmt0, cmt1, cmt2, contractAddr]
	statement := make([]*big.Int, 0, 1+3*nIn+nOut+1)
	statement = append(statement, stMessage)
	for i := 0; i < nIn; i++ {
		statement = append(statement, stTreeNumbers[i], stMerkleRoots[i], stNullifiers[i])
	}
	statement = append(statement, stCommitmentsOut...)
	statement = append(statement, contractAddress)

	return &PaymentResult{
		Proof:           proofStrs,
		Statement:       statement,
		NumberOfInputs:  nIn,
		NumberOfOutputs: nOut,
		CipherText:      ctxt0,
		EncTxData:       ctxtII0,
		SaltB:           wtSaltsOut[0],
		SaltA:           saltA,
		SaltRelayer:     saltRelayer,
		ContractAddress: contractAddress,
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// DvP Initiator (Alice's side)
// ─────────────────────────────────────────────────────────────────────────────

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
//   _pendingTransactions[commitB].targetReceiptId = commitA = statement[0]
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

// ─────────────────────────────────────────────────────────────────────────────
// DvP Destination (Bob's side)
// ─────────────────────────────────────────────────────────────────────────────

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
	saltB *big.Int,     // HKDF(ss_B, "note salt") — used by Alice to build commitB for Bob
	valueAlice *big.Int,  // Alice's delivered amount (from ENC_TX_DATA decryption)
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
		"stMessage":       commitB.String(), // HIGH-3: stMessage == commitB (circuit-enforced)
		"stTreeNumber":    stTreeNumber.String(),
		"stMerkleRoot":    merkleProof.Root.String(),
		"stNullifier":     nf.String(),
		"stCommitA":       commitA.String(),
		"wtSpendKeyIn":    bobKey.PrivateKey.String(),
		"wtValueIn":       valueIn.String(),
		"wtSaltIn":        bobSaltIn.String(),
		"wtTokenIdIn":     tokenIdIn.String(),
		"wtPathElements":  bigIntSliceToStrings(pathElems),
		"wtPathIndex":     pathIndex.String(),
		"wtSpendPkAlice":  aliceSpendPk.String(),
		"wtSaltA":         saltA.String(),
		"wtSaltB":         saltB.String(),
		"wtValueAlice":    valueAlice.String(),
		"wtTokenIdAlice":  tokenIdAlice.String(),
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

// --- legacy map-based provers (used by GnarkProver dispatch) ---

// Erc20Proof generates a JoinSplitERC20 proof (map-based pass-through).
func (c *GnarkClient) Erc20Proof(inputs map[string]interface{}, zkeyPath string) (*ProofResponse, error) {
	endpoint := "/proof/joinSplitERC20"
	if zkeyPath == "./build/JoinSplitErc20_10_2.zkey" {
		endpoint = "/proof/joinSplitERC20_10_2"
	}

	chunks := SplitPathElements(inputs)

	payload := map[string]interface{}{
		"StMessage":              toString(inputs["st_message"]),
		"StTreeNumber":           toStringSlice(inputs["st_treeNumbers"]),
		"StMerkleRoots":          inputs["st_merkleRoots"],
		"StNullifiers":           inputs["st_nullifiers"],
		"StCommitmentOut":        inputs["st_commitmentsOut"],
		"WtPrivateKeysIn":        inputs["wt_privateKeysIn"],
		"WtPublicKeysOut":        inputs["wt_publicKeysOut"],
		"WtPathElements":         chunks,
		"WtPathIndices":          toStringSlice(inputs["wt_pathIndices"]),
		"WtValuesIn":             inputs["wt_valuesIn"],
		"WtValuesOut":            inputs["wt_valuesOut"],
		"WtErc20ContractAddress": toString(inputs["wt_erc20ContractAddress"]),
	}

	_, err := c.PostProof(endpoint, payload)
	if err != nil {
		return nil, err
	}
	return &ProofResponse{Status: 200, Message: "ok"}, nil
}

// Erc721Proof generates an OwnershipERC721 proof (map-based pass-through).
func (c *GnarkClient) Erc721Proof(inputs map[string]interface{}) (*ProofResponse, error) {
	payload := map[string]interface{}{
		"StMessage":       inputs["st_message"],
		"StTreeNumbers":   toStringSlice(inputs["st_treeNumbers"]),
		"StMerkleRoots":   toStringSlice(inputs["st_merkleRoots"]),
		"StNullifiers":    inputs["st_nullifiers"],
		"StCommitmentOut": inputs["st_commitmentsOut"],
		"WtPrivateKeysIn": inputs["wt_privateKeysIn"],
		"WtValues":        inputs["wt_values"],
		"WtPathElements":  []interface{}{inputs["wt_pathElements"]},
		"WtPathIndices":   inputs["wt_pathIndices"],
		"WtPublicKeysOut": inputs["wt_publicKeysOut"],
	}

	_, err := c.PostProof("/proof/ownershipERC721", payload)
	if err != nil {
		return nil, err
	}
	return &ProofResponse{Status: 200, Message: "ok"}, nil
}

// Erc1155FungibleProof generates an ERC1155 fungible proof (map-based pass-through).
func (c *GnarkClient) Erc1155FungibleProof(inputs map[string]interface{}) (*ProofResponse, error) {
	pathElements := toInterfaceSlice(inputs["wt_pathElements"])
	split1, split2 := splitIntoTwo(pathElements, 8)

	payload := map[string]interface{}{
		"StMessage":                inputs["st_message"],
		"StTreeNumbers":            toStringSlice(inputs["st_treeNumbers"]),
		"StMerkleRoots":            inputs["st_merkleRoots"],
		"StCommitmentOut":          inputs["st_commitmentsOut"],
		"StNullifiers":             inputs["st_nullifiers"],
		"StAssetGroupMerkleRoot":   toString(inputs["st_assetGroup_merkleRoot"]),
		"StAssetGroupTreeNumber":   toString(inputs["st_assetGroup_treeNumber"]),
		"WtPrivateKeysIn":          inputs["wt_privateKeysIn"],
		"WtValuesIn":               toStringSlice(inputs["wt_valuesIn"]),
		"WtPathElements":           []interface{}{split1, split2},
		"WtPathIndices":            toStringSlice(inputs["wt_pathIndices"]),
		"WtErc1155ContractAddress": inputs["wt_erc1155ContractAddress"],
		"WtErc1155TokenId":         inputs["wt_erc1155TokenId"],
		"WtPublicKeysOut":          inputs["wt_publicKeysOut"],
		"WtValuesOut":              toStringSlice(inputs["wt_valuesOut"]),
		"WtAssetGroupPathElements": toStringSlice(inputs["wt_assetGroup_pathElements"]),
		"WtAssetGroupPathIndices":  inputs["wt_assetGroup_pathIndices"],
	}

	_, err := c.PostProof("/proof/erc155Fungible", payload)
	if err != nil {
		return nil, err
	}
	return &ProofResponse{Status: 200, Message: "ok"}, nil
}

// Erc1155FungibleAuditorMapProof generates an ERC1155 fungible proof with auditor fields (map-based).
func (c *GnarkClient) Erc1155FungibleAuditorMapProof(inputs map[string]interface{}) (*ProofResponse, error) {
	pathElements := toInterfaceSlice(inputs["wt_pathElements"])
	split1, split2 := splitIntoTwo(pathElements, 8)

	payload := map[string]interface{}{
		"StMessage":                inputs["st_message"],
		"StTreeNumbers":            toStringSlice(inputs["st_treeNumbers"]),
		"StMerkleRoots":            inputs["st_merkleRoots"],
		"StCommitmentOut":          inputs["st_commitmentsOut"],
		"StNullifiers":             inputs["st_nullifiers"],
		"StAssetGroupMerkleRoot":   toString(inputs["st_assetGroup_merkleRoot"]),
		"StAssetGroupTreeNumber":   toString(inputs["st_assetGroup_treeNumber"]),
		"WtPrivateKeysIn":          inputs["wt_privateKeysIn"],
		"WtValuesIn":               toStringSlice(inputs["wt_valuesIn"]),
		"WtPathElements":           []interface{}{split1, split2},
		"WtPathIndices":            toStringSlice(inputs["wt_pathIndices"]),
		"WtErc1155ContractAddress": inputs["wt_erc1155ContractAddress"],
		"WtErc1155TokenId":         inputs["wt_erc1155TokenId"],
		"WtPublicKeysOut":          inputs["wt_publicKeysOut"],
		"WtValuesOut":              toStringSlice(inputs["wt_valuesOut"]),
		"WtAssetGroupPathElements": toStringSlice(inputs["wt_assetGroup_pathElements"]),
		"WtAssetGroupPathIndices":  inputs["wt_assetGroup_pathIndices"],
		"StAuditorPublickey":       inputs["st_auditor_publicKey"],
		"StAuditorAuthKey":         inputs["st_auditor_authKey"],
		"StAuditorNonce":           inputs["st_auditor_nonce"],
		"StAuditorEncryptedValues": inputs["st_auditor_encryptedValues"],
		"WtAuditorRandom":          inputs["wt_auditor_random"],
	}

	_, err := c.PostProof("/proof/erc1155FungibleAuditor", payload)
	if err != nil {
		return nil, err
	}
	return &ProofResponse{Status: 200, Message: "ok"}, nil
}

// Erc1155NonFungibleProof generates an ERC1155 non-fungible proof (map-based pass-through).
func (c *GnarkClient) Erc1155NonFungibleProof(inputs map[string]interface{}) (*ProofResponse, error) {
	payload := map[string]interface{}{
		"StMessage":                toString(inputs["st_message"]),
		"StTreeNumbers":            toStringSlice(inputs["st_treeNumbers"]),
		"StMerkleRoots":            inputs["st_merkleRoots"],
		"StNullifiers":             inputs["st_nullifiers"],
		"StCommitmentOut":          inputs["st_commitmentsOut"],
		"StAssetGroupTreeNumber":   toStringSlice(inputs["st_assetGroup_treeNumbers"]),
		"StAssetGroupMerkleRoot":   inputs["st_assetGroup_merkleRoots"],
		"WtPrivateKeysIn":          inputs["wt_privateKeysIn"],
		"WtValues":                 inputs["wt_values"],
		"WtPathElements":           []interface{}{inputs["wt_pathElements"]},
		"WtPathIndices":            inputs["wt_pathIndices"],
		"WtErc1155TokenId":         inputs["wt_erc1155TokenIds"],
		"WtPublicKeysOut":          inputs["wt_publicKeysOut"],
		"WtErc1155ContractAddress": inputs["wt_erc1155ContractAddress"],
		"WtAssetGroupPathElements": toNestedStringSlice(inputs["wt_assetGroup_pathElements"]),
		"WtAssetGroupPathIndices":  inputs["wt_assetGroup_pathIndices"],
	}

	_, err := c.PostProof("/proof/erc1155NonFungible", payload)
	if err != nil {
		return nil, err
	}
	return &ProofResponse{Status: 200, Message: "ok"}, nil
}

// Erc1155NonFungibleWithAuditorProof generates an ERC1155 non-fungible proof with auditor (map-based pass-through).
func (c *GnarkClient) Erc1155NonFungibleWithAuditorProof(inputs map[string]interface{}) (*ProofResponse, error) {
	payload := map[string]interface{}{
		"StMessage":                toString(inputs["st_message"]),
		"StTreeNumbers":            toStringSlice(inputs["st_treeNumbers"]),
		"StMerkleRoots":            inputs["st_merkleRoots"],
		"StNullifiers":             inputs["st_nullifiers"],
		"StCommitmentOut":          inputs["st_commitmentsOut"],
		"StAssetGroupTreeNumber":   toStringSlice(inputs["st_assetGroup_treeNumbers"]),
		"StAssetGroupMerkleRoot":   inputs["st_assetGroup_merkleRoots"],
		"WtPrivateKeysIn":          inputs["wt_privateKeysIn"],
		"WtValues":                 inputs["wt_values"],
		"WtPathElements":           []interface{}{inputs["wt_pathElements"]},
		"WtPathIndices":            inputs["wt_pathIndices"],
		"WtErc1155TokenIds":        inputs["wt_erc1155TokenIds"],
		"WtPublicKeysOut":          inputs["wt_publicKeysOut"],
		"WtErc1155ContractAddress": inputs["wt_erc1155ContractAddress"],
		"WtAssetGroupPathElements": toNestedStringSlice(inputs["wt_assetGroup_pathElements"]),
		"WtAssetGroupPathIndices":  inputs["wt_assetGroup_pathIndices"],
		"StAuditorPublickey":       inputs["st_auditor_publicKey"],
		"StAuditorAuthKey":         inputs["st_auditor_authKey"],
		"StAuditorNonce":           inputs["st_auditor_nonce"],
		"StAuditorEncryptedValues": inputs["st_auditor_encryptedValues"],
		"WtAuditorRandom":          inputs["wt_auditor_random"],
	}

	_, err := c.PostProof("/proof/erc1155NonFungibleAuditor", payload)
	if err != nil {
		return nil, err
	}
	return &ProofResponse{Status: 200, Message: "ok"}, nil
}

// PrivateMintProof generates a private mint proof and returns the proof and public signals.
func (c *GnarkClient) PrivateMintProof(inputs map[string]interface{}) (*PrivateMintProofResponse, error) {
	salt := "0"
	if s, ok := inputs["salt"]; ok && s != nil {
		salt = toString(s)
	}

	payload := map[string]interface{}{
		"commitment":      toString(inputs["commitment"]),
		"contractAddress": toString(inputs["contractAddress"]),
		"tokenId":         toString(inputs["tokenId"]),
		"salt":            salt,
		"amount":          toString(inputs["amount"]),
		"publicKey":       toString(inputs["publicKey"]),
		"cipherText":      toString(inputs["cipherText"]),
	}

	body, err := c.PostProof("/proof/privateMint", payload)
	if err != nil {
		return nil, fmt.Errorf("PrivateMint proof generation failed: %w", err)
	}

	var resp PrivateMintProofResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse PrivateMint response: %w", err)
	}

	return &resp, nil
}
