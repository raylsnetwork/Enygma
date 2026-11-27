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

## Production ceremony keys

Follow the [ceremony release procedure](../../enygma_ceremony/README.md#approving-production-keys-and-verifiers)
for all six payments circuits. Install the approved keys under `gnark-server/`
at the paths in the export's `keys.json`, with `ceremony-seal.json` at the root
of that directory. Before starting a production prover check that exact key
directory with `ceremony verify-keys` and keep it read-only to the service.

Both deployment scripts require the following settings outside local Hardhat.

| Setting | Value |
| --- | --- |
| `CEREMONY_SRS` | Absolute path to the approved Phase 1 SRS, beside its JSON seal |
| `CEREMONY_PHASE1_SEAL_SHA256` | Release-approved SHA-256 of the Phase 1 seal record |
| `CEREMONY_PHASE2_SEAL_SHA256` | Release-approved SHA-256 of the payments Phase 2 seal record |
| `REVIEWED_VERIFIER_ARTIFACT` | Reviewed transfer verifier build from the approved `Enygma` VK |

The scripts check a fresh R1CS export and the installed keys against the approved
seals, then verify the reviewed transfer artifact before sending deployment
transactions. The local Hardhat scripts continue to accept development keys.

## Register or replace proof verifiers

For each enabled circuit, use the reviewed build artifact and the verifying key from the approved ceremony bundle. The
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
Outside local Hardhat, `deploy_node.js` also requires `VERIFIER_CHANGE_DELAY`
in seconds. An explicit `0` allows immediate verifier replacement.

Before calling a setter, submit a separate owner transaction using
`approveVerifierRuntime(kind, splitCount, runtimeHash)`. Use the Keccak-256
hash of the reviewed artifact's `deployedBytecode`.

| Kind | Split count | Setter | Verifying key relative to `gnark-server` |
| --- | --- | --- | --- |
| 0 | 0 | `addVerifier` | `keys/EnygmaVk.key` |
| 3 | 0 | `addFeeVerifier` | `keys/EnygmaFeeVk.key` |
| 4 | 0 | `addBurnVerifier` | `keys/BurnVk.key` |
| 5 | 0 | `addUsdrVerifier` | `keys/UsdrVk.key` |

### Do not register the withdraw and deposit verifiers

Leave kinds 1 and 2 (`addWithdrawVerifier` with `keys/zkdvp/WithdrawVk6.key`,
`addDepositVerifier` with `keys/zkdvp/DepositVk.key`) unregistered. While they
are unset, `withdraw()` and `deposit()` cannot run. Enabling them would let a
chain observer identify the hidden sender:

- **Linkable tags and blinding factors.** Both circuits still derive message
  tags and blinding factors from `Poseidon(Poseidon(c), SharedSecrets[i],
  BlockNumber)`, with a fixed `c` for each. `BlockNumber` stays the same for a whole epoch, and a pair of
  banks shares the same secret in both directions, so statements within an
  epoch can be linked. The transfer circuit fixed this in H-01/H-02 with a
  per-transaction nonce that depends on direction
  (`gnark-server/pkg/circuits/enygma/circuit.go`). The withdraw and deposit
  circuits did not get that fix.
- **No relayer route.** The payments relayer serves only `/transfer` and
  `/transfer_fee`, so a bank would have to submit withdraw and deposit from its
  own registered address. That address names the bank as the sender.
- **Unsalted bridge note.** The DvP note that `depositThroughEnygma` builds is
  `Poseidon(Poseidon(asset, amount), publicKey)`, from the amount and key that
  Enygma forwards from public calldata. Because it has no salt, a DvP note
  public key that a bank reuses across withdrawals becomes a public pseudonym
  for that bank.

The integration tests register these verifiers on a throwaway chain to test the
contract. That does not make them safe to register in a deployment. Port the
transfer nonce into both circuits and regenerate their keys before enabling
them. See [#435](https://github.com/raylsnetwork/Enygma/issues/435).

The contract compares the approved hash on initial registration and on
replacement proposals. Delayed execution requires both the prior approval
and the runtime bytes to still match the proposal. Approving a zero hash
revokes the slot's approval for future registrations or executions.
Existing deployments require a new contract deployment to enforce this
check. The remote demo setup expects the on-chain approval to exist already.
