package primitives 

import(
	"github.com/consensys/gnark/frontend"
	 pos "gnark_server/poseidon"
)


func PublicKey(api frontend.API, privateKey frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{privateKey})
}


func PublicKeyNative(api frontend.API, privateKey frontend.Variable)frontend.Variable{

	publicKeyId,_:= api.NewHint(PoseidonPrivateKeyNative, 1, privateKey)
		
	return publicKeyId[0]

}