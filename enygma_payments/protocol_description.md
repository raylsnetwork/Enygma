# Protocol Description

## Notation

* Balances are represented as Pedersen commitments:
  * $$Comm(v, r) = vG + rH$$

* Random factors r are derived from hashing a shared secret and a block number:
  * $$r = Hash(s, n_{block})$$

* Shared secret s is obtained from a post-quantum key agreement (i.e., ML-KEM):
  * $$s = Encapsulate(pk', s')$$

* TBD

## System Setup
All participants start with a balance with $$v=0$$ and $$r=0$$. Therefore, the Issuer creates a contract where the initial balance for all the participants is:

$$Comm(0, 0) = 0G + 0H$$

### Key Agreement


### View Key Sharing

## Issuing Tokens
There are two ways of issuing tokens. The issuer can mint tokens in a transparent manner and everyone in the system can see the underlying amounts. Alternatively, the issuer can mint tokens that are shielded from the start. We describe both approaches below. 

### Transparent Issuance
Issuer creates a new Commitment with the random factor set to 0. Therefore: 

$$Comm(v, 0) = vG + 0H = vG$$

This allows anyone to be able to see how much money was minted in the system. 

### Private Issuance
Issuer acts as a system entity and establishes a shared secret with every participant in the network, and creates a new commitment with the random factor set accordingly. Concretely, $$r = Hash(s, n_{block})$$. This commitment gets added to the previous balance (of zero). Therefore, the initial balance after a private issuance is:

$$Comm(v, r) = vG + rH$$


This ensures only the issuer and the recipient know how much money was minted. We note, however, that it's still possible to have verifiability on the minting side, in the sense that every time there is a mint that the system knows a mint occcurred. 
