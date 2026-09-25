package poseidon

import (
	"fmt"
	"math/big"
)

// bn254p is the BN254 scalar field prime.
var bn254p, _ = new(big.Int).SetString(
	"21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

// toBI extracts a *big.Int from a frontend.Variable (which is stored as *big.Int at runtime).
func toBI(v interface{}) *big.Int {
	return new(big.Int).Set(v.(*big.Int))
}

func nativeArk(t int, C []interface{}, r int, state []*big.Int) []*big.Int {
	out := make([]*big.Int, t)
	for i := 0; i < t; i++ {
		out[i] = new(big.Int).Add(state[i], toBI(C[r+i]))
		out[i].Mod(out[i], bn254p)
	}
	return out
}

func nativeSigma(x *big.Int) *big.Int {
	x2 := new(big.Int).Mul(x, x)
	x2.Mod(x2, bn254p)
	x4 := new(big.Int).Mul(x2, x2)
	x4.Mod(x4, bn254p)
	x5 := new(big.Int).Mul(x4, x)
	x5.Mod(x5, bn254p)
	return x5
}

func nativeMix(t int, M [][]interface{}, state []*big.Int) []*big.Int {
	out := make([]*big.Int, t)
	for i := 0; i < t; i++ {
		sum := new(big.Int)
		for j := 0; j < t; j++ {
			term := new(big.Int).Mul(toBI(M[j][i]), state[j])
			term.Mod(term, bn254p)
			sum.Add(sum, term)
			sum.Mod(sum, bn254p)
		}
		out[i] = sum
	}
	return out
}

func nativeMixLast(t int, M [][]interface{}, s int, state []*big.Int) *big.Int {
	sum := new(big.Int)
	for j := 0; j < t; j++ {
		term := new(big.Int).Mul(toBI(M[j][s]), state[j])
		term.Mod(term, bn254p)
		sum.Add(sum, term)
		sum.Mod(sum, bn254p)
	}
	return sum
}

func nativeMixS(t int, S []interface{}, r int, state []*big.Int) []*big.Int {
	out := make([]*big.Int, t)
	baseIndex := (t*2 - 1) * r
	sum := new(big.Int)
	for i := 0; i < t; i++ {
		term := new(big.Int).Mul(toBI(S[baseIndex+i]), state[i])
		term.Mod(term, bn254p)
		sum.Add(sum, term)
		sum.Mod(sum, bn254p)
	}
	out[0] = sum
	for i := 1; i < t; i++ {
		term := new(big.Int).Mul(state[0], toBI(S[baseIndex+t+i-1]))
		term.Mod(term, bn254p)
		out[i] = new(big.Int).Add(state[i], term)
		out[i].Mod(out[i], bn254p)
	}
	return out
}

// extractC converts []frontend.Variable (stored as *big.Int) to []interface{}.
func extractC(t int) []interface{} {
	vars := GetPoseidonC(t)
	out := make([]interface{}, len(vars))
	for i, v := range vars {
		out[i] = v.(*big.Int)
	}
	return out
}

func extractM(t int) [][]interface{} {
	vars := GetPoseidonM(t)
	out := make([][]interface{}, len(vars))
	for j, row := range vars {
		out[j] = make([]interface{}, len(row))
		for i, v := range row {
			out[j][i] = v.(*big.Int)
		}
	}
	return out
}

func extractS(t int) []interface{} {
	vars := GetPoseidonS(t)
	out := make([]interface{}, len(vars))
	for i, v := range vars {
		out[i] = v.(*big.Int)
	}
	return out
}

func extractP(t int) [][]interface{} {
	vars := GetPoseidonP(t)
	out := make([][]interface{}, len(vars))
	for j, row := range vars {
		out[j] = make([]interface{}, len(row))
		for i, v := range row {
			out[j][i] = v.(*big.Int)
		}
	}
	return out
}

// NativePoseidonEx computes PoseidonEx natively (no circuit constraints).
// Equivalent to pos.PoseidonEx(api, inputs, initialState, nOuts) in circuit code.
func NativePoseidonEx(inputs []*big.Int, initialState *big.Int, nOuts int) []*big.Int {
	t := len(inputs) + 1
	nRoundsF := 8
	nRoundsP := N_ROUNDS_P[t-2]

	C := extractC(t)
	S := extractS(t)
	M := extractM(t)
	P := extractP(t)

	// Build initial state vector: [initialState, inputs...]
	state := make([]*big.Int, t)
	state[0] = new(big.Int).Set(initialState)
	for j := 1; j < t; j++ {
		state[j] = new(big.Int).Set(inputs[j-1])
	}

	arkStates := make([][]*big.Int, nRoundsF+1)
	sigmaF := make([][]*big.Int, nRoundsF)
	mixStates := make([][]*big.Int, nRoundsF)

	// Initial ARK
	arkStates[0] = nativeArk(t, C, 0, state)

	// First half full rounds
	for r := 0; r < nRoundsF/2-1; r++ {
		var current []*big.Int
		if r == 0 {
			current = arkStates[0]
		} else {
			current = mixStates[r-1]
		}
		sigmaF[r] = make([]*big.Int, t)
		for j := 0; j < t; j++ {
			sigmaF[r][j] = nativeSigma(current[j])
		}
		arkStates[r+1] = nativeArk(t, C, (r+1)*t, sigmaF[r])
		mixStates[r] = nativeMix(t, M, arkStates[r+1])
	}

	// Last round of first half
	r0 := nRoundsF/2 - 1
	sigmaF[r0] = make([]*big.Int, t)
	if r0 == 0 {
		for j := 0; j < t; j++ {
			sigmaF[r0][j] = nativeSigma(arkStates[0][j])
		}
	} else {
		for j := 0; j < t; j++ {
			sigmaF[r0][j] = nativeSigma(mixStates[r0-1][j])
		}
	}
	arkStates[nRoundsF/2] = nativeArk(t, C, (nRoundsF/2)*t, sigmaF[r0])
	mixStates[nRoundsF/2-1] = nativeMix(t, P, arkStates[nRoundsF/2])

	// Partial rounds
	sigmaP := make([]*big.Int, nRoundsP)
	mixSStates := make([][]*big.Int, nRoundsP)
	for r := 0; r < nRoundsP; r++ {
		if r == 0 {
			sigmaP[r] = nativeSigma(mixStates[nRoundsF/2-1][0])
		} else {
			sigmaP[r] = nativeSigma(mixSStates[r-1][0])
		}
		temp := make([]*big.Int, t)
		cIdx := (nRoundsF/2+1)*t + r
		temp[0] = new(big.Int).Add(sigmaP[r], toBI(C[cIdx]))
		temp[0].Mod(temp[0], bn254p)
		for j := 1; j < t; j++ {
			if r == 0 {
				temp[j] = new(big.Int).Set(mixStates[nRoundsF/2-1][j])
			} else {
				temp[j] = new(big.Int).Set(mixSStates[r-1][j])
			}
		}
		mixSStates[r] = nativeMixS(t, S, r, temp)
	}

	// Second half full rounds
	for r := 0; r < nRoundsF/2-1; r++ {
		sigmaF[nRoundsF/2+r] = make([]*big.Int, t)
		if r == 0 {
			for j := 0; j < t; j++ {
				sigmaF[nRoundsF/2+r][j] = nativeSigma(mixSStates[nRoundsP-1][j])
			}
		} else {
			for j := 0; j < t; j++ {
				sigmaF[nRoundsF/2+r][j] = nativeSigma(mixStates[nRoundsF/2+r-1][j])
			}
		}
		arkStates[nRoundsF/2+r+1] = nativeArk(t, C, (nRoundsF/2+1)*t+nRoundsP+r*t, sigmaF[nRoundsF/2+r])
		mixStates[nRoundsF/2+r] = nativeMix(t, M, arkStates[nRoundsF/2+r+1])
	}

	// Final full round
	sigmaF[nRoundsF-1] = make([]*big.Int, t)
	for j := 0; j < t; j++ {
		sigmaF[nRoundsF-1][j] = nativeSigma(mixStates[nRoundsF-2][j])
	}

	out := make([]*big.Int, nOuts)
	for i := 0; i < nOuts; i++ {
		out[i] = nativeMixLast(t, M, i, sigmaF[nRoundsF-1])
	}
	return out
}

// NativePoseidonDecrypt decrypts ciphertext produced by NativePoseidonEncrypt.
// key is [x, y] of the BabyJubJub shared encryption key.
// nonce must be < 2^128.
// realLength is the number of actual plaintext values expected.
// encrypted must have exactly ceil(realLength/3)*3 + 1 elements (cipher + MAC).
// Returns an error if the MAC does not match (ciphertext tampered or wrong key/nonce).
func NativePoseidonDecrypt(key [2]*big.Int, nonce *big.Int, realLength int, encrypted []*big.Int) ([]*big.Int, error) {
	two128 := new(big.Int).Lsh(big.NewInt(1), 128)

	decryptedLength := realLength
	for decryptedLength%3 != 0 {
		decryptedLength++
	}
	n := (decryptedLength + 1) / 3

	// Initial hash: same as encrypt
	nonceEncoded := new(big.Int).Add(nonce, new(big.Int).Mul(two128, big.NewInt(int64(realLength))))
	nonceEncoded.Mod(nonceEncoded, bn254p)
	inputs0 := []*big.Int{
		new(big.Int).Set(key[0]),
		new(big.Int).Set(key[1]),
		nonceEncoded,
	}
	strategyOuts := make([][]*big.Int, n+1)
	strategyOuts[0] = NativePoseidonEx(inputs0, big.NewInt(0), 4)

	decrypted := make([]*big.Int, decryptedLength)
	for i := 0; i < n; i++ {
		// Decrypt: plain = cipher - strategyOut (mod p)
		for j := 0; j < 3; j++ {
			idx := i*3 + j
			if idx < decryptedLength {
				d := new(big.Int).Sub(encrypted[idx], strategyOuts[i][j+1])
				d.Mod(d, bn254p)
				decrypted[idx] = d
			}
		}
		// Next block uses ciphertext as input (same as encrypt)
		inputsNext := make([]*big.Int, 3)
		for j := 0; j < 3; j++ {
			idx := i*3 + j
			if idx < decryptedLength {
				inputsNext[j] = new(big.Int).Set(encrypted[idx])
			} else {
				inputsNext[j] = big.NewInt(0)
			}
		}
		strategyOuts[i+1] = NativePoseidonEx(inputsNext, strategyOuts[i][0], 4)
	}

	// MAC check: encrypted[decryptedLength] must equal strategyOuts[n][1]
	mac := strategyOuts[n][1]
	if encrypted[decryptedLength].Cmp(mac) != 0 {
		return nil, fmt.Errorf("poseidon decrypt: MAC mismatch (wrong key, nonce, or tampered ciphertext)")
	}

	// Return only the realLength meaningful values
	return decrypted[:realLength], nil
}

// NativePoseidonEncrypt encrypts plaintext using the Poseidon sponge cipher.
// key is [x, y] of the BabyJubJub shared encryption key (authKey = pubKey * random).
// nonce must be < 2^128.
// realLength is the number of actual plaintext values.
// plaintext must have exactly realLength elements.
// Returns [cipher..., mac] with length ceil(realLength/3)*3 + 1.
func NativePoseidonEncrypt(key [2]*big.Int, nonce *big.Int, realLength int, plaintext []*big.Int) []*big.Int {
	two128 := new(big.Int).Lsh(big.NewInt(1), 128)

	decryptedLength := realLength
	for decryptedLength%3 != 0 {
		decryptedLength++
	}

	// Pad plaintext
	plain := make([]*big.Int, decryptedLength)
	for i := 0; i < realLength; i++ {
		plain[i] = new(big.Int).Set(plaintext[i])
	}
	for i := realLength; i < decryptedLength; i++ {
		plain[i] = big.NewInt(0)
	}

	// n = (decryptedLength + 1) / 3  (integer division, same as circuit)
	n := (decryptedLength + 1) / 3

	// Initial hash: inputs = [key[0], key[1], nonce + 2^128 * realLength]
	nonceEncoded := new(big.Int).Add(nonce, new(big.Int).Mul(two128, big.NewInt(int64(realLength))))
	nonceEncoded.Mod(nonceEncoded, bn254p)
	inputs0 := []*big.Int{
		new(big.Int).Set(key[0]),
		new(big.Int).Set(key[1]),
		nonceEncoded,
	}
	strategyOuts := make([][]*big.Int, n+1)
	strategyOuts[0] = NativePoseidonEx(inputs0, big.NewInt(0), 4)

	cipher := make([]*big.Int, decryptedLength)
	for i := 0; i < n; i++ {
		for j := 0; j < 3; j++ {
			idx := i*3 + j
			if idx < decryptedLength {
				cipher[idx] = new(big.Int).Add(plain[idx], strategyOuts[i][j+1])
				cipher[idx].Mod(cipher[idx], bn254p)
			}
		}
		// Prepare next inputs (ciphertext of this block)
		inputsNext := make([]*big.Int, 3)
		for j := 0; j < 3; j++ {
			idx := i*3 + j
			if idx < decryptedLength {
				inputsNext[j] = new(big.Int).Set(cipher[idx])
			} else {
				inputsNext[j] = big.NewInt(0)
			}
		}
		strategyOuts[i+1] = NativePoseidonEx(inputsNext, strategyOuts[i][0], 4)
	}

	// Build result: [cipher[0..decryptedLength-1], mac]
	mac := new(big.Int).Set(strategyOuts[n][1])
	result := make([]*big.Int, decryptedLength+1)
	copy(result, cipher)
	result[decryptedLength] = mac
	return result
}
