package engine

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"math/big"
	"math/bits"

	"enygma_retail_payments/layout"
	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	tags "github.com/raylsnetwork/enygma_retail_payments/private_tags/src"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iden3/go-iden3-crypto/constants"
	"github.com/iden3/go-iden3-crypto/poseidon"
)

var nullifierTopic = crypto.Keccak256Hash([]byte("Nullifier(uint256,uint256,uint256)"))

// formulaNamed is a full run's formula detail: name only, with the vault
// and tree whose chain holds its value.
func formulaNamed(name FormulaName, vault string, tree int) HopDetail {
	return HopDetail{Kind: DetailFormula, Vault: vault, Tree: tree, Formula: &Formula{Name: name}}
}

// spendKeyFormula is the spend key formula of owner, whose spend public key
// the UserRegistry holds as registered: sk_spend hashed, checked against
// what the registry's getKeys returns for owner.
func (c *chain) spendKeyFormula(owner *Actor) (HopDetail, error) {
	pk, err := poseidon.Hash([]*big.Int{owner.Spend.PrivateKey})
	if err != nil {
		return HopDetail{}, fmt.Errorf("%s: Poseidon(sk_spend): %w", owner.Name, err)
	}
	keys, err := rpcore.LookupKeys(c.client, c.registryAddr, owner.Address)
	if err != nil {
		return HopDetail{}, fmt.Errorf("%s: %w", owner.Name, err)
	}
	result, chain := fieldHex(pk), fieldHex(keys.SpendKey)
	return HopDetail{Kind: DetailFormula, Vault: NodeUserRegistry, Formula: &Formula{
		Name:   FormulaSpendKey,
		Inputs: []FormulaValue{{Name: "sk_spend", Value: fieldHex(owner.Spend.PrivateKey)}},
		Result: result,
		Chain:  &FormulaValue{Name: fmt.Sprintf("the pk_spend the UserRegistry holds for the %s (getKeys)", owner.Name), Value: chain},
		OK:     result == chain,
	}}, nil
}

// commitmentFormula is the commitment formula of note n, owner's, which
// deposit cd inserted as its one leaf: its opening hashed, checked against
// that leaf as the vault's Commitment event logged it.
func commitmentFormula(cd HopDetail, owner *Actor, n *note) HopDetail {
	leaf := cd.Leaves[0]
	result := fieldHex(n.commitment)
	return HopDetail{Kind: DetailFormula, Vault: cd.Vault, Tree: cd.Tree, Formula: &Formula{
		Name: FormulaCommitment,
		Inputs: []FormulaValue{
			{Name: noteOpening[0], Value: fieldHex(owner.Spend.PublicKey)},
			{Name: noteOpening[1], Value: fieldHex(n.salt)},
			{Name: noteOpening[2], Value: n.amount.String()},
			{Name: noteOpening[3], Value: fieldHex(tokenId)},
		},
		Result: result,
		Chain:  &FormulaValue{Name: fmt.Sprintf("leaf #%d of tree %d, the vault's Commitment event", leaf.Index, cd.Tree), Value: leaf.Commitment},
		OK:     result == leaf.Commitment,
	}}
}

// merklePathFormula is the Merkle path formula of p, owner's note in the
// vault with node ID vaultNode, bound as vault, whose tree held leaves
// leaves when p's root was taken: the leaf hashed up with each sibling, every
// level's node named, checked against vault's isValidRoot.
func merklePathFormula(ctx context.Context, vault *bind.BoundContract, vaultNode, owner string, p *rpcore.MerkleProof, leaves int) (HopDetail, error) {
	index := int(p.Indices.Int64())
	node := p.Element
	inputs := []FormulaValue{{Name: fmt.Sprintf("leaf #%d · node₀", index), Value: fieldHex(node)}}
	for level, sib := range p.Elements {
		// Bit level of the leaf index: 0, the node is a left child.
		right := index>>level&1 == 1
		side := "right"
		if right {
			side = "left"
		}
		inputs = append(inputs, FormulaValue{Name: fmt.Sprintf("sibling%s · %s", subscript(level), side), Value: fieldHex(sib)})
		if right {
			node = dvpcore.HashLeftRight(sib, node)
		} else {
			node = dvpcore.HashLeftRight(node, sib)
		}
		if level < len(p.Elements)-1 {
			inputs = append(inputs, FormulaValue{Name: "node" + subscript(level+1), Value: fieldHex(node)})
		}
	}
	known, err := hadRoot(ctx, vault, vaultNode, p.TreeNumber, node)
	if err != nil {
		return HopDetail{}, err
	}
	result := fieldHex(node)
	return HopDetail{Kind: DetailFormula, Vault: vaultNode, Tree: p.TreeNumber, Formula: &Formula{
		Name:   FormulaMerklePath,
		Inputs: inputs,
		Result: result,
		Chain: &FormulaValue{Name: fmt.Sprintf("the root %s's proof publishes, one the vault has had in tree %d (isValidRoot → %v)", owner, p.TreeNumber, known),
			Value: fieldHex(p.Root)},
		OK:     known && result == fieldHex(p.Root),
		Leaves: leaves,
	}}, nil
}

// nullifierFormula is the nullifier formula of the i-th note spent along p
// with spend key sk, in the vault with node ID vaultNode at vaultAddr,
// checked against the i-th Nullifier event that vault logged in receipt.
func nullifierFormula(sk *big.Int, vaultNode string, vaultAddr common.Address, p *rpcore.MerkleProof, i int, receipt *ethtypes.Receipt) (HopDetail, error) {
	tree := big.NewInt(int64(p.TreeNumber))
	position := new(big.Int).Add(new(big.Int).Lsh(tree, merkleDepth), p.Indices)
	nullifier, err := dvpcore.GetNullifierWithTree(sk, tree, p.Indices, merkleDepth)
	if err != nil {
		return HopDetail{}, err
	}
	var logged []*big.Int
	for _, l := range receipt.Logs {
		if l.Address == vaultAddr && len(l.Topics) == 4 && l.Topics[0] == nullifierTopic {
			logged = append(logged, l.Topics[3].Big())
		}
	}
	if i >= len(logged) {
		return HopDetail{}, fmt.Errorf("tx %s logged %d Nullifier events in the %s, want one per spent note", receipt.TxHash.Hex(), len(logged), vaultNode)
	}
	result, chain := fieldHex(nullifier), fieldHex(logged[i])
	return HopDetail{Kind: DetailFormula, Vault: vaultNode, Tree: p.TreeNumber, Formula: &Formula{
		Name: FormulaNullifier,
		Inputs: []FormulaValue{
			{Name: "sk_spend", Value: fieldHex(sk)},
			{Name: "tree", Value: tree.String()},
			{Name: "leaf index", Value: p.Indices.String()},
			{Name: fmt.Sprintf("tree · 2%s + leaf index", superscript(merkleDepth)), Value: position.String()},
		},
		Result: result,
		Chain:  &FormulaValue{Name: fmt.Sprintf("the vault's Nullifier event in the settlement, tx %s", receipt.TxHash.Hex()), Value: chain},
		OK:     result == chain,
	}}, nil
}

// valueRange is the bound of the circuits' range check (valuebound.Bound): every
// amount a proof spends or makes is 0 ≤ v < 10³⁶.
var valueRange = new(big.Int).Exp(big.NewInt(10), big.NewInt(36), nil)

// balance is in − (out₀ + out₁), 0 when a 1-in, 2-out proof conserves
// value, and whether each amount passes the range check.
func balance(in, out0, out1 *big.Int) (*big.Int, bool) {
	inRange := true
	for _, x := range []*big.Int{in, out0, out1} {
		inRange = inRange && x.Sign() >= 0 && x.Cmp(valueRange) < 0
	}
	return new(big.Int).Sub(in, new(big.Int).Add(out0, out1)), inRange
}

// conservationFormula is the value conservation formula of a 1-in Payment
// proof res, from's note ins[0] (in tree tree) spent into amount for to
// and change back:
// Σin − Σout, each amount range-checked, and checked against the Verifier's
// verifyProof for res's proof and public inputs, an eth_call.
func (c *chain) conservationFormula(ctx context.Context, cfg Config, from, to *Actor, ins []*note, tree int, amount, change *big.Int, res *rpcore.PaymentResult) (HopDetail, error) {
	if len(ins) != 1 {
		return HopDetail{}, fmt.Errorf("value conservation card: %d spent notes, want a 1-in Payment", len(ins))
	}
	_, verified, err := c.verifyProof(ctx, cfg, res)
	if err != nil {
		return HopDetail{}, fmt.Errorf("value conservation card: %w", err)
	}

	diff, inRange := balance(ins[0].amount, amount, change)
	return HopDetail{Kind: DetailFormula, Vault: NodeVault, Tree: tree, Formula: &Formula{
		Name: FormulaConservation,
		Inputs: []FormulaValue{
			{Name: fmt.Sprintf("in₀ · the %s's spent note", from.Name), Value: ins[0].amount.String()},
			{Name: fmt.Sprintf("out₀ · %s's note", to.Name), Value: amount.String()},
			{Name: fmt.Sprintf("out₁ · the %s's change", from.Name), Value: change.String()},
		},
		Result: diff.String(),
		Chain:  &FormulaValue{Name: verifyProofName, Value: fmt.Sprint(verified)},
		OK:     diff.Sign() == 0 && inRange && verified,
	}}, nil
}

// feeConservationFormula is the value conservation formula of payer's
// UsdrFee proof fee: their USDr note spent into the relayer's fee note
// (out₀) and their USDr change (out₁), Σin − Σout, each amount
// range-checked, and out₀ pinned to the proof's public Fee. It is checked
// against the fee EnygmaDvp's settlement requires that Fee to equal, its
// usdrFixedFeeAmount, read now.
func (c *chain) feeConservationFormula(payer *Actor, fee *usdrFeeLeg) (HopDetail, error) {
	fixed, err := c.usdrFixedFee()
	if err != nil {
		return HopDetail{}, fmt.Errorf("fee conservation card: %w", err)
	}
	if len(fee.statement) != layout.UsdrFeeLen {
		return HopDetail{}, fmt.Errorf("fee conservation card: %d public inputs, want %d", len(fee.statement), layout.UsdrFeeLen)
	}
	in, out0, out1, feeInput := fee.in.amount, fee.feeAmount, fee.change.amount, fee.statement[layout.UsdrFeeFeeOffset]
	diff, inRange := balance(in, out0, out1)
	return HopDetail{Kind: DetailFormula, Vault: NodeUsdrVault, Tree: fee.tree, Formula: &Formula{
		Name: FormulaFeeConservation,
		Inputs: []FormulaValue{
			{Name: fmt.Sprintf("in₀ · the %s's USDr note", payer.Name), Value: in.String()},
			{Name: "out₀ · the relayer's fee note", Value: out0.String()},
			{Name: fmt.Sprintf("out₁ · the %s's USDr change", payer.Name), Value: out1.String()},
			{Name: "Fee · the proof's public input", Value: feeInput.String()},
		},
		Result: diff.String(),
		Chain:  &FormulaValue{Name: "EnygmaDvp's usdrFixedFeeAmount, the fee its settlement requires the public Fee to equal", Value: fixed.String()},
		OK:     diff.Sign() == 0 && inRange && out0.Cmp(feeInput) == 0 && feeInput.Cmp(fixed) == 0,
	}}, nil
}

// verifyProofName names the chain's value of a card checked by the
// Verifier's verifyProof, an eth_call.
const verifyProofName = "the Verifier's verifyProof, for this proof with these public inputs (eth_call)"

// verifyProof is the Verifier's verifyProof call for a 1-in Payment proof
// res and its public inputs, and the answer the chain gives it (an
// eth_call).
func (c *chain) verifyProof(ctx context.Context, cfg Config, res *rpcore.PaymentResult) (VerifyCall, bool, error) {
	v, err := DeployedVerifier(cfg)
	if err != nil {
		return VerifyCall{}, false, err
	}
	verifierABI, err := loadABI(cfg.RetailRoot, "Verifier")
	if err != nil {
		return VerifyCall{}, false, err
	}
	call, err := verifyCall(verifierABI, res.Proof, res.ContractStatement())
	if err != nil {
		return VerifyCall{}, false, err
	}
	addr := common.HexToAddress(v.Address)
	data, err := hexutil.Decode(call.Data)
	if err != nil {
		return VerifyCall{}, false, fmt.Errorf("verifyProof call data: %w", err)
	}
	out, err := c.client.CallContract(ctx, ethereum.CallMsg{To: &addr, Data: data}, nil)
	if err != nil {
		return VerifyCall{}, false, fmt.Errorf("verifyProof eth_call: %w", err)
	}
	answer, err := verifierABI.Unpack("verifyProof", out)
	if err != nil {
		return VerifyCall{}, false, fmt.Errorf("verifyProof answer %x: %w", out, err)
	}
	if len(answer) != 1 {
		return VerifyCall{}, false, fmt.Errorf("verifyProof answer %x: %d values, want 1", out, len(answer))
	}
	verified, ok := answer[0].(bool)
	if !ok {
		return VerifyCall{}, false, fmt.Errorf("verifyProof answered %v, not a bool", answer[0])
	}
	return call, verified, nil
}

// proofPoints names the 8 elements of a Groth16 proof as the Verifier takes
// them: A, then B (each coordinate in F_p², two elements), then C.
var proofPoints = [8]string{"A.x", "A.y", "B.x₀", "B.x₁", "B.y₀", "B.y₁", "C.x", "C.y"}

// groth16Formula is the Groth16 check of a 1-in Payment proof res as the
// vault verified it: its points A, B and C and its public inputs, checked
// against the Verifier's verifyProof answer for them (an eth_call). The
// pairing e(A,B) = e(α,β)·e(L,γ)·e(C,δ) is the Verifier's: L is not computed
// here.
func (c *chain) groth16Formula(ctx context.Context, cfg Config, tree int, res *rpcore.PaymentResult) (HopDetail, error) {
	call, verified, err := c.verifyProof(ctx, cfg, res)
	if err != nil {
		return HopDetail{}, fmt.Errorf("Groth16 card: %w", err)
	}
	var inputs []FormulaValue
	for i, v := range call.Proof {
		inputs = append(inputs, FormulaValue{Name: proofPoints[i], Value: v})
	}
	for i, x := range call.Inputs {
		inputs = append(inputs, FormulaValue{Name: fmt.Sprintf("x%s · %s[%d]", subscript(i+1), x.Name, x.Index), Value: x.Value})
	}
	return HopDetail{Kind: DetailFormula, Vault: NodeVault, Tree: tree, Formula: &Formula{
		Name:   FormulaGroth16,
		Inputs: inputs,
		Result: fmt.Sprint(verified),
		Chain:  &FormulaValue{Name: verifyProofName, Value: fmt.Sprint(verified)},
		OK:     verified,
	}}, nil
}

// noteEncryptionFormula is the note encryption formula of the note with
// commitment cm, owner's, as the vault's Payment (or EncryptedNote) event
// published it: its capsule decapsulated with owner's view key, the salt
// and AES key derived from the shared secret, the payload opened, and the
// opening hashed with owner's pk_spend, checked against the commitment the
// event published with the ciphertext.
func (c *chain) noteEncryptionFormula(ctx context.Context, owner *Actor, cm *big.Int) (HopDetail, error) {
	logs, err := c.client.FilterLogs(ctx, ethereum.FilterQuery{
		Addresses: []common.Address{c.dvpAddr, c.vaultAddr},
		Topics:    [][]common.Hash{{paymentTopic, encryptedNoteTopic}, {common.BigToHash(erc20VaultId)}, {common.BigToHash(cm)}},
	})
	if err != nil {
		return HopDetail{}, fmt.Errorf("note encryption card: note events: %w", err)
	}
	if len(logs) != 1 {
		return HopDetail{}, fmt.Errorf("note encryption card: %d note events for %s's note 0x%s, want 1", len(logs), owner.Name, cm.Text(16))
	}
	l := logs[0]
	vals, err := abiBytesPair().Unpack(l.Data)
	if err != nil {
		return HopDetail{}, fmt.Errorf("note encryption card: decode the note event in tx %s: %w", l.TxHash.Hex(), err)
	}
	if len(vals) != 2 {
		return HopDetail{}, fmt.Errorf("note encryption card: the note event in tx %s carries %d values, want the capsule and the sealed payload", l.TxHash.Hex(), len(vals))
	}
	capsule, sealed := vals[0].([]byte), vals[1].([]byte)
	ss, err := dvpcore.Decapsulate(owner.View.DecapsKey, capsule)
	if err != nil {
		return HopDetail{}, fmt.Errorf("note encryption card: %s: %w", owner.Name, err)
	}
	saltB, err := dvpcore.DerivePaymentSalt(ss)
	if err != nil {
		return HopDetail{}, err
	}
	key, err := dvpcore.DerivePaymentKey(ss)
	if err != nil {
		return HopDetail{}, err
	}
	token, amount, err := dvpcore.DecryptPayload(key, sealed)
	if err != nil {
		return HopDetail{}, fmt.Errorf("note encryption card: %s's view key does not open the note in tx %s: %w", owner.Name, l.TxHash.Hex(), err)
	}
	salt := dvpcore.SaltBToField(saltB)
	opened, err := rpcore.Erc20CommitmentV2(owner.Spend.PublicKey, salt, amount, token)
	if err != nil {
		return HopDetail{}, err
	}
	event := "Payment"
	if l.Topics[0] == encryptedNoteTopic {
		event = "EncryptedNote"
	}
	result, chain := fieldHex(opened), fieldHex(l.Topics[2].Big())
	return HopDetail{Kind: DetailFormula, Vault: NodeVault, Formula: &Formula{
		Name: FormulaNoteEncryption,
		Inputs: []FormulaValue{
			{Name: "capsule · ML-KEM-768", Value: bytesHex(capsule)},
			{Name: "sealed · AES-GCM", Value: bytesHex(sealed)},
			{Name: "amount · opened", Value: amount.String()},
			{Name: "salt · opened", Value: fieldHex(salt)},
			{Name: "token · opened", Value: fieldHex(token)},
		},
		Result: result,
		Chain:  &FormulaValue{Name: fmt.Sprintf("the commitment the vault's %s event published with this ciphertext, tx %s", event, l.TxHash.Hex()), Value: chain},
		OK:     result == chain,
	}}, nil
}

// channelRecord is a tag channel's record as TagChannelRegistry holds it.
type channelRecord struct {
	sender         common.Address
	c1, c2, bitmap []byte
}

// channelRecordArgs are getChannel's outputs: sender, c1, c2, bitmap.
func channelRecordArgs() abi.Arguments {
	addressT, _ := abi.NewType("address", "", nil)
	bytesT, _ := abi.NewType("bytes", "", nil)
	return abi.Arguments{{Type: addressT}, {Type: bytesT}, {Type: bytesT}, {Type: bytesT}}
}

// channelRecord reads tag channel #idx from TagChannelRegistry's getChannel
// (an eth_call).
func (c *chain) channelRecord(ctx context.Context, idx uint64) (channelRecord, error) {
	uint256T, _ := abi.NewType("uint256", "", nil)
	arg, err := abi.Arguments{{Type: uint256T}}.Pack(new(big.Int).SetUint64(idx))
	if err != nil {
		return channelRecord{}, err
	}
	data := append(crypto.Keccak256([]byte("getChannel(uint256)"))[:4], arg...)
	out, err := c.client.CallContract(ctx, ethereum.CallMsg{To: &c.channelRegistryAddr, Data: data}, nil)
	if err != nil {
		return channelRecord{}, fmt.Errorf("TagChannelRegistry.getChannel(%d): %w", idx, err)
	}
	vals, err := channelRecordArgs().Unpack(out)
	if err != nil || len(vals) != 4 {
		return channelRecord{}, fmt.Errorf("TagChannelRegistry.getChannel(%d) answered %x: %v", idx, out, err)
	}
	return channelRecord{sender: vals[0].(common.Address), c1: vals[1].([]byte), c2: vals[2].([]byte), bitmap: vals[3].([]byte)}, nil
}

// openChannelPayload is c₂ of a tag channel record opened under k with c₁
// as associated data, as the private tags library seals it (AES-256-GCM,
// nonce ‖ ciphertext), and decoded to its msg and senderId (each a 4-byte
// big-endian length, then its bytes). An error is the AEAD failing: the
// record is not for whoever derived k.
func openChannelPayload(k [32]byte, c1, c2 []byte) (msg, senderId []byte, err error) {
	block, err := aes.NewCipher(k[:])
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	if len(c2) < gcm.NonceSize() {
		return nil, nil, fmt.Errorf("c₂ is %d bytes, shorter than its nonce", len(c2))
	}
	plain, err := gcm.Open(nil, c2[:gcm.NonceSize()], c2[gcm.NonceSize():], c1)
	if err != nil {
		return nil, nil, err
	}
	field := func() ([]byte, error) {
		if len(plain) < 4 || uint64(len(plain)-4) < uint64(binary.BigEndian.Uint32(plain)) {
			return nil, fmt.Errorf("c₂ opens to a truncated payload")
		}
		n := binary.BigEndian.Uint32(plain)
		v := plain[4 : 4+n]
		plain = plain[4+n:]
		return v, nil
	}
	if msg, err = field(); err != nil {
		return nil, nil, err
	}
	if senderId, err = field(); err != nil {
		return nil, nil, err
	}
	return msg, senderId, nil
}

// popcount is how many bits of b are set: the users a tag channel's bitmap
// covers.
func popcount(b []byte) int {
	n := 0
	for _, x := range b {
		n += bits.OnesCount8(x)
	}
	return n
}

// channelPayloadText is a tag channel's msg ‖ senderId as the cards show it.
func channelPayloadText(msg, senderId []byte) string {
	return string(msg) + " ‖ " + string(senderId)
}

// tagChannelSetupFormula is the setup formula of tag channel #idx, which from
// opened to to's view key with shared secret ss, capsule c1, sealed payload
// c2 of msg ‖ senderId and bitmap: k_ch derived from ss, checked against
// the record TagChannelRegistry holds (the same c₁, c₂ and bitmap) and the
// bitmap against the users the UserRegistry holds (every one of them).
func (c *chain) tagChannelSetupFormula(ctx context.Context, from, to *Actor, idx uint64, ss, c1, c2, bitmap, msg, senderId []byte) (HopDetail, error) {
	rec, err := c.channelRecord(ctx, idx)
	if err != nil {
		return HopDetail{}, fmt.Errorf("tag channel setup card: %w", err)
	}
	users, err := tags.GetUserCount(c.client, c.registryAddr)
	if err != nil {
		return HopDetail{}, fmt.Errorf("tag channel setup card: %w", err)
	}
	k := tags.DeriveChannelKey(ss)
	covered := popcount(bitmap)
	return HopDetail{Kind: DetailFormula, Vault: NodeTagChannelRegistry, Formula: &Formula{
		Name: FormulaTagChannelSetup,
		Inputs: []FormulaValue{
			{Name: fmt.Sprintf("pk_view · %s's view key", to.Name), Value: bytesHex(to.View.EncapsKey)},
			{Name: "ss · ML-KEM-768 shared secret", Value: bytesHex(ss)},
			{Name: "c₁ · ML-KEM-768 capsule", Value: bytesHex(c1)},
			{Name: "k_ch · HKDF-SHA256(ss, \"enygma-channel-key-v1\")", Value: bytesHex(k[:])},
			{Name: "msg", Value: string(msg)},
			{Name: fmt.Sprintf("senderId · the %s's address", from.Name), Value: string(senderId)},
			{Name: fmt.Sprintf("bitmap · PrivacyFull, %d of %d registered users", covered, users), Value: bytesHex(bitmap)},
		},
		Result: bytesHex(c2),
		Chain: &FormulaValue{Name: fmt.Sprintf("c₂ of tag channel #%d as TagChannelRegistry holds it (getChannel), beside the same c₁ and a bitmap of all %d users the UserRegistry holds", idx, users),
			Value: bytesHex(rec.c2)},
		OK: bytes.Equal(rec.c1, c1) && bytes.Equal(rec.c2, c2) && bytes.Equal(rec.bitmap, bitmap) && covered == users,
	}}, nil
}

// tagChannelDetectionFormula is the detection formula of tag channel f, which
// payee's scan of checked records found among opened: the record as
// TagChannelRegistry holds it, decapsulated with payee's view key to the
// payer's shared secret and its c₂ opened under the key derived from it, to
// the payer's senderId; and decapsulated with other's view key to some
// other secret, under whose key c₂ fails to open.
func (c *chain) tagChannelDetectionFormula(ctx context.Context, payer, payee, other *Actor, f tags.FoundChannel, checked uint64, opened int) (HopDetail, error) {
	rec, err := c.channelRecord(ctx, f.ChannelIdx)
	if err != nil {
		return HopDetail{}, fmt.Errorf("tag channel detection card: %w", err)
	}
	ss, err := payee.View.DecapsKey.Decapsulate(rec.c1)
	if err != nil {
		return HopDetail{}, fmt.Errorf("tag channel detection card: %s's Decaps: %w", payee.Name, err)
	}
	k := tags.DeriveChannelKey(ss)
	msg, senderId, err := openChannelPayload(k, rec.c1, rec.c2)
	if err != nil {
		return HopDetail{}, fmt.Errorf("tag channel detection card: %s's key does not open tag channel #%d: %w", payee.Name, f.ChannelIdx, err)
	}
	otherSS, err := other.View.DecapsKey.Decapsulate(rec.c1)
	if err != nil {
		return HopDetail{}, fmt.Errorf("tag channel detection card: %s's Decaps: %w", other.Name, err)
	}
	_, _, otherErr := openChannelPayload(tags.DeriveChannelKey(otherSS), rec.c1, rec.c2)
	otherOpen := fmt.Sprintf("fails: not %s's", other.Name)
	if otherErr == nil {
		otherOpen = "opens"
	}
	records := "records"
	if checked == 1 {
		records = "record"
	}
	return HopDetail{Kind: DetailFormula, Vault: NodeTagChannelRegistry, Formula: &Formula{
		Name: FormulaTagChannelDetection,
		Inputs: []FormulaValue{
			{Name: fmt.Sprintf("c₁ · tag channel #%d", f.ChannelIdx), Value: bytesHex(rec.c1)},
			{Name: fmt.Sprintf("c₂ · tag channel #%d", f.ChannelIdx), Value: bytesHex(rec.c2)},
			{Name: fmt.Sprintf("ss′ = Decaps(sk_view, c₁) · %s", payee.Name), Value: bytesHex(ss)},
			{Name: "k′ = HKDF-SHA256(ss′, \"enygma-channel-key-v1\")", Value: bytesHex(k[:])},
			{Name: fmt.Sprintf("ss″ = Decaps with %s's view key · a secret too", other.Name), Value: bytesHex(otherSS)},
			{Name: fmt.Sprintf("AES-256-GCM.Open of c₂ under %s's key", other.Name), Value: otherOpen},
		},
		Result: channelPayloadText(msg, senderId),
		Chain: &FormulaValue{Name: fmt.Sprintf("%s's scan of the records TagChannelRegistry published since their last one (getChannelCount); the one their view key opens is tag channel #%d, the %s's", payee.Name, f.ChannelIdx, payer.Name),
			Value: fmt.Sprintf("%d %s checked, %d opened", checked, records, opened)},
		OK: bytes.Equal(ss, f.SharedSecret) && bytes.Equal(ss, payer.tags.out[payee]) && string(senderId) == payer.Address.Hex() && otherErr != nil && opened == 1,
	}}, nil
}

// tagFormula is the tag formula of the tag from published to to's spend key
// on the tag channel with shared secret ss, landed in block b by tx:
// t_b = Poseidon(b, pk_spend, ss mod p), and the reverse direction's, with
// from's spend key, which differs; checked against the tag TagRegistry's
// TagPublished event logged in tx.
func (c *chain) tagFormula(ctx context.Context, from, to *Actor, ss []byte, b uint64, tx common.Hash) (HopDetail, error) {
	receipt, err := c.client.TransactionReceipt(ctx, tx)
	if err != nil {
		return HopDetail{}, fmt.Errorf("tag card: receipt for %s: %w", tx.Hex(), err)
	}
	t, err := tags.DeriveTag(b, to.Spend.PublicKey, ss)
	if err != nil {
		return HopDetail{}, fmt.Errorf("tag card: %w", err)
	}
	reverse, err := tags.DeriveTag(b, from.Spend.PublicKey, ss)
	if err != nil {
		return HopDetail{}, fmt.Errorf("tag card: %w", err)
	}
	var published []*ethtypes.Log
	for _, l := range receipt.Logs {
		if l.Address == c.tagRegistryAddr && len(l.Topics) >= 3 && l.Topics[0] == tagPublishedTopic {
			published = append(published, l)
		}
	}
	if len(published) != 1 {
		return HopDetail{}, fmt.Errorf("tag card: tx %s logged %d TagPublished events, want one", receipt.TxHash.Hex(), len(published))
	}
	logged := published[0]
	result := bytesHex(t[:])
	return HopDetail{Kind: DetailFormula, Vault: NodeTagRegistry, Formula: &Formula{
		Name: FormulaTag,
		Inputs: []FormulaValue{
			{Name: "b · the block the tag landed in", Value: fmt.Sprint(b)},
			{Name: fmt.Sprintf("pk_spend^payee · %s's spend key", to.Name), Value: fieldHex(to.Spend.PublicKey)},
			{Name: "ss · the tag channel's shared secret", Value: bytesHex(ss)},
			{Name: "ss mod p · in BN254's scalar field", Value: fieldHex(new(big.Int).Mod(new(big.Int).SetBytes(ss), constants.Q))},
			{Name: fmt.Sprintf("t_b′ = Poseidon(b, pk_spend^%s, ss mod p) · the reverse direction, %s → %s", from.Name, to.Name, from.Name), Value: bytesHex(reverse[:])},
		},
		Result: result,
		Chain: &FormulaValue{Name: fmt.Sprintf("the tag TagRegistry's TagPublished event logged for block %d (tx %s), the one of the payer's %d-block window the relayer published", logged.Topics[1].Big(), receipt.TxHash.Hex(), tagWindow),
			Value: logged.Topics[2].Hex()},
		OK: logged.Topics[2].Hex() == result && logged.Topics[1].Big().Uint64() == b && reverse != t,
	}}, nil
}

// tagPayloadFormula is the tag payload formula of f, a note payee found by
// its tag: the block key k_b of f's tag channel for f's block and payee's
// spend key, which opens the payload to amount ‖ token ‖ salt, and the
// commitment those hash to with payee's pk_spend, checked against the
// vault's Commitment event for it.
func (c *chain) tagPayloadFormula(ctx context.Context, payee *Actor, f taggedNote) (HopDetail, error) {
	k := tags.DeriveBlockKey(f.ch.SharedSecret, f.block, payee.Spend.PublicKey)
	cm, err := rpcore.Erc20CommitmentV2(payee.Spend.PublicKey, f.opened.Salt, f.opened.Amount, f.opened.TokenId)
	if err != nil {
		return HopDetail{}, fmt.Errorf("tag payload card: %w", err)
	}
	logs, err := c.client.FilterLogs(ctx, ethereum.FilterQuery{
		Addresses: []common.Address{c.vaultAddr},
		Topics:    [][]common.Hash{{commitmentTopic}, nil, {common.BigToHash(cm)}},
	})
	if err != nil {
		return HopDetail{}, fmt.Errorf("tag payload card: vault Commitment events: %w", err)
	}
	if len(logs) != 1 {
		return HopDetail{}, fmt.Errorf("tag payload card: the vault logged %d Commitment events for 0x%x, want one", len(logs), cm)
	}
	result := fieldHex(cm)
	return HopDetail{Kind: DetailFormula, Vault: NodeVault, Formula: &Formula{
		Name: FormulaTagPayload,
		Inputs: []FormulaValue{
			{Name: "ss · the tag channel's shared secret", Value: bytesHex(f.ch.SharedSecret)},
			{Name: "b · the block the tag landed in", Value: fmt.Sprint(f.block)},
			{Name: fmt.Sprintf("pk_spend^payee · %s's spend key", payee.Name), Value: fieldHex(payee.Spend.PublicKey)},
			{Name: `k_b · HKDF-SHA256(ss, "enygma-block-key-v1" ‖ b ‖ pk_spend^payee)`, Value: bytesHex(k[:])},
			{Name: "ct · the tag's payload in TagRegistry", Value: bytesHex(f.payload)},
			{Name: "amount · opened", Value: f.opened.Amount.String()},
			{Name: "token · opened", Value: fieldHex(f.opened.TokenId)},
			{Name: "salt · opened", Value: fieldHex(f.opened.Salt)},
		},
		Result: result,
		Chain: &FormulaValue{Name: fmt.Sprintf("the vault's Commitment event for %s's note (tx %s)", payee.Name, logs[0].TxHash.Hex()),
			Value: fieldHex(logs[0].Topics[2].Big())},
		OK: logs[0].Topics[2].Big().Cmp(cm) == 0,
	}}, nil
}

// subscript is n in subscript digits, e.g. "₁₂".
func subscript(n int) string { return digits(n, "₀₁₂₃₄₅₆₇₈₉") }

// superscript is n in superscript digits, e.g. "⁸".
func superscript(n int) string { return digits(n, "⁰¹²³⁴⁵⁶⁷⁸⁹") }

// digits is n written with set's ten runes for 0–9.
func digits(n int, set string) string {
	rs := []rune(set)
	out := ""
	for _, c := range fmt.Sprint(n) {
		out += string(rs[c-'0'])
	}
	return out
}
