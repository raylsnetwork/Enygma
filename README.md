# Enygma
At Rayls, we have created a new suite of privacy protocols, which we call Enygma. 

Concretely, there are two variants of Enygma: 

* [Enygma Payments](./enygma_payments) (Account-based)
* [Enygma Delivery-vs-Payment (DvP)](./enygma_dvp) (UTXO-based)

## System Architecture

* **Users**: Traditional users of the system who want to transact with other users. 

* **Privacy Node(s)**: High-performance single node EVM blockchain(s) designed to support the needs of financial institutions and high-throughput. 

* **Private Network Hub**: Underlying blockchain that acts as a global ledger, (ZK) verifier, and bulletin board for Privacy Nodes to transact and communicate. 

* **Auditor** (optional): The auditor is an entity that oversees the transactions that take place in the system. This exists solely in case the system is deployed in a permissioned setting with the involvement of financial institutions. We note that the auditor only has a complete view of transactions that happen in the network iff the participants share the 'view' key with the auditor. We note that the auditor has no ability to spend funds on behalf of any system entity. 

```mermaid
 ---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR
a["User A"]
pl_a["Privacy<br>Ledger A "]
sh("Private Network Hub")
pl_b["Privacy<br>Ledger B "]
b["User B"]

auditor["Auditor<br>(Optional)"]
issuer["Issuer"]

a <-->pl_a <--> sh
auditor <-.-> sh
sh <--> pl_b <--> b
sh <-.-> issuer
```

### Adversarial Model
We assume that the Private Network Hub runs a Byzantine Fault Tolerant consensus and provides both safety and liveness. 

We assume an active network adversary with complete view of the network. Therefore, the adversary actively tries to infer which parties are transacting given the inboud/outbound messages in the system. 

We assume every Privacy Node runs a full node of the underlying blockchain (i.e., the Private Network Hub). As a result, Privacy Nodes can always perform a simple Private Information Retrieval (PIR) protocol that downloads the latest block and performs the lookups locally. 

Additionally, if present, we assume that the auditor is honest/trusted. In other words, the auditor will ensure the privacy of all the key material and sensitive data. 

## Helpful Mental Model
If the reader is familiar with the Ethereum ecosystem, the easiest way to think about our approach is probably the following:

The Private Network Hub is an underlying L1. The Privacy Nodes are (somewhat) equivalent to high-performance custom (validium) L2s. The balance (aka TVL) of each L2 is in the underlying L1 in a shielded manner to ensure the privacy of each institution. All the different shielded balances are recorded in a single L1 contract to ensure the liquidity is unified, as opposed to fragmented across different contracts. This approach ensures that entities can quickly transact with each other without very expensive operations on the underlying L1. 
