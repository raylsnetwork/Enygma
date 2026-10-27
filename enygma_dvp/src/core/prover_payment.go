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
	Proof []string // 8-element Groth16 proof
	// Statement is the public signals exactly as the gnark server returned
	// them: the proved witness's public part, in the circuit's own order.
	Statement       []*big.Int
	NumberOfInputs  int
	NumberOfOutputs int
	// Bob's note discovery data (output 0 only — published on-chain).
	CipherText []byte // ML-KEM capsule for Bob (1088 bytes)
	EncTxData  []byte // AES-256-GCM ciphertext of (tokenId || amount) for Bob
	// Bob's salt — needed by the tag-based notification path (private tag layer).
	SaltB *big.Int // saltB field element used in Bob's output commitment
	// Alice's change note data (private — not published on-chain).
	SaltA *big.Int // random change salt; Alice must store this locally to open Commitment_A
	// ContractAddress is non-nil when the proof was generated via BoundPaymentProof (VULN-5 fix):
	// the vault the proof is bound to.
	ContractAddress *big.Int
	// Fee is non-nil when generated via BoundPaymentFeeProof: the proof's public fee.
	Fee *big.Int
}

// ContractStatement returns the statement for on-chain submission. The
// server already returns it in the order the vault contracts read, so it is
// Statement unchanged.
func (r *PaymentResult) ContractStatement() []*big.Int {
	return r.Statement
}

// paymentResponse decodes a Payment-family prove handler's response: the
// proof as decimal strings and the public signals it was proved against.
func paymentResponse(body []byte) (proof []string, statement []*big.Int, err error) {
	var resp struct {
		Proof        []json.Number `json:"proof"`
		PublicSignal []json.Number `json:"publicSignal"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, err
	}
	proof = make([]string, len(resp.Proof))
	for i, n := range resp.Proof {
		proof[i] = n.String()
	}
	statement = make([]*big.Int, len(resp.PublicSignal))
	for i, n := range resp.PublicSignal {
		v, ok := new(big.Int).SetString(n.String(), 10)
		if !ok {
			return nil, nil, fmt.Errorf("publicSignal[%d]: %q is not a decimal integer", i, n)
		}
		statement[i] = v
	}
	return proof, statement, nil
}

// BoundPaymentProof generates a Payment circuit proof for Alice paying some amount
// to Bob, with optional change back to herself, binding the proof to a specific
// vault contract address (StContractAddress, constrained in-circuit) and, via
// the tree-bound nullifier, the tree the input note lives in.
//
// contractAddress must be the uint160 representation of the Erc20CoinVault address
// (i.e., new(big.Int).SetBytes(vaultAddr.Bytes())).
//
// Circuit config: 1 input / 2 outputs / Merkle depth 8 (Payment), or with two
// input notes 2 inputs / 2 outputs (Payment2in, merging two notes).
//   - Inputs: Alice's real notes.
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
			// nullifier: Poseidon(sk, treeNumber*2^treeDepth+leafIndex), the
			// formula every circuit spending from this vault uses (the DvP
			// circuits too), so a note has one nullifier however it is spent.
			nf, err := GetNullifierWithTree(keysIn[i].PrivateKey, stTreeNumbers[i], wtPathIndices[i], merkleDepth)
			if err != nil {
				return nil, fmt.Errorf("GetNullifierWithTree input %d: %w", i, err)
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

	endpoint := "/proof/payment"
	if nIn == 2 {
		endpoint = "/proof/payment2in"
	}
	body, err := c.PostProof(endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("bound payment proof request failed: %w", err)
	}

	proofStrs, statement, err := paymentResponse(body)
	if err != nil {
		return nil, fmt.Errorf("parse bound payment proof response: %w", err)
	}

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
			// The same tree-bound nullifier as every other circuit spending from
			// this vault (see BoundPaymentProof).
			nf, err := GetNullifierWithTree(keysIn[i].PrivateKey, stTreeNumbers[i], wtPathIndices[i], merkleDepth)
			if err != nil {
				return nil, fmt.Errorf("GetNullifierWithTree input %d: %w", i, err)
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

	proofStrs, statement, err := paymentResponse(body)
	if err != nil {
		return nil, fmt.Errorf("parse payment fee proof response: %w", err)
	}

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
