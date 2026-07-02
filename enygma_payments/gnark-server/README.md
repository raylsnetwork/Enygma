### Gnark Server

The Gnark Server is a zero-knowledge proof (ZK-SNARK) service that provides cryptographic verification for Enygma's privacy-preserving payment and DVP (Delivery vs. Payment) systems. It generates and verifies proofs using the Groth16 proof system on the BabyJubJub curve.

#### Installation

1. Navigate to Server Directory

```bash
cd gnark-server
```

2. Install Dependencies

```bash
go mod download
```

#### ⚠️ Proving keys and Verification Keys are only for demo purpose ‼️

3. Verify Keys are Present

```bash
#Check if keys directory exists
ls -la keys/

# Expected output:
# keys/EnygmaPk.key
# keys/EnygmaVk.key
# keys/zkdvp/WithdrawPk1.key to WithdrawPk6.key
# keys/zkdvp/WithdrawVk1.key to WithdrawVk6.key
# keys/zkdvp/DepositPk.key
# keys/zkdvp/DepositVk.key
```

#### Circuit Overview

##### 1. Enygma Circuit

File: `pkg/circuits/enygma/circuit.go`

Purpose: Validates standard private payment transactions in the Enygma system.

##### 2. Withdraw Circuit

File: `pkg/circuits/withdraw/circuit.go`

Purpose: Validates withdrawals from Enygma Payment layer to Enygma DVP (Delivery vs Payment) layer.

##### 3. Deposit Circuit

File: `pkg/circuits/deposit/circuit.go`

Purpose: Validates deposits from Enygma DVP back into the Enygma Payment system.

### Keys

Keys are required for proof generation and verification.
Each circuit has its own proving key (Pk) and verification key (Vk):

Keys Files Location

```
keys/
│
│
├── EnygmaPk.key
├── EnygmaVk.key
│
└── 📁 zkdvp/
    │
    ├── WithdrawPk1.key
    ├── WithdrawVk1.key
    ├── WithdrawPk2.key
    ├── WithdrawVk2.key
    ├── WithdrawPk3.key
    ├── WithdrawVk3.key
    ├── WithdrawPk4.key
    ├── WithdrawVk4.key
    ├── WithdrawPk5.key
    ├── WithdrawVk5.key
    ├── WithdrawPk6.key
    ├── WithdrawVk6.key
    ├── DepositPk.key
    └── DepositVk.key
```

Generating Keys

If Keys are not present, you can generate them in

```bash
go run ./keygen/generate_keys.go
```

---

### Run Gnark Server

To start the Gnark API server:

```bash
go run cmd/server/main.go
```

This launches the local ZK-SNARK service that listens for proof-generation and verification requests.

### Transparent Setup (Generator H)

The second generator H used in Pedersen commitments is derived from a nothing-up-my-sleeve number so that no one knows its discrete log relative to G. To reproduce or verify the derivation:

```bash
cd gnark-server
go run ./cmd/setup/main.go
```

This hashes the seed `1` via SHA256 repeatedly until the result is a valid Baby JubJub X-coordinate, clears the cofactor (×8), and verifies the resulting point on-chain via a Groth16 proof. The printed `Q.X` / `Q.Y` values are the canonical H coordinates hardcoded into the Enygma circuit.

---

### Tech Stack

- Language: Go

- ZKP Framework: Gnark

- Proof System: Groth16

- Field: Baby Jubjub

- Purpose: Enforce verifiable privacy-preserving transactions across Enygma’s payment ecosystem.
