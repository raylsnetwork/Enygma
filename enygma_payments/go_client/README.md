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

Fix M-10: `sk` (the spend authority), `previousV` and `previousR` (the sender's balance opening) are read from environment variables, not command-line arguments — `os.Args` is world-readable via `ps aux` / `/proc/<pid>/cmdline` on a shared host and persists in shell history.

```bash
cd go_client
ENYGMA_SK=<sk> ENYGMA_PREVIOUS_V=<balance> ENYGMA_PREVIOUS_R=<blinding> \
  go run ./transaction/main.go <qtyBank> <value> <senderId>
```

**Note (Fix L-13):** `zkdvp/`, `interfacezkdvp/`, and `utils/` — an experimental `enygma_dvp` integration layer, not used by the core payment flow — were removed. That code never compiled (`zkdvp/deposit.go` and `withdraw.go` both declared `func main()` in the same package), POSTed to `/relay/deposit` and `/relay/withdraw` routes the relayer never registered, and derived every DvP blinding factor from constants checked into the repository (`secrets := []*big.Int{1234567890, ...}`) rather than the ML-KEM `agreement` package — commitments built from it were never actually hiding. See the audit finding for the full analysis; its own remediation names deletion as an acceptable fix given the code shipped no working path to begin with.
