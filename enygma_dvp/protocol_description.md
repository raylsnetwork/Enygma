# Protocol Description


## Notation


In Enygma DvP, the commitments have the following form:

$$C = Hash(pk^{spend} | salt | token_{ID} | amount)$$

To spend the commitment, the user proves in zero-knowledge that they know the secret spend key associated with this commitment, and publish a nullifier that spends the corresponding commitment. 

## 1 - System Setup
TBD

## 2 - Key Generation
Each privacy node generates two keypairs: one to spend funds, and one to 'view' transactions. Concretely: 

* Privacy node A generates an [ML-KEM](https://nvlpubs.nist.gov/nistpubs/FIPS/NIST.FIPS.203.pdf) (view) keypair and obtains $$(sk_{A}^{view}, pk_{A}^{view})$$

* Privacy node A generates a simple hash-based (spend) keypair and obtains $$(sk_{A}^{spend}, pk_{A}^{spend})$$.
  *  $$sk_{A}^{spend} \longleftarrow \\\{{0, 1\\\}}^{256}$$
  *  $$pk_{A}^{spend} = Hash(sk_{A}^{spend})$$
 
The goal here is to have segregation of functionalities with each keypair. 

* To spend, the user proves in zero-knowledge that they know a secret key $$sk^{spend}$$ corresponding to one public key $$pk^{spend}$$ in an anonymity set of size $$k$$. We note that the hashing used in this step is ZK-friendly (i.e., Poseidon).
* The view key pair is used to decrypt the values that are inserted into the received commitments. 

## Private Issuance

To spend the funds, the recipient must be able to open the commitment. Concretely, the user must know the spend key pair, the salt, the token ID, and the amount. Therefore, we require a mechanism that allows the issuer to share the salt with the recipient. An initial approach could have the recipient communicate with the issuer in advance and send the already-formed commitment. The issuer, then simply performs a mint directly to that commitment. This is possible, but not elegant and makes the payment extremely interactive. A user should simply be able to privately send funds to the other user right away. 

Issuer:

* generates a random salt:
   * $$salt \longleftarrow \lbrace0, 1\rbrace^{\lambda}$$
