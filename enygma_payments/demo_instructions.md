# Enygma Payment Demo - Quick Start Guide

This guide walks through a complete end-to-end demo: deploy the smart contracts, start the ZK proof server, and run a confidential payment that is verified on-chain.

## Overview

What happens in the demo:

1. **Smart contract deployment** — `Enygma.sol` (the payment contract) and `Verifier.sol` (the Groth16 verifier) are deployed to a local chain.
2. **ZK proof generation** — a gnark server generates a Groth16 proof that a payment of 100 tokens is valid without revealing sender/receiver balances.
3. **On-chain verification** — the Enygma contract calls the verifier, checks the proof, and updates the homomorphic balance commitments.

```
  Go integration test
         │
         │ 1. POST /proof/enygma (inputs)
         ▼
  ┌─────────────────┐
  │  gnark server   │  generates Groth16 proof (~30s)
  └────────┬────────┘
           │ 2. proof + public signals
           ▼
  Go integration test
           │
           │ 3. Transfer(commitmentDeltas, proof, participantIds)
           ▼
  ┌──────────────────────┐
  │  Enygma.sol          │  verifies proof on-chain
  │  + Verifier.sol      │  updates balance commitments
  └──────────────────────┘
```

## Prerequisites

- **Go** 1.21+
- **Node.js** 18+ with `npx hardhat` available (from the `enygma_dvp` project)
- **Python 3.8+** with the `run_scripts/.venv` virtualenv (created with `pip install -r run_scripts/requirements.txt`)
- **macOS**: use `CC=/usr/bin/clang` for all Go commands that import go-ethereum (CGo dependency)

## Step 1 — Start a local Hardhat node

The integration test expects a chain at `localhost:8545` with `chainId=1337`. Use the Hardhat node from the `enygma_dvp` project, which has the correct account configuration:

```bash
# In a dedicated terminal — keep this running
cd ../enygma_dvp
npx hardhat node
```

> **Important:** the test uses a fixed sender private key tied to account[0] of this node's mnemonic. Start from a **fresh node** for every test run — the test uses fixed Pedersen commitment randomness that does not survive chain reuse.

## Step 2 — Start the gnark server

The gnark server exposes `POST /proof/enygma` and generates Groth16 proofs on request. It must be started from the `gnark-server/` directory because key paths are relative.

```bash
# In a dedicated terminal — keep this running
cd gnark-server
go run ./cmd/server/main.go
```

Expected output:

```
[GIN-debug] POST   /proof/enygma             --> enygma-server/pkg/circuits/enygma.NewHandler.func1 (3 handlers)
[GIN-debug] POST   /proof/withdraw/1         --> enygma-server/pkg/circuits/withdraw.NewHandler.func1 (3 handlers)
...
[GIN-debug] Listening and serving HTTP on :8080
```

## Step 3 — Deploy the contracts

Deploy `Enygma.sol` and `Verifier.sol` to the running node. The script reads compiled artifacts from `contracts/enygma/artifacts/` and writes the deployed addresses to `run_scripts/build/enygma/web3/deploy_receipts.json`.

```bash
cd run_scripts
.venv/bin/python3 deploy_direct.py
```

Expected output:

```
Deployer: 0x0F1013e0e46B97144b25b3131668EF99858BD8D0
Block: 0

Deploying Enygma...
  deployed at: 0x...

Deploying EnygmaVerifier...
  deployed at: 0x...

Wrote .../deploy_receipts.json
```

> If the Hardhat artifacts need to be regenerated (e.g. after editing `Enygma.sol`), run `npx hardhat compile` from `contracts/enygma/` first.

## Step 4 — Run the integration test

The test registers 6 banks, mints 500 tokens to bank 0, requests a ZK proof from the gnark server, submits the `Transfer` transaction, and verifies the homomorphic balance update.

```bash
cd go_client/enygma_test
CC=/usr/bin/clang go test -run TestFullTransactionFlow -v -timeout 300s
```

Proof generation takes ~30 seconds. Expected output:

```
=== RUN   TestFullTransactionFlow
    transaction_test.go: TOKEN:    0x...
    transaction_test.go: VERIFIER: 0x...
    transaction_test.go: contract initialized
    transaction_test.go: verifier registered
    transaction_test.go: registered 6 banks (accountIds 1–6)
    transaction_test.go: minted 500 to bank 0 (accountId=1)
    transaction_test.go: requesting proof (may take ~30s)...
    transaction_test.go: proof received
    transaction_test.go: Transfer succeeded: 0x...
    transaction_test.go: bank 0 homomorphic check PASSED (prevBal + txDelta = newBal)
    transaction_test.go: bank 1 homomorphic check PASSED
--- PASS: TestFullTransactionFlow (1.77s)
```

## Re-running the demo

Restart the Hardhat node (step 1) before each run — this resets the chain state. Steps 2–4 can then be repeated without restarting the gnark server.

## Key files

| File | Purpose |
|---|---|
| `contracts/enygma/contracts/Enygma.sol` | Core payment contract |
| `contracts/enygma/contracts/EnygmaVerifier.sol` | Groth16 verifier (generated from `gnark-server/keys/EnygmaVerifier.sol`) |
| `gnark-server/keys/EnygmaVerifier.sol` | Canonical verifier source — regenerate with `gnark-server/keygen/generate_keys.go` if keys change |
| `run_scripts/deploy_direct.py` | Deploys both contracts and writes `deploy_receipts.json` |
| `go_client/enygma_test/transaction_test.go` | End-to-end integration test |
| `test/enygma_test.go` | Standalone unit tests (no chain or server required) |
