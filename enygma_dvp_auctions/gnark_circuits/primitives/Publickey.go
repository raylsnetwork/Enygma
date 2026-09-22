package primitives

import (
	pos "enygma_dvp_auctions/gnark_circuits/poseidon"
	"github.com/consensys/gnark/frontend"
)

func PublicKey(api frontend.API, privateKey frontend.Variable) frontend.Variable {
	return pos.Poseidon(api, []frontend.Variable{privateKey})
}

func PublicKeyNative(api frontend.API, privateKey frontend.Variable) frontend.Variable {

	publicKeyId, _ := api.NewHint(PoseidonPrivateKeyNative, 1, privateKey)

	return publicKeyId[0]

}
