# Enygma
At Rayls, we have created a new suite of privacy protocols, which we call Enygma. Concretely, there are two variants of Enygma: Payments and Delivery-vs-Payment (DvP)


## System Architecture

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

a <-->pl_a <--> sh <--> pl_b <--> b

```

## [Enygma Payments](./enygma_payments)


## [Enygma DvP](./enygma_dvp)
