package engine

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"enygma_retail_payments/layout"
	"github.com/ethereum/go-ethereum/common"
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
	d.Siblings = siblings(p)
	return d
}

// siblings are Merkle proof p's siblings, from the leaf up.
func siblings(p *rpcore.MerkleProof) []string {
	out := make([]string, len(p.Elements))
	for i, s := range p.Elements {
		out[i] = fieldHex(s)
	}
	return out
}

// witness is the witness hop detail of payment proof res by from, spending
// ins along proofs, paying to amount and the change back to from.
func witness(circuit string, from, to *Actor, ins []*note, proofs []*rpcore.MerkleProof, amount, change *big.Int, res *rpcore.PaymentResult) HopDetail {
	w := &Witness{SpendKey: fieldHex(from.Spend.PrivateKey), Token: fieldHex(tokenId),
		Out: []NoteOpening{
			{Owner: to.Name, PkSpend: fieldHex(to.Spend.PublicKey), Amount: amount.String(), Salt: fieldHex(res.SaltB)},
			{Owner: from.Name, PkSpend: fieldHex(from.Spend.PublicKey), Amount: change.String(), Salt: fieldHex(res.SaltA)},
		}}
	for i, n := range ins {
		p := &NotePath{Tree: proofs[i].TreeNumber, Index: int(proofs[i].Indices.Int64()), Siblings: siblings(proofs[i])}
		w.In = append(w.In, NoteOpening{Owner: from.Name, Amount: n.amount.String(), Salt: fieldHex(n.salt), Path: p})
	}
	return HopDetail{Kind: DetailWitness, Vault: NodeVault, Tree: proofs[0].TreeNumber, Circuit: circuit, Witness: w}
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

// settleChecks is the settleChecks hop detail of a circuit proof with
// public signals stmt (already length-checked by publicInputs), spending from
// the vault with node ID vault and settled in transaction tx.
func settleChecks(circuit, vault string, stmt []*big.Int, tx common.Hash) HopDetail {
	field := map[string]statementField{}
	for _, f := range statements[circuit].fields {
		field[f.name] = f
	}
	trees, roots, nullifiers := field["TreeNumbers"], field["MerkleRoots"], field["Nullifiers"]
	d := HopDetail{Kind: DetailSettleChecks, Vault: vault, Tree: int(stmt[trees.offset].Int64()), Circuit: circuit, TxHash: tx.Hex()}
	// One tree number, root and nullifier per spent note.
	for i := range nullifiers.size {
		d.Spent = append(d.Spent, SpentNote{Tree: int(stmt[trees.offset+i].Int64()),
			Root: fieldHex(stmt[roots.offset+i]), Nullifier: fieldHex(stmt[nullifiers.offset+i])})
	}
	return d
}

// prover is the prover hop detail of a circuit proof spending from the
// vault with node ID vault, its first spent note in tree tree, read off
// wire, the proof request's http detail: the stats the gnark server's
// response carries and, in a walkthrough (withWitness), the private witness
// the request sent, in the circuit's witness order, and the proof and
// public signals the response returned.
func prover(circuit, vault string, tree int, wire HopDetail, withWitness bool) (HopDetail, error) {
	var resp struct {
		Stats        *ProverStats  `json:"stats"`
		Proof        []json.Number `json:"proof"`
		PublicSignal []json.Number `json:"publicSignal"`
	}
	if err := decodeNumbers(wire.Response, &resp); err != nil {
		return HopDetail{}, fmt.Errorf("%s proof response: %w", circuit, err)
	}
	p := &Prover{Stats: resp.Stats}
	d := HopDetail{Kind: DetailProver, Vault: vault, Tree: tree, Circuit: circuit, Prover: p}
	if !withWitness {
		return d, nil
	}
	var req map[string]any
	if err := decodeNumbers(wire.Request, &req); err != nil {
		return HopDetail{}, fmt.Errorf("%s proof request: %w", circuit, err)
	}
	for _, name := range statements[circuit].private {
		v, ok := req["wt"+name]
		if !ok {
			return HopDetail{}, fmt.Errorf("%s proof request has no wt%s", circuit, name)
		}
		values, err := flatHex(v)
		if err != nil {
			return HopDetail{}, fmt.Errorf("%s proof request's wt%s: %w", circuit, name, err)
		}
		p.Witness = append(p.Witness, WitnessInput{Name: name, Values: values})
	}
	var err error
	if p.Proof, err = flatHex(numbers(resp.Proof)); err != nil {
		return HopDetail{}, fmt.Errorf("%s proof: %w", circuit, err)
	}
	if p.Signals, err = flatHex(numbers(resp.PublicSignal)); err != nil {
		return HopDetail{}, fmt.Errorf("%s public signals: %w", circuit, err)
	}
	return d, nil
}

// decodeNumbers decodes JSON body into v, numbers kept as json.Number: the
// gnark server sends field elements as bare numbers too long for a float.
func decodeNumbers(body string, v any) error {
	dec := json.NewDecoder(strings.NewReader(body))
	dec.UseNumber()
	return dec.Decode(v)
}

// numbers is ns as a list of JSON values, for flatHex.
func numbers(ns []json.Number) []any {
	out := make([]any, len(ns))
	for i, n := range ns {
		out[i] = n
	}
	return out
}

// flatHex is a decoded JSON value (a decimal string or number, or a nested
// list of them) as 0x-hex field elements, flattened in order.
func flatHex(v any) ([]string, error) {
	switch x := v.(type) {
	case []any:
		var out []string
		for _, e := range x {
			hs, err := flatHex(e)
			if err != nil {
				return nil, err
			}
			out = append(out, hs...)
		}
		return out, nil
	case string, json.Number:
		n, ok := new(big.Int).SetString(fmt.Sprint(x), 10)
		if !ok {
			return nil, fmt.Errorf("%q is not a decimal field element", x)
		}
		return []string{fieldHex(n)}, nil
	}
	return nil, fmt.Errorf("%v (%T) is not a field element or a list of them", v, v)
}
