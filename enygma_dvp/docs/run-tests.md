# Running the Full Test Suite Locally

This guide covers every step needed to run the integration tests from a clean state,
starting from a fresh Hardhat node.

You will need **3 terminals** open at the project root (`enygma_dvp/`).

---

## Prerequisites

- Go 1.22+
- Node.js + npm (`npx hardhat` available)
- `solc` installed and on PATH (for `cmd/patch_verifier`)
- gnark circuit keys already generated in `gnark_circuits/scripts/keys/`

---

## Terminal 1 — Start the local Hardhat node

```bash
npx hardhat node
```

Leave this running. It listens on `http://127.0.0.1:8545`.

---

## Terminal 2 — Deploy and initialise contracts

### Step 1 — Regenerate Poseidon artifacts

> Required because `npx hardhat compile` overwrites the Poseidon artifacts with
> placeholder implementations. Always run this before deploying.

```bash
node scripts/regen_poseidon.js
```

Expected output:
```
Poseidon artifacts regenerated
```

---

### Step 2 — Deploy contracts

```bash
cd scripts && CC=/usr/bin/clang go build -o /tmp/deploy_contracts ./cmd/deploy
cd .. && /tmp/deploy_contracts
```

Expected output ends with:
```
All contracts deployed. Receipts saved to build/receipts.json
```

---

### Step 3 — Export gnark VKs to circom format

```bash
cd gnark_circuits && go run ./cmd/export_vk_init/ ../build
```

Expected output ends with:
```
Done: 25/25 VKs exported to ../build
```

---

### Step 4 — Initialise contracts (register VKs, vaults, tokens)

```bash
cd ../scripts && CC=/usr/bin/clang go build -o /tmp/init_contracts ./cmd/init
cd .. && /tmp/init_contracts
```

Expected output ends with:
```
Initialisation complete.
```

---

## Terminal 3 — Start the gnark proof server

```bash
cd gnark_circuits && go run ./cmd/server
```

Expected output:
```
[GIN-debug] Listening and serving HTTP on :8081
```

Leave this running.

---

## Terminal 2 — Run the integration tests

```bash
cd test && CC=/usr/bin/clang go test -run "TestV2Erc20OnChain_PrivateMint|TestV2Erc20Payment|TestV2DvP|TestV2DvP_WithDeadline" -v -timeout 300s
```

### Expected results

```
--- PASS: TestV2Erc20OnChain_PrivateMint  (0.35s)
--- PASS: TestV2Erc20Payment              (0.64s)
--- PASS: TestV2DvP                       (0.54s)
--- PASS: TestV2DvP_WithDeadline          (3.32s)
  --- PASS: TestV2DvP_WithDeadline/FullSwap          (1.22s)
  --- PASS: TestV2DvP_WithDeadline/DeadlineExpired   (0.76s)
  --- PASS: TestV2DvP_WithDeadline/InvalidProof      (1.34s)

ok  enygma_dvp/test  5.35s
```

---

## Terminal 2 — Verify Merkle tree status

After the tests complete, confirm the on-chain Merkle roots match the local reconstruction:

```bash
curl -s -X POST http://localhost:8081/util/merkleStatus \
  -H 'Content-Type: application/json' \
  -d '{}' | python3 -m json.tool
```

### Expected result

```json
{
  "enygmaDvpRegistryCheck": {
    "allMatch": true,
    ...
  },
  "vaults": [
    { "name": "Erc20CoinVault",       "match": true, "leafCount": 9 },
    { "name": "Erc721CoinVault",      "match": true, "leafCount": 5 },
    { "name": "Erc1155CoinVault",     "match": true, "leafCount": 0 },
    { "name": "EnygmaErc20CoinVault", "match": true, "leafCount": 0 }
  ]
}
```

All vaults must show `"match": true`.

---

## What each test covers

| Test | Circuit | Vault | Description |
|---|---|---|---|
| `TestV2Erc20OnChain_PrivateMint` | `privateMint` | ERC-20 | Mint a private note directly into the vault |
| `TestV2Erc20Payment` | `payment` | ERC-20 | 2-input / 2-output private transfer (Alice → Bob + change) |
| `TestV2DvP` | `dvpInitiator` + `dvpDestination` | ERC-20 + ERC-721 | Atomic swap: Alice's USDT for Bob's NFT ticket |
| `TestV2DvP_WithDeadline/FullSwap` | same | same | Full swap with deadline — both legs settle |
| `TestV2DvP_WithDeadline/DeadlineExpired` | same | same | Deadline expires — Alice reclaims via `claimSwapTimeout` |
| `TestV2DvP_WithDeadline/InvalidProof` | same | same | Bob submits invalid proof — rejected on-chain, Alice reclaims |

---

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `connect: connection refused` on port 8081 | gnark server not running | Start `go run ./cmd/server` in Terminal 3 |
| `connect: connection refused` on port 8545 | Hardhat node not running | Start `npx hardhat node` in Terminal 1 |
| Merkle root mismatch (`match: false`) | Poseidon artifacts were overwritten | Re-run `node scripts/regen_poseidon.js` and redeploy |
| `already minted` error in ERC-721 test | Stale Hardhat state from a previous run | Restart Hardhat node and redeploy from Step 1 |
| `log.Fatal` / server exits on proof request | Unsatisfied circuit constraints | Check that request inputs are consistent (values balance, Merkle path is correct) |
