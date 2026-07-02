# Enygma Go Client

Go module for interacting with the Enygma smart contracts. Provides the ABI binding, internal cryptographic helpers, and the end-to-end integration test.

## Structure

```
contracts/          ABI binding generated from Enygma.sol (enygma.go)
internal/
  contract/         Ethereum client wrapper
  curve/            Baby Jubjub curve constants (G, H, P)
  randomness/       Shared-secret and commitment derivation
  proof/            gnark server HTTP client
  types/            Shared types
transaction/        Standalone CLI for manual demo transactions
enygma_test/        End-to-end integration test (separate Go module)
zkdvp/              DVP integration helpers
interfacezkdvp/     DVP contract interface
```

## Running the integration test

The integration test is the primary way to verify the full flow end-to-end. It registers banks, mints tokens, requests a ZK proof from the gnark server, submits a `Transfer`, and checks the homomorphic balance update.

```bash
cd enygma_test
CC=/usr/bin/clang go test -run TestFullTransactionFlow -v -timeout 300s
```

Prerequisites: Hardhat node on `:8545`, gnark server on `:8080`, contracts deployed via `run_scripts/deploy_direct.py`. See `demo_instructions.md` for the full setup sequence.

## Running the standalone CLI

`transaction/main.go` is a manual demo tool for running a single transaction from the command line. It reads the contract address from `config/address.json` (update this after each deployment).

```bash
cd go_client
go run ./transaction/main.go <qtyBank> <value> <senderId> <sk> <previousV> <previousR>
```

**Note:** `zkdvp/`, `interfacezkdvp/`, and `utils/` support the `enygma_dvp` integration layer and are not used by the core payment flow.
