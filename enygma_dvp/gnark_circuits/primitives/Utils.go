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

	q.DivMod(value, p, r)     // q = value / p, r = value % p

    res[0] = r  // remainder
    res[1] = q  // quotient
    return nil
		
}


func ERC155UniqueIdNative(mod *big.Int, inputs []*big.Int, res []*big.Int)error{
	p := new(big.Int)
	p.SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

	address := inputs[0]
	id := inputs[1]
	amount := inputs[2]
	id1, _ := poseidon.Hash([]*big.Int{address,id})
	id1.Mod(id1, p)


	erc1155Id,_ := poseidon.Hash([]*big.Int{id1,amount})
	erc1155Id.Mod(erc1155Id, p)
	res[0] = erc1155Id
	return nil
}


func PoseidonNative(mod *big.Int, inputs []*big.Int, res []*big.Int)error{
	p := new(big.Int)
	p.SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

	value := inputs[0]
	random := inputs[1]

	hash, _ := poseidon.Hash([]*big.Int{value,random})
	
	hash.Mod(hash, p)

	res[0] = hash
	return nil
}


func PoseidonPrivateKeyNative(mod *big.Int, inputs []*big.Int, res []*big.Int)error{
	p := new(big.Int)
	p.SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

	privateKey := inputs[0]
	
	hash, _ := poseidon.Hash([]*big.Int{privateKey})
	
	hash.Mod(hash, p)

	res[0] = hash
	return nil
}