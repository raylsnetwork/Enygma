package engine

import (
	"context"
	"fmt"
	"math/big"

	"enygma_retail_payments/layout"
	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
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

// valueRange is the bound of the circuits' range check, TmRange: every
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
