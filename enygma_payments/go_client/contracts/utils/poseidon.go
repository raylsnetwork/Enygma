package main
import(
	"fmt"
	"math/big"
	"github.com/iden3/go-iden3-crypto/poseidon")


func getNegative(x *big.Int) *big.Int {
	p := new(big.Int)
	p.SetString("2736030358979909402780800718157159386076813972158567259200215660948447373041", 10)

	inverse := big.NewInt(0).Sub(p, x)

	return inverse
}

func main() {
	//P, _  := new(big.Int).SetString("2736030358979909402780800718157159386076813972158567259200215660948447373041", 10)
	number0, _ := new(big.Int).SetString("10", 10)
	number1, _ := new(big.Int).SetString("1089500522410495261217637735585785092230470237069473528161195012019712635542", 10)
	
	

	inputs := []*big.Int{number0, number1}
	PoseidonHash, _ := poseidon.Hash(inputs)
	

	fmt.Println(PoseidonHash)
	
	//PoseidonHash.Mod(PoseidonHash, P)

}