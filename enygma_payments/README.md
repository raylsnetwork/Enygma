# Enygma Payments

## System Architecture

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

    PLA(["Privacy Ledger"])
    PLB(["Privacy Ledger"])
    PLC(["Privacy Ledger"])

    B(["Blockchain"])
    I(["Issuer"])
    A(["Auditor"])
    

    PLA & PLB & PLC <-.-> B <-.-> I & A

    UA <-.-> PLA
    UB <-.-> PLB
    UC <-.-> PLC

```

## Protocol Flows

### Issuer

```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    %% Entities
    issuer["Issuer"]

    %% I (Setup)
    i_setup["Issuer<br>(Setup)"]
    deploy(["Deploy Enygma<br>Contract"])

    %% I (Mint)
    i_mint(["Issuer<br>(Mint)"])
    mint_transparent(["Mint<br>(Transparent) Funds"])
    mint_ttx(["Mint Comm = vG"])
    mint_shield(["Mint<br>(Shielded)Funds"])
    calculate_r(["Calculate<br>Random Factor"])
    mint_stx(["Mint Comm = vG+rH"])


    %% Flow Connections
    issuer -.-> i_setup & i_mint
    i_setup -.-> deploy
    i_mint -.-> mint_transparent -.-> mint_ttx
    i_mint -.-> mint_shield -.-> calculate_r -.-> mint_stx

```

### Privacy Ledger

First, each privacy ledger generates and registers two keypairs (view and spend) on the underlying blockchain. This blockchain effectively acts as a Public-Key Infrastructure (PKI) containing a registry of all public-keys of the registered privacy ledgers. Subsequently, each privacy ledger performs a post-quantum key agreement (i.e., ML-KEM) and establishes individual shared secrets with all the other privacy ledgers. At this point, privacy ledgers can start transacting privately with each other. The transaction protocol includes a hash-based private messaging tag component that allow recipients to detect privately whether or not a transaction is for them. Therefore, we also introduce a protocol to fetch (and decrypt) transactions.


```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    %% Entities
    pl["Privacy Ledger"]

    %% PL (Setup)
    pl_setup["Privacy Ledger<br>(Setup)"]
    keygen(["Key<br>Generation"])
    register(["Key<br>Registration"])
    kem(["Key<br>Agreement"])
    publish(["Publish<br>Key Fingerprints"])

    %% PL (Send)
    pl_send["Privacy Ledger<br>(Send Tx)"]
    getblock_send(["Get Latest<br>Block Number"])
    derivesendkey(["Derive Ephemeral<br>(Symmetric) Key"])
    calcR_send(["Calculate<br>Random Factor"])
    tx_commits(["Generate 'k'<br>Pedersen Commitments"])
    nullifier(["Calculate<br>Nullifier"])
    zk_proof(["Create<br>ZK Proof"])
    encrypt_ad(["Encrypt Additional Data<br>(w/ ephemeral key)"])
    send_tx(["Send commits, nullifier, zk proof, and ciphertext"])

    %% PL (Receive)
    pl_receive["Privacy Ledger<br>(Receive Tx)"]
    derivereceivekey(["Derive Ephemeral<br>(Symmetric) Key"])
    calcR_receive(["Calculate<br>Random Factor"])
    getblock_receive(["Get Latest<br>Block"])

    pl -.-> pl_setup & pl_send & pl_receive

    pl_setup -.-> keygen -.-> register -.-> kem -.-> publish
    pl_send -.-> getblock_send -.-> derivesendkey -.-> calcR_send -.-> tx_commits -.-> nullifier -.-> zk_proof -.-> encrypt_ad -.-> send_tx
    pl_receive -.-> getblock_receive -.-> derivereceivekey -.-> calcR_receive

```


### Blockchain
```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR

    %% Entities
    b["Blockchain"]

    %% Blockchain (Verifier)
    verify_tx(["Blockchain<br>(Verify Tx)"])
    check_nullifier(["Check if nullifier<br>exists"])
    check_commit(["Check if commitments add up to 0"])
    check_zk(["Check ZK Proof"])
    tally(["Approve Tx"])


    b -.-> verify_tx -.-> check_nullifier -.-> check_commit -.-> check_zk -.-> tally
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

## Implementation Details
* **Client**: Golang
* **Circuits**: Gnark
* **Verifier**: Solidity


## Peer-Reviewed Publications
- [Rayls: A Novel Design for CBDCs](https://eprint.iacr.org/2025/1639), published at [The 6th Workshop on Coordination of Decentralized Finance (CoDecFin) 2025](https://fc25.ifca.ai/codecfin/)
- [Rayls II: Fast, Private, and Compliant CBDCs](https://eprint.iacr.org/2025/1638), published at [Financial Cryptography in Rome (FCiR) 2025](https://www.decifris.it/fcir25/)
