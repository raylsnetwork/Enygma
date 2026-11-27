package core

import (
	"fmt"
	"math/big"

	"enygma_dvp/layout"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
)

// PaymentResult holds the output of the Payment-family proof functions —
// everything Alice needs to submit the transaction on-chain and that each
// recipient needs to scan their note.
type PaymentResult struct {
	Proof []string // 8-element Groth16 proof
	// Statement is the public signals exactly as the gnark server returned
	// them: the proved witness's public part, in statement layout order.
	Statement       []*big.Int
	NumberOfInputs  int
	NumberOfOutputs int
	// Bob's note discovery data (output 0 only — published on-chain). The
	// proof binds their hash (PaymentCiphertextHash), so the receipt settles
	// only through EnygmaDvp.payment* with exactly these two.
	CipherText []byte // ML-KEM capsule for Bob (1088 bytes)
	EncTxData  []byte // AES-256-GCM ciphertext of (tokenId || amount) for Bob
	// Bob's salt — needed by the tag-based notification path (private tag layer).
	SaltB *big.Int // saltB field element used in Bob's output commitment
	// Alice's change note data (private — not published on-chain).
	// SaltA is the change salt, WithdrawChangeSalt(sk, nullifier0): Alice can
	// rebuild it, and so Commitment_A, from her spend key and the payment's
	// first nullifier (WithdrawChangeNote), without storing it.
	SaltA *big.Int

	circuit paymentLayout // the statement layout of the circuit that proved Statement
}

// paymentLayout is the statement layout of one Payment-family circuit.
type paymentLayout struct {
	length                                        int
	message, treeNumbers, merkleRoots, nullifiers field
	commitmentsOut, contractAddress, fee          field
}

var (
	paymentStatement = paymentLayout{
		length:          layout.PaymentLen,
		message:         field{layout.PaymentMessageOffset, layout.PaymentMessageSize},
		treeNumbers:     field{layout.PaymentTreeNumbersOffset, layout.PaymentTreeNumbersSize},
		merkleRoots:     field{layout.PaymentMerkleRootsOffset, layout.PaymentMerkleRootsSize},
		nullifiers:      field{layout.PaymentNullifiersOffset, layout.PaymentNullifiersSize},
		commitmentsOut:  field{layout.PaymentCommitmentsOutOffset, layout.PaymentCommitmentsOutSize},
		contractAddress: field{layout.PaymentContractAddressOffset, layout.PaymentContractAddressSize},
	}
	payment2inStatement = paymentLayout{
		length:          layout.Payment2inLen,
		message:         field{layout.Payment2inMessageOffset, layout.Payment2inMessageSize},
		treeNumbers:     field{layout.Payment2inTreeNumbersOffset, layout.Payment2inTreeNumbersSize},
		merkleRoots:     field{layout.Payment2inMerkleRootsOffset, layout.Payment2inMerkleRootsSize},
		nullifiers:      field{layout.Payment2inNullifiersOffset, layout.Payment2inNullifiersSize},
		commitmentsOut:  field{layout.Payment2inCommitmentsOutOffset, layout.Payment2inCommitmentsOutSize},
		contractAddress: field{layout.Payment2inContractAddressOffset, layout.Payment2inContractAddressSize},
	}
	paymentFeeStatement = paymentLayout{
		length:          layout.PaymentFeeLen,
		message:         field{layout.PaymentFeeMessageOffset, layout.PaymentFeeMessageSize},
		treeNumbers:     field{layout.PaymentFeeTreeNumbersOffset, layout.PaymentFeeTreeNumbersSize},
		merkleRoots:     field{layout.PaymentFeeMerkleRootsOffset, layout.PaymentFeeMerkleRootsSize},
		nullifiers:      field{layout.PaymentFeeNullifiersOffset, layout.PaymentFeeNullifiersSize},
		commitmentsOut:  field{layout.PaymentFeeCommitmentsOutOffset, layout.PaymentFeeCommitmentsOutSize},
		contractAddress: field{layout.PaymentFeeContractAddressOffset, layout.PaymentFeeContractAddressSize},
		fee:             field{layout.PaymentFeeFeeOffset, layout.PaymentFeeFeeSize},
	}
)

// snarkScalarField is the BN254 scalar field order, the modulus every public
// signal is reduced by.
var snarkScalarField, _ = new(big.Int).SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

var bytesArrays = func() abi.Arguments {
	t, err := abi.NewType("bytes[]", "", nil)
	if err != nil {
		panic(err)
	}
	return abi.Arguments{{Type: t}, {Type: t}}
}()

// PaymentCiphertextHash is the StCiphertextHash a Payment-family proof binds:
// keccak256(abi.encode(ciphertextI, ciphertextII)) mod the scalar field, the
// value CiphertextHash.sol recomputes from the ciphertexts a settlement
// publishes. ciphertextI[i]/ciphertextII[i] are one note's ML-KEM capsule and
// AEAD ciphertext: one note (output 0's) for EnygmaDvp.payment*, one per
// output for Erc20CoinVault.transferV2, none for transfer.
func PaymentCiphertextHash(ciphertextI, ciphertextII [][]byte) *big.Int {
	if ciphertextI == nil {
		ciphertextI = [][]byte{}
	}
	if ciphertextII == nil {
		ciphertextII = [][]byte{}
	}
	enc, err := bytesArrays.Pack(ciphertextI, ciphertextII)
	if err != nil {
		panic(fmt.Sprintf("abi-encode ciphertexts: %v", err)) // only on a type mismatch, which [][]byte cannot have
	}
	h := new(big.Int).SetBytes(crypto.Keccak256(enc))
	return h.Mod(h, snarkScalarField)
}

// paymentNoteCiphertextHash is PaymentCiphertextHash for the one note
// EnygmaDvp.payment* publishes.
func paymentNoteCiphertextHash(ctxt, encTxData []byte) *big.Int {
	return PaymentCiphertextHash([][]byte{ctxt}, [][]byte{encTxData})
}

// ContractStatement returns the statement for on-chain submission: Statement,
// as the server returned it.
func (r *PaymentResult) ContractStatement() []*big.Int { return r.Statement }

// Message is the statement's message: 0 for a payment, 1 for a withdrawal.
func (r *PaymentResult) Message() *big.Int { return r.circuit.message.one(r.Statement) }

// TreeNumbers are the trees the input notes live in, one per input.
func (r *PaymentResult) TreeNumbers() []*big.Int { return r.circuit.treeNumbers.in(r.Statement) }

// MerkleRoots are the roots the input notes are proved against, one per input.
func (r *PaymentResult) MerkleRoots() []*big.Int { return r.circuit.merkleRoots.in(r.Statement) }

// Nullifiers are the input notes' nullifiers, one per input.
func (r *PaymentResult) Nullifiers() []*big.Int { return r.circuit.nullifiers.in(r.Statement) }

// CommitmentsOut are the output notes' commitments, one per output.
func (r *PaymentResult) CommitmentsOut() []*big.Int { return r.circuit.commitmentsOut.in(r.Statement) }

// ContractAddress is the vault the proof is bound to (VULN-5 fix).
func (r *PaymentResult) ContractAddress() *big.Int { return r.circuit.contractAddress.one(r.Statement) }

// Fee is the proof's public fee, or nil for a circuit without one (Payment).
func (r *PaymentResult) Fee() *big.Int { return r.circuit.fee.one(r.Statement) }

// BoundPaymentProof generates a Payment circuit proof for Alice paying some amount
// to Bob, with optional change back to herself, binding the proof to a specific
// vault contract address (StContractAddress, constrained in-circuit) and, via
// the tree-bound nullifier, the tree the input note lives in.
//
// contractAddress must be the uint160 representation of the Erc20CoinVault address
// (i.e., new(big.Int).SetBytes(vaultAddr.Bytes())).
//
// Circuit config: 1 input / 2 outputs (Payment), or with two input notes
// 2 inputs / 2 outputs (Payment2in, merging two notes), both at the Merkle
// depth the caller passes as merkleDepth, which must be the circuits'
// (layout.MerkleTreeDepth).
//   - Inputs: Alice's real notes.
//   - Output 0: payment to Bob; Output 1: change back to Alice.
//
// Salt derivation follows the protocol:
//   - Output 0 (destination): salt derived via HKDF from the ML-KEM shared secret
//     so Bob can recover it by decapsulating the ML-KEM ciphertext.
//   - Output 1 (change): salt is WithdrawChangeSalt(sk, nullifier0), so Alice
//     can rebuild her change note from her key and the spent nullifier, even
//     if the payment's ciphertexts are replaced and her own transaction
//     reverts (see paymentChangeSalt).
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
	res, err := c.provePayment(contractAddress, stMessage, wtValuesIn, keysIn, wtSaltsIn, wtValuesOut, recipientSpendPks,
		SaltBToField(saltB0), paymentNoteCiphertextHash(ctxt0, ctxtII0), merkleDepth, merkleProofs, stTreeNumbers, wtTokenId)
	if err != nil {
		return nil, err
	}
	res.CipherText, res.EncTxData = ctxt0, ctxtII0
	return res, nil
}

// ChannelPaymentNote is what a channel payment (#451) publishes for its payee
// besides the proof, and the salt the payer chose for the payee's note: the
// tag channel layer (private_tags' PrepareChannelPayment) makes it.
type ChannelPaymentNote struct {
	Salt        *big.Int // the payee's note salt, random in the field; private
	BlockNumber uint64   // block_n, the block the tag is for
	Tag         [32]byte // the payee's tag for block_n
	Ctxt        []byte   // the note's opening, sealed under the channel's block key
}

// channelPaymentDomain opens the encoding ChannelPaymentCiphertextHash hashes,
// as CiphertextHash.sol's CHANNEL_PAYMENT_DOMAIN does.
var channelPaymentDomain = crypto.Keccak256Hash([]byte("enygma-channel-payment-v1"))

var channelPaymentArgs = func() abi.Arguments {
	b32, _ := abi.NewType("bytes32", "", nil)
	u256, _ := abi.NewType("uint256", "", nil)
	bs, _ := abi.NewType("bytes", "", nil)
	return abi.Arguments{{Type: b32}, {Type: u256}, {Type: b32}, {Type: bs}}
}()

// ChannelPaymentCiphertextHash is the StCiphertextHash a channel payment's
// proof binds, the value CiphertextHash.ofChannelPayment recomputes from what
// EnygmaDvp.channelPayment is given:
//
//	keccak256(abi.encode(keccak256("enygma-channel-payment-v1"), blockNumber, tag, ctxt)) mod p
//
// The leading domain word keeps it apart from PaymentCiphertextHash's
// encoding, whose first word is an array offset: a channel receipt cannot
// settle through payment*, nor a payment's through channelPayment.
func ChannelPaymentCiphertextHash(blockNumber uint64, tag [32]byte, ctxt []byte) *big.Int {
	enc, err := channelPaymentArgs.Pack(channelPaymentDomain, new(big.Int).SetUint64(blockNumber), tag, ctxt)
	if err != nil {
		panic(fmt.Sprintf("abi-encode channel payment: %v", err)) // only on a type mismatch, which the fixed types cannot have
	}
	h := new(big.Int).SetBytes(crypto.Keccak256(enc))
	return h.Mod(h, snarkScalarField)
}

// BoundChannelPaymentProof proves a channel payment (#451): a Payment, or a
// Payment2in with two input notes, as BoundPaymentProof does, except that
// output 0's salt is the payer's note.Salt instead of one derived from a
// per-payment ML-KEM capsule, and the proof binds the note's block, tag and
// ciphertext (ChannelPaymentCiphertextHash). It settles only through
// EnygmaDvp.channelPayment with exactly those three. The result carries no
// CipherText or EncTxData: there is no capsule.
func (c *GnarkClient) BoundChannelPaymentProof(
	contractAddress *big.Int,
	wtValuesIn []*big.Int,
	keysIn []KeyPair,
	wtSaltsIn []*big.Int,
	wtValuesOut []*big.Int,
	recipientSpendPks []*big.Int,
	note ChannelPaymentNote,
	merkleDepth int,
	merkleProofs []*MerkleProof,
	stTreeNumbers []*big.Int,
	wtTokenId *big.Int,
) (*PaymentResult, error) {
	return c.provePayment(contractAddress, big.NewInt(0), wtValuesIn, keysIn, wtSaltsIn, wtValuesOut, recipientSpendPks,
		note.Salt, ChannelPaymentCiphertextHash(note.BlockNumber, note.Tag, note.Ctxt), merkleDepth, merkleProofs, stTreeNumbers, wtTokenId)
}

// provePayment proves a Payment (one input note) or Payment2in (two) whose
// output 0, the payee's, has salt salt0 and whose proof binds ciphertextHash
// as StCiphertextHash. Output 1, the change, gets paymentChangeSalt.
func (c *GnarkClient) provePayment(
	contractAddress *big.Int,
	stMessage *big.Int,
	wtValuesIn []*big.Int,
	keysIn []KeyPair,
	wtSaltsIn []*big.Int,
	wtValuesOut []*big.Int,
	recipientSpendPks []*big.Int,
	salt0 *big.Int,
	ciphertextHash *big.Int,
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
	var saltA *big.Int

	wtSaltsOut[0] = salt0
	cmt0, err := Erc20CommitmentV2(recipientSpendPks[0], wtSaltsOut[0], wtValuesOut[0], wtTokenId)
	if err != nil {
		return nil, fmt.Errorf("Erc20CommitmentV2 Bob output: %w", err)
	}
	stCommitmentsOut[0] = cmt0

	for i := 1; i < nOut; i++ {
		salt, err := paymentChangeSalt(i, keysIn[0].PrivateKey, stNullifiers[0])
		if err != nil {
			return nil, fmt.Errorf("change output %d salt: %w", i, err)
		}
		if i == 1 {
			saltA = salt
		}
		wtSaltsOut[i] = salt
		cmt, err := Erc20CommitmentV2(recipientSpendPks[i], salt, wtValuesOut[i], wtTokenId)
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
		"stCiphertextHash":     ciphertextHash.String(),
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

	endpoint, circuit := "/proof/payment", paymentStatement
	if nIn == 2 {
		endpoint, circuit = "/proof/payment2in", payment2inStatement
	}
	body, err := c.PostProof(endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("bound payment proof request failed: %w", err)
	}

	proofStrs, statement, err := proofResponse(body, circuit.length)
	if err != nil {
		return nil, fmt.Errorf("parse bound payment proof response: %w", err)
	}

	return &PaymentResult{
		Proof:           proofStrs,
		Statement:       statement,
		NumberOfInputs:  nIn,
		NumberOfOutputs: nOut,
		SaltB:           wtSaltsOut[0],
		SaltA:           saltA,
		circuit:         circuit,
	}, nil
}

// paymentChangeSalt is the salt of change output i of a payment whose first
// input is spent by sk with nullifier0. Output 1 gets WithdrawChangeSalt(sk,
// nullifier0), the salt enygma_retail_payments' Payment-family circuits
// require of every proof (payments as well as withdrawals), so the change
// note never depends on locally stored data. Any further output, and a proof
// whose first input is zero padding (nullifier0 = 0, which those circuits
// reject), gets a random salt as before: no circuit constrains it.
func paymentChangeSalt(i int, sk, nullifier0 *big.Int) (*big.Int, error) {
	if i == 1 && nullifier0.Sign() != 0 {
		return WithdrawChangeSalt(sk, nullifier0)
	}
	return RandomInField()
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
		salt, err := paymentChangeSalt(i, keysIn[0].PrivateKey, stNullifiers[0])
		if err != nil {
			return nil, fmt.Errorf("change output %d salt: %w", i, err)
		}
		if i == 1 {
			saltA = salt
		}
		wtSaltsOut[i] = salt
		cmt, err := Erc20CommitmentV2(recipientSpendPks[i], salt, wtValuesOut[i], wtTokenId)
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
		"stCiphertextHash":     paymentNoteCiphertextHash(cipherText, encTxData).String(),
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

	proofStrs, statement, err := proofResponse(body, paymentFeeStatement.length)
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
		circuit:         paymentFeeStatement,
	}, nil
}
