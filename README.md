# Enygma
At Rayls, we have created a new suite of privacy protocols, which we call Enygma. Concretely, there are two variants of Enygma: Payments and Delivery-vs-Payment (DvP)


## [Enygma Payments](./enygma_payments)


```mermaid
 ---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
flowchart LR
a["User"]
pl_a["Privacy<br>Ledger A "]
sh("Subnet Hub")

a <-->pl_a <--> sh

```


## [Enygma DvP](./enygma_dvp)
