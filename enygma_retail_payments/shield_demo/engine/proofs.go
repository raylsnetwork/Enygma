package engine

import (
	"fmt"
	"math/big"

	"enygma_retail_payments/layout"
	rpcore "github.com/raylsnetwork/enygma_retail_payments/src/core"
)

// statementField is one named field of a circuit's statement layout.
type statementField struct {
	name         string
	offset, size int
}

// statement is a circuit's statement layout, as enygma_retail_payments/layout
// has it, and the names of its private witnesses (its Wt fields).
type statement struct {
	len     int
	fields  []statementField
	private []string
}

// The statements of the circuits the demo proves, by circuit name.
var statements = map[string]statement{
	"Payment": {layout.PaymentLen, []statementField{
		{"Message", layout.PaymentMessageOffset, layout.PaymentMessageSize},
		{"TreeNumbers", layout.PaymentTreeNumbersOffset, layout.PaymentTreeNumbersSize},
		{"MerkleRoots", layout.PaymentMerkleRootsOffset, layout.PaymentMerkleRootsSize},
		{"Nullifiers", layout.PaymentNullifiersOffset, layout.PaymentNullifiersSize},
		{"CommitmentsOut", layout.PaymentCommitmentsOutOffset, layout.PaymentCommitmentsOutSize},
		{"ContractAddress", layout.PaymentContractAddressOffset, layout.PaymentContractAddressSize},
	}, paymentPrivate},
	"Payment2in": {layout.Payment2inLen, []statementField{
		{"Message", layout.Payment2inMessageOffset, layout.Payment2inMessageSize},
		{"TreeNumbers", layout.Payment2inTreeNumbersOffset, layout.Payment2inTreeNumbersSize},
		{"MerkleRoots", layout.Payment2inMerkleRootsOffset, layout.Payment2inMerkleRootsSize},
		{"Nullifiers", layout.Payment2inNullifiersOffset, layout.Payment2inNullifiersSize},
		{"CommitmentsOut", layout.Payment2inCommitmentsOutOffset, layout.Payment2inCommitmentsOutSize},
		{"ContractAddress", layout.Payment2inContractAddressOffset, layout.Payment2inContractAddressSize},
	}, paymentPrivate},
	"UsdrFee": {layout.UsdrFeeLen, []statementField{
		{"Message", layout.UsdrFeeMessageOffset, layout.UsdrFeeMessageSize},
		{"TreeNumbers", layout.UsdrFeeTreeNumbersOffset, layout.UsdrFeeTreeNumbersSize},
		{"MerkleRoots", layout.UsdrFeeMerkleRootsOffset, layout.UsdrFeeMerkleRootsSize},
		{"Nullifiers", layout.UsdrFeeNullifiersOffset, layout.UsdrFeeNullifiersSize},
		{"CommitmentsOut", layout.UsdrFeeCommitmentsOutOffset, layout.UsdrFeeCommitmentsOutSize},
		{"ContractAddress", layout.UsdrFeeContractAddressOffset, layout.UsdrFeeContractAddressSize},
		{"Fee", layout.UsdrFeeFeeOffset, layout.UsdrFeeFeeSize},
		{"TokenId", layout.UsdrFeeTokenIdOffset, layout.UsdrFeeTokenIdSize},
	}, []string{"PrivateKeysIn", "ValuesIn", "SaltsIn", "PathElements", "PathIndices", "SpendPublicKeysOut", "ValuesOut", "SaltsOut"}},
}

// paymentPrivate is the Payment circuit's private witnesses: the token id
// is private here, public in UsdrFee.
var paymentPrivate = []string{"PrivateKeysIn", "ValuesIn", "SaltsIn", "PathElements", "PathIndices", "TokenId", "SpendPublicKeysOut", "ValuesOut", "SaltsOut"}

// merklePath is the merklePath hop detail of p, the Merkle proof of owner's
// note in the vault with node ID vault.
func merklePath(vault, owner string, p *rpcore.MerkleProof) HopDetail {
	d := HopDetail{Kind: DetailMerklePath, Vault: vault, Tree: p.TreeNumber, Root: fieldHex(p.Root),
		// The path's indices are the leaf's index: bit i says the path's node
		// at level i is a right child.
		Leaf: &Leaf{Index: int(p.Indices.Int64()), Commitment: fieldHex(p.Element), Owner: owner}}
	for _, s := range p.Elements {
		d.Siblings = append(d.Siblings, fieldHex(s))
	}
	return d
}

// publicInputs is the publicInputs hop detail of a circuit proof spending
// from the vault with node ID vault, its first spent note in tree tree, with
// public signals stmt.
func publicInputs(circuit, vault string, tree int, stmt []*big.Int) (HopDetail, error) {
	st, ok := statements[circuit]
	if !ok {
		return HopDetail{}, fmt.Errorf("no statement layout for circuit %s", circuit)
	}
	if len(stmt) != st.len {
		return HopDetail{}, fmt.Errorf("%s statement has %d signals, want %d", circuit, len(stmt), st.len)
	}
	d := HopDetail{Kind: DetailPublicInputs, Vault: vault, Tree: tree, Circuit: circuit, Inputs: make([]PublicInput, st.len), Private: st.private}
	for _, f := range st.fields {
		for i := range f.size {
			d.Inputs[f.offset+i] = PublicInput{Name: f.name, Index: i, Value: fieldHex(stmt[f.offset+i])}
		}
	}
	return d, nil
}
