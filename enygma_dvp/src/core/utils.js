const crypto = require("crypto");
const { poseidon } = require("circomlibjs");
const ethers = require("ethers");
const { randomBytes } = require("node:crypto");

const { PublicKey, PrivateKey, Signature, utils, ffUtils, Hex } = require("@iden3/js-crypto");

const {
  formatPrivKeyForBabyJub,
  genRandomBabyJubValue,
  poseidonDecrypt,
  poseidonEncrypt,
} = require("maci-crypto");

const BASE_POINT_ORDER = BigInt(
  "2736030358979909402780800718157159386076813972158567259200215660948447373041");

const SNARK_SCALAR_FIELD = BigInt(
  "21888242871839275222246405745257275088548364400416034343698204186575808495617"
);

function writeToJson(filePath, data) {
  var fs = require("fs");
  fs.writeFile(filePath, JSON.stringify(data, null, 4), function (err) {
    if (err) {
      console.log(err);
    }
  });
}

function buffer2BigInt(buf) {
  if (buf.length === 0) {
    return 0n;
  }
  return BigInt(`0x${buf.toString("hex")}`);
}

function randomInField(){
  return buffer2BigInt(Buffer.from(crypto.randomBytes(32))) % SNARK_SCALAR_FIELD;
}

function newKeyPair() {
  const privateKey = poseidon([
    buffer2BigInt(Buffer.from(crypto.randomBytes(32))),
  ]);
  const publicKey = poseidon([privateKey]);
  return {
    privateKey,
    publicKey,
  };
}


function getNullifier(privateKey, pathIndices) {
  return poseidon([privateKey, pathIndices]);
}

function getCommitment(uniqueId, publicKey) {
  return poseidon([uniqueId, publicKey]);
}


function getPublicKey(privateKey) {
  return poseidon([privateKey]);
}
function getAuctionId(commitment) {
  return poseidon([commitment]);
}

function blindedPublicKey(publicKey){
  return poseidon([publicKey]);
}

// Helper to encode an integer as a point (toy example)
function encodeMessage(babyjub, m) {
    // Encode small integer as m*Base8
    return babyjub.mulPointEScalar(babyjub.Base8, BigInt(m));
}

// Helper to decode point back to integer
function decodeMessage(babyjub, point, maxM) {
    // Brute-force search up to maxM to recover m
    console.log("Allowed range: ",maxM);
    for (let m = 0n; m < maxM; m++) {
        const candidate = encodeMessage(babyjub, m);
        if (babyjub.F.eq(candidate[0], point[0]) && babyjub.F.eq(candidate[1], point[1])) {
            return m;
        }
    }
    throw new Error("Message not found");
}

function babyEncrypt(babyjub, m, r, pubKey){
    // c1 = r * G
    const c1 = babyjub.mulPointEScalar(babyjub.Base8, r);
    // rPub = r * publicKey
    const rPub = babyjub.mulPointEScalar(pubKey, r);
    // mG = m * G
    const mG = encodeMessage(babyjub, m);
    // console.log("mG: ", mG);
    
    // c2 = mG + rPub = (m+rx)G 
    const c2 = babyjub.addPoint(mG, rPub);

    return [c1, c2];
}

function babyDecrypt(babyjub, c1, c2, privateKey, allowed_range=1000n){
    const rPub = babyjub.mulPointEScalar(c1, privateKey);

    const M_dec = babyjub.addPoint(c2, [babyjub.p - rPub[0], rPub[1]]);
    // console.log("mG2: ", M_dec);


    try{
      const decrypted = decodeMessage(babyjub, M_dec, allowed_range);
      return decrypted;
    }
    catch(ex){
      console.log("Invalid value");
      return null;
    }

}
function randomNonce() {
  const bytes = randomBytes(16);
  // add 1 to make sure it's non-zero
  return BigInt(`0x${bytes.toString("hex")}`) + 1n;
}

function poseidonEncryptWrapper(babyjub, inputs,publicKey) {
  const nonce = randomNonce();
  
  const randomValue = randomInField() / 10n;
  const authKey = babyjub.mulPointEScalar(babyjub.Base8, randomValue);
  const sharedKey = babyjub.mulPointEScalar(publicKey, randomValue);

  const encrypted = poseidonEncrypt(inputs, sharedKey, nonce);

  return { encrypted, nonce, randomValue, sharedKey, authKey };
}

function poseidonDecryptWrapper(babyjub, encrypted, authKey, nonce, privateKey, length){
  const sharedKey = babyjub.mulPointEScalar(authKey, privateKey);

  const decrypted = poseidonDecrypt(encrypted, sharedKey, nonce, length);

  return decrypted.slice(0, length);
}

function babyKeyPair(babyjub){
    // Private key
    const privateKey = randomInField() % babyjub.subOrder;

    // Public key
    const pubKey = babyjub.mulPointEScalar(babyjub.Base8, privateKey);

    return {"privateKey": privateKey, "publicKey": pubKey};
}

function erc20UniqueId(erc20ContractAddress, amount) {
  return poseidon([BigInt(erc20ContractAddress), amount])
}


function erc721UniqueId(erc721ContractAddress, erc721TokenId) {
  return poseidon([BigInt(erc721ContractAddress), erc721TokenId])
}


function erc1155UniqueId(erc1155ContractAddress, tokenId, amount) {
  const uid1 = poseidon([BigInt(erc1155ContractAddress), tokenId]);
  return poseidon([uid1, amount])
}


function pedersen(amount, random) {
  return poseidon([amount, random]);
}


function keccak256(preimage) {
  return (
    buffer2BigInt(
      Buffer.from(ethers.utils.keccak256(preimage).slice(2), "hex"),
    ) % SNARK_SCALAR_FIELD
  );
}

function stringifyBigInts(o) {
  if (typeof o === "bigint" || o.eq !== undefined) {
    return o.toString(10);
  }
  if (Array.isArray(o)) {
    return o.map(stringifyBigInts);
  }
  if (typeof o === "object") {
    const res = {};
    const keys = Object.keys(o);
    keys.forEach((k) => {
      res[k] = stringifyBigInts(o[k]);
    });
    return res;
  }
  return o;
}
module.exports = {
  SNARK_SCALAR_FIELD,
  stringifyBigInts,
  randomInField,
  newKeyPair,
  getCommitment,
  getNullifier,
  keccak256,
  buffer2BigInt,
  writeToJson,
  poseidon,
  erc20UniqueId,
  erc721UniqueId,
  erc1155UniqueId,
  getAuctionId,
  blindedPublicKey,
  getPublicKey,
  pedersen,
  babyEncrypt,
  babyDecrypt,
  babyKeyPair,
  poseidonEncryptWrapper,
  poseidonDecryptWrapper,
};
