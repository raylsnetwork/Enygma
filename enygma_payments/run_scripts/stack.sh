#!/usr/bin/env bash
# enygma_payments' end-to-end test stack in one command: compiles the
# contracts and builds relayer/relayer_bin (both gitignored, so a stale copy
# would otherwise be tested), then starts a fresh Hardhat node on :8545 and
# the gnark server on :8080. The suite deploys its own contracts and spawns
# relayer_bin itself. Ctrl-C (or any failure) stops what it started.
#
# Usage:
#   bash enygma_payments/run_scripts/stack.sh
#   make payments-stack       # the same, detached: returns once the stack is up
#   make payments-test        # then the suite
#   make payments-stack-stop
#
# Keys: the Burn keys are gitignored; generate them once, and after a burn
# circuit change: cd gnark-server && go run ./keygen/generate_keys.go -circuit burn
# Logs and the built gnark server go to $LOG_DIR.
set -euo pipefail
PAY_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CONTRACTS="$PAY_ROOT/contracts/enygma"
LOG_DIR="${LOG_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/payments_stack.XXXXXX")}"
BIN_DIR="$LOG_DIR/bin"
mkdir -p "$BIN_DIR"
# go-ethereum needs CGo; Homebrew clang on macOS points at a missing SDK.
[ "$(uname -s)" = Darwin ] && export CC="${CC:-/usr/bin/clang}"
say() { echo "[stack.sh] $*"; }
listening() { (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null; }

node_major() { node -p 'process.versions.node.split(".")[0]' 2>/dev/null || echo 0; }
NVM_SH="${NVM_DIR:-$HOME/.nvm}/nvm.sh"
if [ "$(node_major)" -lt 20 ] && [ -s "$NVM_SH" ]; then
    set +u
    . "$NVM_SH"
    nvm use --silent 22 >/dev/null 2>&1 || true
    set -u
fi

# preflight — fails in a second, with the fix, on what otherwise fails later.
ok=1
if [ "$(node_major)" -lt 20 ]; then
    say "Node $(node -v 2>/dev/null || echo missing) is too old for Hardhat; use 20 or later: nvm use 22" >&2
    ok=0
fi
if [ ! -x "$CONTRACTS/node_modules/.bin/hardhat" ]; then
    say "the contracts' npm packages are not installed: cd $CONTRACTS && npm ci" >&2
    ok=0
fi
for k in BurnPk BurnVk; do
    if [ ! -s "$PAY_ROOT/gnark-server/keys/$k.key" ]; then
        say "no gnark-server/keys/$k.key (gitignored): cd $PAY_ROOT/gnark-server && go run ./keygen/generate_keys.go -circuit burn" >&2
        ok=0
    fi
done
[ "$ok" = 1 ] || exit 1

for port in 8545 8080; do
    if listening "$port"; then
        say "port $port is already in use — stop whatever is running there first (every run needs a fresh chain)." >&2
        exit 1
    fi
done
say "logs: $LOG_DIR"

PIDS=()
cleanup() {
    trap - EXIT INT TERM
    say "stopping services…"
    for pid in ${PIDS[@]+"${PIDS[@]}"}; do
        kill -TERM -- "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
    done
    wait 2>/dev/null || true
    say "stopped. Logs: $LOG_DIR"
}
trap cleanup EXIT INT TERM

# start NAME DIR CMD... — runs CMD in DIR in its own process group, logging to $LOG_DIR/NAME.log.
start() {
    local name=$1 dir=$2
    shift 2
    (cd "$dir" && exec perl -e 'use POSIX qw(setsid); setsid(); exec @ARGV or die "exec: $!"' -- "$@" >"$LOG_DIR/$name.log" 2>&1) &
    PIDS+=($!)
}
wait_port() {
    local name=$1 port=$2
    for _ in $(seq 1 600); do
        listening "$port" && { say "$name is up on :$port"; return 0; }
        sleep 0.5
    done
    say "$name did not come up on :$port — last log lines:" >&2
    tail -20 "$LOG_DIR/$name.log" >&2
    exit 1
}
# step NAME DIR CMD... — runs CMD in DIR, logging to $LOG_DIR/NAME.log; on failure shows the log's tail.
step() {
    local name=$1 dir=$2
    shift 2
    say "${name}…"
    (cd "$dir" && "$@") >"$LOG_DIR/$name.log" 2>&1 || {
        say "$name failed — last log lines:" >&2
        tail -20 "$LOG_DIR/$name.log" >&2
        exit 1
    }
}

step compile "$CONTRACTS" npx hardhat compile
step build-relayer "$PAY_ROOT/relayer" env CGO_ENABLED=0 go build -o relayer_bin .
step build-gnark "$PAY_ROOT/gnark-server" go build -o "$BIN_DIR/gnark_server" ./cmd/server

start hardhat "$CONTRACTS" npx hardhat node
start gnark "$PAY_ROOT/gnark-server" "$BIN_DIR/gnark_server"
wait_port hardhat 8545
wait_port gnark 8080

say "stack is up (logs $LOG_DIR)"
wait
