pragma circom 2.0.0;
include "../circomlib/circuits/poseidon.circom";

// Takes privateKey and pathIndex as inputs 
// Outputs Hash(privateKey || pathIndex)

template Nullifier(){
  signal input privateKey;
  signal input pathIndex;
  signal output out;

  component hasher = Poseidon(2);
  hasher.inputs[0] <== privateKey;
  hasher.inputs[1] <== pathIndex;
  out <== hasher.out;
}