#!/usr/bin/env bash
# Enygma Retail Shield Demo — one-command launcher.
#
# Starts a fresh Hardhat node, runs the retail setup (compile, Poseidon
# regen, deploy, init, USDr fixed fee = 1), starts the gnark server and the
# relayer, then builds and serves the demo on http://localhost:9092.
# Ctrl-C (or any failure) stops everything this script started.
#
# Usage:
#   cd enygma_retail_payments/shield_demo && bash run.sh
#   bash run.sh --no-demo     # bring up the stack only (e.g. for the integration test)
#
# Service logs go to $LOG_DIR (default: a fresh temp directory). The relayer
# keeps its fee-note openings in build/relayer_fee_notes.jsonl, where the
# integration test reads them; each run starts it afresh with the chain.

set -euo pipefail

DEMO_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$DEMO_DIR/.." && pwd)"
DVP_ROOT="$(cd "$ROOT/../enygma_dvp" && pwd)"
LOG_DIR="${LOG_DIR:-$(mktemp -d -t shield_demo)}"
BIN_DIR="$LOG_DIR/bin"
mkdir -p "$BIN_DIR"

START_DEMO=1
[ "${1:-}" = "--no-demo" ] && START_DEMO=0

# go-ethereum needs CGo; Homebrew clang points at a missing macOS SDK.
export CC=/usr/bin/clang

# Hardhat account[2] — the relayer's signing key (demo only).
RELAYER_KEY=9883c26cc126a37158c4ffcc9d401d3ffa41187d9b1a18ce4912398d22597cda
RELAYER_API_KEY=test-api-key-dev-only

PIDS=()
cleanup() {
    trap - EXIT INT TERM
    echo ""
    echo "[run.sh] stopping services…"
    for pid in "${PIDS[@]}"; do
        # Each service runs in its own process group; kill the whole group so
        # children (npx → node) go too.
        kill -TERM -- "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
    done
    wait 2>/dev/null || true
    echo "[run.sh] stopped. Logs: $LOG_DIR"
}
trap cleanup EXIT INT TERM

# start NAME DIR CMD... — runs CMD in DIR in its own process group, logging to $LOG_DIR/NAME.log.
start() {
    local name=$1 dir=$2
    shift 2
    # perl's setsid: stock macOS has no setsid(1).
    (cd "$dir" && exec perl -e 'use POSIX qw(setsid); setsid(); exec @ARGV or die "exec: $!"' -- "$@" >"$LOG_DIR/$name.log" 2>&1) &
    PIDS+=($!)
}

# wait_port NAME PORT — waits until PORT accepts connections, or fails with NAME's log.
wait_port() {
    local name=$1 port=$2
    for _ in $(seq 1 600); do
        if nc -z localhost "$port" 2>/dev/null; then
            echo "[run.sh] $name is up on :$port"
            return 0
        fi
        sleep 0.5
    done
    echo "[run.sh] $name did not come up on :$port — last log lines:" >&2
    tail -20 "$LOG_DIR/$name.log" >&2
    exit 1
}

for port in 8545 8082 8090 9092; do
    if nc -z localhost "$port" 2>/dev/null; then
        echo "[run.sh] port $port is already in use — stop whatever is running there first (every run needs a fresh chain)." >&2
        exit 1
    fi
done

echo "[run.sh] logs: $LOG_DIR"
FEE_NOTES="$ROOT/build/relayer_fee_notes.jsonl"
mkdir -p "$ROOT/build"
rm -f "$FEE_NOTES"

echo "[run.sh] [1/7] starting a fresh Hardhat node…"
start hardhat "$DVP_ROOT" npx hardhat node
wait_port hardhat 8545

echo "[run.sh] [2/7] compiling contracts and regenerating Poseidon…"
(cd "$DVP_ROOT" && npx hardhat compile >"$LOG_DIR/compile.log" 2>&1)
# Poseidon.sol is a placeholder: compile overwrites the real artifacts.
(cd "$DVP_ROOT" && node regen_poseidon.mjs >>"$LOG_DIR/compile.log" 2>&1)
cp "$DVP_ROOT/artifacts/contracts/core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json" "$ROOT/contracts/abis/Erc20CoinVault.json"
cp "$DVP_ROOT/artifacts/contracts/core/contracts/EnygmaDvp.sol/EnygmaDvp.json" "$ROOT/contracts/abis/EnygmaDvp.json"

echo "[run.sh] [3/7] deploying and initialising the retail contracts…"
(cd "$ROOT/gnark_circuits" && go run ./cmd/export_vk/ ../build >"$LOG_DIR/setup.log" 2>&1)
go build -C "$ROOT/scripts" -o "$BIN_DIR/deploy" ./cmd/deploy
go build -C "$ROOT/scripts" -o "$BIN_DIR/init" ./cmd/init
(cd "$ROOT" && "$BIN_DIR/deploy" >>"$LOG_DIR/setup.log" 2>&1 && "$BIN_DIR/init" >>"$LOG_DIR/setup.log" 2>&1) || {
    echo "[run.sh] deploy/init failed — see $LOG_DIR/setup.log" >&2
    tail -20 "$LOG_DIR/setup.log" >&2
    exit 1
}

echo "[run.sh] [4/7] setting the USDr fixed fee to 1…"
(cd "$DEMO_DIR" && go run ./cmd/set_usdr_fee -root "$ROOT" -fee 1)

receipt_addr() {
    python3 -c "import json,sys; print(json.load(open('$ROOT/build/receipts.json'))['$1']['contractAddress'])"
}

echo "[run.sh] [5/7] starting the gnark server…"
go build -C "$ROOT/gnark_circuits" -o "$BIN_DIR/gnark_server" ./cmd/server
start gnark "$ROOT/gnark_circuits" "$BIN_DIR/gnark_server"
wait_port gnark 8082

echo "[run.sh] [6/7] starting the relayer…"
go build -C "$ROOT/relayer" -o "$BIN_DIR/relayer" .
start relayer "$ROOT/relayer" env \
    RELAYER_PRIVATE_KEY="$RELAYER_KEY" \
    RELAYER_API_KEY="$RELAYER_API_KEY" \
    RELAYER_FEE_SPEND_PRIVATE_KEY=123456789 \
    RELAYER_MIN_FEE=1 \
    RELAYER_FEE_NOTES_PATH="$FEE_NOTES" \
    RELAYER_TAG_REGISTRY_ADDR="$(receipt_addr TagRegistry)" \
    RELAYER_TAG_CHANNEL_REGISTRY_ADDR="$(receipt_addr TagChannelRegistry)" \
    "$BIN_DIR/relayer"
wait_port relayer 8090

if [ "$START_DEMO" = 0 ]; then
    echo "[run.sh] stack is up (no demo). Ctrl-C to stop."
    wait
    exit 0
fi

echo "[run.sh] [7/7] building and starting the demo…"
go build -C "$DEMO_DIR" -o "$BIN_DIR/shield_demo" .
start demo "$DEMO_DIR" "$BIN_DIR/shield_demo" -root "$ROOT"
wait_port demo 9092
echo "[run.sh] demo: http://localhost:9092  (Ctrl-C to stop everything)"
command -v open >/dev/null && open http://localhost:9092 || true
wait
