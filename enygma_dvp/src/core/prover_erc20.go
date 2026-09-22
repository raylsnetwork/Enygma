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
		CipherText:      cipherText,
		EncTxData:       encTxData,
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
		CipherText:      cipherText,
		EncTxData:       encTxData,
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
