# Protocol Description

In this document, we describe the Enygma payments protocol, which is comprised of different sub-protocols. Concretely, we have an initial **System Setup** where the system parameters are defined. Subsequently, we have a **Key Generation** step where parties generate keypairs. A **Key Registration** step where parties register the public keys on the underlying blockchain, and a **Key Agreement** step where parties run a key agreement protocol to establish pairwise shared secrets. Once completed, parties can start transacting privately. This transfer stage has a **Send**, **Process**, and **Retrieve** step. Finally, if required, the system supports an **Auditing** step. 

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    setup["System<br>Setup"]
    keygen["Key<br>Generation"]
    registration["Key<br>Registration"]
    agreement["Key<br>Agreement"]
    issuance["Issuance<br>of Funds"]

    send_txs["Private Transfers<br>(Send)"]
    process_txs["Private Transfers<br>(Process)"]
    receive_txs["Private Transfers<br>(Retrieve)"]
    auditing["Auditing<br>(Optional)"]

    %% Flow Connections
    setup -.-> keygen -.-> registration -.-> agreement -.-> issuance -.-> send_txs

    subgraph Enygma Transfers
      send_txs -.-> process_txs -.-> receive_txs
    end

    receive_txs -.-> auditing

```


## 1 - System Setup

### Transparent Setup for Generator H

We use Pedersen commitments to mask the balances of the parties in the system and balances of the transactions. Such commitment relies on two generators: $$G$$ and $$H$$. We highlight, however, that knowing the relationship between these two generators is insecure as it breaks the binding property of the scheme. Concretely, if an entity knows the relationship between the generators $$G$$ and $$H = dG$$ (i.e., knows the value $$d$$), then such entity can open their commitments in any way they want. To avoid this, Enygma uses a [nothing-up-my-sleeve number](https://en.wikipedia.org/wiki/Nothing-up-my-sleeve_number) obtained by hashing a constant to the curve that is used in the system. This adds an additional layer of transparency. 

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    zero["00...00"]
    hash(["Hash-To-Curve(x)"])
    generator(["H"])

    %% Flow Connections
    zero -.-> hash -.-> generator

```
### ZK Trusted Setup (Groth16)
Enygma relies on the Groth16 ZK scheme, which requires an initial trusted setup. Ideally, such a trusted setup is in the form of an MPC protocol where different participants contribute with random secrets, which must be destroyed after the ceremony to ensure that a single party does not have the ability to subvert the system (i.e., forge proofs). The output of this trusted setup is the Common Reference String (CRS) for the circuit. 

We envision this step to take place involving different Privacy Nodes in the system. We note that each Privacy Node represents a regulated financial institution. Therefore, it is reasonable to assume that at least one of the institutions will abide by the protocol and preserve the security of the trusted setup stage. 

### Issuer Setup
The setup for the issuer is straightforward as it consists simply of deploying the corresponding Enygma smart contract. Optionally, to support certain private functionalities, the issuer may need to register some key material. 

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    %% I (Setup)
    i_setup["Issuer<br>(Setup)"]
    register(["(optional)<br>Register Key Material"])
    deploy(["Deploy Enygma<br>Contract"])

    %% Flow Connections
    i_setup -.-> register -.-> deploy
```

### Balance Setup
All participants start with a balance with $$v=0$$ and $$r=0$$. 

Therefore, the Issuer creates a contract where the initial balance for all the participants is:

$$Comm(0, 0) = 0G + 0H$$


## 2 - Key Generation
Each privacy node generates two keypairs: one to spend funds, and one to 'view' transactions. Concretely: 

* Privacy node A generates an [ML-KEM](https://nvlpubs.nist.gov/nistpubs/FIPS/NIST.FIPS.203.pdf) (view) keypair and obtains $$(sk_{A}^{view}, pk_{A}^{view})$$

* Privacy node A generates a simple hash-based (spend) keypair and obtains $$(sk_{A}^{spend}, pk_{A}^{spend})$$.
  *  $$sk_{A}^{spend} \longleftarrow \\\{{0, 1\\\}}^{256}$$
  *  $$pk_{A}^{spend} = Hash(sk_{A}^{spend})$$
 
The goal here is to have segregation of functionalities with each keypair. To spend, the user proves in zero-knowledge that they know $$sk^{spend}$$ corresponding to a $$pk^{spend}$$ in an anonymity set. We note that the hashing used in this step is ZK-friendly (i.e., Poseidon). On the other hand, the view key pair is used to generate shared secrets with other participants, which are then subsequently used to derive random factors for every block and ephemeral symmetric encryption keys for symmetric encryption. 

## 3 - Key Registration
Privacy node registers both the view and spend public keys on the underlying blockchain. 

For example, if privacy node A registers, the tuple below should be the output of the registration step: 

$$(id_{A}, pk_{A}^{view}, pk_{A}^{spend})$$

## 4 - Key Agreement
Party downloads the counterparty's ML-KEM public-key $$pk_{i}'$$, generates a pre-secret $$s'$$ and encapsulates it using the downloaded public-key, thus obtaining $$Encapsulate(pk_{i}', s')$$. Sender calculates $$id = Hash(s')$$ and publishes both $$< i, id, Encapsulate(pk', s')>$$ on the underlying blockchain. 

Counterparty knows their index $i$ and detects that a new publishing took place. Party $i$ downloads the bundle $$< i, id, Encapsulate(pk', s')>$$. Upon download, the entity decapsulates the published payload, obtains $s'$, calculates $id' = Hash(s')$ and checks if the obtained $id'$ matches the published $id$. If so, the party $i$ publishes a sign-off message and attests that the $id$ posted initially is correct and is ready to receive private transactions. 

## 5 - Issuing Tokens
There are two ways of issuing tokens. The issuer can either mint a commitment with the random factor set to zero which publicly discloses the minted amount or, alternatively, act as a participant in the network and mint a shielded balance in the form of Pedersen commitment where the random factor is derived from the shared secret between the issuer and the receiver of funds. This specific mechanism to generate the random factor ensures that the issuer can mint a specific amount and the recipient can detect the minted amount. 

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    %% I (Mint)
    i_mint(["Issuer<br>(Mint)"])
    mint_transparent(["Mint<br>(Transparent) Funds"])
    mint_ttx(["Mint Comm = vG"])
    mint_shield(["Mint<br>(Shielded) Funds"])
    calculate_r(["Calculate<br>Random Factor"])
    mint_stx(["Mint Comm = vG+rH"])


    %% Flow Connections
    i_mint -.-> mint_transparent -.-> mint_ttx
    i_mint -.-> mint_shield -.-> calculate_r -.-> mint_stx

```

### Transparent Issuance
Issuer creates a new commitment with the random factor set to 0. Therefore: 

$$Comm(v, 0) = vG + 0H = vG$$

This allows anyone to be able to see how much money was minted in the system. 

### Private Issuance
Issuer acts as a system entity and establishes a shared secret with every participant in the network, and creates a new commitment with the random factor set accordingly. Concretely, $$r = Hash(s, n_{block})$$. This commitment gets added to the previous balance (of zero). Therefore, the initial balance after a private issuance is:

$$Comm(v, r) = vG + rH$$

This ensures only the issuer and the recipient know how much money was minted. We note, however, that it's still possible to have verifiability on the minting side, in the sense that every time there is a mint that the system knows a mint occcurred. 

## 6 - Private Transfers

### Transaction Structure
We assume an anonymity set of size $$k$$, from which the sender is in index $$j$$. The exact transaction payload consists of a set of $$k$$ (Pedersen) commitments, a nullifier, a zero-knowledge proof $$\pi$$, a set of $$k$$ private messaging tags, and a set of $$k$$ ciphertexts

<div align="center">


| $$Commit_1$$ | $$\ldots$$ | $$Commit_k$$ | $$\text{nullifier}$$ | $$\pi$$ | $$t_1$$ | $$\ldots$$ | $$t_k$$ | $$ctxt_1$$ | $$\ldots$$ | $$ctxt_k$$ |
|--------------|------------|--------------|----------------------|---------|---------|------------|---------|------------|------------|------------|

</div>

#### Commitments
We use Pedersen commitments as shielded balances. Represented as follows: 

$$
\forall i \in \lbrace1,\ldots,k\rbrace:\quad
Commit_i = v_{i}G + r_{i}H
$$

The **amount** to be received by each entity $$v_{i}$$ is simply the number of monetary units to be transferred to that entity. The **random factor** of each recipient is obtained by hashing the shared secret between the sender $$j$$ and the recipient $$i$$: 

$$
\forall i \in \lbrace1,\dots,k\rbrace,\ i \neq j:\quad r_{i} = H(s_{i, j}, n_{block})
$$

The commitment of the sender contains the amount $$v_{j}$$ and random factor $$r_{j}$$. We have the following constraints

**The amount** in the commitment of the sender is the negative (since it's a debit) of all the amount that is being sent. This ensures that no new money enters the system. 

$$
v_j = - \sum_{i=1}^{k} v_i \text{ , } \text{where } i \neq j
$$

**The random factor** of the commitment of the sender is the negative of the sum of all the other random factors of the recipients. This ensures that the addition of all the $$k$$ commitments ensures that the amounts and random factors cancel out and it's possible to verify at a contract level that all the commitments in the transaction add up to $$0$$. 

$$
r_j = - \sum_{i=1}^{k} r_i \text{ , } \text{where } i \neq j
$$

**Our system ensures the following balance conservation invariant:**

Amounts sent in a transaction must add up to zero. 

$$
\sum_{i=1}^{i=k} v_i = 0
$$

Random factors for a set of commitments must add up to zero. 

$$
\sum_{i=1}^{i=k} r_i = 0
$$

#### Nullifier

The nullifier is calculated by simple hashing the (spend) secret key of the sender $$j$$ and the latest block number $$n_{block}$$. 

$$\text{nullifier} = Hash(sk_{j}, n_{block})$$


#### Private Messaging Tags
The private messaging tags are obtained by hashing the shared secret $$s$$ between both parties. For the slot of the sender, we instead hash the random value $$r$$ of the previous balance commitment. We choose this random value because it is a secret value, known only to the sender, and it is already an input to the ZK circuit we use in our system. 

$$
\forall i \in \lbrace 1,\ldots,k \rbrace:\quad
t_i =
\begin{cases}
H(r_j^{prev},n_{block}) & \text{if } i = j,\\
H(s_{i, j},n_{block}) & \text{if } i \neq j.
\end{cases}
$$

#### Zero-Knowledge Proof
The privacy node creates a ZK proof $$\pi$$ that proves the following: 

* I know the secret key of one of the items in this anonymity set of $$k$$ public keys;
* I know the amount and random factor of the commitment that contains my balance in this set of $$k$$ commitments;
* The nullifier is well-formed and uses my secret key and the latest block number;
* The private messaging tags are well-formed using the shared secret I have obtained previously with each of the $$k-1$$ participants and the latest block number.


### Sending a Transaction
To send a transaction, the privacy node needs to be in sync with the latest block on the blockchain. The purpose for this is twofold: first, the privacy node needs to create a nullifier and random factors that depend on that specific last block; and second, the privacy node needs to know what is the latest shielded balance it has in order to be able to spend funds. Therefore, the first step to send a transaction is to obtain the latest block. From the latest block number, the privacy node derives the ephemeral symmetric key used to encrypt additional/associated data of the transaction. The privacy node then calculates the corresponding random factors to be used in the transaction, and the nullifier for this block. The privacy node also calculates a set of $$k$$ (i.e., anonymity set) Pedersen commitments using the previously obtained random factors and the amount to be sent to each party along with a nullifier that proves that the sender is submitting its only allowed transaction in this block, without revealing details about who they are. Once these values are calculated, 


```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    %% PL (Send)
    pl_send["Privacy Node<br>(Send Tx)"]
    getblock_send(["Get Latest<br>Block"])
    derivesendkey(["Derive Ephemeral<br>(Symmetric) Key"])
    calc_r(["Calculate<br>Random Factor"])
    calc_tags(["Calculate Private<br>Messaging Tags"])
    tx_commits(["Generate 'k'<br>Pedersen Commitments"])
    nullifier(["Calculate<br>Nullifier"])
    zk_proof(["Create<br>ZK Proof"])
    encrypt_ad(["Encrypt Additional Data<br>(w/ ephemeral key)"])
    send_tx(["Send commits, nullifier, zk proof, and ciphertext"])

    pl_send -.-> getblock_send -.-> derivesendkey -.-> calc_r -.-> calc_tags -.-> tx_commits -.-> nullifier -.-> zk_proof -.-> encrypt_ad -.-> send_tx

```

#### Verifying a TX (Blockchain)
The smart contract receives a set of commitments, a nullifier, a ZK proof, and an encrypted payload. 

We note that the encrypted payload is not checked by the smart contract and can be maliciously formed as its correctness is not included in the ZK proof. We note that this attack forces the sender to send funds, which the recipient is able to open since the Pedersen commitment is well-formed. The same recipient is then able to prove that the sender maliciously formed the ciphertext and eventually  have the sender face repercussions for a purposeful malicious action. The ciphertext is not included in the ZK proof because proving the correctness of an AES-GCM encryption is too expensive to perform for the execution of real-time transactions. 

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    %% Blockchain (Verifier)
    verify_tx["Blockchain<br>(Verify Tx)"]
    check_nullifier(["Check if nullifier<br>exists"])
    check_commit(["Check if commitments add up to 0"])
    check_zk(["Check ZK Proof"])
    tally(["Approve Tx"])
    store(["Store encrypted payload"])

    verify_tx -.-> check_nullifier -.-> check_commit -.-> check_zk -.-> tally -.-> store
```

#### Querying For New Transactions
We assume each privacy node runs a full node of the underlying blockchain. Therefore, each node has the ability (and responsibility) to download the latest block and performs a lookup (locally) for transactions that include the privacy node in the anonymity set (i.e., transactions that may be for them). This is effectively the trivial [Private Information Retrieval](https://en.wikipedia.org/wiki/Private_information_retrieval) protocol, where a client downloads the entire dataset and performs the lookup locally. We leverage the full node role and leverage it to provide a private querying functionality to ensure privacy nodes do not reveal which transactions they are querying. 

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    %% PL (Receive)
    pl_receive["Privacy Node<br>(Querying)"]
    getblock_receive(["Download Latest<br>Block"])
    get_anon_set(["Get Subset of<br>Enygma Transactions"])

    pl_receive -.-> getblock_receive -.-> get_anon_set
```

### Retrieving a Transaction
Privacy node derives the private messaging tags, symmetric keys, and random factors for all the entities in the anonymity set(s) of all the transactions in such a block. 

Once this value is obtained, the privacy node can either:

* brute-force the value of $$v$$ (time-consuming but feasible since this is a monetary amount and should not be a very high amount)
* have a precomputed table containing all the possible reasonable values for $$vG$$
* use an efficient algorithm to compute the discrete log of this element (e.g., [baby-step giant-step](https://en.wikipedia.org/wiki/Baby-step_giant-step))

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    %% PL (Receive)
    pl_receive["Privacy Node<br>(Receive Tx)"]
    calc_tags(["Calculate Private<br>Messaging Tags"])
    detect_sender(["Detect Sender"])
    derivereceivekey(["Derive Ephemeral<br>(Symmetric) Key"])
    calc_r(["Calculate<br>Random Factor(s)"])
    obtain_vG(["Obtain vG"])
    obtain_v(["Obtain received amount"])
    calc_c(["Calculate Latest<br>(Shielded) Balance"])


    pl_receive -.-> calc_tags -.-> detect_sender -.-> derivereceivekey -.-> calc_r -.-> obtain_vG -.-> obtain_v -.-> calc_c

```

#### Simple Example
Let us assume there are two Enygma transactions (with an anonymity set $$k = 3$$) in a specific block. One transaction includes Alice, Bob, and Charlie. A second transaction includes Alice, Bob, and Dave. This is the anonymity set for each of the transactions. Traditionally, $$k=6$$, but for simplicity, we use $$k = 3$$. Let us also assume Alice is the person checking if there are funds for them. In this case, Alice is going to obtain the private messaging tags, symmetric encryption keys, and random factors associated with Bob, Charlie, and Dave for that specific block. After obtaining such tag(s), Alice checks (i.e., via brute-force) who is the sender of the transaction by comparing the private messaging tag for each of the entities in the anonymity set with the one included in the transaction. 

Let us assume Bob is the sender of the first transaction and Charlie of the second transaction. Alice uses the corresponding symmetric key with Bob to decrypt the additional data appended to the first transaction and the same for the second transaction using the key with Charlie. We note that this additional data should include the amount sent to make the receiving process simpler. However, in the event this data is not included, Alice obtains the random factor $$r = H(s_{A-B}, n_{block})$$ and is able to remove the random component of the received Pedersen commitment and obtain the remainder $$vG$$. Alice then obtains the corresponding value $$v$$ and is able to calculate the received amount and can now open their balance and spend the funds in subsequent blocks. 

If $$v = 0$$, then Alice was part of the anonymity set and has not received any funds. We note that it may be the case that no entity in the anonymity set has received funds and it is a dummy transaction (i.e., sending $$0$$ to all participants to add additional noise to the system). 

## 7 - Auditing
There are multiple types of auditing supported by the protocol. Concretely, the auditor can have a 'universal view' and have the ability of seeing all the transactions that take place in the network. 

#### View Key Sharing
If there is an auditor that needs the complete view of the transactions in the network, then each privacy node shares their view key pair with the auditor upon the key registration step. To do so, each privacy node encrypts their view secret key (i.e., $$sk_{A}^{view}$$) and publishes it on the blockchain for the auditor to fetch. 

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    pn["Privacy Node<br>(View Key Sharing)"]
    keygen(["(View) Key<br>Generation"])
    key_registration(["(View) Key<br>Registration"])
    get_auditor_key(["Get pk of<br>Auditor"])
    encrypt(["Encapsulate sk<br>(using pk of Auditor)"])
    publish_ciphertext(["Publish<br>Ciphertext"])

    pn -.-> keygen -.-> key_registration -.-> get_auditor_key -.-> encrypt -.-> publish_ciphertext


    auditor2["Auditor<br>(View Key Sharing)"]
    get_ciphertext(["Get Ciphertext"])
    decrypt(["Decapsulate and Obtain<br>sk of Privacy Node"])
    check(["Check Key Correctness"])

    auditor2 -.-> get_ciphertext -.-> decrypt -.-> check
```

For example, privacy node A publishes:

$$ ctxt = Encapsulate(pk_{audit}^{view}, sk_{A}^{view})$$

#### Ephemeral Symmetric (View) Key Sharing
Our system also supports the opening of individual transctions without compromising the secrecy of previous/future transactions. Since the system uses symmetric key encryption with ephemeral (per block) keys, we have a mode of operation where the sender or recipient can simply disclose individual symmetric keys and open individual transctions. 

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    pn["Privacy Node<br>(Ephemeral View Key Sharing)"]
    keygen(["(View) Key<br>Generation"])
    key_registration(["(View) Key<br>Registration"])
    get_auditor_key(["Get pk of<br>Auditor"])
    receive_request(["Receive Auditor Request"])
    encrypt(["Encapsulate symmetric encryption key <br>(using pk of Auditor)"])
    zk_prove(["Create ZK proof that k<br> is derived from shared secret"])
    publish(["Publish<br>Ciphertext and ZK Proof"])

    pn -.-> keygen -.-> key_registration -.-> get_auditor_key -.-> receive_request -.-> encrypt -.-> zk_prove -.-> publish


    auditor2["Auditor<br>(View Key Sharing)"]
    get_ciphertext(["Get Ciphertext"])
    decrypt(["Decapsulate and Obtain<br>k of Privacy Node"])
    check_zk(["Check ZK Proof Correctness"])
    check_enc(["Check k correctly decrypts payload"])
    audit_tx(["Audit Transaction"])

    auditor2 -.-> get_ciphertext -.-> decrypt -.-> check_zk -.-> check_enc -.-> audit_tx

```

The symmetric key $$k$$ for block $$n$$ (i.e, $$k_{n}$$) is obtained the following way: 

$$k_{n} = HKDF(s, n_{block})$$

#### Universal Auditing
Besides the auditor, we highlight that any entity in the system can always monitor the following parameters just from looking at the underlying blockchain: 

* Entity can check the ZK proof and attest that the set of commitments are well-formed and add up to 0.
* Entity can check how many mints/burns exist in the network.

Additionally, depending on the choice of random factors and the issuance process, it may be possible for any entity to check the total supply of the asset at all times. 

## Additional Remarks

### Memory Complexity
<div align="center">

| Component        | Complexity                                             | Additional Remarks                                                     |
|------------------|--------------------------------------------------------|------------------------------------------------------------------------|
| Key Registration |$$O(N \times \|pk^{view}\| )$$            | Each privacy registers a key on the underlying blockchain.            |
| Key Agreement    |$$O(N + (N \times \|ok\|) )$$ | Each privacy node establishes a key with all other privacy nodes.    |
| Tx Size          |$$O(k (\|C\| + \|t\| + \|ctxt\|) + \|\pi \| + \|nf\|)$$| $$k$$ commitments, tags, ciphertexts, a zk proof, and a nullifier.      |
| Block Size       |$$O(N \times \|tx\|)$$                  | Each bank can submit **at most** one tx per block.                      |

</div>

### Computational Complexity
<div align="center">

| Component     | Complexity                                             | Additional Remarks                                                     |
|---------------|--------------------------------------------------------|------------------------------------------------------------------------|
| Key Agreement | $$O(N - 1)$$                                           | Each privacy node establishes a key with all the other privacy nodes.  |


</div>

