package core

import (
	"encoding/json"
	"fmt"
	"math/big"
)

// PaymentResult holds the output of the Payment-family proof functions —
// everything Alice needs to submit the transaction on-chain and that each
// recipient needs to scan their note.
type PaymentResult struct {
	Proof           []string   // 8-element Groth16 proof
	Statement       []*big.Int // interleaved: [msg, tree0, root0, null0, ..., cmt0, cmt1, contractAddr?]
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
		out[1+i] = r.Statement[base]
		out[1+nIn+i] = r.Statement[base+1]
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

// BoundPaymentProof generates a Payment circuit proof for Alice paying some amount
// to Bob, with optional change back to herself, binding the proof to a specific
// vault contract address (and, via NullifierBoundTree, the tree the input note
// lives in — see Nullifier.go's StContractAddress/StTreeNumbers fix).
//
// contractAddress must be the uint160 representation of the Erc20CoinVault address
// (i.e., new(big.Int).SetBytes(vaultAddr.Bytes())).
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
//
// The caller submits the resulting ctxts/encTxDatas alongside the proof on-chain.
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
			// nullifier: Poseidon(sk, treeNumber*2^treeDepth+leafIndex, contractAddress)
			// — GetNullifierBoundTree, not the plain GetNullifier, so the proof
			// is bound to this specific vault contract AND tree (matches the
			// circuit-side NullifierBoundTree fix).
			nf, err := GetNullifierBoundTree(keysIn[i].PrivateKey, stTreeNumbers[i], wtPathIndices[i], merkleDepth, contractAddress)
			if err != nil {
				return nil, fmt.Errorf("GetNullifierBoundTree input %d: %w", i, err)
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
			// GetNullifierBoundTree (not the plain GetNullifier), matching the
			// circuit-side NullifierBoundTree fix — binds the proof to this
			// vault AND tree.
			nf, err := GetNullifierBoundTree(keysIn[i].PrivateKey, stTreeNumbers[i], wtPathIndices[i], merkleDepth, contractAddress)
			if err != nil {
				return nil, fmt.Errorf("GetNullifierBoundTree input %d: %w", i, err)
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
