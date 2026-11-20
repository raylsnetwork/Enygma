# Enygma Run Scripts

Deploys `Enygma.sol` and `EnygmaVerifier.sol` to a running local chain and writes the contract addresses to `build/enygma/web3/deploy_receipts.json` for use by the integration test.

## Setup

```bash
python3 -m venv .venv
.venv/bin/pip install -r requirements.txt
```

## Deploy

```bash
.venv/bin/python3 deploy_direct.py
```

Requires a Hardhat node running at `localhost:8545` (chainId 1337). See `demo_instructions.md` for the full end-to-end flow.

## Register or replace proof verifiers

Use the reviewed build artifact and verifying key for the circuit being registered. The
artifact's `deployedBytecode` is the reference runtime. Build it from the reviewed
Solidity export for the approved key and circuit, and retain it with the key review
record. Use a direct verifier deployment so the runtime itself contains the verification
logic. A proxy's runtime hash only identifies its forwarding code.

From `enygma_payments/gnark-server`, check the deployed runtime before submitting an
owner transaction. For example, for the main transfer verifier:

```bash
go run ./cmd/verify_deployed_verifier \
  -vk keys/EnygmaVk.key -artifact /path/to/reviewed/Verifier.json \
  -rpc "$RPC_URL" -address "$VERIFIER_ADDRESS"
```

The command checks the artifact for the expected key constants and compares the
deployed bytes with the artifact exactly. The artifact must come from the reviewed
build. An artifact reconstructed from the target address would make the comparison
circular. For deployments outside local Hardhat, set `REVIEWED_VERIFIER_ARTIFACT`
to the approved artifact before invoking either deployment script. Both scripts
include the main verifier's expected `runtimeHash` in their receipts.

Before calling a setter, submit a separate owner transaction using
`approveVerifierRuntime(kind, splitCount, runtimeHash)`. Use the Keccak-256
hash of the reviewed artifact's `deployedBytecode`.

| Kind | Split count | Setter | Verifying key relative to `gnark-server` |
| --- | --- | --- | --- |
| 0 | 0 | `addVerifier` | `keys/EnygmaVk.key` |
| 1 | 6 | `addWithdrawVerifier` | `keys/zkdvp/WithdrawVk6.key` |
| 2 | 0 | `addDepositVerifier` | `keys/zkdvp/DepositVk.key` |
| 3 | 0 | `addFeeVerifier` | `keys/EnygmaFeeVk.key` |
| 4 | 0 | `addBurnVerifier` | `keys/BurnVk.key` |
| 5 | 0 | `addUsdrVerifier` | `keys/UsdrVk.key` |

The contract compares the approved hash on initial registration and on
replacement proposals. Delayed execution requires both the prior approval
and the runtime bytes to still match the proposal. Approving a zero hash
revokes the slot's approval for future registrations or executions.
Existing deployments require a new contract deployment to enforce this
check. The remote demo setup expects the on-chain approval to exist already.
