pragma circom 2.0.0;

include "../circomlib/circuits/babyjub.circom";
include "../circomlib/circuits/escalarmulany.circom";
include "./PoseidonDecrypt.circom";


// Checks whether auditor has the desired access
template AuditorAccess(tm_plainLength) {

    var decLength = tm_plainLength;
    while(decLength  % 3 != 0) {
        decLength += 1;
    }
    var encLength = decLength + 1;

    // Auditor's publicKey
    signal input st_publicKey[2];
    // Encrypted Data by Auditor's publicKey
    signal input st_encryptedValues[ encLength];

    // the plain value that is being encrypted.
    signal input st_nonce;

    // The random value that has been used for encryption
    signal input wt_random;
    // the plain value that is being encrypted.
    signal input wt_values[tm_plainLength];

    // prime subGroup order:
    var baseOrder = 2736030358979909402780800718157159386076813972158567259200215660948447373041;

    // Checking publicKey to be on the curve
    component cp_isOnCurve1 = BabyCheck();
    cp_isOnCurve1.x <== st_publicKey[0];
    cp_isOnCurve1.y <== st_publicKey[1];

    // Checking randomValue to be in range of baseOrder
    for(var j = 0; j< tm_plainLength; j++) {
        assert(0 <= wt_values[j]);
        assert(wt_values[j] < baseOrder);
    }

    assert(0 < wt_random);
    assert(wt_random < baseOrder);

    component cp_checkEncKey = BabyScalarMul();
    cp_checkEncKey.scalar <== wt_random;
    cp_checkEncKey.point[0] <== st_publicKey[0];
    cp_checkEncKey.point[1] <== st_publicKey[1];

    // Verifying that st_encryptedValue == Enc(value)
    component cp_poseidonDecrypt = PoseidonDecrypt(tm_plainLength);
    cp_poseidonDecrypt.ciphertext <== st_encryptedValues;
    cp_poseidonDecrypt.key[0] <== cp_checkEncKey.Ax;
    cp_poseidonDecrypt.key[1] <== cp_checkEncKey.Ay;
    cp_poseidonDecrypt.nonce <== st_nonce;

    for(var j = 0; j< tm_plainLength; j++) {
        cp_poseidonDecrypt.decrypted[j] === wt_values[j];
    }


}
