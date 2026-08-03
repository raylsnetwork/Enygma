#!/usr/bin/env bash
# Enygma Payments Live Demo launcher
# Usage:
#   cd demo
#   RELAYER_API_KEY=enygma-test-secret RELAYER_PRIVATE_KEY=ac0974... ./run.sh

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "==> Tidying modules..."
go mod tidy

echo "==> Building..."
CC=/usr/bin/clang go build -o /tmp/enygma_demo .

echo ""
echo "  Dashboard → http://localhost:9090"
echo ""
echo "  Prerequisites (must be running before clicking Run):"
echo "    1. Hardhat   :  npx hardhat node              (in contracts/enygma/)"
echo "    2. Deploy    :  python run_scripts/deploy_enygma.py  (writes deploy_receipts.json)"
echo "    3. Gnark     :  cd gnark-server && go run ./cmd/server/main.go"
echo "    4. Relayer   :  cd relayer && RELAYER_PRIVATE_KEY=... RELAYER_API_KEY=... go run ."
echo ""

CC=/usr/bin/clang /tmp/enygma_demo
