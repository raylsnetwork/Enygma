package primitives 

import(
	"math/big"
	"github.com/consensys/gnark/frontend"
 	"github.com/consensys/gnark/std/math/cmp"

	pos "gnark_server/poseidon"
)



func PoseidonDecrypt(api frontend.API,  tm_realLength int, nouce frontend.Variable, key [2]frontend.Variable, ciphertext []frontend.Variable)[]frontend.Variable{


	tmRealLength := tm_realLength

	decryptedLength := tmRealLength
	for decryptedLength%3 != 0 {
		decryptedLength++
	}

	two128 := big.NewInt(1)
	two128.Lsh(two128, 128) // Shift left by 128 bits to get 2^128
	two128Var := frontend.Variable(two128)

	// Check if Nouce is less than two128
	isValid0 := cmp.IsLess(api,nouce, two128Var)
	api.AssertIsEqual(isValid0, 1)

	n := (decryptedLength + 1) / 3
	
	strategyOuts := make([][]frontend.Variable, n+1)

	decrypted := make([]frontend.Variable,decryptedLength)

	inputs:= make([]frontend.Variable, 3)
	inputs[0] = key[0]
	inputs[1] = key[1]
	inputs[2] =  api.Add(nouce,(api.Mul(two128Var, tmRealLength)))
	strategyOuts[0] = pos.PoseidonEx(api,inputs,0,4)


	// Main decryption loop
	for i := 0; i < n; i++ {
		// Decrypt 3 elements at a time
		for j := 0; j < 3; j++ {
			if i*3+j < decryptedLength {
				decrypted[i*3+j] = api.Sub(ciphertext[i*3+j], strategyOuts[i][j+1])
			}
		}
		
		// Prepare inputs for next strategy call
		inputsNext := make([]frontend.Variable, 3)
		for j := 0; j < 3; j++ {
			if i*3+j < len(ciphertext)-1 { // -1 because last element is for verification
				inputsNext[j] = ciphertext[i*3+j]
			} else {
				inputsNext[j] = 0 // Pad with zeros if needed
			}
		}
		
		// Next strategy call
		strategyOuts[i+1] = pos.PoseidonEx(api, inputsNext, strategyOuts[i][0], 4)
	}

	api.AssertIsEqual(ciphertext[decryptedLength], strategyOuts[n][1])


	if (tm_realLength%3 >0){
		if(tm_realLength%3 ==1){
			decrypted[decryptedLength-1] =0 
		}else if(tm_realLength%3 ==2){
			decrypted[decryptedLength-1] =0 
			decrypted[decryptedLength-2] =0 
		}


	}
	return decrypted

}
