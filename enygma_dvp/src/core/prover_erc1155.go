package core

import (
	"encoding/json"
	"fmt"
	"math/big"
)

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
		CipherText:      ctI,
		EncTxData:       ctII,
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
		CipherText:      [][]byte{ctI},
		EncTxData:       [][]byte{ctII},
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
		CipherText:      [][]byte{ctI},
		EncTxData:       [][]byte{ctII},
	}, nil
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
		CipherText:      ctI,
		EncTxData:       ctII,
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
		CipherText:      [][]byte{ctI},
		EncTxData:       [][]byte{ctII},
		AuditData: &AuditEncryptionData{
			AuthKeyX:   authKeyX,
			AuthKeyY:   authKeyY,
			Nonce:      auditorNonce,
			Encrypted:  encrypted,
			RealLength: len(plaintext),
		},
	}, nil
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
