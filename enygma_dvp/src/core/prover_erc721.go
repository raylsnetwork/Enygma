package core

import (
	"encoding/json"
	"fmt"
	"math/big"
)

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
		CipherText:      [][]byte{ctI},
		EncTxData:       [][]byte{ctII},
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
		CipherText:      [][]byte{ctI},
		EncTxData:       [][]byte{ctII},
	}, nil
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
