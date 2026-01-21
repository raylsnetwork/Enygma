pragma circom 2.0.0;
include "../circomlib/circuits/poseidon.circom";

template Erc1155UniqueId(){
    signal input erc1155ContractAddress;
    signal input erc1155TokenId;
    signal input amount;
    signal output out;
    component hasher1 = Poseidon(2);
    component hasher2 = Poseidon(2);
    hasher1.inputs[0] <== erc1155ContractAddress;
    hasher1.inputs[1] <== erc1155TokenId;
    hasher2.inputs[0] <== hasher1.out;
    hasher2.inputs[1] <== amount;
    out <== hasher2.out;
}
