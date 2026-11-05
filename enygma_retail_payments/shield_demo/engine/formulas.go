package engine

import (
	"context"
	"fmt"
	"math/big"

	dvpcore "github.com/raylsnetwork/enygma_dvp/src/core"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
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
