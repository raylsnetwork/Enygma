pragma circom 2.0.0;
include "../circomlib/circuits/poseidon.circom";

// Gets contractAddress and id/amount as inputs 
// and returns Hash(contractAddress, idOrAmount) as output uniqueId

template UniqueId(){
    signal input contractAddress;
    signal input amount;
    signal output out;
    component hasher = Poseidon(2);
    hasher.inputs[0] <== contractAddress;
    hasher.inputs[1] <== amount;
    out <== hasher.out;
}
