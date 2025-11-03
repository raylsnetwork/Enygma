# Protocol Description

## Notation

Balances are represented as Pedersen commitments. 
$$Comm(v, r) = vG + rH$$

## Transparent Issuance
- Issuer I mints a new Commitment with the random factor set to 0. Therefore

$$Comm(v, 0) = vG + 0H = vG$$

  This allows anyone to be able to see how much money was minted in the system. 
