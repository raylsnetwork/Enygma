pragma circom 2.0.0;
include "../circomlib/circuits/poseidon.circom";

template Pedersen(){
    signal input amount;
    signal input random;
    signal output out;
    component hasher = Poseidon(2);
    hasher.inputs[0] <== amount;
    hasher.inputs[1] <== random;
    out <== hasher.out;
}