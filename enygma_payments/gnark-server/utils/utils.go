package utils

import (
	"os"
	"math/big"	
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"

	
)

var(
	A = big.NewInt(168700) 
	D =  big.NewInt(168696)
    gx, _ = new(big.Int).SetString("5299619240641551281634865583518297030282874472190772894086521144482721001553", 10)
	gy, _ = new(big.Int).SetString("16950150798460657717958625567821834550301663161624707787222815936182638968203", 10)
    GBabyJub     = &babyjub.Point{X: gx, Y: gy}

 	hx, _ = new(big.Int).SetString("10031262171927540148667355526369034398030886437092045105752248699557385197826", 10)
 	hy, _ = new(big.Int).SetString("633281375905621697187330766174974863687049529291089048651929454608812697683", 10)
 	HBabyJub     = &babyjub.Point{X: hx, Y: hy}

	P, _  = new(big.Int).SetString("2736030358979909402780800718157159386076813972158567259200215660948447373041", 10)
														
)									


func GetPkHash(sk *big.Int) *big.Int{
	inputs := []*big.Int{sk,sk}
	hash,_ := poseidon.Hash(inputs)
	return hash
}


func GetPK(v *big.Int) *babyjub.Point {
	rG := babyjub.NewPoint().Mul(v, GBabyJub)
	return rG
}

func GetH(v *big.Int) *babyjub.Point {
	rG := babyjub.NewPoint().Mul(v, HBabyJub)
	return rG
}

func PedersenCommitmentBabyJub(v *big.Int, r *big.Int) *babyjub.Point {

	vG := GetPK(v)
	vH := GetH(r)

	return AddPks(vG, vH)
}

func AddPks(pk1 *babyjub.Point, pk2 *babyjub.Point) *babyjub.Point {
	return babyjub.NewPoint().Projective().Add(pk1.Projective(), pk2.Projective()).Affine()
}


func LoadProvingKey(curve ecc.ID, filename string) (groth16.ProvingKey, error) {
    file, err := os.Open( filename)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    pk := groth16.NewProvingKey(curve) // e.g., ecc.BN254
    _, err = pk.ReadFrom(file)
    return pk, err
}

// Load verifying key from file
func LoadVerifyingKey(curve ecc.ID, filename string) (groth16.VerifyingKey, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    vk := groth16.NewVerifyingKey(curve) // e.g., ecc.BN254
    _, err = vk.ReadFrom(file)
    return vk, err
}

func ModHint(mod *big.Int, inputs []*big.Int, res []*big.Int) error {
	p := new(big.Int)
	p.SetString("2736030358979909402780800718157159386076813972158567259200215660948447373041", 10)
	
	value := inputs[0]
	q := new(big.Int)
    r := new(big.Int)

	q.DivMod(value, p, r)     // q = value / p, r = value % p

    res[0] = r  // remainder
    res[1] = q  // quotient
    return nil
		
    return nil
}

func ParseBigInt(s string) *big.Int {
    n, _ := new(big.Int).SetString(s, 10)
    return n
}
