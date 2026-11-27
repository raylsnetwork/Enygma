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

For production keys, follow the [ceremony release procedure](../../enygma_ceremony/README.md#approving-production-keys-and-verifiers)
and [payments deployment checks](../run_scripts/README.md#production-ceremony-keys).

Generating development keys

The keys (except the gitignored Burn keys) are committed. After a circuit
change, regenerate them all from the repo root, which also stamps them with
their circuit sources (`keys/keys.src`, committed, and `keys/burn.src`):

```bash
make payments-keygen
```

`make payments-keys-check` (part of `make check`) fails until then, and names
the verifiers to copy into `../contracts/enygma/contracts/` afterwards.

---

### Run Gnark Server

To start the Gnark API server:

```bash
go run cmd/server/main.go
```

The prover listens on `127.0.0.1:8080` by default. Run one prover per bank, on the
same host or in the same private network namespace as its payment client. Limit
local access to bank-controlled processes and leave the port unpublished. Keep
witnesses in JSON request bodies and keep those bodies out of logs and traces.

#### Spending witnesses and remote proving

The prover receives `secret_key`, which gives spending authority, as well as
private balances and amounts and their commitment blinding factors. The transfer,
fee, USDr, deposit and withdrawal requests also contain shared secrets used for
fingerprints or tags. Burn takes the spend key and the balance opening. The proof
and public signals can leave the bank after proving.

A remote prover must be operated within the same bank's trust boundary. Before
enabling it, the bank must approve the host and its operators for access to those
witnesses. The server rejects a non-loopback bind unless it can start with mutual
TLS and all of the following settings:

| Setting | Value |
| --- | --- |
| `GNARK_BIND_HOST` | The IP address of the bank's prover interface |
| `GNARK_ALLOW_REMOTE` | `1`, the bank operator's explicit opt-in |
| `GNARK_TLS_CERT` | Server certificate chain in PEM format |
| `GNARK_TLS_KEY` | Server private key in PEM format |
| `GNARK_TLS_CLIENT_CA` | PEM CA bundle dedicated to that bank's authorized proving clients |

Remote clients use HTTPS, verify the server's identity and present a client
certificate issued by that CA. Use a certificate with a SAN for the DNS name or IP
the client uses. The bundled Go payment client and demo use loopback. A remote
integration must supply its own mTLS client configuration and keep the request
inside the bank-controlled service.

### Statement Layouts

Each circuit's statement layout (the order, offsets and sizes of its public signals) is derived from the circuit's own public fields (`enygma_gnark_shared/statement`, shared with enygma_retail_payments; e.g. `enygma.StatementLayout()`; likewise `enygma_fee`, `usdr`, `withdraw`, `deposit` and `burn`). Two committed files are generated from the layouts:

- `../layout/layout.go`: a dependency-free Go package imported by the relayer and the test suites (`../test`, `../go_client/enygma_test`);
- `../contracts/enygma/contracts/StatementLayout.sol`: the constants library used by `Enygma.sol`, whose `*_LEN` constants also size `IEnygma.sol`'s public-signal arrays.

After changing any circuit's public signals, regenerate both:

```bash
go generate ./cmd/genlayout
```

`go test ./cmd/genlayout` fails while either file is stale. It also pins the arity in each of `Enygma.sol`'s `verifyProof(uint256[8],uint256[N])` selector strings to its circuit's statement length, since Solidity cannot build those strings from the generated constants.

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
