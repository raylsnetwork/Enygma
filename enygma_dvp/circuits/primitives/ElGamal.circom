pragma circom 2.0.0;

include "../circomlib/circuits/babyjub.circom";
include "../circomlib/circuits/escalarmulany.circom";
// ElGamal encryption over BabyJubJub curve while preserving the additively homomorphic property.
// The scheme maps a scalar to a point on the curve and then adds it to the public key point. It outputs the two points of the resulting ciphertext (c1, c2).
template ElGamalEncrypt() {
    signal input random;
    signal input pk[2];
    signal input msg[2];
    signal output encryptedC1X;
    signal output encryptedC1Y;
    signal output encryptedC2X;
    signal output encryptedC2Y;

    component cp_checkPkOnCurve = BabyCheck();
    cp_checkPkOnCurve.x <== pk[0];
    cp_checkPkOnCurve.y <== pk[1];

    component cp_checkMessageOnCurve = BabyCheck();
    cp_checkMessageOnCurve.x <== msg[0];
    cp_checkMessageOnCurve.y <== msg[1];

    component cp_bitifyRandom = Num2Bits(253);
    cp_bitifyRandom.in <== random;

    component cp_mulG = BabyPbk();
    cp_mulG.in <== random;

    component cp_mulAny = EscalarMulAny(253);
    for (var i = 0; i < 253; i ++) {
        cp_mulAny.e[i] <== cp_bitifyRandom.out[i];
    }
    cp_mulAny.p[0] <== pk[0];
    cp_mulAny.p[1] <== pk[1];
    
    component cp_add = BabyAdd();
    cp_add.x1 <== msg[0];
    cp_add.y1 <== msg[1];
    cp_add.x2 <== cp_mulAny.out[0];
    cp_add.y2 <== cp_mulAny.out[1];

    encryptedC1X <== cp_mulG.Ax;
    encryptedC1Y <== cp_mulG.Ay;
    encryptedC2X <== cp_add.xout;
    encryptedC2Y <== cp_add.yout;

}

// ElGamal Decryption scheme over BabyJub curve while preserving the additively homomorphic property.
// The scheme takes the two points of the ciphertext (c1, c2) and the private key and outputs the message, mapped to a point.

template ElGamalDecrypt() {
    signal input c1[2];
    signal input c2[2];
    signal input privKey;
    signal output outx;
    signal output outy;

    component cp_checkC1OnCurve = BabyCheck();
    cp_checkC1OnCurve.x <== c1[0];
    cp_checkC1OnCurve.y <== c1[1];

    component cp_checkC2OnCurve = BabyCheck();
    cp_checkC2OnCurve.x <== c2[0];
    cp_checkC2OnCurve.y <== c2[1];

    // Convert the private key to bits
    component cp_bitifyPrivateKey = Num2Bits(253);
    cp_bitifyPrivateKey.in <== privKey;

    // c1 ** x
    component cp_mulAny_c1x = EscalarMulAny(253);
    for (var i = 0; i < 253; i ++) {
        cp_mulAny_c1x.e[i] <== cp_bitifyPrivateKey.out[i];
    }
    cp_mulAny_c1x.p[0] <== c1[0];
    cp_mulAny_c1x.p[1] <== c1[1];

    // (c1 * x) * -1
    signal cp_inverse_c1x;
    cp_inverse_c1x <== 0 - cp_mulAny_c1x.out[0];

    // ((c1 * x) * - 1) * c2
    component cp_add = BabyAdd();
    cp_add.x1 <== cp_inverse_c1x;
    cp_add.y1 <== cp_mulAny_c1x.out[1];
    cp_add.x2 <== c2[0];
    cp_add.y2 <== c2[1];

    outx <== cp_add.xout;
    outy <== cp_add.yout;
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