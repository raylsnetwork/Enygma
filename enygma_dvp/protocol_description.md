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


Issuer:

* generates a random salt:
   * $$salt \longleftarrow \lbrace0, 1\rbrace^{\lambda}$$
