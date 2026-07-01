package core

import (
	"fmt"
	"math/big"
)

// AuctionInitProof generates a proof for AuctionInit.circom.
// It prepares circuit inputs and sends them to the gnark server.
func (c *GnarkClient) AuctionInitProof(
	stBeacon *big.Int,
	tokenId *big.Int,
	wtContractAddress *big.Int,
	keyIn KeyPair,
	merkleDepth int,
	merkleProof *MerkleProof,
	merkleRoot *big.Int,
	stTreeNumber *big.Int,
	vaultId int,
	assetGroupMerkleRoot *big.Int,
	assetGroupMerkleProof *MerkleProof,
	wtIdParams []*big.Int,
) (*ProofResult, error) {
	wtPathElements := merkleProof.Elements
	wtPathIndices := merkleProof.Indices

	stNullifier, err := GetNullifier(keyIn.PrivateKey, wtPathIndices)
	if err != nil {
		return nil, fmt.Errorf("failed to compute nullifier: %w", err)
	}

	uniqueId, err := computeUniqueId(vaultId, wtContractAddress, wtIdParams)
	if err != nil {
		return nil, fmt.Errorf("failed to compute uniqueId: %w", err)
	}

	wtCommitment, err := GetCommitment(uniqueId, keyIn.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to compute commitment: %w", err)
	}

	stAuctionId, err := GetAuctionId(wtCommitment)
	if err != nil {
		return nil, fmt.Errorf("failed to compute auctionId: %w", err)
	}

	wtAssetGroupPathElements, wtAssetGroupPathIndices := resolveAssetGroupProof(
		assetGroupMerkleRoot, assetGroupMerkleProof, merkleDepth,
	)

	payload := map[string]interface{}{
		"StBeacon":                 stBeacon.String(),
		"StVaultId":                fmt.Sprintf("%d", vaultId),
		"StAuctionId":              stAuctionId.String(),
		"StNullifier":              stNullifier.String(),
		"StTreeNumber":             stTreeNumber.String(),
		"StMerkleRoot":             merkleRoot.String(),
		"WtPrivateKey":             keyIn.PrivateKey.String(),
		"WtPathElements":           bigIntSliceToStrings(wtPathElements),
		"WtPathIndices":            wtPathIndices.String(),
		"WtCommitment":             wtCommitment.String(),
		"StAssetGroupMerkleRoot":   assetGroupMerkleRoot.String(),
		"WtAssetGroupPathElements": bigIntSliceToStrings(wtAssetGroupPathElements),
		"WtAssetGroupPathIndices":  bigIntToString(wtAssetGroupPathIndices),
		"WtIdParams":               bigIntSliceToStrings(wtIdParams),
		"WtContractAddress":        wtContractAddress.String(),
	}

	_, err = c.PostProof("/proof/auctionInit", payload)
	if err != nil {
		return nil, fmt.Errorf("auctionInit proof request failed: %w", err)
	}

	statement := []*big.Int{
		stBeacon,
		big.NewInt(int64(vaultId)),
		stAuctionId,
		stTreeNumber,
		merkleRoot,
		stNullifier,
		assetGroupMerkleRoot,
	}

	return &ProofResult{
		Statement:       statement,
		NumberOfInputs:  1,
		NumberOfOutputs: 1,
	}, nil
}

// AuctionBidProof generates a proof for AuctionBidErc20.circom.
func (c *GnarkClient) AuctionBidProof(
	stAuctionId *big.Int,
	wtBidAmount *big.Int,
	wtBidRandom *big.Int,
	assetAddress *big.Int,
	wtValuesIn []*big.Int,
	keysIn []KeyPair,
	wtValuesOut []*big.Int,
	keysOut []KeyPair,
	merkleDepth int,
	merkleProofs []*MerkleProof,
	stMerkleRoots []*big.Int,
	stTreeNumbers []*big.Int,
	stVaultId int,
	wtIdParamsIn [][]*big.Int,
	wtIdParamsOut [][]*big.Int,
	stAssetGroupMerkleRoot *big.Int,
	assetGroupMerkleProof *MerkleProof,
) (*ProofResult, error) {
	stBlindedBid, err := Pedersen(wtBidAmount, wtBidRandom)
	if err != nil {
		return nil, fmt.Errorf("failed to compute blindedBid: %w", err)
	}

	stCommitmentsOut := make([]*big.Int, len(keysOut))
	stNullifiers := make([]*big.Int, len(wtValuesIn))
	wtPathIndices := make([]*big.Int, len(wtValuesIn))
	wtPathElements := make([]*big.Int, 0)

	for i := 0; i < len(wtValuesIn); i++ {
		_, err := computeUniqueId(stVaultId, assetAddress, wtIdParamsIn[i])
		if err != nil {
			return nil, fmt.Errorf("failed to compute uniqueId for input %d: %w", i, err)
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

		nullifier, err := GetNullifier(keysIn[i].PrivateKey, wtPathIndices[i])
		if err != nil {
			return nil, fmt.Errorf("failed to compute nullifier for input %d: %w", i, err)
		}
		stNullifiers[i] = nullifier
	}

	for i := 0; i < len(keysOut); i++ {
		uniqueIdOut, err := computeUniqueId(stVaultId, assetAddress, wtIdParamsOut[i])
		if err != nil {
			return nil, fmt.Errorf("failed to compute uniqueId for output %d: %w", i, err)
		}
		cmt, err := GetCommitment(uniqueIdOut, keysOut[i].PublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to compute commitment for output %d: %w", i, err)
		}
		stCommitmentsOut[i] = cmt
	}

	wtAssetGroupPathElements, wtAssetGroupPathIndices := resolveAssetGroupProof(
		stAssetGroupMerkleRoot, assetGroupMerkleProof, merkleDepth,
	)

	pathElementChunks := chunkBigIntSlice(wtPathElements, merkleDepth)
	idParamsInFlat := flattenBigIntSlice(wtIdParamsIn)
	idParamsOutFlat := flattenBigIntSlice(wtIdParamsOut)

	payload := map[string]interface{}{
		"StAuctionId":              stAuctionId.String(),
		"StBlindedBid":             stBlindedBid.String(),
		"StVaultId":                fmt.Sprintf("%d", stVaultId),
		"StTreeNumbers":            bigIntSliceToStrings(stTreeNumbers),
		"StMerkleRoots":            bigIntSliceToStrings(stMerkleRoots),
		"StNullifiers":             bigIntSliceToStrings(stNullifiers),
		"StCommitmentsOuts":        bigIntSliceToStrings(stCommitmentsOut),
		"StAssetGroupMerkleRoot":   stAssetGroupMerkleRoot.String(),
		"WtBidAmount":              wtBidAmount.String(),
		"WtBidRandom":              wtBidRandom.String(),
		"WtPrivateKeys":            bigIntSliceToStrings(extractPrivateKeys(keysIn)),
		"WtValuesIn":               bigIntSliceToStrings(wtValuesIn),
		"WtPathElements":           bigIntChunksToStringChunks(pathElementChunks),
		"WtPathIndices":            bigIntSliceToStrings(wtPathIndices),
		"WtContractAddress":        assetAddress.String(),
		"WtRecipientPK":            bigIntSliceToStrings(extractPublicKeys(keysOut)),
		"WtValuesOut":              bigIntSliceToStrings(wtValuesOut),
		"WtAssetGroupPathElements": bigIntSliceToStrings(wtAssetGroupPathElements),
		"WtAssetGroupPathIndices":  bigIntToString(wtAssetGroupPathIndices),
		"WtIdParamsIn":             bigIntSliceToStrings(idParamsInFlat),
		"WtIdParamsOut":            bigIntSliceToStrings(idParamsOutFlat),
	}

	_, err = c.PostProof("/proof/auctionBid", payload)
	if err != nil {
		return nil, fmt.Errorf("auctionBid proof request failed: %w", err)
	}

	statement := make([]*big.Int, 0)
	statement = append(statement, stAuctionId, stBlindedBid, big.NewInt(int64(stVaultId)))
	statement = append(statement, stTreeNumbers...)
	statement = append(statement, stMerkleRoots...)
	statement = append(statement, stNullifiers...)
	statement = append(statement, stCommitmentsOut...)
	statement = append(statement, stAssetGroupMerkleRoot)

	return &ProofResult{
		Statement:       statement,
		NumberOfInputs:  2,
		NumberOfOutputs: 2,
	}, nil
}

// AuctionPrivateOpeningProof2 generates a proof for AuctionPrivateOpening.circom.
func (c *GnarkClient) AuctionPrivateOpeningProof2(
	stAuctionId *big.Int,
	stBlindedBid *big.Int,
	wtBidAmount *big.Int,
	wtBidRandom *big.Int,
) (*ProofResult, error) {
	payload := map[string]interface{}{
		"StVaultId":    stAuctionId.String(),
		"StBlindedBid": stBlindedBid.String(),
		"WtBidAmount":  wtBidAmount.String(),
		"WtBidRandom":  wtBidRandom.String(),
	}

	_, err := c.PostProof("/proof/auctionPrivateOpening", payload)
	if err != nil {
		return nil, fmt.Errorf("auctionPrivateOpening proof request failed: %w", err)
	}

	statement := []*big.Int{stAuctionId, stBlindedBid}

	return &ProofResult{
		Statement:       statement,
		NumberOfInputs:  1,
		NumberOfOutputs: 1,
	}, nil
}

// AuctionNotWinningBidProof2 generates a proof that a bid is less than the winning bid.
func (c *GnarkClient) AuctionNotWinningBidProof2(
	stAuctionId *big.Int,
	stBidBlockNumber *big.Int,
	stWinningBidBlockNumber *big.Int,
	wtBidAmount *big.Int,
	wtBidRandom *big.Int,
	wtWinningBidAmount *big.Int,
	wtWinningBidRandom *big.Int,
) (*ProofResult, error) {
	blindedBid, err := Pedersen(wtBidAmount, wtBidRandom)
	if err != nil {
		return nil, fmt.Errorf("failed to compute blindedBid: %w", err)
	}

	blindedWinningBid, err := Pedersen(wtWinningBidAmount, wtWinningBidRandom)
	if err != nil {
		return nil, fmt.Errorf("failed to compute blindedWinningBid: %w", err)
	}

	// stBlindedBidDifference = (blindedWinningBid - blindedBid + SNARK_SCALAR_FIELD) % SNARK_SCALAR_FIELD
	stBlindedBidDifference := new(big.Int).Sub(blindedWinningBid, blindedBid)
	stBlindedBidDifference.Add(stBlindedBidDifference, SNARK_SCALAR_FIELD)
	stBlindedBidDifference.Mod(stBlindedBidDifference, SNARK_SCALAR_FIELD)

	payload := map[string]interface{}{
		"StAuctionId":             stAuctionId.String(),
		"StBlindedBidDifference":  stBlindedBidDifference.String(),
		"StBidBlockNumber":        stBidBlockNumber.String(),
		"StWinningBidBlockNumber": stWinningBidBlockNumber.String(),
		"WtBidAmount":             wtBidAmount.String(),
		"WtBidRandom":             wtBidRandom.String(),
		"WtWinningBidAmount":      wtWinningBidAmount.String(),
		"WtWinningBidRandom":      wtWinningBidRandom.String(),
	}

	_, err = c.PostProof("/proof/auctionNotWinning", payload)
	if err != nil {
		return nil, fmt.Errorf("auctionNotWinning proof request failed: %w", err)
	}

	statement := []*big.Int{
		stAuctionId,
		stBlindedBidDifference,
		stBidBlockNumber,
		stWinningBidBlockNumber,
	}

	return &ProofResult{
		Statement:       statement,
		NumberOfInputs:  1,
		NumberOfOutputs: 1,
	}, nil
}

// AuctionInitMapProof generates an auction init proof (map-based pass-through, non-auditor).
func (c *GnarkClient) AuctionInitMapProof(inputs map[string]interface{}) (*ProofResponse, error) {
	payload := map[string]interface{}{
		"StBeacon":                toString(inputs["st_beacon"]),
		"StVaultId":               toString(inputs["st_vaultId"]),
		"StAuctionId":             toString(inputs["st_auctionId"]),
		"StTreeNumber":            toString(inputs["st_treeNumber"]),
		"StMerkleRoot":            inputs["st_merkleRoot"],
		"StNullifier":             inputs["st_nullifier"],
		"StAssetGroupMerkleRoot":  toString(inputs["st_assetGroup_merkleRoot"]),
		"WtCommitment":            inputs["wt_commitment"],
		"WtPathElements":          inputs["wt_pathElements"],
		"WtPathIndices":           toString(inputs["wt_pathIndices"]),
		"WtPrivateKey":            inputs["wt_privateKey"],
		"WtIdParams":              inputs["wt_idParams"],
		"WtContractAddress":       inputs["wt_contractAddress"],
		"WtAssetGroupPathElements": toStringSlice(inputs["wt_assetGroup_pathElements"]),
		"WtAssetGroupPathIndices":  inputs["wt_assetGroup_pathIndices"],
	}

	_, err := c.PostProof("/proof/auctionInit", payload)
	if err != nil {
		return nil, err
	}
	return &ProofResponse{Status: 200, Message: "ok"}, nil
}

// AuctionBidMapProof generates an auction bid proof (map-based pass-through, non-auditor).
func (c *GnarkClient) AuctionBidMapProof(inputs map[string]interface{}) (*ProofResponse, error) {
	pathElements := toInterfaceSlice(inputs["wt_pathElements"])
	split1, split2 := splitIntoTwo(pathElements, 8)

	payload := map[string]interface{}{
		"StAuctionId":             toString(inputs["st_auctionId"]),
		"StBlindedBid":            toString(inputs["st_blindedBid"]),
		"StVaultId":               toString(inputs["st_vaultId"]),
		"StTreeNumbers":           toStringSlice(inputs["st_treeNumbers"]),
		"StMerkleRoots":           toStringSlice(inputs["st_merkleRoots"]),
		"StNullifiers":            toStringSlice(inputs["st_nullifiers"]),
		"StCommitmentsOuts":       toStringSlice(inputs["st_commitmentsOut"]),
		"StAssetGroupMerkleRoot":  toString(inputs["st_assetGroup_merkleRoot"]),
		"WtBidAmount":             toString(inputs["wt_bidAmount"]),
		"WtBidRandom":             toString(inputs["wt_bidRandom"]),
		"WtPrivateKeys":           toStringSlice(inputs["wt_privateKeysIn"]),
		"WtPathElements": []interface{}{
			toStringSlice(split1),
			toStringSlice(split2),
		},
		"WtPathIndices":            toStringSlice(inputs["wt_pathIndices"]),
		"WtContractAddress":        toString(inputs["wt_contractAddress"]),
		"WtRecipientPK":            toStringSlice(inputs["wt_publicKeysOut"]),
		"WtValuesOut":              toStringSlice(inputs["wt_valuesOut"]),
		"WtAssetGroupPathElements": toStringSlice(inputs["wt_assetGroup_pathElements"]),
		"WtAssetGroupPathIndices":  toString(inputs["wt_assetGroup_pathIndices"]),
		"WtIdParamsIn":             toNestedStringSlice(inputs["wt_idParamsIn"]),
		"WtIdParamsOut":            toNestedStringSlice(inputs["wt_idParamsOut"]),
	}

	_, err := c.PostProof("/proof/auctionBid", payload)
	if err != nil {
		return nil, err
	}
	return &ProofResponse{Status: 200, Message: "ok"}, nil
}

// AuctionBidAuditorProof generates an auction bid proof with auditor fields.
func (c *GnarkClient) AuctionBidAuditorProof(inputs map[string]interface{}) (*ProofResponse, error) {
	pathElements := toInterfaceSlice(inputs["wt_pathElements"])
	split1, split2 := splitIntoTwo(pathElements, 8)

	payload := map[string]interface{}{
		"stBeacon":                toString(inputs["st_beacon"]),
		"stAuctionId":             toString(inputs["st_auctionId"]),
		"stBlindedBid":            toString(inputs["st_blindedBid"]),
		"stVaultId":               toString(inputs["st_vaultId"]),
		"stTreeNumbers":           toStringSlice(inputs["st_treeNumbers"]),
		"stMerkleRoots":           toStringSlice(inputs["st_merkleRoots"]),
		"stNullifiers":            toStringSlice(inputs["st_nullifiers"]),
		"stCommitmentsOuts":       toStringSlice(inputs["st_commitmentsOut"]),
		"stAssetGroupTreeNumber":  toString(inputs["st_assetGroup_treeNumber"]),
		"stAssetGrupoMerkleRoot":  toString(inputs["st_assetGroup_merkleRoot"]),
		"stAuctioneerPublicKey":   toStringSlice(inputs["st_auctioneer_publicKey"]),
		"stAuctioneerAuthKey":     toStringSlice(inputs["st_auctioneer_authKey"]),
		"stAuctioneerNonce":       toString(inputs["st_auctioneer_nonce"]),
		"stAuctioneerEncryptedValues": toStringSlice(inputs["st_auctioneer_encryptedValues"]),
		"wtAuctioneerRandom":      toString(inputs["wt_auctioneer_random"]),
		"stAuditorPublicKey":      toStringSlice(inputs["st_auditor_publicKey"]),
		"stAuditorAuthKey":        toStringSlice(inputs["st_auditor_authKey"]),
		"stAuditorNonce":          toString(inputs["st_auditor_nonce"]),
		"stAuditorEncryptedValues": toStringSlice(inputs["st_auditor_encryptedValues"]),
		"wtAuditoRandom":          toString(inputs["wt_auditor_random"]),
		"wtBidAmount":             toString(inputs["wt_bidAmount"]),
		"wtBidRandom":             toString(inputs["wt_bidRandom"]),
		"wtPrivateKeysIn":         toStringSlice(inputs["wt_privateKeysIn"]),
		"wtPathElements": []interface{}{
			toStringSlice(split1),
			toStringSlice(split2),
		},
		"wtPathIndices":            toStringSlice(inputs["wt_pathIndices"]),
		"wtContractAddress":        toString(inputs["wt_contractAddress"]),
		"wtPublicKeysOut":          toStringSlice(inputs["wt_publicKeysOut"]),
		"wtAssetGroupPathElements": toStringSlice(inputs["wt_assetGroup_pathElements"]),
		"wtAssetGroupPathIndices":  toString(inputs["wt_assetGroup_pathIndices"]),
		"wtIdParamsIn":             toNestedStringSlice(inputs["wt_idParamsIn"]),
		"wtIdParamsOut":            toNestedStringSlice(inputs["wt_idParamsOut"]),
	}

	_, err := c.PostProof("/proof/auctionBidAuditor", payload)
	if err != nil {
		return nil, err
	}
	return &ProofResponse{Status: 200, Message: "ok"}, nil
}

// AuctionInitAuditorProof generates an auction init proof with auditor fields.
func (c *GnarkClient) AuctionInitAuditorProof(inputs map[string]interface{}) (*ProofResponse, error) {
	payload := map[string]interface{}{
		"StBeacon":                 toString(inputs["st_beacon"]),
		"StAuctionId":              toString(inputs["st_auctionId"]),
		"StVaultId":                toString(inputs["st_vaultId"]),
		"StTreeNumber":             toString(inputs["st_treeNumber"]),
		"StMerkleRoot":             inputs["st_merkleRoot"],
		"StNullifier":              inputs["st_nullifier"],
		"StAuditorPublicKey":       toStringSlice(inputs["st_auditor_publicKey"]),
		"StAuditorAuthKey":         toStringSlice(inputs["st_auditor_authKey"]),
		"StAuditorNonce":           inputs["st_auditor_nonce"],
		"StAuditorEncryptedValues": toStringSlice(inputs["st_auditor_encryptedValues"]),
		"WtAuditorRandom":          inputs["wt_auditor_random"],
		"StAssetGroupTreeNumber":   inputs["st_assetGroup_treeNumber"],
		"StAssetGroupMerkleRoot":   toString(inputs["st_assetGroup_merkleRoot"]),
		"WtCommitment":             inputs["wt_commitment"],
		"WtPathElements":           inputs["wt_pathElements"],
		"WtPathIndices":            toString(inputs["wt_pathIndices"]),
		"WtPrivateKey":             inputs["wt_privateKey"],
		"WtIdParams":               inputs["wt_idParams"],
		"WtContractAddress":        inputs["wt_contractAddress"],
		"WtAssetGroupPathElements": toStringSlice(inputs["wt_assetGroup_pathElements"]),
		"WtAssetGroupPathIndices":  inputs["wt_assetGroup_pathIndices"],
	}

	_, err := c.PostProof("/proof/auctionInitAuditor", payload)
	if err != nil {
		return nil, err
	}
	return &ProofResponse{Status: 200, Message: "ok"}, nil
}

// AuctionPrivateOpeningProof generates an auction private opening proof (map-based pass-through).
func (c *GnarkClient) AuctionPrivateOpeningProof(inputs map[string]interface{}) (*ProofResponse, error) {
	payload := map[string]interface{}{
		"StVaultId":    toString(inputs["st_auctionId"]),
		"StBlindedBid": toString(inputs["st_blindedBid"]),
		"WtBidAmount":  toString(inputs["wt_bidAmount"]),
		"WtBidRandom":  toString(inputs["wt_bidRandom"]),
	}

	_, err := c.PostProof("/proof/auctionPrivateOpening", payload)
	if err != nil {
		return nil, err
	}
	return &ProofResponse{Status: 200, Message: "ok"}, nil
}

// AuctionNotWinningBidProof generates a proof that a bid is less than the winning bid (map-based pass-through).
func (c *GnarkClient) AuctionNotWinningBidProof(inputs map[string]interface{}) (*ProofResponse, error) {
	payload := map[string]interface{}{
		"StVaultId":               toString(inputs["st_auctionId"]),
		"StBlindedBidDifference":  toString(inputs["st_blindedBidDifference"]),
		"StBidBlockNumber":        toString(inputs["st_bidBlockNumber"]),
		"StWinningBidBlockNumber": toString(inputs["st_winningBidBlockNumber"]),
		"WtBidAmount":             toString(inputs["wt_bidAmount"]),
		"WtBidRandom":             toString(inputs["wt_bidRandom"]),
		"WtWinningBidAmount":      toString(inputs["wt_winningBidAmount"]),
		"WtWinningBidRandom":      toString(inputs["wt_winningBidRandom"]),
	}

	_, err := c.PostProof("/proof/auctionNotWinning", payload)
	if err != nil {
		return nil, err
	}
	return &ProofResponse{Status: 200, Message: "ok"}, nil
}

// --- internal helpers ---

// computeUniqueId computes a token uniqueId based on vaultId type.
func computeUniqueId(vaultId int, contractAddress *big.Int, idParams []*big.Int) (*big.Int, error) {
	switch vaultId {
	case 0:
		return Erc20UniqueId(contractAddress, idParams[0])
	case 1:
		return Erc721UniqueId(contractAddress, idParams[0])
	case 2:
		return Erc1155UniqueId(contractAddress, idParams[1], idParams[0])
	default:
		return nil, fmt.Errorf("unknown vaultId: %d", vaultId)
	}
}

// resolveAssetGroupProof returns the asset group path elements and indices.
// If the merkle root is zero, it returns zero-filled elements.
func resolveAssetGroupProof(
	assetGroupMerkleRoot *big.Int,
	assetGroupMerkleProof *MerkleProof,
	merkleDepth int,
) ([]*big.Int, *big.Int) {
	if assetGroupMerkleRoot.Sign() != 0 && assetGroupMerkleProof != nil {
		return assetGroupMerkleProof.Elements, assetGroupMerkleProof.Indices
	}
	zeros := make([]*big.Int, merkleDepth)
	for i := range zeros {
		zeros[i] = big.NewInt(0)
	}
	return zeros, big.NewInt(0)
}

// prepareErc1155ProofParams prepares common ERC1155 proof parameters.
// saltsIn must contain the salt used when each input note was originally created.
// Output salts are derived via ML-KEM encapsulation using recipientViewEncapKeys.
func prepareErc1155ProofParams(
	valuesIn []*big.Int,
	keysIn []KeyPair,
	saltsIn []*big.Int,
	valuesOut []*big.Int,
	keysOut []KeyPair,
	recipientViewEncapKeys [][]byte,
	merkleDepth int,
	merkleProofs []*MerkleProof,
	contractAddress *big.Int,
	tokenId *big.Int,
) (commitmentsOut []*big.Int, saltsOut []*big.Int, cipherText [][]byte, encTxData [][]byte, nullifiers []*big.Int, pathIndices []*big.Int, pathElements []*big.Int, err error) {
	commitmentsOut = make([]*big.Int, len(keysOut))
	saltsOut = make([]*big.Int, len(keysOut))
	cipherText = make([][]byte, len(keysOut))
	encTxData = make([][]byte, len(keysOut))
	nullifiers = make([]*big.Int, len(valuesIn))
	pathIndices = make([]*big.Int, len(valuesIn))
	pathElements = make([]*big.Int, 0)

	for i := 0; i < len(valuesIn); i++ {
		_, err := Erc1155Commitment(tokenId, valuesIn[i], keysIn[i].PublicKey, saltsIn[i])
		if err != nil {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to compute erc1155Commitment for input %d: %w", i, err)
		}

		if valuesIn[i].Sign() == 0 {
			pathIndices[i] = big.NewInt(0)
			zeros := make([]*big.Int, merkleDepth)
			for j := range zeros {
				zeros[j] = big.NewInt(0)
			}
			pathElements = append(pathElements, zeros...)
		} else {
			pathIndices[i] = merkleProofs[i].Indices
			pathElements = append(pathElements, merkleProofs[i].Elements...)
		}

		nullifier, err := GetNullifier(keysIn[i].PrivateKey, pathIndices[i])
		if err != nil {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to compute nullifier for input %d: %w", i, err)
		}
		nullifiers[i] = nullifier
	}

	for i := 0; i < len(keysOut); i++ {
		if recipientViewEncapKeys[i] == nil {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("recipientViewEncapKeys[%d] is required for non-interactive note delivery", i)
		}
		ss, ctI, encErr := Encapsulate(recipientViewEncapKeys[i])
		if encErr != nil {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to encapsulate for output %d: %w", i, encErr)
		}
		cipherText[i] = ctI
		saltB, saltErr := DerivePaymentSalt(ss)
		if saltErr != nil {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to derive payment salt for output %d: %w", i, saltErr)
		}
		encKey, keyErr := DerivePaymentKey(ss)
		if keyErr != nil {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to derive payment key for output %d: %w", i, keyErr)
		}
		saltBField := SaltBToField(saltB)
		ctII, encErr := EncryptPayload(encKey, tokenId, valuesOut[i])
		if encErr != nil {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to encrypt payload for output %d: %w", i, encErr)
		}
		encTxData[i] = ctII
		saltsOut[i] = saltBField
		cmt, err := Erc1155Commitment(tokenId, valuesOut[i], keysOut[i].PublicKey, saltBField)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to compute erc1155Commitment for output %d: %w", i, err)
		}
		commitmentsOut[i] = cmt
	}

	return commitmentsOut, saltsOut, cipherText, encTxData, nullifiers, pathIndices, pathElements, nil
}

// bigIntSliceToStrings converts []*big.Int to []string.
func bigIntSliceToStrings(vals []*big.Int) []string {
	result := make([]string, len(vals))
	for i, v := range vals {
		result[i] = v.String()
	}
	return result
}

// bigIntToString converts a *big.Int to string, handling nil.
func bigIntToString(v *big.Int) string {
	if v == nil {
		return "0"
	}
	return v.String()
}

// bigIntNestedToStringNested converts [][]*big.Int to [][]string.
func bigIntNestedToStringNested(vals [][]*big.Int) [][]string {
	result := make([][]string, len(vals))
	for i, inner := range vals {
		result[i] = bigIntSliceToStrings(inner)
	}
	return result
}

// chunkBigIntSlice splits a flat []*big.Int into chunks of chunkSize.
func chunkBigIntSlice(arr []*big.Int, chunkSize int) [][]*big.Int {
	if chunkSize <= 0 {
		return nil
	}
	var result [][]*big.Int
	for i := 0; i < len(arr); i += chunkSize {
		end := i + chunkSize
		if end > len(arr) {
			end = len(arr)
		}
		result = append(result, arr[i:end])
	}
	return result
}

// bigIntChunksToStringChunks converts [][]*big.Int to [][]string.
func bigIntChunksToStringChunks(chunks [][]*big.Int) [][]string {
	return bigIntNestedToStringNested(chunks)
}

// flattenBigIntSlice flattens [][]*big.Int to []*big.Int.
func flattenBigIntSlice(nested [][]*big.Int) []*big.Int {
	var result []*big.Int
	for _, inner := range nested {
		result = append(result, inner...)
	}
	return result
}

// extractPrivateKeys extracts private keys from a slice of KeyPairs.
func extractPrivateKeys(keys []KeyPair) []*big.Int {
	result := make([]*big.Int, len(keys))
	for i, k := range keys {
		result[i] = k.PrivateKey
	}
	return result
}

// extractPublicKeys extracts public keys from a slice of KeyPairs.
func extractPublicKeys(keys []KeyPair) []*big.Int {
	result := make([]*big.Int, len(keys))
	for i, k := range keys {
		result[i] = k.PublicKey
	}
	return result
}
