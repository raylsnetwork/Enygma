# Enygma Payments

## System Architecture
Our system is simple: **users** (e.g., a bank customers) are directly connected to **privacy nodes** (i.e., a high-performance single-node EVM blockchain). Each of the privacy nodes, is connected to a **private network hub**, which will effectively act as a bulletin board for all privacy nodes to leverage as a universal (encrypted) messaging layer and verification layer. **Issuer(s)** are owners of specific assets on the private network hub. Optionally, there is an **auditor** that oversees (some of) the transactions that take place in the network. 

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR
    UA(["User(s)"])
    UB(["User(s)"])
    UC(["User(s)"])

    PLA(["Privacy Node"])
    PLB(["Privacy Node"])
    PLC(["Privacy Node"])

    B(["Blockchain"])
    I(["Issuer"])
    A(["Auditor"])
    

    PLA & PLB & PLC <-.-> B <-.-> I & A

    UA <-.-> PLA
    UB <-.-> PLB
    UC <-.-> PLC

```

## Protocol Flows

### System Setup


#### Transparent Setup for Generator H

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

#### ZK Trusted Setup (Groth16)
Enygma relies on the Groth16 ZK scheme, which requires an initial trusted setup. Ideally, such a trusted setup is in the form of an MPC protocol where different participants contribute with random secrets, which must be destroyed after the ceremony to ensure that a single party does not have the ability to subvert the system (i.e., forge proofs). The output of this trusted setup is the Common Reference String (CRS) for the circuit. 

We envision this step to take place involving different Privacy Nodes in the system. We note that each Privacy Node represents a regulated financial institution. Therefore, it is reasonable to assume that at least one of the institutions will abide by the protocol and preserve the security of the trusted setup stage. 

### Issuer

#### Issuer - Setup
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

#### Issuer - Mint

There are two minting flows for the issuer is able to mint funds on the underlying smart contract. The issuer can either mint a commitment with the random factor set to zero which publicly discloses the minted amount or, alternatively, act as a participant in the network and mint a shielded balance in the form of Pedersen commitment where the random factor is derived from the shared secret between the issuer and the receiver of funds. 

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

### Privacy Node

#### Privacy Node - Setup
First, each Privacy Node generates and registers two keypairs (view and spend) on the underlying blockchain. This blockchain effectively acts as a Public-Key Infrastructure (PKI) containing a registry of all public-keys of the registered Privacy Nodes. Subsequently, each Privacy Node performs a post-quantum key agreement (i.e., ML-KEM) and establishes individual shared secrets with all the other Privacy Nodes. At this point, Privacy Nodes can start transacting privately with each other. The transaction protocol includes a hash-based private messaging tag component that allow recipients to detect privately whether or not a transaction is for them. Therefore, we also introduce a protocol to fetch (and decrypt) transactions.

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    %% PL (Setup)
    pl_setup["Privacy Node<br>(Setup)"]
    keygen(["Key<br>Generation"])
    register(["Key<br>Registration"])
    kem(["Key<br>Agreement"])
    publish(["Publish<br>Key Fingerprints"])

    pl_setup -.-> keygen -.-> register -.-> kem -.-> publish
```

#### Privacy Node - Sending a TX
To send a transaction, the privacy node needs to be in sync with the latest block on the blockchain. The purpose for this is twofold: first, the privacy node needs to create a nullifier and random factors for that specific block; and second, the privacy node needs to know what is the latest shielded balance it has in order to be able to spend it. Therefore, the first step to send a transaction is to obtain the latest block. From the latest block number, the privacy node can derive the ephemeral symmetric key used to encrypt additional/associated data in this block, can calculate the corresponding random factors to be used in the transaction, and the nullifier for this block. The privacy node calculates a set of $$k$$ (i.e., anonymity set) Pedersen commitments using the corresponding random factors and the amount to be sent to each party. 


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
    calcR_send(["Calculate<br>Random Factor"])
    tx_commits(["Generate 'k'<br>Pedersen Commitments"])
    nullifier(["Calculate<br>Nullifier"])
    zk_proof(["Create<br>ZK Proof"])
    encrypt_ad(["Encrypt Additional Data<br>(w/ ephemeral key)"])
    send_tx(["Send commits, nullifier, zk proof, and ciphertext"])

    pl_send -.-> getblock_send -.-> derivesendkey -.-> calcR_send -.-> tx_commits -.-> nullifier -.-> zk_proof -.-> encrypt_ad -.-> send_tx

```

#### Privacy Node - Receiving a TX
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
    derivereceivekey(["Derive Ephemeral<br>(Symmetric) Key"])
    calcR_receive(["Calculate<br>Random Factor"])
    getblock_receive(["Get Latest<br>Block"])

    pl_receive -.-> getblock_receive -.-> derivereceivekey -.-> calcR_receive

```

### Blockchain
We now describe the flows associated with the underlying blockchain. In this case, we have just one: the verification of enygma transactions. 

#### Blockchain - Verify TX
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

## Cryptographic Primitives

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart TD
    A(["Enygma Payments"])
    
    Symmetric("Symmetric Crypto")
    Asymmetric("Asymmetric Crypto")

    A --> Symmetric & Asymmetric & ZK("Zero-Knowledge Proofs") & Commits("Commitments")
    
    Asymmetric --> View("View Keypair") & Spend("Spend Keypair")

    Symmetric --> AES("Authenticated Encryption<br>(AES-GCM-256)") & HKDF("Key Derivation Function<br>(HKDF)")
    View --> MLKEM("Lattice-based<br>(ML-KEM)")
    Spend --> Hash("Hash-based<br>(Poseidon)")

    ZK --> snarks("ZK-SNARKs<br>(Groth16)")
    Commits --> pedersen("Pedersen Commitments")
    pedersen --> Babyjubjub("Elliptic Curve Crypto<br>(Baby Jubjub)")
```

Note: In the future we want to update the ZK module to use a quantum-secure ZK scheme. This update will make the entire system quantum-secure.

## Implementation Details
* **Client**: Golang
* **ZK Circuit(s)**: Gnark
* **Verifier**: Solidity


## Peer-Reviewed Publications
- [Rayls: A Novel Design for CBDCs](https://eprint.iacr.org/2025/1639), published at [The 6th Workshop on Coordination of Decentralized Finance (CoDecFin) 2025](https://fc25.ifca.ai/codecfin/)
- [Rayls II: Fast, Private, and Compliant CBDCs](https://eprint.iacr.org/2025/1638), published at [Financial Cryptography in Rome (FCiR) 2025](https://www.decifris.it/fcir25/)
