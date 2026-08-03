#!/usr/bin/env bash
# Enygma DvP · NFT ↔ ERC20 Swap Demo — build and launch script.
#
# Prerequisites (start each in a separate terminal before running this):
#
#   1. Hardhat node
#        cd enygma_dvp && npx hardhat node
#
#   2. Deploy + init contracts (writes build/receipts.json)
#        cd enygma_dvp/scripts
#        go build -o /tmp/deploy_contracts deploy.go enygma.go && cd .. && /tmp/deploy_contracts
#        cd gnark_circuits && go run ./cmd/export_vk_init/ ../build && cd ..
#        cd scripts && go build -o /tmp/init_contracts init.go enygma.go && cd .. && /tmp/init_contracts
#
#   3. Gnark server (port 8081) — must include DvP Initiator/Destination keys
#        cd enygma_dvp/gnark_circuits && go run main.go
#
# Usage:
#   cd demo && bash run.sh
#   open http://localhost:9092

set -euo pipefail
cd "$(dirname "$0")"

# Use the system clang (not Homebrew) because go-ethereum requires CGo and
# Homebrew clang has a hardcoded MacOSX26.sdk path that does not exist.
export CC=/usr/bin/clang

echo "[run.sh] building demo binary…"
go build -o /tmp/dvp_swap_demo .

echo "[run.sh] starting demo server on http://localhost:9092"
exec /tmp/dvp_swap_demo "$@"
