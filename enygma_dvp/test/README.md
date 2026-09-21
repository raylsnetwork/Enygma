# Integration Tests

These tests hit a live Hardhat node and a running gnark proof server — no mocks.
Three services need to be up before you run anything.

---

## Prerequisites

Before running any test, start all three services in order:

### 1. Hardhat node

```bash
cd <repo-root>
npx hardhat node
# Keep this running in a separate terminal
```

### 2. Deploy and initialize contracts

```bash
# Build and run the deploy script (from repo root)
cd scripts && go build -o /tmp/deploy_contracts ./cmd/deploy
cd .. && /tmp/deploy_contracts
# → writes build/receipts.json

# Export gnark verifying keys to circom format
cd gnark_circuits && go run ./cmd/export_vk_init/ ../build

# Build and run the init script (from repo root)
cd scripts && go build -o /tmp/init_contracts ./cmd/init
cd .. && /tmp/init_contracts
```

> **Poseidon artifact warning**: if you ran `npx hardhat compile` before deploying,
> the Poseidon artifacts may have been overwritten with empty stubs. See
> `contracts/core/contracts/Poseidon.sol` for details and the regeneration command.

### 3. Gnark proof server

```bash
cd gnark_circuits
go run main.go
# Starts on :8081 — keep this running in a separate terminal
```

---

## Running tests

Run all integration tests:

```bash
cd test
go test ./... -v -timeout 600s
```

Run a single test:

```bash
cd test
go test -run <TestFunctionName> -v -timeout 600s
```

If the Hardhat node or gnark server isn't reachable the test prints `SKIP` and exits cleanly.

---

## Test matrix

| File | Test function | Description |
|------|---------------|-------------|
| `01_v2_erc20_chain_transfer_test.go` | `TestV2Erc20ChainOnChain_AliceBobCarolWithdraw` | depositV2 → Alice→Bob transferV2 → Bob→Carol transferV2 → Carol withdrawV2 |
| `02_v2_erc20_consolidation_test.go` | `TestV2Erc20ConsolidationOnChain` | 5 × depositV2 (10 tokens each) → 10-in/2-out transferV2 → Alice receives 50 tokens consolidated |
| `03_v2_erc20_deposit_transfer_withdraw_onchain_test.go` | `TestV2Erc20OnChain_DepositTransferWithdraw` | depositV2 → transferV2 (Alice→Bob) → withdrawV2 (Bob) |
| `04_v2_erc721_onchain_test.go` | `TestV2Erc721OnChain_DepositTransfer` | ERC721 deposit → ownership transfer (Alice→Bob) |
| `05_v2_zkdvp_two_phase_swap_test.go` | `TestV2ZkDvp_TwoPhaseSwapOnChain` | Two-phase ZkDvP swap of ERC20 USDT ↔ ERC20 ticket (different tokenIds) via `submitPartialSettlement` |
| `06_v2_erc1155_fungible_onchain_test.go` | `TestV2Erc1155FungibleOnChain_DepositTransfer` | ERC1155 fungible deposit → JoinSplit transfer (Alice→Bob) |
| `07_v2_erc1155_nonfungible_onchain_test.go` | `TestV2Erc1155NonFungibleOnChain_DepositTransferV2` | ERC1155 non-fungible deposit → ownership transfer (Alice→Bob) |
| `08_v2_swap_erc721_erc20_onchain_test.go` | `TestV2Swap_Erc721ForErc20_OnChain` | Atomic DVP swap: Alice's ERC721 NFT ↔ Bob's ERC20 payment via `EnygmaDvp.swap()` |
| `09_v2_erc20_private_mint_test.go` | `TestV2Erc20OnChain_PrivateMint` | ERC20 private mint → transferV2 |
| `10_v2_swap_erc1155nonfungible_erc20_onchain_test.go` | `TestV2Swap_Erc1155NonFungibleForErc20_OnChain` | Atomic DVP swap: Alice's ERC1155 NFT ↔ Bob's ERC20 payment via `EnygmaDvp.swap()` |
| `helpers_test.go` | *(no test functions)* | Shared helpers: `loadVaultMerkleTree`, `hardhatAuth`, etc. |

---

## Notes

- **Test 05** hardcodes `tokenIdTicket = 25`. Re-running it on the same Hardhat node
  without restarting will fail with `ERC721: token already minted`. Restart the node
  before re-running test 05.

- **Test 10** calls `EnygmaDvp.addTokenToGroup` to register the ERC1155 token in the
  `NonFungibleAssetGroup` before the swap. This is required for all ERC1155 non-fungible
  swaps (unlike ERC721 which is pre-registered at init time).

- Each test generates fresh random key pairs and (where needed) random token IDs to
  avoid collisions across runs, with the exception of test 05.

- The 600s timeout covers the gnark proof generation time. Individual proofs typically
  take 5–30 seconds each.
