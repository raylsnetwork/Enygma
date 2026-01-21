const { PublicKey, PrivateKey, Signature, utils, ffUtils, Hex, babyJub } = require("@iden3/js-crypto");
const { expect } = require("chai");
const jsUtils = require("../src/core/utils");
const myWeb3 = require("../src/web3");
const crypto = require("crypto")
const testHelpers = require("./testHelpers.js");

// Test has been copied from iden3/js-crypto repo

describe('eddsa keys(Private, Public, Signature)', () => {
  it('pub key hex compress & decompress', () => {
    const x = BigInt(
      '17777552123799933955779906779655732241715742912184938656739573121738514868268'
    );
    const y = BigInt(
      '2626589144620713026669568689430873010625803728049924121243784502389097019475'
    );
    const expectedHex = '53b81ed5bffe9545b54016234682e7b2f699bd42a5e9eae27ff4051bc698ce85';
    const pKey = new PublicKey([x, y]);

    const compressedPKey = pKey.compress();

    expect(Hex.encodeString(compressedPKey)).equal(expectedHex);
    expect(pKey.hex()).equal(expectedHex);

    const fromHexPKey = PublicKey.newFromHex(expectedHex);

    expect(fromHexPKey.p[0]).equal(x);
    expect(fromHexPKey.p[1]).equal(y);
  });

  it('test signature flow', () => {
    const msgBuf = Hex.decodeString('000102030405060708090000');
    const msg = ffUtils.leBuff2int(msgBuf);

    const skBuff = Hex.decodeString(
      '0001020304050607080900010203040506070809000102030405060708090001'
    );

    const privateKey = new PrivateKey(skBuff);

    const pubKey = privateKey.public();
    expect(pubKey.p[0].toString()).equal(
      '13277427435165878497778222415993513565335242147425444199013288855685581939618'
    );
    expect(pubKey.p[1].toString()).equal(
      '13622229784656158136036771217484571176836296686641868549125388198837476602820'
    );

    const signature = privateKey.signPoseidon(msg);

    expect(signature.R8[0].toString()).equal(
      '11384336176656855268977457483345535180380036354188103142384839473266348197733'
    );
    expect(signature.R8[1].toString()).equal(
      '15383486972088797283337779941324724402501462225528836549661220478783371668959'
    );
    expect(signature.S.toString()).equal(
      '1672775540645840396591609181675628451599263765380031905495115170613215233181'
    );

    const pSignature = signature.compress();
    expect(Hex.encodeString(pSignature)).equal(
      '' +
        'dfedb4315d3f2eb4de2d3c510d7a987dcab67089c8ace06308827bf5bcbe02a2' +
        '9d043ece562a8f82bfc0adb640c0107a7d3a27c1c7c1a6179a0da73de5c1b203'
    );
    expect(signature.hex()).equal(
      '' +
        'dfedb4315d3f2eb4de2d3c510d7a987dcab67089c8ace06308827bf5bcbe02a2' +
        '9d043ece562a8f82bfc0adb640c0107a7d3a27c1c7c1a6179a0da73de5c1b203'
    );

    const uSignature = Signature.newFromCompressed(pSignature);

    expect(pubKey.verifyPoseidon(msg, uSignature)).equal(true);
  });

  it('Private key toBigInt', () => {
    const skBuff = Uint8Array.from(new Array(32).fill(1).map((_, i) => i));

    const privateKey = new PrivateKey(skBuff);

    expect(privateKey.bigInt().toString()).equal(
      '3817885988578745122822765953778691808009834824977012551803821922027918401423'
    );
  });


  it('should encrypt and decrypt in BabyJubJub.', () => {

    // Private key
    const privateKey = 123n;

    // Public key
    const pubKey = babyJub.mulPointEScalar(babyJub.Base8, privateKey);

    const r = jsUtils.randomInField() % babyJub.subOrder;

    const m = 505n;


    console.log(" m = ", m);
    console.log(" r = ", r);

    const [c1, c2] = jsUtils.babyEncrypt(babyJub, m,r, pubKey);

    console.log("Should be able to decrypt the evalue that is in range")
    const m2 = jsUtils.babyDecrypt(babyJub, c1,c2, privateKey, 1000n);
    
    expect(m2).equal(m);

    console.log("Should throw exception and return null for out of range value")
    const m3 = jsUtils.babyDecrypt(babyJub, c1,c2, privateKey, 500n);

    expect(m3).equal(null);
  });
});

describe("Testing Poseidon Encrypt/Decrypt", () => {

    it('Proper Poiseidon Encryption/ Decryption', () => {

    const m = [505n, 12321312n, 1n,2347n,34n, 10000n];
    const messageLength = m.length;

    // Private key
    const privateKey = 123n;

    // Public key
    const pubKey = babyJub.mulPointEScalar(babyJub.Base8, privateKey);

    const encPack = jsUtils.poseidonEncryptWrapper(babyJub, m,pubKey);

    console.log(JSON.stringify
      (encPack, null, 8));

    const decrypted = jsUtils.poseidonDecryptWrapper(
      babyJub,
      encPack.encrypted, 
      encPack.authKey, 
      encPack.nonce, 
      privateKey,
      messageLength
      );


    console.log(JSON.stringify(decrypted, null, 4));

    for(var i = 0;i < messageLength; i++){
      expect(decrypted[i]).equal(m[i]);
    }


  });

});