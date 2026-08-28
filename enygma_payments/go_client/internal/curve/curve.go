package curve

import(
	"math/big"
	"github.com/iden3/go-iden3-crypto/babyjub"
	)

var (
	// G: Pedersen value generator. NUMS hash-to-curve derivation, seed "2"
	// (H-11 fix). Reproduce with: cd gnark-server && go run ./cmd/derive_generator
	gx, _ = new(big.Int).SetString("12337812418750581066638756637363471856433191340622504180842886595232027947307", 10)
	gy, _ = new(big.Int).SetString("15225366398330386329633463986700597127113326976080712967801565482915963669722", 10)
	G     = &babyjub.Point{X: gx, Y: gy}
	//Another Baby Jubjub point (randomnly generated)
	hx, _ = new(big.Int).SetString("10100005861917718053548237064487763771145251762383025193119768015180892676690", 10)
	hy, _ = new(big.Int).SetString("7512830269827713629724023825249861327768672768516116945507944076335453576011", 10)
	H     = &babyjub.Point{X: hx, Y: hy}

	//subgroup order of BabyJubJub Eliptic curve
	P, _  = new(big.Int).SetString("2736030358979909402780800718157159386076813972158567259200215660948447373041", 10)
	
)

func GetPK(v *big.Int) *babyjub.Point {
	rG := babyjub.NewPoint().Mul(v, G)
	return rG
}

func GetH(v *big.Int) *babyjub.Point {
	rG := babyjub.NewPoint().Mul(v, H)
	return rG
}

func PedersenCommitment(v *big.Int, r *big.Int) *babyjub.Point {

	vG := GetPK(v)
	vH := GetH(r)

	return AddPks(vG, vH)
}

func AddPks(pk1 *babyjub.Point, pk2 *babyjub.Point) *babyjub.Point {
	return babyjub.NewPoint().Projective().Add(pk1.Projective(), pk2.Projective()).Affine()
}

func GetNegative(x *big.Int) *big.Int {

	inverse := big.NewInt(0).Sub(P, x)

	return inverse
}