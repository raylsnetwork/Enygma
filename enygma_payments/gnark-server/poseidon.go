
package main

import (
	"fmt"
	"math/big"
	"github.com/iden3/go-iden3-crypto/poseidon"

)

func main(){

	num1,_ := new(big.Int).SetString("12467758923472378", 10)

	inputs := []*big.Int{num1,num1}
	PoseidonHash, _ := poseidon.Hash(inputs)
	
	fmt.Println(PoseidonHash)
	
}  