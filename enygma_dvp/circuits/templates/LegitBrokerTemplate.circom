pragma circom 2.0.0;

include "../primitives/Blinder.circom";
include "../primitives/PublicKey.circom";

template LegitBrokerTemplate() {

    // Statement
    signal input st_beacon; 
    signal input st_blindedPublicKey;

    // Witness
    signal input wt_privateKey;

    component cp_publicKey;
    cp_publicKey = PublicKey();
    cp_publicKey.privateKey <== wt_privateKey;

    component cp_blinder;
    cp_blinder = Blinder();
    cp_blinder.in <== cp_publicKey.out;

    cp_blinder.out === st_blindedPublicKey;
}
