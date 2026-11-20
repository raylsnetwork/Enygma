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

# Seconds a replacement of an already-set verifier waits before
# executeVerifierChange can apply it (Enygma.verifierChangeDelay, at most 30
# days). 0 applies replacements at once: local development only.
VERIFIER_CHANGE_DELAY = int(os.environ.get("VERIFIER_CHANGE_DELAY", "0"))

ROOT = os.path.dirname(os.path.abspath(__file__))
CONTRACTS_DIR = os.path.join(ROOT, "..", "contracts", "enygma", "artifacts", "contracts")

ENYGMA_ARTIFACT = os.path.join(CONTRACTS_DIR, "Enygma.sol", "Enygma.json")
VERIFIER_ARTIFACT = os.path.join(CONTRACTS_DIR, "EnygmaVerifier.sol", "Verifier.json")
reviewed_path = os.environ.get("REVIEWED_VERIFIER_ARTIFACT")
if not IS_LOCAL_TARGET and not reviewed_path:
    sys.exit("set REVIEWED_VERIFIER_ARTIFACT to the reviewed transfer verifier build for remote deployment")
REVIEWED_VERIFIER_ARTIFACT = os.path.abspath(reviewed_path) if reviewed_path else VERIFIER_ARTIFACT

RECEIPTS_DIR = os.path.join(ROOT, "build", "enygma", "web3")
RECEIPTS_PATH = os.path.join(RECEIPTS_DIR, "deploy_receipts.json")

GNARK_SERVER_DIR = os.path.join(ROOT, "..", "gnark-server")
VERIFIER_VK = os.path.join("keys", "EnygmaVk.key")


def verify_deployed_verifier(rpc_url, address):
    """Require the deployed runtime to match the reviewed verifier artifact."""
    subprocess.run(
        [
            "go", "run", "./cmd/verify_deployed_verifier",
            "-vk", VERIFIER_VK,
            "-artifact", REVIEWED_VERIFIER_ARTIFACT,
            "-rpc", rpc_url,
            "-address", address,
        ],
        cwd=GNARK_SERVER_DIR,
        check=True,
    )


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
    if receipt.status != 1:
        raise RuntimeError(f"deployment failed: {receipt}")
    print(f"  deployed at: {receipt.contractAddress}")
    return receipt


def main():
    reviewed_artifact = load_artifact(REVIEWED_VERIFIER_ARTIFACT)
    verifier_artifact = load_artifact(VERIFIER_ARTIFACT)
    if reviewed_artifact["deployedBytecode"] != verifier_artifact["deployedBytecode"]:
        sys.exit("local verifier build differs from the reviewed runtime")
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

    print(f"\nDeploying Enygma (epochInterval={EPOCH_INTERVAL}, verifierChangeDelay={VERIFIER_CHANGE_DELAY})...")
    enygma_artifact = load_artifact(ENYGMA_ARTIFACT)
    enygma_receipt = deploy_contract(w3, account, enygma_artifact, constructor_args={
        "_epochInterval": EPOCH_INTERVAL,
        "_verifierChangeDelay": VERIFIER_CHANGE_DELAY,
    })

    print("\nDeploying EnygmaVerifier...")
    verifier_receipt = deploy_contract(w3, account, verifier_artifact)

    verify_deployed_verifier(RPC_URL, verifier_receipt.contractAddress)

    receipts = {
        "TOKEN": {"contractAddress": enygma_receipt.contractAddress},
        "VERIFIER": {
            "contractAddress": verifier_receipt.contractAddress,
            "runtimeHash": Web3.to_hex(Web3.keccak(hexstr=reviewed_artifact["deployedBytecode"])),
        },
    }

    os.makedirs(RECEIPTS_DIR, exist_ok=True)
    with open(RECEIPTS_PATH, "w") as f:
        json.dump(receipts, f, indent=2)

    print(f"\nWrote {RECEIPTS_PATH}:")
    print(json.dumps(receipts, indent=2))


if __name__ == "__main__":
    main()
