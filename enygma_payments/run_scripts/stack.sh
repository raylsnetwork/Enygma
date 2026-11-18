#!/usr/bin/env bash
# enygma_payments' end-to-end test stack in one command: compiles the
# contracts and builds relayer/relayer_bin (both gitignored, so a stale copy
# would otherwise be tested), then starts a fresh Hardhat node on :8545 and
# the gnark server on :8080. The suite deploys its own contracts and spawns
# relayer_bin itself. Ctrl-C, any failure, or either service exiting stops
# what it started.
#
# Usage:
#   bash enygma_payments/run_scripts/stack.sh
#   make payments-stack       # the same, detached: returns once the stack is up
#   make payments-test        # then the suite
#   make payments-stack-stop
#
# Keys: the Burn keys are gitignored; generate them once, and after a
# payments circuit change: make payments-burn-keygen (from the repo root). It
# stamps them with their circuit sources (keys/burn.src), and the preflight
# fails on keys built from other sources than the working tree's.
# Logs and the built gnark server go to $LOG_DIR.
set -euo pipefail
PAY_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CONTRACTS="$PAY_ROOT/contracts/enygma"
. "$PAY_ROOT/../scripts/stack_lib.sh"
use_log_dir payments_stack
use_node 22

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
KEYS="$PAY_ROOT/gnark-server/keys"
for k in BurnPk BurnVk; do
    if [ ! -s "$KEYS/$k.key" ]; then
        say "no gnark-server/keys/$k.key (gitignored): make payments-burn-keygen" >&2
        ok=0
    fi
done
# make payments-burn-keygen stamps the Burn keys with their circuit sources.
check_keys_stamp 'Burn keys' enygma_payments/gnark-server ./keygen \
    enygma_payments/gnark-server/keys/burn.src payments-burn-keygen || ok=0
[ "$ok" = 1 ] || exit 1

require_free_ports 8545 8080
say "logs: $LOG_DIR"

step compile "$CONTRACTS" npx hardhat compile
step build-relayer "$PAY_ROOT/relayer" env CGO_ENABLED=0 go build -o relayer_bin .
step build-gnark "$PAY_ROOT/gnark-server" go build -o "$BIN_DIR/gnark_server" ./cmd/server

start hardhat "$CONTRACTS" npx hardhat node
start gnark "$PAY_ROOT/gnark-server" "$BIN_DIR/gnark_server"
wait_port hardhat 8545
wait_port gnark 8080

say "stack is up (logs $LOG_DIR)"
watch_services
