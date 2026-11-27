# Protocol Description

> **Implementation status (Fix M-12).** This document describes the
> Enygma protocol **as designed**, written throughout in the present
> tense ("the system supports...", "these ciphertexts are encrypted...").
> That present tense does **not** mean every mechanism below is built in
> `enygma_payments` today. Sections 6's ciphertext payload (AES-GCM-256
> encryption, HKDF-based per-block key rotation) and all of Section 7
> (**Auditing**, in its entirety — long-term view-key sharing, ephemeral
> view-key sharing, and universal auditing) are **design, not
> implemented**: there is no AES-GCM/HKDF code anywhere in this repository
> outside this document, no "store encrypted payload" step, no
> tag-scanning retrieval path, and no auditor role, address, escrow, or
> event of any kind. Everything through Section 5 (**System Setup** —
> **Key Generation** — **Key Registration** — **Key Agreement** — **Issuing
> Tokens**) and the core of Section 6 (commitments, nullifiers, the ZK
> proof, and on-chain verification) matches the shipped code, except that
> every shipped mint publishes its amount (see the note in Section 5 and
> Section 8). If your
> institution has relied on this document to evaluate the system's privacy
> or auditing properties, the mechanisms above are not properties you can
> rely on today — treat them as a roadmap, not a shipped guarantee.
> Separately (documented for completeness, not because it changes the
> above): even if built, this specific construction could not deliver
> *scoped* auditing as written — the value that opens a transaction is a
> Pedersen blinding shared jointly by both parties, and the epoch anchor
> means one disclosure opens every transaction between that pair for the
> whole epoch window in both directions, so the only auditing mode this
> design can actually support is permanent and universal, not
> per-transaction and revocable.

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

## Notation

* G and H are generators of the used group (i.e., BN254).
* Hash(x) represents a cryptographically-secure (and ZK-friendly) hash of preimage 'x'.
* Hash-To-Curve(x) represents an algorithm for hashing an arbitrary string 'x' to a point on the elliptic curve.
* $$Comm(v, r)$$ is a (Pedersen) commitment to value $$v$$ with a random factor $$r$$.
* $$sk_{i}^{spend}$$ is the spend secret key for entity $$i$$.
* $$pk_{i}^{spend}$$ is the spend public key for entity $$i$$.
* $$sk_{i}^{view}$$ is the view secret key for entity $$i$$.
* $$pk_{i}^{view}$$ is the view public key for entity $$i$$.
* $$n_{block}$$ is the latest block number.
* $$k$$ is the anonymity set of the system.
* $$t_{i}$$ is the private messaging tag of index $$i$$, s.t.,  $$i \in \lbrace 1,\ldots,k \rbrace$$
* $$s_{i, j}$$ is the shared secret established between party $$i$$ and party $$j$$.
* $$\text{nullifier}$$ is the nullifier that is sent in each transaction to ensure entities are not able to double-spend.
* $$\pi$$ is the zero-knowledge proof that is appended to an Enygma transaction.
* $$K_{i, j}^{n}$$ is the symmetric encryption key used for encrypting payloads on block $$n$$ between participants $$i$$ and $$j$$.

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

Therefore, the Issuer creates a contract where the default initial balance for all the participants is:

$$Comm(0, 0) = 0G + 0H$$


## 2 - Key Generation
Each privacy node generates two keypairs: one to spend funds, and one to 'view' transactions. Concretely: 

* Privacy node A generates an [ML-KEM](https://nvlpubs.nist.gov/nistpubs/FIPS/NIST.FIPS.203.pdf) (view) keypair and obtains $$(sk_{A}^{view}, pk_{A}^{view})$$

* Privacy node A generates a simple hash-based (spend) keypair and obtains $$(sk_{A}^{spend}, pk_{A}^{spend})$$.
  *  $$sk_{A}^{spend} \longleftarrow \\\{{0, 1\\\}}^{256}$$
  *  $$pk_{A}^{spend} = Hash(sk_{A}^{spend})$$
 
The goal here is to have segregation of functionalities with each keypair. 

* To spend, the user proves in zero-knowledge that they know a secret key $$sk^{spend}$$ corresponding to one public key $$pk^{spend}$$ in an anonymity set of size $$k$$. We note that the hashing used in this step is ZK-friendly (i.e., Poseidon).
* The view key pair is used to generate shared secrets with other participants, which are then subsequently used to derive random factors for the Pedersen commitments and ephemeral symmetric encryption keys for symmetric encryption. These (symmetric) values are derived deterministically from the pre-established secrets every block.  

## 3 - Key Registration
Privacy node registers both the view and spend public keys on the underlying blockchain. 

For example, if privacy node A registers, the tuple below should be the output of the registration step: 

$$(id_{A}, pk_{A}^{view}, pk_{A}^{spend})$$

## 4 - Key Agreement
We run the following ML-KEM protocol adapted to use the underlying blockchain as a global PKI for the network and a bulletin board for universal ciphertext publishing. 

* The sender $$j$$ downloads the counterparty's $$i$$ ML-KEM public key $$pk_{i}^{view}$$ and runs $$\text{ML-KEM.Encapsulate}(pk_{i}^{view})$$ and obtains a ciphertext $$ctxt_{i,j}$$ and a shared secret $$s_{i,j}$$. 

* The sender computes $$id = Hash(s_{i,j})$$ and publishes $$⟨i, j, id, ctxt_{i, j}⟩$$ on the underlying blockchain. This allows the recipient to know that they have a new message for them and who is publishing it.

* The counterparty $$i$$ watches the chain, downloads $$⟨i, j, id, ctxt_{i, j}⟩$$, runs $$\text{MLKEM.Decaps}(sk_{i}^{view}, ctxt_{i, j})$$, obtains a shared secret $$s'_{i,j}$$, and computes $$id' = Hash(s')$$. If $$id' = id$$, party $$i$$ publishes a sign-off message and is ready to receive private transactions from the sender.

## 5 - Issuing Tokens
> **As shipped.** `mintSupply` always takes and emits a plaintext `amount`, whatever random factor the minted commitment uses, and nothing checks that the commitment opens to that amount. So a private issuance does not hide the amount from chain observers, and a transparent one does not prove it. See Section 8.

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
Issuer acts as a system entity and establishes a shared secret $$s$$ with each participant in the network, and creates a new commitment with the random factor set accordingly. Concretely, $$r = Hash(s, n_{block})$$. This commitment gets added to the previous balance (of zero). Therefore, the initial balance after a private issuance is:

$$Comm(v, r) = vG + rH$$

This ensures only the issuer and the recipient know how much money was minted. We highlight that the issuer knows both $$v$$ and $$r$$ for every private mint. This does not degrade confidentiality beyond the fact that the issuer is the one minting and thus intentionally knows the amount and beneficiary. We note that it's also possible to have verifiability on the minting side, in the sense that every time there is a mint that the system knows a mint occcurred. 

On a final note, the issuer can transparently publish the total circulating supply along with a ZK proof that shows that the addition of all Pedersen commitment balances on-chain results in that published balance. This is an alternative to provide further transparency in a system where the issuer does not want to disclose individual mint amounts. 


## 6 - Private Transfers

### Transaction Structure
We assume an anonymity set of size $$k$$, from which the sender is in index $$j$$. The exact transaction payload consists of a set of $$k$$ (Pedersen) commitments, a nullifier, a zero-knowledge proof $$\pi$$, a set of $$k$$ private messaging tags, and a set of $$k$$ ciphertexts. 

Since we publish $$k$$ commitments per transaction, we effectively support the sender to pay up to $$k-1$$ banks (all $$k$$ members in the anonymity set except themselves). This allows for batching of payments. Additionally, we append a ciphertext to each of these commitments, which can contain additional information about each of the payments. Therefore, we can encode additional information that allows for double batching. 

**For example, a single transaction from a financial institution can pay multiple clients in each of the other banks. This is a payment with double aggregation.**

<div align="center">


| $$Commit_1$$ | $$\ldots$$ | $$Commit_k$$ | $$\text{nullifier}$$ | $$t_1$$ | $$\ldots$$ | $$t_k$$ | $$\pi$$| $$ctxt_1$$ | $$\ldots$$ | $$ctxt_k$$ |
|--------------|------------|--------------|----------------------|---------|------------|---------|---------|------------|------------|-----------|

</div>

#### Commitments
We use Pedersen commitments as shielded balances. Represented as follows: 

$$
\forall i \in \lbrace1,\ldots,k\rbrace:\quad
Commit_i = v_{i}G + r_{i}H
$$

The **amount** to be received by each entity $$v_{i}$$ is simply the number of monetary units to be transferred to that entity. The **random factor** of each recipient is obtained by hashing the shared secret between the sender $$j$$ and the recipient $$i$$: 

$$
\forall i \in \lbrace1,\dots,k\rbrace,\ i \neq j:\quad r_{i} = Hash(s_{i, j}, n_{block})
$$

The commitment of the sender contains the amount $$v_{j}$$ and random factor $$r_{j}$$. We have the following constraints

**The amount** in the commitment of the sender is the negative (since it's a debit) of all the amount that is being sent. This ensures that no new money enters the system. 

$$
v_j = - \sum_{i=1}^{k} v_i \text{ , } \text{where } i \neq j
$$

**The sender's random factor** is the negative of the sum of the recipients' random factors. Across all $$k$$ commitments, both the amounts and the random factors cancel, so the commitments sum to the group identity.

$$
r_j = - \sum_{i=1}^{k} r_i \text{ , } \text{where } i \neq j
$$

The transfer circuit enforces the following balance conservation invariant. In `transfer()`, the configured transfer verifier is the only check on conservation of the main asset. The contract binds the submitted commitment deltas to the proof's public inputs.

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

$$\text{nullifier} = Hash(sk_{j}^{spend}, n_{block})$$


#### Private Messaging Tags
The private messaging tags are obtained by hashing the shared secret $$s$$ between both parties. For the slot of the sender, one must use a different value as a sender does not have a shared secret established with themselves. A poor choice of this value can also break the privacy of the sender as it could potentially allow a network observer to infer additional information about that specific tag. Instead, we hash the random value $$r$$ of the previous balance commitment. We note that a sender has a shielded balance on-chain, which they're always able to open (as they know the amount they have and are always able to obtain the corresponding random factor). We choose this random value because it is a secret value, known only to the sender, and it is already an input to the ZK circuit we use in our system. 

$$
\forall i \in \lbrace 1,\ldots,k \rbrace:\quad
t_i =
\begin{cases}
Hash(r_j^{prev},n_{block}) & \text{if } i = j,\\
Hash(s_{i, j},n_{block}) & \text{if } i \neq j.
\end{cases}
$$

#### Zero-Knowledge Proof
The privacy node creates a ZK proof $$\pi$$ that proves the following: 

* I know the secret key of one of the items in this anonymity set of $$k$$ public keys;
* I know the amount and random factor of the commitment that contains my balance in this set of $$k$$ commitments;
* I am not spending more funds than the funds I have;
* The total amount of funds I am spending is being credited to the other participants in this anonymity set;
* The nullifier is well-formed. Therefore, it is derived from my secret key and the latest block number;
* The private messaging tags are well-formed. Therefore, they use the shared secret I have obtained previously with each of the $$k-1$$ participants and the latest block number or the random factor that opens a commitment from the anonymity set for the index of the sender.

#### Ciphertexts

> **Design, not implemented (Fix M-12).** No AES-GCM-256 or HKDF code
> exists anywhere in `enygma_payments`; no ciphertext is ever produced,
> transmitted, or stored on chain today. See the notice at the top of
> this document.

A transaction payload includes a set of $$k$$ ciphertexts (encrypted using AES-GCM-256). These ciphertexts should contain additional information for the recipient to be able to quickly open the transaction details (i.e., the Pedersen commitments) and potentially get more information about the specific recipient (e.g., user Bob, who is a client of that specific receiving bank). **These ciphertexts are encrypted with an ephemeral symmetric key that is rotated in every new block**. This key rotation is useful and by design to allow specific auditors to potentially request for individual transactions without compromising the confidentiality of past/future ciphertexts. 

The symmetric key $$K$$ for block $$n$$ between participants $$i$$ and $$j$$ is obtained the following way: 

$$K_{i, j}^{n} = HKDF(s_{i,j}, n_{block})$$

The corresponding ciphertext is then calculated by encrypting (using AES-GCM-256) a message $$m$$ that contains additional transaction data. 

$$ctxt_{i} = Enc(K_{i, j}^{n}, m)$$

We note that the correctness of these ciphertext is not part of the ZK proof and these values can indeed be maliciously formed. This is by design to keep the circuit cheap. The commitment, however, will always be of a valid amount and will correspond to a debit. Therefore, performing this attack will always cost funds and will allow the recipient to prove that the sender is indeed malicious, by simply showing that the received ciphertext does not open under the corresponding symmetric key for that block. In other words, the only effect of a malformed ciphertext is that the sender burns funds and produces a publicly verifiable incriminating record.

We believe this trade-off is ideal for multiple reasons. Concretely, the trust model with financial institutions is different as these are regulated institutions. In this setting, it is very easy to prove malfeasance. Finally, this approach allows the design to have a very cheap overall prover cost, which allows institutions to make very fast (aggregated) payments. 

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

#### Implementation Note: the Relayer (`enygma_payments/relayer`)

**This subsection describes the current reference implementation, not the
protocol.** The steps above ("Send commits, nullifier, zk proof, and
ciphertext") describe a privacy node submitting its own transaction
directly to the blockchain, and nothing in the protocol as specified
requires otherwise — a bank *could* hold its own signing key, be its own
`msg.sender`, and submit `transfer()` itself.

The shipped `enygma_payments/relayer` component does not do that. Each
bank's `registerAccount` binds `addressToAccountId` to the bank's own
address (`go_client/cmd/register_bank`), and the relayer is registered
as an account of its own. That bank address is what authorizes the
bank's fingerprint confirmations (Section 8), but the relayer holds the only
private key that is ever used to call `transfer()` / `transferWithFee()` /
`withdraw()` / `deposit()`. Concretely, this means:

- **The relayer is a mandatory, single point of submission.** A bank
  builds its own proof and commitments locally (nothing in that process
  touches the relayer or reveals private inputs to it), then hands the
  finished, already-valid request to the relayer over HTTP for on-chain
  submission. The relayer cannot forge a transaction — it never sees a
  bank's private witness data, and every value it *could* tamper with
  (public signals, participant ids, commitments) is bound into the
  Groth16 proof itself and independently checked on-chain — but it is the
  only party that can make an otherwise-valid, already-built request
  *reach* the chain at all.
- **The relayer can censor.** Since it is the sole submitter, declining to
  call `transfer()` for a given request leaves no on-chain trace — a
  censored bank cannot distinguish "the relayer refused" from "the relayer
  is down" or "the chain is congested," and has no independent path to
  submit the same, already-valid request itself.
- **The relayer totally orders submissions.** Because it is the only
  signer, it necessarily decides the relative order of every transaction
  it accepts; where two transactions conflict (e.g. would double-spend the
  same commitment), the relayer's ordering deterministically decides which
  one lands.
- **The relayer's own on-chain identity is the only one visible.**
  `msg.sender` on every `TransactionSuccessful` event is always the
  relayer's address, never the submitting bank's — so the chain's own
  audit trail carries no per-bank attribution. (The relayer's HTTP logs do
  carry per-bank attribution as of the per-bank credentials described in
  §3/§4 below, but that attribution exists only in the relayer's own logs,
  not on-chain.)
- **The relayer's slot needs a real pair secret.** With USDr fees the
  relayer is a participant in every relayed transfer's anonymity set, and
  the proof accepts any `SharedSecrets` value the sender picks for its
  slot: nothing on-chain checks it against the relayer. That value must
  still be full-entropy and known only to the sender and the relayer.
  Each slot's tag is
  `Poseidon(HashTag, SharedSecrets[i], Poseidon(Nullifier, Poseidon(SenderId, AnonymitySet[i])))`,
  and `Nullifier` and `AnonymitySet` are public, so anyone who knows the
  relayer slot's secret tries the six `SenderId` values, matches the
  relayer's tag, and names the sender. A constant or guessable placeholder
  (such as `demo/main.go`'s `demoDefaults`, which are demo-only) does this
  for every relayed transfer. Until the relayer takes part in a real key
  agreement, draw a fresh random secret per sender–relayer pair and share
  it privately.

**What this does not affect.** The relayer cannot steal, redirect, or
double-spend funds — every check that matters (proof validity, nullifier
uniqueness, block-number freshness, commitment/public-key binding) is
enforced on-chain, independent of anything the relayer does or doesn't do.
A client that wants a stronger guarantee than "the relayer said 200 OK"
should independently verify the resulting balance change against its own
RPC connection after a relay call, exactly as `demo/main.go` already does
(`inst.GetBalance` compared against `inst.AddPedComm` of the previous
balance and the transaction's own commitment) — a relayer that returns
success without actually submitting, or that gets front-run, produces a
detectable mismatch rather than a silently accepted lie.

#### Verifying a TX (Blockchain)
The smart contract receives a set of commitments, a nullifier, a ZK proof, and an encrypted payload. 

The owner chooses the verifier that enforces transfer conservation and can approve a verifier runtime hash immediately. Installing an approved verifier in an empty slot takes effect immediately. For an occupied slot, the owner can execute a proposed replacement after `verifierChangeDelay` seconds. A zero delay makes the replacement immediate.

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
    check_zk(["Verify transfer proof<br>for balance conservation"])
    tally(["Approve Tx"])
    store(["Store encrypted payload"])

    verify_tx -.-> check_zk -.-> check_nullifier -.-> tally -.-> store
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

> **Design, not implemented (Fix M-12).** No tag-scanning or ciphertext-
> decryption retrieval path exists in `enygma_payments` today. See the
> notice at the top of this document.

Privacy node derives the private messaging tags, symmetric keys, and random factors for all the entities in the anonymity set(s) of all the transactions in such a block. The privacy node is then going to brute-force try the symmetric decryption of each transaction. This work is paralellizable and extremely fast as it is simply performing $$k-1$$ hash operations for each received transaction. Once the sender is detected, the recipient can perform an AES-GCM decryption and obtain more information about the transfer details in the corresponding plaintext. In the event that the sender was malicious, the recipient now knows the sender associated with this transaction was malicious and is able to prove that the symmetric key for that specific block does not result in a successful AES-GCM decryption. 

In order to open the balance from the transaction. Since the Pedersen commitments are well-formed (enforced by the ZK proof), the recipient, who knows the corresponding random factor $$r$$, can remove the randomness component and obtain $$vG$$.

Concrete deployments will impose limits on transfer amounts (i.e., the value of $$v$$), which ensures that the discrete-log-based recovery of $$v$$ from $$vG$$ is practical enough to be feasible to conduct in a catastrophic scenario. To obtain the received amount $$v$$ from $$vG$$, we provide three options:

* brute-force the value of $$v$$ (time-consuming but feasible since this is a monetary amount and should not be a very high amount)
* have a precomputed table containing all the possible reasonable values for $$vG$$ for all nodes in the network
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

Let us assume Bob is the sender of the first transaction and Charlie of the second transaction. Alice uses the corresponding symmetric key with Bob to decrypt the additional data appended to the first transaction and the same for the second transaction using the key with Charlie. We note that this additional data should include the amount sent to make the receiving process simpler. However, in the event this data is not included, Alice obtains the random factor $$r = Hash(s_{A, B}, n_{block})$$ and is able to remove the random component of the received Pedersen commitment and obtain the remainder $$vG$$. Alice then obtains the corresponding value $$v$$ and is able to calculate the received amount and can now open their balance and spend the funds in subsequent blocks. 

If $$v = 0$$, then Alice was part of the anonymity set and has not received any funds. We note that it may be the case that no entity in the anonymity set has received funds and it is a dummy transaction (i.e., sending $$0$$ to all participants to add additional noise to the system). 

## 7 - Auditing

> **Design, not implemented (Fix M-12).** No auditor role, address,
> escrow, key-sharing mechanism, or event exists anywhere in
> `enygma_payments` today — this entire section describes design intent
> only. Even if built, this construction could not deliver *scoped*
> auditing as written; see the notice at the top of this document for why.

There are multiple types of auditing supported by the protocol. Concretely, the auditor can have a 'universal view' and have the ability of seeing all the transactions that take place in the network. 

#### Long-term (View) Key Sharing
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

For example, privacy node B does the following: 

* Downloads the ML-KEM public key of the auditor $$pk_{audit}^{view}$$ and runs $$\text{ML-KEM.Encapsulate}(pk_{audit}^{view})$$ and obtains a ciphertext $$ctxt_{audit,B}$$ and a pre-secret $$s_{audit,B}$$.
  
* Using the pre-secret $$s_{audit,B}$$, the privacy node encrypts their (view) secret key $$sk_{B}^{view}$$ and obtains $$ctxt_{B} = \text{Enc}(s_{audit,B},sk_{B}^{view})$$. This is the long-term view key that the auditor can use to audit all future transactions involving privacy node B. **Having this view key does not allow the auditor to spend any funds**.

* Publishes $$("audit", B, ctxt_{audit, B}, ctxt_{B})$$ on the underlying blockchain. This allows the auditor to know that they have a new ciphertext for them and who is publishing it.

* The auditor watches the chain, downloads the payload, runs $$\text{MLKEM.Decaps}(sk_{audit}^{view}, ctxt_{audit, B})$$, obtains the pre-secret $$s'_{audit,B}$$.
  
* Decrypts $$ctxt_{B}$$ and obtains $$sk_{B}^{view}$$. Auditor is now ready to start auditing upcoming transactions involving B. 


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



#### Universal Auditing
Besides the auditor, we highlight that any entity in the system can always monitor the following parameters just from looking at the underlying blockchain: 

* Anyone can verify a transfer proof against the reviewed transfer circuit's verifying key to check balance conservation.
* Entity can check how many mints/burns exist in the network.

Additionally, depending on the choice of random factors and the issuance process, it may be possible for any entity to check the total supply of the asset at all times. 

## 8 - Trust Assumptions and Known Limitations
The deployed contract (`enygma_payments/contracts/enygma/contracts/Enygma.sol`) relies on the assumptions below, some on purpose and some by omission. Except for a leaked EOA (second item), none of them loses funds on the shipped flows, but operators and integrators must know them. The first four (subgroup points, EOA authority, liveness, `AnonymitySet`) could each be closed later by a small `Enygma.sol` change. The visible fee submitter and the public burn and withdraw amounts would need new circuits and keys, or a vault change.

**What the owner and each account must do**

* **Every commitment point the contract accepts must lie in the prime-order subgroup.** `_isValidCommitment` checks that a point's coordinates are in range and that it lies on BabyJubJub, but not that it lies in the prime-order subgroup. Every point it checks is submitted by the owner: `registerAccount`'s initial commitment, `initializeUsdrBalance`'s initial USDr commitment and `mintSupply`'s mint commitment. An account that computes its own registration `Com(0, r)` (as `registerAccount`'s NatSpec describes) must hand the owner a subgroup point, and the owner should check $$[l]P = O$$ before submitting any point. A point outside the subgroup is accepted, but no opening $$(v, r)$$ matches the balance it lands in, so that balance cannot be opened or spent until the owner adds the opposite small-order point (for example `mintSupply(0, id, …)` with that point). Only the owner can submit such a point, and the account whose balance it is bears the cost.
* **An account's EOA must be protected like its spend key.** The address bound in `registerAccount` cannot be rotated or revoked, and it can be frozen only by pausing the whole contract (`pause()` also stops every value-moving call). It is the only authority over that account's side of every fingerprint confirmation (`registerFingerprint`), and a confirmed fingerprint is overwritten as soon as both sides post matching new claims. So an attacker who holds a leaked EOA and colludes with a counterparty can replace that pair's confirmed fingerprint with one for a secret the victim does not know. The counterparty can then send a transfer naming the victim with that secret in `SharedSecrets`, which shifts the victim's blinding factor by a value the victim cannot compute. The victim's whole balance becomes unopenable, and the victim can no longer transfer, withdraw or burn. Losing an EOA without a leak is less severe: the EOA authorizes only `registerFingerprint`, and the account's transfers are authorized by its spend-key proof. To move the account to a fresh EOA, the owner can burn its balance (with a proof from the holder's spend key), register a new account for the new address and mint the balance to it.

* **The owner must set the fees after deploy.** Both fee checks are exact matches against an owner-set value that starts at 0, and no deployment script sets either, so until the owner (`onlyOwner`) acts, every fee must be 0:
  * `protocolFee`, default 0, set with `setProtocolFee(fee)`: the fee `transferWithFee`'s proof must carry (it is burned, `FeeBurned`).
  * `usdrFixedFeeAmount`, default 0, set with `setUsdrFixedFee(amount)` (below 2^64): the USDr fee `transfer`'s USDr proof must carry.

**Liveness: proofs go stale and nothing orders writes**

* Every proof must name the current epoch: its block-number signal must equal `lastBlockNum`, and every balance-changing call moves `lastBlockNum` to the current epoch start. So the first write in a new epoch, by any account or an owner mint, makes every pending proof stale, including proofs that share no participant with that write. A transfer statement also commits to the live balances of all its participants (`PREVIOUS_COMMITMENTS`), so within an epoch any write that changes one of those balances stales every pending proof that names that account. A stale proof must be regenerated. The contract neither orders nor rate-limits writes, so any registered account can keep other accounts' proofs stale, either by putting them in its anonymity sets or by writing once per epoch. This stalls transfers but moves no funds. With `deploy_node.js`'s default `EPOCH_INTERVAL=1` an epoch is one block, so in practice only one proof-carrying write lands per block. That is the source of the "at most one tx per block" figure in the size table below, which the contract does not otherwise enforce.
* **The registry holds at most 80 accounts (`MAX_REGISTERED_ACCOUNTS`).** Every ledger write copies each registered account's balance commitments forward on both ledgers, about 100k gas per account, and the first write of an epoch costs most. The costliest is a withdraw into ten deposits (each a real vault note, about 450k gas): at 80 accounts it takes about 14M gas on an epoch's first block, under the 2^24 (about 16.8M) per-transaction gas cap of EIP-7825, and from about 106 accounts it would not fit; a transfer takes about 10M. `registerAccount` refuses the 81st account with `RegistryFull`. The cap is a constant: a chain with a lower per-transaction or block gas limit needs a lower one, which means changing it and redeploying.

**What the on-chain checks do not bind**

* **`AnonymitySet` is not bound to `participantIds`.** `transfer` compares the proof's `AnonymitySet` signals only with the USDr proof's copy. It does not compare them with the `participantIds` the contract uses. Public keys and previous balances are bound to `participantIds`, so funds cannot be redirected. But receiver $$i$$'s blinding factor is derived from `Poseidon(SenderId, AnonymitySet[i])`, so a receiver must take `AnonymitySet[i]`, and the candidate `SenderId` values (the other published `AnonymitySet` entries), from the proof's public signals and never from `participantIds`. A receiver that uses `participantIds` instead computes the wrong blinding factor and cannot open its new balance.
* **Mint amounts are owner-asserted.** `mintSupply` adds the owner's commitment to the recipient's balance and records the owner-stated `amount` in `totalSupplyAmount` and in the `SupplyMinted` event. Nothing checks that the commitment opens to `amount` (the function's NatSpec says so). Published supply figures are only as good as the owner. `mintUsdrSupply` emits the same `SupplyMinted` event, and the event has no asset field, so summing `SupplyMinted` mixes main-asset and USDr mints. Read the main supply from `TotalSupply()` or from the `mintSupply` calls.

**What is public**

* **The fee submitter is visible, and the owner can decide who transacts.** `transfer` requires a USDr proof whose `FeeRecipientKey` is the key of `msg.sender`, and the USDr circuit gives the whole fee to that slot, which can never be the sender's. The submitter is therefore a publicly known non-sender, and the effective anonymity set is five accounts, not six. When `usdrFixedFeeAmount` is nonzero, USDr exists only through the owner's `mintUsdrSupply`, so the owner decides who can pay the fee and therefore who can transact. It defaults to 0 and no deployment script sets it; with a zero fee, a never-funded USDr balance is enough.
* **Amounts outside transfers are public, and so are fees.** Transfer amounts stay hidden. Mint, USDr mint, burn and withdraw publish their amounts in calldata, in proof public signals or in events (`SupplyMinted`, `BurnSuccessful`). A deposit is public too: it redeems a note that a withdraw created from a plaintext amount, and it publishes that note's hash, so an observer can match the deposit to the withdraw and learn the credited amount. Fees are public: `transfer`'s USDr proof publishes `FeeAmount`, which goes in full to the known submitter, and `transferWithFee` publishes its fee and emits `FeeBurned`. The rule that amounts stay hidden on chain covers transfer amounts only.

## Additional Remarks

### Memory Complexity
<div align="center">

| Component        | Complexity                                             | Additional Remarks                                                     |
|------------------|--------------------------------------------------------|------------------------------------------------------------------------|
| Key Registration |$$O(N ( \|pk^{view}\| + \|pk^{spend}\| ))$$             | Each privacy registers a key on the underlying blockchain.             |
| Key Agreement    |$$O(N + (N \times \|ok\|) )$$                           | Each privacy node establishes a key with all other privacy nodes.      |
| Tx Size          |$$O(k (\|C\| + \|t\| + \|ctxt\|) + \|\pi \| + \|nf\|)$$ | $$k$$ commitments, tags, ciphertexts, a zk proof, and a nullifier.     |
| Block Size       |$$O(N \times \|tx\|)$$                                  | Each bank can submit **at most** one tx per block.                     |

</div>

### Computational Complexity
<div align="center">

| Component     | Complexity                                             | Additional Remarks                                                     |
|---------------|--------------------------------------------------------|------------------------------------------------------------------------|
| Key Agreement | $$O(N - 1)$$                                           | Each privacy node establishes a key with all the other privacy nodes.  |


</div>

