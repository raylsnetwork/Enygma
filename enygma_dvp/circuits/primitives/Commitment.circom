pragma circom 2.0.0;
include "../circomlib/circuits/poseidon.circom";

template Commitment(){
  signal input uniqueId;
  signal input publicKey;
  signal output out;

  component hasher = Poseidon(2);
  hasher.inputs[0] <== uniqueId;
  hasher.inputs[1] <== publicKey;
  out <== hasher.out;
}