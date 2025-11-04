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
    UD(["User(s)"])

    UE(["User(s)"])
    UF(["User(s)"])
    UG(["User(s)"])
    UH(["User(s)"])

    PLA(["Privacy Ledger"])
    PLB(["Privacy Ledger"])
    PLC(["Privacy Ledger"])
    PLD(["Privacy Ledger"])

    B(["Blockchain"])
    PLE(["Privacy Ledger"])
    PLF(["Privacy Ledger"])
    PLG(["Privacy Ledger"])
    PLH(["Privacy Ledger"])

    PLA & PLB & PLC & PLD <-.-> B <-.-> PLE & PLF & PLG & PLH

    UA <-.-> PLA
    UB <-.-> PLB
    UC <-.-> PLC
    UD <-.-> PLD

    PLE <-.-> UE
    PLF <-.-> UF
    PLG <-.-> UG
    PLH <-.-> UH
```

## Sub-Protocols

First, each privacy ledger registers two keypairs (view and spend) on the underlying blockchain. This blockchain effectively acts as a Public-Key Infrastructure (PKI) containing a registry of all public-keys of the registered privacy ledgers. Second, each privacy ledger perform a post-quantum key agreement (i.e., ML-KEM) and establish individual shared secrets with all the other privacy ledgers. At this point, privacy ledgers can now start transacting privately with each other. The transaction protocol includes a hash-based private messaging tag component that allow recipients to detect privately whether or not a transaction is for them. Therefore, we also introduce a protocol to fetch (and decrypt) transactions.

## Protocol Flows
```mermaid
---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR
    issuer["Issuer"]
    i_setup["Issuer<br>(Setup)"]
    deploy(["Deploy Enygma<br>Contract"])

    i_mint(["Issuer<br>(Mint)"])
    mint_shield(["Mint<br>(Shielded)Funds"])
    mint_transparent(["Mint<br>(Transparent) Funds"])

    pl["Privacy Ledger"]
    pl_setup["Privacy Ledger<br>(Setup)"]
    keygen(["Key<br>Generation"])
    register(["Key<br>Registration"])
    kem(["Key<br>Agreement"])
    publish(["Publish<br>Key Fingerprints"])

    pl_send["Privacy Ledger<br>(Send Tx)"]
    getblock(["Get Latest<br>Block"])
    derivekey(["Derive Ephemeral<br>(Symmetric) Key"])
    calcR(["Calculate<br>Random Factor"])

    pl_receive["Privacy Ledger<br>(Receive Tx)"]


    issuer -.-> i_setup & i_mint
    i_setup -.-> deploy
    i_mint -.-> mint_shield & mint_transparent

    pl -.-> pl_setup & pl_send & pl_receive

    pl_setup -.-> keygen -.-> register -.-> kem -.-> publish
    pl_send -.-> getblock -.-> derivekey & calcR
    pl_receive -.-> getblock -.-> derivekey & calcR
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
