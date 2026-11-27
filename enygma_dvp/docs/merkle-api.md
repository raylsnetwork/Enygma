# Gnark Server — Utility Endpoints

Base URL: `http://localhost:8081`

---

## POST /util/merkleStatus

Returns the Merkle tree status for **all vaults**, plus a cross-check that verifies the vault addresses registered in the EnygmaDvP contract match those in `build/receipts.json`.

### Request body (all fields optional)

```json
{
  "rpcUrl": "http://127.0.0.1:8545",
  "receiptsPath": "../build/receipts.json"
}
```

| Field          | Type   | Default                  | Description                      |
| -------------- | ------ | ------------------------ | -------------------------------- |
| `rpcUrl`       | string | `http://127.0.0.1:8545`  | Ethereum JSON-RPC endpoint       |
| `receiptsPath` | string | `../build/receipts.json` | Path to deployment receipts file |

### Response

```json
{
  "enygmaDvpRegistryCheck": {
    "enygmaDvpAddress": "0x6b01819C...",
    "allMatch": true,
    "entries": [
      {
        "vaultId": 0,
        "name": "Erc20CoinVault",
        "addressInDvP": "0x20c856d1...",
        "addressInReceipts": "0x20c856d1...",
        "match": true
      }
    ],
    "error": "string (omitted if no error)"
  },
  "vaults": [
    {
      "name": "Erc20CoinVault",
      "address": "0x20C856d1...",
      "onChainRoot": "19309031...",
      "localRoot": "19309031...",
      "match": true,
      "leafCount": 9,
      "treeNumber": 0,
      "tree": {
        "depth": 16,
        "root": "19309031...",
        "levels": [
          ["6081264...", "7393454...", "..."],
          ["1874983...", "7467260...", "..."],
          "...",
          ["19309031..."]
        ],
        "zeros": ["21786163...", "18548196...", "..."]
      },
      "error": "string (omitted if no error)"
    }
  ]
}
```

### Response fields

#### `enygmaDvpRegistryCheck`

| Field                         | Type   | Description                                                        |
| ----------------------------- | ------ | ------------------------------------------------------------------ |
| `enygmaDvpAddress`            | string | Address of the EnygmaDvP contract from receipts                    |
| `allMatch`                    | bool   | `true` if all vault addresses match between EnygmaDvP and receipts |
| `entries[].vaultId`           | uint64 | Vault index in EnygmaDvP (0–3)                                     |
| `entries[].name`              | string | Vault contract name                                                |
| `entries[].addressInDvP`      | string | Address returned by `vaultById(id)` on EnygmaDvP                   |
| `entries[].addressInReceipts` | string | Address from `build/receipts.json`                                 |
| `entries[].match`             | bool   | Whether the two addresses agree                                    |
| `error`                       | string | Set if the check could not be completed                            |

#### `vaults[]`

| Field         | Type         | Description                                                                                    |
| ------------- | ------------ | ---------------------------------------------------------------------------------------------- |
| `name`        | string       | Vault contract name                                                                            |
| `address`     | string       | Vault contract address                                                                         |
| `onChainRoot` | string       | Current Merkle root from `currentRoot()` on the vault contract                                 |
| `localRoot`   | string       | Root computed locally by replaying on-chain `Commitment` events                                |
| `match`       | bool         | `true` if `localRoot == onChainRoot`                                                           |
| `leafCount`   | int          | Total number of `Commitment` events found on-chain                                             |
| `treeNumber`  | uint64       | Current tree number (increments when a batch would reach the last slot: at most 65,535 leaves) |
| `tree.depth`  | int          | Tree depth (`MerkleTreeDepth` in `gnark_circuits/templates/deployed.go`, 16)                   |
| `tree.root`   | string       | Same as `localRoot`                                                                            |
| `tree.levels` | `[][]string` | All tree levels in decimal. `levels[0]` = leaves, `levels[depth]` = `[root]`                   |
| `tree.zeros`  | `[]string`   | Zero values per level. `zeros[i]` is the implicit sibling when a node at level `i` has no pair |
| `error`       | string       | Set if this vault could not be checked                                                         |

---

## POST /util/merkleVault

Returns the Merkle tree status for a **single vault**, identified by name or vault ID.

### Request body

Provide either `vault` or `vaultId` — not both required.

```json
{
  "vault": "Erc20CoinVault",
  "vaultId": 0,
  "rpcUrl": "http://127.0.0.1:8545",
  "receiptsPath": "../build/receipts.json"
}
```

| Field          | Type   | Required     | Description                                                         |
| -------------- | ------ | ------------ | ------------------------------------------------------------------- |
| `vault`        | string | one of these | Vault name (see table below)                                        |
| `vaultId`      | uint64 | one of these | Vault ID 0–3 (see table below)                                      |
| `rpcUrl`       | string | no           | Ethereum JSON-RPC endpoint (default `http://127.0.0.1:8545`)        |
| `receiptsPath` | string | no           | Path to deployment receipts file (default `../build/receipts.json`) |

#### Valid vault identifiers

| `vaultId` | `vault`                | Asset type                   |
| --------- | ---------------------- | ---------------------------- |
| `0`       | `Erc20CoinVault`       | ERC-20                       |
| `1`       | `Erc721CoinVault`      | ERC-721                      |
| `2`       | `Erc1155CoinVault`     | ERC-1155                     |
| `3`       | `EnygmaErc20CoinVault` | Enygma ERC-20 (private mint) |

### Response

Same shape as a single entry in `vaults[]` from `/util/merkleStatus`:

```json
{
  "name": "Erc20CoinVault",
  "address": "0x20C856d1...",
  "onChainRoot": "19309031...",
  "localRoot": "19309031...",
  "match": true,
  "leafCount": 9,
  "treeNumber": 0,
  "tree": {
    "depth": 16,
    "root": "19309031...",
    "levels": [
      ["6081264...", "7393454...", "..."],
      ["1874983...", "..."],
      "...",
      ["19309031..."]
    ],
    "zeros": ["21786163...", "18548196...", "..."]
  },
  "error": "string (omitted if no error)"
}
```

---

## How `match` is computed

For each vault the service:

1. Calls `currentRoot()` on the vault contract via `eth_call` to get the on-chain root.
2. Fetches all `Commitment(uint256 indexed vaultId, uint256 indexed commitment)` events emitted by the vault contract via `eth_getLogs`.
3. Replays the commitments in order into a local Poseidon Merkle tree (depth `MerkleTreeDepth`, 16, zero value = `keccak256("ZkDvp") % SNARK_SCALAR_FIELD`), mirroring the contract's tree algorithm.
4. Compares the computed local root against the on-chain root.

`match: true` means the local reconstruction agrees with the contract state.

---

## Example — check a single vault with Postman

- Method: `POST`
- URL: `http://localhost:8081/util/merkleVault`
- Body → raw → JSON:

```json
{
  "vaultId": 0
}
```

---

## Example — curl

```bash
# All vaults
curl -s -X POST http://localhost:8081/util/merkleStatus \
  -H 'Content-Type: application/json' \
  -d '{}' | python3 -m json.tool

# Single vault by ID
curl -s -X POST http://localhost:8081/util/merkleVault \
  -H 'Content-Type: application/json' \
  -d '{"vaultId": 0}' | python3 -m json.tool

# Single vault by name
curl -s -X POST http://localhost:8081/util/merkleVault \
  -H 'Content-Type: application/json' \
  -d '{"vault": "Erc721CoinVault"}' | python3 -m json.tool
```
