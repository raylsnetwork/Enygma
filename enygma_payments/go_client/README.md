## Enygma Go Client

The Enygma Go Client is a command-line tool that enables private transactions using zero-knowledge proofs. It generates cryptographic commitments, communicates with the gnark proof server, and submits verified transactions to the Enygma smart contracts.

#### Overview

The Go Client performs the following operations:

1. Generates Pedersen Commitments - Creates cryptographic commitments for transaction values
2. Requests ZK Proofs - Communicates with the gnark server to generate zero-knowledge proofs
3. Submits Transactions - Sends verified transactions to the Enygma smart contract
4. Maintains Privacy - Ensures transaction details remain confidential while being verifiable

Key Features:

- Private multi-party payments
- Nullifier-based double-spend protection
- Pedersen commitment scheme for value hiding

---

#### Configuration

Environment Setup
The client requires several configuration values that are currently hardcoded in main.go. Before running, ensure these are set correctly:

1. Contract Address Configuration

The contract address is read from `.config/address.json`:

```json
{
  "address": "0xYourEnygmaContractAddress"
}
```

1. Network Configuration (in `./config/config.go`)

```go
// RPC endpoint for the blockchain
CommitChainURL = "http://127.0.0.1:8545"  // Change for different networks

// Gnark proof server URL
ProofServerURL = "http://127.0.0.1:8080/proof/enygma"

// Private key for signing transactions (DO NOT commit this!)
PrivateKey = "YOUR_PRIVATE_KEY_HERE"
```

3. Bank Secrets Configuration (in `./transaction/main.go`)

This is only for demo purpose. It was randomly created. Please refer to protocol description to read how to proper manage secret

```go
// Secret values for each bank (used for commitment randomness)
secrets = [][]*big.Int{
  {
			big.NewInt(412321),
			big.NewInt(634609235),
			big.NewInt(8352331231),
			big.NewInt(289412412),
			big.NewInt(8932589237),
			big.NewInt(423423523),
		},
    ....
}
```

#### Usage

Basic Command

```bash
go run ./transaction/main.go <qtyBank> <value> <senderId> <sk> <previousV> <previousR>

```

Transaction Values Configuration

The transaction distribution is configured in `./transaction/main.go`:

```go
// Current configuration (modify as needed)
txValues := []*big.Int{
    vNegate,          // Position 0: Sender (negative value)
    big.NewInt(60),   // Position 1: Bank 1 receives 60
    big.NewInt(40),   // Position 2: Bank 2 receives 40
    big.NewInt(0),    // Position 3: Bank 3 receives 0
    big.NewInt(0),    // Position 4: Bank 4 receives 0
    big.NewInt(0),    // Position 5: Bank 5 receives 0
}
```

**Note**: The sum of all values (excluding sender) must equal the value being sent. The sender's value is automatically negated.

KIndex Participation configuration

KIndex array is configured in `./transaction/main.go`:

```go

  k0:= big.NewInt(0)
  k1:= big.NewInt(1)
  k2:= big.NewInt(2)
  k3:= big.NewInt(3)
  k4:= big.NewInt(4)
  k5:= big.NewInt(5)


	kIndex := []*big.Int{k0, k1, k2,k3,k4,k5}

```

Examples

```bash
go run . 6 100 0 35 1000 0
```

Breakdown:

- 6 banks in the network
- 100 total tokens to send
- 0 is the sender ID (Bank 0)
- 35 is the sender's secret key
- 1000 was the previous transaction value for this account
- 0 was the previous randomness

---

**Observation**

The following folder and files are for `enygma_dvp` integration

- `go_client/zkdvp/`
- `go_client/interfacezkdvp/`
- `go_client/utils/`

---
