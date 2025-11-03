# Enygma
At Rayls, we have created a new suite of privacy protocols, which we call Enygma. 

Concretely, there are two variants of Enygma: 

* [Enygma Payments](./enygma_payments)
* [Enygma Delivery-vs-Payment (DvP)](./enygma_dvp)

## System Architecture

* **Users**:
* **Privacy Ledger(s)**:
* **Subnet Hub**:

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
sh("Subnet Hub")
pl_b["Privacy<br>Ledger B "]
b["User B"]

auditor["Auditor"]

a <-->pl_a <--> sh <--> pl_b <--> b
auditor <--> sh

```

## Enygma Payments


## Enygma DvP
