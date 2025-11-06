### Gnark Server

The Gnark Server provides a zero-knowledge proof (ZK-SNARK) verification layer for Enygma’s payment and DVP (Delivery vs. Payment) systems.
It is designed around three independent circuits, each validating a different transaction type while ensuring data integrity and privacy through cryptographic constraints.

#### 1. Enygma Circuit

Validates whether an Enygma Payment transaction conforms to the defined ZK constraints.

File: pkg/circuits/enygma/circuit.go

Purpose: Ensures that all payment transactions follow Enygma’s protocol rules.

#### 2. Withdraw Circuit

Validates withdrawal transactions from the Enygma Payment system to the Enygma DVP layer.

File: pkg/circuits/withdraw/circuit.go

Purpose: Checks the integrity and structure of multi-part withdrawal operations.

#### 3. Deposit Circuit

Validates deposit transactions from Enygma DVP back into the Enygma Payment system.

File: pkg/circuits/deposit/circuit.go

Purpose: Ensures that deposited assets meet ZK constraints and correctly update payment balances.

### Keys

Keys are required for proof generation and verification.
Each circuit has its own proving key (Pk) and verification key (Vk):

1. Enygma keys: `keys/EnygmaPk.key ` and `keys/EnygmaVk.key `
2. ZkDvp Withdraw keys : `keys/zkdvp/WithdrawPkN.key` and `keys/zkdvp/WithdrawVkN.key`
3. ZkDvp Deposit keys : `keys/DepositPk.key` and `keys/zkdvp/DepositVk.key`

ps: Withdraw keys has 1 to 6 keys because withdraw transaction can be split up to 6 part.

### Run Gnark Server

To start the Gnark API server:

```javascript
go run cmd/server/main.go
```

This launches the local ZK-SNARK service that listens for proof-generation and verification requests.

### Tech Stack

- Language: Go

- ZKP Framework: Gnark

- Proof System: Groth16

- Field: Baby Jubjub

- Purpose: Enforce verifiable privacy-preserving transactions across Enygma’s payment ecosystem.
