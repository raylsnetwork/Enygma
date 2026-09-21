#!/usr/bin/env bash
# deploy.sh — Regenerate Poseidon artifacts, build and run the deploy script.
#
# Must be run from the project root:
#   cd /path/to/enygma_dvp && bash scripts/deploy.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "==> [1/3] Regenerating Poseidon artifacts..."
node scripts/regen_poseidon.js

echo "==> [2/3] Building deploy binary..."
cd scripts
CC=/usr/bin/clang go build -o /tmp/deploy_contracts ./cmd/deploy
cd "$PROJECT_ROOT"

echo "==> [3/3] Deploying contracts..."
/tmp/deploy_contracts

echo ""
echo "Done. Contract addresses saved to build/receipts.json"
