#!/usr/bin/env python3
"""
Direct deployment script for Enygma + EnygmaVerifier using web3.py.
Reads compiled Hardhat artifacts, deploys both contracts, writes deploy_receipts.json.
"""
import json
import os
import subprocess
import sys

sys.path.insert(0, os.path.dirname(__file__))

from web3 import Web3

OWNER_KEY = os.environ["OWNER_KEY"]   # export OWNER_KEY=<hex> — never hardcode mainnet key

# Fix M-07: this used to default CHAIN_ID/RPC_URL to Rayls MAINNET
# (72957 / https://mainnet-rpc.rayls.com) while demo_instructions.md's
# documented procedure — the only procedure this script's own README
# section describes — says "start a local Hardhat node" and shows the
# deploy command with no environment overrides at all. Following the
# project's own instructions therefore deployed to production by default.
# Local Hardhat is now the default, matching deploy_node.js and the docs;
# a remote target requires both RPC_URL/CHAIN_ID *and* the explicit
# opt-in below — unsetting only one is not enough to reach mainnet by
# accident.
CHAIN_ID = int(os.environ.get("CHAIN_ID", "1337"))
RPC_URL = os.environ.get("RPC_URL", "http://127.0.0.1:8545")
IS_LOCAL_TARGET = CHAIN_ID == 1337 and RPC_URL in (
    "http://127.0.0.1:8545",
    "http://localhost:8545",
)
if not IS_LOCAL_TARGET and os.environ.get("I_UNDERSTAND_THIS_TARGETS_A_REMOTE_CHAIN") != "1":
    sys.exit(
        "refusing to deploy: CHAIN_ID/RPC_URL do not point at local Hardhat "
        f"(CHAIN_ID={CHAIN_ID}, RPC_URL={RPC_URL}).\n"
        "If this is deliberate, also set:\n"
        "  export I_UNDERSTAND_THIS_TARGETS_A_REMOTE_CHAIN=1"
    )

# Number of blocks per epoch. Transactions in the same epoch share the same
# balance-commitment slot, so lastBlockNum only advances every EPOCH_INTERVAL
# blocks. At ~0.5-1s Rayls block times:
#   10  blocks ≈  5-10 s
#   30  blocks ≈ 15-30 s
#   60  blocks ≈ 30-60 s
EPOCH_INTERVAL = int(os.environ.get("EPOCH_INTERVAL", "30"))

ROOT = os.path.dirname(os.path.abspath(__file__))
CONTRACTS_DIR = os.path.join(ROOT, "..", "contracts", "enygma", "artifacts", "contracts")

ENYGMA_ARTIFACT = os.path.join(CONTRACTS_DIR, "Enygma.sol", "Enygma.json")
VERIFIER_ARTIFACT = os.path.join(CONTRACTS_DIR, "EnygmaVerifier.sol", "Verifier.json")

RECEIPTS_DIR = os.path.join(ROOT, "build", "enygma", "web3")
RECEIPTS_PATH = os.path.join(RECEIPTS_DIR, "deploy_receipts.json")

# Fix H-16 (deploy-time verifier-binding item): gnark-server/cmd/verify_deployed_verifier
# already implements "read back the deployed verifier's embedded constants and compare
# them to the local verifying key" — the audit's own wording — but until now nothing
# actually called it, so a verifier compiled from a stale/wrong/swapped key would still
# deploy and register silently, the exact class of bug that let H-16's mismatched zkdvp
# verifiers go unnoticed. Wired in here as a hard post-deploy gate, matching the doc
# comment's own "intended to be wired into deployment/CI as a hard gate" note.
GNARK_SERVER_DIR = os.path.join(ROOT, "..", "gnark-server")
VERIFIER_VK = os.path.join("keys", "EnygmaVk.key")  # relative to GNARK_SERVER_DIR


def verify_deployed_verifier(rpc_url, address):
    """Run the H-16 deploy-time gate: fail the deploy if the just-deployed
    EnygmaVerifier's embedded constants don't match the local EnygmaVk.key.
    Exits the whole script (sys.exit) on any mismatch or tooling failure —
    an unverifiable verifier is treated the same as a mismatched one."""
    print(f"\nVerifying deployed EnygmaVerifier at {address} against {VERIFIER_VK}...")
    result = subprocess.run(
        [
            "go", "run", "./cmd/verify_deployed_verifier",
            "-vk", VERIFIER_VK,
            "-rpc", rpc_url,
            "-address", address,
        ],
        cwd=GNARK_SERVER_DIR,
        capture_output=True,
        text=True,
    )
    sys.stdout.write(result.stdout)
    sys.stderr.write(result.stderr)
    if result.returncode != 0:
        sys.exit(
            f"H-16 gate failed: the deployed EnygmaVerifier at {address} does not "
            f"correspond to {VERIFIER_VK} (or the check tool could not run — see output "
            "above). Refusing to register this verifier. Do NOT call addVerifier with "
            "this address."
        )
    print("H-16 gate passed: deployed verifier matches the local verifying key.")


def load_artifact(path):
    with open(path) as f:
        return json.load(f)


def deploy_contract(w3, account, artifact, constructor_args=None):
    contract = w3.eth.contract(abi=artifact["abi"], bytecode=artifact["bytecode"])
    nonce = w3.eth.get_transaction_count(account.address)
    txn = contract.constructor(**(constructor_args or {})).build_transaction({
        "chainId": CHAIN_ID,
        "gas": 10_000_000,
        "gasPrice": w3.eth.gas_price,
        "nonce": nonce,
    })
    signed = account.sign_transaction(txn)
    tx_hash = w3.eth.send_raw_transaction(signed.rawTransaction if hasattr(signed, 'rawTransaction') else signed.raw_transaction)
    print(f"  tx: {tx_hash.hex()}")
    receipt = w3.eth.wait_for_transaction_receipt(tx_hash, timeout=600)
    assert receipt.status == 1, f"deployment failed: {receipt}"
    print(f"  deployed at: {receipt.contractAddress}")
    return receipt


def main():
    w3 = Web3(Web3.HTTPProvider(RPC_URL))
    assert w3.is_connected(), "Cannot connect to chain at " + RPC_URL
    # Fix M-07: is_connected() succeeds against ANY reachable chain,
    # including the wrong one — it says nothing about CHAIN_ID being
    # correct. This is the check the audit found missing: CHAIN_ID was
    # used only for signing and never compared against the chain actually
    # being talked to.
    actual_chain_id = w3.eth.chain_id
    if actual_chain_id != CHAIN_ID:
        sys.exit(
            f"CHAIN_ID mismatch: configured for {CHAIN_ID} but {RPC_URL} "
            f"reports chain id {actual_chain_id}. Refusing to deploy — set "
            "CHAIN_ID to match, or point RPC_URL at the intended chain."
        )
    account = w3.eth.account.from_key(OWNER_KEY)
    print(f"Deployer: {account.address}")
    print(f"Chain ID: {actual_chain_id}")
    print(f"Block: {w3.eth.block_number}")

    print(f"\nDeploying Enygma (epochInterval={EPOCH_INTERVAL})...")
    enygma_artifact = load_artifact(ENYGMA_ARTIFACT)
    enygma_receipt = deploy_contract(w3, account, enygma_artifact, constructor_args={"_epochInterval": EPOCH_INTERVAL})

    print("\nDeploying EnygmaVerifier...")
    verifier_artifact = load_artifact(VERIFIER_ARTIFACT)
    verifier_receipt = deploy_contract(w3, account, verifier_artifact)

    verify_deployed_verifier(RPC_URL, verifier_receipt.contractAddress)

    receipts = {
        "TOKEN": {"contractAddress": enygma_receipt.contractAddress},
        "VERIFIER": {"contractAddress": verifier_receipt.contractAddress},
    }

    os.makedirs(RECEIPTS_DIR, exist_ok=True)
    with open(RECEIPTS_PATH, "w") as f:
        json.dump(receipts, f, indent=2)

    print(f"\nWrote {RECEIPTS_PATH}:")
    print(json.dumps(receipts, indent=2))


if __name__ == "__main__":
    main()
