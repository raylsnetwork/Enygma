package primitives

import (
	//"fmt"
	"math/big"
	
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


