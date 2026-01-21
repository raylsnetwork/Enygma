pragma circom 2.0.0;

include "../circomlib/circuits/poseidon.circom";
include "../circomlib/circuits/babyjub.circom";
include "../circomlib/circuits/escalarmulany.circom";

template PoseidonDecrypt(tm_realLength) {


    var decryptedLength = tm_realLength;
    while (decryptedLength % 3 != 0) {
        decryptedLength += 1;
    }

    signal input ciphertext[decryptedLength + 1];
    signal input nonce;
    signal input key[2];
    signal output decrypted[decryptedLength];

    var two128 = 2 ** 128;

    component bitCheck1 = Num2Bits(252);
    bitCheck1.in <== nonce;

    component bitCheck2 = Num2Bits(252);
    bitCheck2.in <== two128;

    component lt = LessThan(252);
    lt.in[0] <== nonce;
    lt.in[1] <== two128;
    lt.out === 1;

    var n = (decryptedLength + 1) \ 3;

    component strategies[n + 1];
    strategies[0] = PoseidonEx(3, 4);
    strategies[0].initialState <== 0;
    strategies[0].inputs[0] <== key[0];
    strategies[0].inputs[1] <== key[1];
    strategies[0].inputs[2] <== nonce + (tm_realLength * two128);

    for (var i = 0; i < n; i ++) {
        for (var j = 0; j < 3; j ++) {
            decrypted[i * 3 + j] <== ciphertext[i * 3 + j] - strategies[i].out[j + 1];
        }

        strategies[i + 1] = PoseidonEx(3, 4);
        strategies[i + 1].initialState <== strategies[i].out[0];
        for (var j = 0; j < 3; j ++) {
            strategies[i + 1].inputs[j] <== ciphertext[i * 3 + j];
        }
    }

    // Check the last ciphertext element
    ciphertext[decryptedLength] === strategies[n].out[1];

    // If length > 3, check if the last (3 - (tm_size mod 3)) elements of the message
    // are 0
    if (tm_realLength % 3 > 0) {
        if (tm_realLength % 3 == 1) {
            decrypted[decryptedLength - 1] === 0;
        } else if (tm_realLength % 3 == 2) {
            decrypted[decryptedLength - 1] === 0;
            // TODO:: loosened for PoC release. fix it.
            decrypted[decryptedLength - 2] === 0;
        }
    }
}

template BabyScalarMul() {
    signal input  scalar;
    signal input point[2];
    signal output Ax;
    signal output Ay;

    component checkPoint = BabyCheck();
    checkPoint.x <== point[0];
    checkPoint.y <== point[1];

    component scalarBits = Num2Bits(253);
    scalarBits.in <== scalar;

    component mulAny = EscalarMulAny(253);
    mulAny.p[0] <== point[0];
    mulAny.p[1] <== point[1];

    var i;
    for (i=0; i<253; i++) {
        mulAny.e[i] <== scalarBits.out[i];
    }
    Ax  <== mulAny.out[0];
    Ay  <== mulAny.out[1];
}
