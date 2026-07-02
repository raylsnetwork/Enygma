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
