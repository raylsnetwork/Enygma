
from ecdsa import SECP256k1, SigningKey
from Crypto.Hash import keccak
import hashlib
from pycoin.ecdsa.secp256k1 import secp256k1_generator
import secrets


def solidity_sha3(value):
    k = keccak.new(digest_bits=256)
    k.update(value.to_bytes(32, byteorder='big'))  # Assuming input is an integer
    return k.digest()

private_key = SigningKey.generate(curve=SECP256k1)
public_key = private_key.get_verifying_key()

privKey = secrets.randbelow(secp256k1_generator.order())
pubKey = (secp256k1_generator * privKey)
print(privKey)
print(pubKey)

hash0 = solidity_sha3(0)
privKey = int.from_bytes(hash0, "big")   % secp256k1_generator.order()
print(privKey)
pubKey = (secp256k1_generator * privKey)
print(pubKey)

