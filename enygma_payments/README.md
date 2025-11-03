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
    PLA(["Privacy Ledger A"])
    PLB(["Privacy Ledger B"])
    PLC(["Privacy Ledger C"])
    PLD(["Privacy Ledger D"])

    B(["Blockchain"])
    PLE(["Privacy Ledger E"])
    PLF(["Privacy Ledger F"])
    PLG(["Privacy Ledger G"])
    PLH(["Privacy Ledger H"])

    PLA & PLB & PLC & PLD <--> B <--> PLE & PLF & PLG & PLH
```

## Sub-Protocols
To simplify the protocol, we compose it into different parts. Concretely: 

* Registration
* Key Agreement
* Sending a Transaction
* Receiving a Transaction

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
- [Rayls II: Fast, Private, and Compliant CBDCs](https://eprint.iacr.org/2025/1638), published at [FCiR25 - Financial Cryptography in Rome 2025](https://www.decifris.it/fcir25/)
