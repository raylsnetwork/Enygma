package poseidon

import (
	"github.com/consensys/gnark/frontend"
)

var N_ROUNDS_P = []int{56, 57, 56, 60, 60, 63, 64, 63, 60, 66, 60, 65, 70, 60, 64, 68}


func sigma(api frontend.API, in frontend.Variable) frontend.Variable {
	in2 := api.Mul(in, in)
	in4 := api.Mul(in2, in2)
	return api.Mul(in4, in)
}

func ark(api frontend.API, t int, C []frontend.Variable, r int, in []frontend.Variable) []frontend.Variable {
	out := make([]frontend.Variable, t)
	for i := 0; i < t; i++ {
		out[i] = api.Add(in[i], C[r+i])
	}
	return out
}


func mix(api frontend.API, t int, M [][]frontend.Variable, in []frontend.Variable) []frontend.Variable {
	out := make([]frontend.Variable, t)
	for i := 0; i < t; i++ {
		sum := frontend.Variable(0)
		for j := 0; j < t; j++ {
			sum = api.Add(sum, api.Mul(M[j][i], in[j]))
		}
		out[i] = sum
	}
	return out
}

// MixLast computes a single output as a specific linear combination:
// out = Σ₍ⱼ₌0₎^(t-1) M[j][s] * in[j].
func mixLast(api frontend.API, t int, M [][]frontend.Variable, s int, in []frontend.Variable) frontend.Variable {
	sum := frontend.Variable(0)
	for j := 0; j < t; j++ {
		sum = api.Add(sum, api.Mul(M[j][s], in[j]))
	}
	return sum
}


func mixS(api frontend.API, t int, S []frontend.Variable, r int, in []frontend.Variable) []frontend.Variable {
	out := make([]frontend.Variable, t)
	baseIndex := (t*2 - 1) * r
	sum := frontend.Variable(0)
	for i := 0; i < t; i++ {
		sum = api.Add(sum, api.Mul(S[baseIndex+i], in[i]))
	}
	out[0] = sum
	for i := 1; i < t; i++ {
		out[i] = api.Add(in[i], api.Mul(in[0], S[baseIndex+t+i-1]))
	}
	return out
}

// --- Main Poseidon Functions ---


func PoseidonEx(api frontend.API, inputs []frontend.Variable, initialState frontend.Variable, nOuts int) []frontend.Variable {
	
	t := len(inputs) + 1 // state length = number of inputs + 1
	nRoundsF := 8
	nRoundsP := N_ROUNDS_P[t-2]
	
	C := GetPoseidonC(t)
	S := GetPoseidonS(t)
	M := GetPoseidonM(t)
	P := GetPoseidonP(t)

	// --- Round 0: Ark with initial state ---
	ark0 := make([]frontend.Variable, t)
	ark0[0] = initialState
	for j := 1; j < t; j++ {
		ark0[j] = inputs[j-1]
	}
	arkStates := make([][]frontend.Variable, nRoundsF+1)
	sigmaF := make([][]frontend.Variable, nRoundsF)
	mixStates := make([][]frontend.Variable, nRoundsF)
	arkStates[0] = ark(api, t, C, 0, ark0) // ark[0]

	// --- First half full rounds ---
	for r := 0; r < nRoundsF/2-1; r++ {
		var currentInput []frontend.Variable
		if r == 0 {
			currentInput = arkStates[0]
		} else {
			currentInput = mixStates[r-1]
		}
		sigmaF[r] = make([]frontend.Variable, t)
		for j := 0; j < t; j++ {
			sigmaF[r][j] = sigma(api, currentInput[j])
		}
		arkStates[r+1] = ark(api, t, C, (r+1)*t, sigmaF[r])
		mixStates[r] = mix(api, t, M, arkStates[r+1])
	}
	// r = nRoundsF/2 - 1 (last full round of first half)
	r0 := nRoundsF/2 - 1
	sigmaF[r0] = make([]frontend.Variable, t)
	if r0 == 0 {
		for j := 0; j < t; j++ {
			sigmaF[r0][j] = sigma(api, arkStates[0][j])
		}
	} else {
		for j := 0; j < t; j++ {
			sigmaF[r0][j] = sigma(api, mixStates[r0-1][j])
		}
	}
	arkStates[nRoundsF/2] = ark(api, t, C, (nRoundsF/2)*t, sigmaF[r0])
	mixStates[nRoundsF/2-1] = mix(api, t, P, arkStates[nRoundsF/2])

	// --- Partial rounds ---
	sigmaP := make([]frontend.Variable, nRoundsP)
	mixSStates := make([][]frontend.Variable, nRoundsP)
	for r := 0; r < nRoundsP; r++ {
		if r == 0 {
			sigmaP[r] = sigma(api, mixStates[nRoundsF/2-1][0])
		} else {
			sigmaP[r] = sigma(api, mixSStates[r-1][0])
		}
		temp := make([]frontend.Variable, t)
		// temp[0] = sigmaP[r] + C[(nRoundsF/2+1)*t + r]
		temp[0] = api.Add(sigmaP[r], C[(nRoundsF/2+1)*t+r])
		for j := 1; j < t; j++ {
			if r == 0 {
				temp[j] = mixStates[nRoundsF/2-1][j]
			} else {
				temp[j] = mixSStates[r-1][j]
			}
		}
		mixSStates[r] = mixS(api, t, S, r, temp)
	}

	// --- Second half full rounds ---
	for r := 0; r < nRoundsF/2-1; r++ {
		sigmaF[nRoundsF/2+r] = make([]frontend.Variable, t)
		if r == 0 {
			for j := 0; j < t; j++ {
				sigmaF[nRoundsF/2+r][j] = sigma(api, mixSStates[nRoundsP-1][j])
			}
		} else {
			for j := 0; j < t; j++ {
				sigmaF[nRoundsF/2+r][j] = sigma(api, mixStates[nRoundsF/2+r-1][j])
			}
		}
		arkStates[nRoundsF/2+r+1] = ark(api, t, C, (nRoundsF/2+1)*t+nRoundsP+r*t, sigmaF[nRoundsF/2+r])
		mixStates[nRoundsF/2+r] = mix(api, t, M, arkStates[nRoundsF/2+r+1])
	}
	// Final full round
	sigmaF[nRoundsF-1] = make([]frontend.Variable, t)
	for j := 0; j < t; j++ {
		sigmaF[nRoundsF-1][j] = sigma(api, mixStates[nRoundsF-2][j])
	}
	// --- Squeeze outputs via MixLast ---
	out := make([]frontend.Variable, nOuts)
	for i := 0; i < nOuts; i++ {
		out[i] = mixLast(api, t, M, i, sigmaF[nRoundsF-1])
	}
	return out
}

// Poseidon is a simple wrapper that hashes a variable–length input (as bits or field elements)
// and returns a single field element. It uses initialState = 0.
func Poseidon(api frontend.API, inputs []frontend.Variable) frontend.Variable {
	out := PoseidonEx(api, inputs, 0, 1)
	return out[0]
}