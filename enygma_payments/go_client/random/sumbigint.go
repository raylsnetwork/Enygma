package main

import(
	"fmt"
"math/big"
)

func main(){
	numStr2 := "1677225829047290318334782463897745216488460720515234756006337105031411411607"
	numStr1 := "1677225829047290318334782463897745216488460720515234756006337105031411411607"
	P, _  := new(big.Int).SetString("2736030358979909402780800718157159386076813972158567259200215660948447373041", 10)
	                                 
	// Initialize big.Int objects
	num1,_ := new(big.Int).SetString(numStr1, 10)
	num2,_ := new(big.Int).SetString(numStr2, 10)
	sum := new(big.Int)
	sum.Add(num1, num2)
	sum.Mod(sum, P)
	fmt.Println(sum)
}