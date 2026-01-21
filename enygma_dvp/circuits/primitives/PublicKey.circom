pragma circom 2.0.0;
include "../circomlib/circuits/poseidon.circom";

// Gets privateKey as input 
// and returns Hash(privateKey) as output publicKey

template PublicKey(){
  signal input privateKey;
  signal output out;
  component hasher = Poseidon(1);
  hasher.inputs[0] <== privateKey;
  out <== hasher.out;
}