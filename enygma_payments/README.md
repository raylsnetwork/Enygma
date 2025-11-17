# Enygma Payments

## System Architecture
Our system is simple: **users** (e.g., a bank customers) are directly connected to **privacy nodes** (i.e., a high-performance single-node EVM blockchain). Each of the privacy nodes, is connected to a **private network hub**, which effectively acts as a bulletin board for all privacy nodes to leverage as a universal (encrypted) messaging layer and verification layer. **Issuer(s)** are owners of specific assets on the private network hub. Optionally, there is an **auditor** that oversees (some of) the transactions that take place in the network. The protocol flows involving each of these entities are further described [here](./protocol_flows.md). Alternatively, a more formal protocol description is documented [here](./protocol_description.md).


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

Note: We want to update the ZK module to use a quantum-secure ZK scheme. This update will make the entire system quantum-secure.

## Implementation Details
* **Client**: Golang
* **ZK Circuit(s)**: Gnark
* **Verifier**: Solidity


## Peer-Reviewed Publications
- [Rayls: A Novel Design for CBDCs](https://eprint.iacr.org/2025/1639), published at [The 6th Workshop on Coordination of Decentralized Finance (CoDecFin) 2025](https://fc25.ifca.ai/codecfin/)
- [Rayls II: Fast, Private, and Compliant CBDCs](https://eprint.iacr.org/2025/1638), published at [Financial Cryptography in Rome (FCiR) 2025](https://www.decifris.it/fcir25/)
