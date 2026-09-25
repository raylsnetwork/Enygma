#!/usr/bin/env bash
# init.sh — Export gnark VKs to circom format and run the init script
#           to register verifying keys on-chain.
#
# Must be run from the project root:
#   cd /path/to/enygma_dvp && bash scripts/init.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "==> [1/3] Exporting gnark VKs to build/ ..."
cd gnark_circuits
go run ./cmd/export_vk_init/ ../build
cd "$PROJECT_ROOT"

echo "==> [2/3] Building init binary..."
cd scripts
CC=/usr/bin/clang go build -o /tmp/init_contracts ./cmd/init
cd "$PROJECT_ROOT"

echo "==> [3/3] Registering VKs on-chain..."
/tmp/init_contracts

echo ""
echo "Done. VKs (including DvPInitiator slot 23 and DvPDestination slot 24) registered on-chain."
