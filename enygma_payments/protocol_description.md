# Protocol Description

## Notation

* Balances are represented as Pedersen commitments:
  * $$Comm(v, r) = vG + rH$$

* Random factors r are derived from hashing a shared secret and a block number:
  * $$r = Hash(s, n_{block})$$

* Shared secret s is obtained from a post-quantum key agreement (i.e., ML-KEM):
  * $$s = Encapsulate(pk', s')$$

* TBD

### Transparent Issuance
Issuer I mints a new Commitment with the random factor set to 0. Therefore: 

$$Comm(v, 0) = vG + 0H = vG$$

  This allows anyone to be able to see how much money was minted in the system. 

### Private Issuance
Issuer I acts as a system entity and establishes a shared secret with every participant in the network, and mints a new commitment with the random factor set accordingly. Therefore: 

$$Comm(v, 0) = v*G + Hash(s, n_{block})*H = vG$$
