package primitives 

import(
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/bits"
	pos "gnark_server/poseidon"
)

func MerkleProof(api frontend.API, leaf frontend.Variable,pathIndices frontend.Variable,pathElements []frontend.Variable)frontend.Variable{

	levels:= len(pathElements)
	idxBits := bits.ToBinary(api, pathIndices, bits.WithNbDigits(levels))

	currentHash := leaf
	 for i := 0; i < levels; i++ {
        bit := idxBits[i]
        sibling := pathElements[i]
        left := api.Select(bit, sibling, currentHash)
        right := api.Select(bit, currentHash, sibling)


        // Poseidon([outL, outR])
        hash := pos.Poseidon(api, []frontend.Variable{left,right})

        currentHash = hash
	}

	return currentHash

}

func MerkleProofNative(api frontend.API, leaf frontend.Variable,pathIndices frontend.Variable,pathElements []frontend.Variable)frontend.Variable{

	levels:= len(pathElements)
	idxBits := bits.ToBinary(api, pathIndices, bits.WithNbDigits(levels))
	
	currentHash := leaf
	 for i := 0; i < levels; i++ {

        bit := idxBits[i]
        sibling := pathElements[i]
		
        left := api.Select(bit, sibling, currentHash)
        right := api.Select(bit, currentHash, sibling)
	
		hash,_ := api.NewHint(PoseidonNative, 1, left,right)
		// api.Println("hashApi", hash, "i",i, "left", left, "right", right)
        currentHash = hash[0]
	}

	return currentHash

}