pragma circom 2.0.0;
include "../circomlib/circuits/poseidon.circom";

// Gets commitment as input 
// and returns Hash(commitment) as output auctionId

template AuctionId(){
  signal input commitment;
  signal output out;
  component hasher = Poseidon(1);
  hasher.inputs[0] <== commitment;
  out <== hasher.out;
}