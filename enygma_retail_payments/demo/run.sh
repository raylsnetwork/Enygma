#!/usr/bin/env bash
# Enygma Retail Payments Demo — build and launch script.
#
# Prerequisites (start each in a separate terminal before running this):
#
#   1. Hardhat node
#        cd ../enygma_dvp && npx hardhat node
#
#   2. Deploy + init contracts  (also deploys TagChannelRegistry & TagRegistry)
#        cd enygma_retail_payments && bash setup.sh
#
#   3. Gnark server (port 8082)
#        cd gnark_circuits && go run main.go
#
#   4. Relayer (port 8090) — plain mode only:
#        cd relayer
#        RELAYER_PRIVATE_KEY=9883c26cc126a37158c4ffcc9d401d3ffa41187d9b1a18ce4912398d22597cda \
#        RELAYER_API_KEY=test-api-key-dev-only \
#        go run main.go
#
#        For "With Tag" mode, also pass tag registry addresses (printed below):
#        RELAYER_TAG_REGISTRY_ADDR=<TagRegistry addr>
#        RELAYER_TAG_CHANNEL_REGISTRY_ADDR=<TagChannelRegistry addr>
#
# Usage:
#   cd demo && bash run.sh
#   open http://localhost:9091

set -euo pipefail
cd "$(dirname "$0")"

# lattice_zk is a ghost dependency listed in src/go.mod but never imported.
# Go 1.17+ lazy loading adds it to the transitive require list; create a
# placeholder go.mod so `go mod tidy` can resolve it without the source.
LATTICE_DIR="$(pwd)/../lattice_zk"
if [ ! -d "$LATTICE_DIR" ]; then
    mkdir -p "$LATTICE_DIR"
    printf 'module lattice_zk\n\ngo 1.24.0\n' > "$LATTICE_DIR/go.mod"
    echo "[run.sh] created lattice_zk placeholder at $LATTICE_DIR"
fi

# Use the system clang (not Homebrew) because go-ethereum requires CGo and
# Homebrew clang has a hardcoded MacOSX26.sdk path that does not exist.
export CC=/usr/bin/clang

echo "[run.sh] running go mod tidy…"
go mod tidy

echo "[run.sh] building demo binary…"
go build -o /tmp/retail_payments_demo .

# Print tag registry addresses from receipts.json for "With Tag" relayer setup.
RECEIPTS="$(pwd)/../build/receipts.json"
if [ -f "$RECEIPTS" ]; then
    TAG_CH=$(python3 -c "import json,sys; d=json.load(open('$RECEIPTS')); print(d.get('TagChannelRegistry',{}).get('contractAddress','<not deployed>'))" 2>/dev/null || echo "<error>")
    TAG_R=$(python3 -c "import json,sys; d=json.load(open('$RECEIPTS')); print(d.get('TagRegistry',{}).get('contractAddress','<not deployed>'))" 2>/dev/null || echo "<error>")
    echo ""
    echo "[run.sh] For 'With Tag' mode, start the relayer with:"
    echo "  cd ../relayer"
    echo "  RELAYER_PRIVATE_KEY=9883c26cc126a37158c4ffcc9d401d3ffa41187d9b1a18ce4912398d22597cda \\"
    echo "  RELAYER_API_KEY=test-api-key-dev-only \\"
    echo "  RELAYER_TAG_CHANNEL_REGISTRY_ADDR=$TAG_CH \\"
    echo "  RELAYER_TAG_REGISTRY_ADDR=$TAG_R \\"
    echo "  go run main.go"
    echo ""
fi

echo "[run.sh] starting demo server on http://localhost:9091"
exec /tmp/retail_payments_demo "$@"
