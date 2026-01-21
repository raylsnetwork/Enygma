package primitives 

import(
	"github.com/consensys/gnark/frontend"
 	"github.com/consensys/gnark/std/algebra/native/twistededwards"

	utils "gnark_server/utils"
)

var(
	B = twistededwards.Point{
		X: "5299619240641551281634865583518297030282874472190772894086521144482721001553",
		Y: "16950150798460657717958625567821834550301663161624707787222815936182638968203",
	}
)

func ElGammalEncrypt(api frontend.API, random frontend.Variable, pk [2]frontend.Variable, msg [2]frontend.Variable)([2]frontend.Variable,[2]frontend.Variable){

	//Check if pk in on the curve
	utils.AssertPointsIsOnCurve(api,pk[0],pk[1])

	//Check if msg in on the curve
	utils.AssertPointsIsOnCurve(api,msg[0],msg[1])

	publicKey := utils.ScalarMul(api, B, random) 

	PkTwist := twistededwards.Point{
		X: pk[0],
		Y: pk[1],
	}

	publickey2:= utils.ScalarMul(api, PkTwist, random)

	
	msgTwist := twistededwards.Point{
		X:msg[0],
		Y:msg[1],
	}

	sumVar := utils.PointAdd(api, msgTwist,publickey2 )

	publicKeyFV := [2]frontend.Variable{
		frontend.Variable(publicKey.X),
		frontend.Variable(publicKey.Y),
	}

	sumVaryFV := [2]frontend.Variable{
		frontend.Variable(sumVar.X),
		frontend.Variable(sumVar.Y),
	}


	return publicKeyFV, sumVaryFV
}

func ElGammalDecrypt(api frontend.API, c1 [2]frontend.Variable,c2 [2]frontend.Variable,privKey frontend.Variable) (twistededwards.Point){


	//Check if c1 in on the curve
	utils.AssertPointsIsOnCurve(api,c1[0],c1[1])

	//Check if c2 in on the curve
	utils.AssertPointsIsOnCurve(api,c2[0],c2[1])

	PkTwist := twistededwards.Point{
		X: c1[0],
		Y: c2[1],
	}

	PublicKey := utils.ScalarMul(api, PkTwist, privKey)

	

	inves := [2]frontend.Variable{
		api.Sub(0, PublicKey.X),
		PublicKey.Y,

	}

	invesTwist:= twistededwards.Point{
		X: inves[0],
		Y: inves[1],
	}

	c2Twist := twistededwards.Point{
		X: c2[0],
		Y: c2[1],
	}

	sumVar := utils.PointAdd(api, invesTwist,c2Twist )
	return sumVar
}	