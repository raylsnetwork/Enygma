#!/usr/bin/env python3
"""
Direct deployment script for Enygma + EnygmaVerifier using web3.py.
Reads compiled Hardhat artifacts, deploys both contracts, writes deploy_receipts.json.
"""
import json
import os
import sys

sys.path.insert(0, os.path.dirname(__file__))

from web3 import Web3

OWNER_KEY = os.environ["OWNER_KEY"]   # export OWNER_KEY=<hex> — never hardcode mainnet key
CHAIN_ID = int(os.environ.get("CHAIN_ID", "72957"))
RPC_URL = os.environ.get("RPC_URL", "https://mainnet-rpc.rayls.com")

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
    w3 = Web3(Web3.HTTPProvider(RPC_URL))
    if not w3.is_connected():
        raise RuntimeError("Cannot connect to chain at " + RPC_URL)
    account = w3.eth.account.from_key(OWNER_KEY)
    print(f"Deployer: {account.address}")
    print(f"Block: {w3.eth.block_number}")

    print(f"\nDeploying Enygma (epochInterval={EPOCH_INTERVAL})...")
    enygma_artifact = load_artifact(ENYGMA_ARTIFACT)
    enygma_receipt = deploy_contract(w3, account, enygma_artifact, constructor_args={"_epochInterval": EPOCH_INTERVAL})

    print("\nDeploying EnygmaVerifier...")
    verifier_artifact = load_artifact(VERIFIER_ARTIFACT)
    verifier_receipt = deploy_contract(w3, account, verifier_artifact)

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
