package primitives

import (
	//"fmt"
	"math/big"

	"github.com/iden3/go-iden3-crypto/poseidon"
)

func ModHint(mod *big.Int, inputs []*big.Int, res []*big.Int) error {
	p := new(big.Int)
	p.SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

	value := inputs[0]
	q := new(big.Int)
	r := new(big.Int)

	q.DivMod(value, p, r) // q = value / p, r = value % p

	res[0] = r // remainder
	res[1] = q // quotient
	return nil

}

// PoseidonNative and PoseidonPrivateKeyNative — ported from
// enygma_retail_payments/gnark_circuits/primitives/Utils.go for the Payment-
// family circuits (Payment/PaymentFee/PaymentRelayerFeePublic/UsdrFee),
// registered defensively via solver.RegisterHint in their handlers even
// though these particular Define() methods don't call them directly (they
// use the Nullifier/PublicKey/MerkleProof in-circuit gadgets instead, which
// already exist in this file's sibling primitives and are unchanged here).

func PoseidonNative(mod *big.Int, inputs []*big.Int, res []*big.Int) error {
	p := new(big.Int)
	p.SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

	value := inputs[0]
	random := inputs[1]

	hash, _ := poseidon.Hash([]*big.Int{value, random})

	hash.Mod(hash, p)

	res[0] = hash
	return nil
}

func PoseidonPrivateKeyNative(mod *big.Int, inputs []*big.Int, res []*big.Int) error {
	p := new(big.Int)
	p.SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

	privateKey := inputs[0]

	hash, _ := poseidon.Hash([]*big.Int{privateKey})

	hash.Mod(hash, p)

	res[0] = hash
	return nil
}
