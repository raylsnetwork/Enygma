package main

import (
	"fmt"
	"math/big"
	"github.com/iden3/go-iden3-crypto/poseidon"

)

func main(){
	P, _  := new(big.Int).SetString("2736030358979909402780800718157159386076813972158567259200215660948447373041", 10)
	PreviousV,_ :=new(big.Int).SetString("1000", 10)
	sk,_ := new(big.Int).SetString("35", 10)
	
	inputs := []*big.Int{PreviousV,sk}
	PoseidonHash, _ := poseidon.Hash(inputs)
	PoseidonHash.Mod(PoseidonHash, P)
	
	fmt.Println(PoseidonHash)

	
}
