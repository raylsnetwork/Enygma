# Enygma Protocol Diagrams

## Key Registration
```mermaid
 ---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
  sequenceDiagram
      participant pla as Privacy Ledger A
      participant b as Blockchain
      participant plb as Privacy Ledger B

      pla -->> b: hi
```


## Key Agreement
```mermaid
 ---
config:
  theme: redux
  layout: elk
  look: handDrawn
---
  sequenceDiagram
      participant pla as Privacy Ledger A
      participant b as Blockchain
      participant plb as Privacy Ledger B

      pla -->> plb: $$x + 3$$

```
