package primitives 

import(
	"github.com/consensys/gnark/frontend"
 	"github.com/consensys/gnark/std/algebra/native/twistededwards"
	"github.com/consensys/gnark/std/math/cmp"

	utils "gnark_server/utils"
)


type AuditorAccessCircuit struct {
	TmRealLength  		int                    `gnark:"-"`
	StPublicKey   		[2]frontend.Variable   
	StNounce      		frontend.Variable      `gnark:"nonce,public"`
	StEncryptedValues   []frontend.Variable    `gnark:"key,secret"`
	WtRandom     		frontend.Variable 
	WtValues 			[]frontend.Variable     
}




func (circuit *AuditorAccessCircuit) Define(api frontend.API) error {
	
	tmRealLength := circuit.TmRealLength

	decryptedLength := tmRealLength
	for decryptedLength%3 != 0 {
		decryptedLength++
	}

	barseOrder := frontend.Variable("2736030358979909402780800718157159386076813972158567259200215660948447373041")

	utils.AssertPointsIsOnCurve(api,circuit.StPublicKey[0],circuit.StPublicKey[1])

	for j:=0; j < circuit.TmRealLength;j++{

		isValid0 := cmp.IsLessOrEqual(api, 0,circuit.WtValues[j])
		
		api.AssertIsEqual(isValid0, 1)
		
		isValid1 := cmp.IsLess(api,circuit.WtValues[j], barseOrder)
		
		api.AssertIsEqual(isValid1, 1)
	}

	isValid3 := cmp.IsLess(api,0, circuit.WtRandom)
	
	api.AssertIsEqual(isValid3, 1)
	
	isValid4 := cmp.IsLess(api,circuit.WtRandom, barseOrder)
	
	api.AssertIsEqual(isValid4, 1)
	

	PkTwist := twistededwards.Point{
		X: circuit.StPublicKey[0],
		Y: circuit.StPublicKey[1],
	}

	checkEncKey := utils.ScalarMul(api,PkTwist, circuit.WtRandom)

	key:= [2]frontend.Variable{
		frontend.Variable(checkEncKey.X),
		frontend.Variable(checkEncKey.Y),
	}
	

	PoseidonDecryptValue := PoseidonDecrypt(api, circuit.TmRealLength,circuit.StNounce,key, circuit.StEncryptedValues )

	for j:=0; j< circuit.TmRealLength; j++{
		
		api.AssertIsEqual(PoseidonDecryptValue[j], circuit.WtValues[j])
	}

	return nil
}