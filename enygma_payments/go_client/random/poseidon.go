package main

import (
	"fmt"
	"math/big"
	"github.com/iden3/go-iden3-crypto/poseidon"

)

func main(){

	address,_ := new(big.Int).SetString("984741162923299469131306392226555244975478449736", 10)
	amount,_ := new(big.Int).SetString("11", 10)
	inputs := []*big.Int{address,amount}
	PoseidonHash, _ := poseidon.Hash(inputs)
	pk,_ := new(big.Int).SetString("7344690997738223295645154053021918994799603882408193002967283753145648589458",10)
	inputs2 := []*big.Int{PoseidonHash,pk}
	
	Hash, _ := poseidon.Hash(inputs2)
	fmt.Println(Hash)

	
}
