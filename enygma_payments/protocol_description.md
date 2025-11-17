# Protocol Description


## Notation

* Balances are represented as Pedersen commitments:
  * $$Comm(v, r) = vG + rH$$
  * Note: We use a nothing-up-my-sleeve approach to obtaining the generator $$H$$. To do so, we hash-to-curve the number $$0$$. 

* Each privacy node has two keypairs. One for viewing transactions (i.e., ML-KEM), other for spending (i.e., hash-based). Both are quantum-secure: 
  * $$(sk_{A}^{view}, pk_{A}^{view})$$
  * $$(sk_{A}^{spend}, pk_{A}^{spend})$$

* Shared secret s is randomly generated and shared using a post-quantum key agreement (i.e., ML-KEM):
  * $$Encapsulate(pk', s')$$

* Block number:
  * $$n_{block}$$

* Random factor $$r$$ is derived from hashing a shared secret and a block number:
  * $$r = Hash(s, n_{block})$$ 

## System Setup
All participants start with a balance with $$v=0$$ and $$r=0$$. 

Therefore, the Issuer creates a contract where the initial balance for all the participants is:

$$Comm(0, 0) = 0G + 0H$$


## Key Generation
* privacy node A generates an ML-KEM pair and obtains $$(sk_{A}^{view}, pk_{A}^{view})$$

* privacy node A generates a simple hash-based keypair and obtains $$(sk_{A}^{view}, pk_{A}^{view})$$.
  *  $$sk_{A}^{view} \longleftarrow \\\{{0, 1\\\}}^{256}$$
  *  $$pk_{A}^{view} = Hash(sk_{A}^{view})$$
 
The goal here is to have segregation of functionalities with each keypair. To spend, the user proves in zero-knowledge that they know $$sk^{spend}$$ corresponding to a $$pk^{spend}$$ in the anonymity set. We note that the hashing used in this step is ZK-friendly (i.e., Poseidon). On the other hand, the view key pair is used to generate a shared secret, which is then subsequently used to derive random factors for every block and ephemeral symmetric encryption keys for symmetric encryption. 

## Key Agreement
Party downloads the counterparty's ML-KEM public-key $$pk_{i}'$$, generates a pre-secret $$s'$$ and encapsulates it using the downloaded public-key, thus obtaining $$Encapsulate(pk_{i}', s')$$. Sender calculates $$id = Hash(s')$$ and publishes both $$< i, id, Encapsulate(pk', s')>$$ on the underlying blockchain. 

Counterparty knows their index $i$ and detects that a new publishing took place. Party $i$ downlods the bundle $$< i, id, Encapsulate(pk', s')>$$. Upon download, the entity decapsulates the published payload, obtains $s'$, calculates $id' = Hash(s')$ and checks if the obtained $id'$ matches the published $id$. If so, the party $i$ publishes a sign-off message and attests that the $id$ posted initially is correct and is ready to receive private transactions. 

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

## Auditing
There are multiple types of auditing supported by the protocol. Concretely, the auditor can have a 'universal view' and have the ability of seeing all the transactions that take place in the network. 

#### View Key Sharing
If there is an auditor that needs the complete view of the transactions in the network, then each privacy node shares their view key pair with the auditor upon the key registration step. To do so, each privacy node encrypts their view secret key (i.e., $$sk_{A}^{view}$$) and publishes it on the blockchain for the auditor to fetch. 

For example, privacy node A publishes:

$$ ctxt = Encapsulate(pk_{audit}^{view}, sk_{A}^{view})$$

#### Ephemeral Symmetric (View) Key Sharing
Our system also supports the opening of individual transctions without compromising the secrecy of previous/future transactions. Since the system uses symmetric key encryption with ephemeral (per block) keys, we have a mode of operation where the sender or recipient can simply disclose individual symmetric keys and open individual transctions. 

The symmetric key $$k$$ for block $$n$$ (i.e, $$k_{n}$$) is obtained the following way: 

$$k_{n} = HKDF(s, n_{block})$$

#### Universal Auditing
Besides the auditor, we highlight that any entity in the system can always monitor the following parameters just from looking at the underlying blockchain: 

* Entity can check the ZK proof and attest that the set of commitments are well-formed and add up to 0.
* Entity can check how many mints/burns exist in the network.

Additionally, depending on the choice of random factors and the issuance process, it may be possible for any entity to check the total supply of the asset at all times. 

## Complexity Analysis

<div align="center">

| Protocol      | Complexity                                             | Additional Remarks                                                     |
|---------------|--------------------------------------------------------|------------------------------------------------------------------------|
| Key Agreement | $$O(n_{\text{banks}} - 1 )$$                           | Each privacy node establishes a key with all the other privacy nodes.  |
| Tx Size       | $$O(k (\|C\| + \|t\| + \|ctxt\|) + \|\pi \| + \|nf\|)$$| $$k$$ commitments, tags, ciphertexts, a zk proof, and a nullifier.     |


</div>


The total complexity of this step is: 

$$O(n_{banks} \times (n_{banks} - 1)$$

### Transaction decryption



### Transaction Decryption
$$O(n_{banks} \times n_{tx} )$$



