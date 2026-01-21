package primitives

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/bits"
	//"gnark_server/primitives"
)

const MuxBits = 4  // CORRECT
const MuxSize = 16 


func UniqueIdMix(api frontend.API, vaultId frontend.Variable, contractAddress frontend.Variable, idParams []frontend.Variable)frontend.Variable{

	sel := bits.ToBinary(api, vaultId, bits.WithNbDigits(MuxBits))

	erc20Out   := UniqueId(api,contractAddress, idParams[0])
	erc721Out  := UniqueId(api,contractAddress, idParams[0])

	erc1155Out := Erc1155UniqueId2(api,contractAddress, idParams[1],idParams[0])
	
	var candidates [MuxSize]frontend.Variable
	candidates[0] = erc20Out
	candidates[1] = erc721Out
	candidates[2] = erc1155Out
	candidates[3] = frontend.Variable(0)

	for i:=4; i<MuxSize; i++{
		candidates[i]=frontend.Variable(0)
	}

	result := frontend.Variable(0)
	for i := 0; i < MuxSize; i++ {
		eq := frontend.Variable(1)
		for j := 0; j < MuxBits; j++ {
			bit := (i >> j) & 1
            if bit == 1 {
                eq = api.Mul(eq, sel[j])
            } else {
                eq = api.Mul(eq, api.Sub(1, sel[j]))
            }
		}
		// accumulate
		result = api.Add(result, api.Mul(eq, candidates[i]))
	}

	return result

}

