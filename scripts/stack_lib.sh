# Helpers for the local stack scripts (enygma_dvp/scripts/stack.sh,
# enygma_payments/run_scripts/stack.sh, enygma_retail_payments/shield_demo/run.sh):
# start services, wait for their ports, build steps, and stop everything on a
# signal, a failure or a service exiting. Sourced, after set -euo pipefail:
#
#   . "$REPO_ROOT/scripts/stack_lib.sh"
#   use_log_dir dvp_stack      # LOG_DIR (default: a fresh temp dir) and BIN_DIR
#   use_node "$(cat .nvmrc)"   # Node 20+ through nvm when the shell's is older
#   check_keys_stamp keys enygma_dvp/gnark_circuits ./cmd/keygen \
#     enygma_dvp/gnark_circuits/scripts/keys/keys.src dvp-keygen || ok=0
#   require_free_ports 8545 8081
#   step compile "$DIR" npx hardhat compile
#   start hardhat "$DIR" npx hardhat node
#   wait_port hardhat 8545
#   say "stack is up (logs $LOG_DIR)"
#   watch_services             # until a service exits, or a signal
#
# Tested by scripts/stack_lib.test.js.

# say MSG… — prints MSG tagged with the stack script's name.
say() { echo "[$(basename "$0")] $*"; }
# listening PORT — whether something accepts connections on 127.0.0.1:PORT.
listening() { (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null; }

# go-ethereum needs CGo; Homebrew clang on macOS points at a missing SDK.
if [ "$(uname -s)" = Darwin ]; then export CC="${CC:-/usr/bin/clang}"; fi

# use_log_dir PREFIX — LOG_DIR, made absolute (services start in their own
# dirs), defaulting to a fresh temp dir named PREFIX.*; BIN_DIR within it.
use_log_dir() {
    LOG_DIR="${LOG_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/$1.XXXXXX")}"
    mkdir -p "$LOG_DIR"
    LOG_DIR="$(cd "$LOG_DIR" && pwd)"
    BIN_DIR="$LOG_DIR/bin"
    mkdir -p "$BIN_DIR"
}

# node_major — the shell's Node major version, 0 when there is none.
node_major() { node -p 'process.versions.node.split(".")[0]' 2>/dev/null || echo 0; }
# use_node VERSION — a shell that hasn't loaded nvm (an agent's, a fresh
# terminal) may be on a Node too old for Hardhat: switch to VERSION with nvm
# when it's there. The caller's preflight still checks node_major.
use_node() {
    local nvm_sh="${NVM_DIR:-$HOME/.nvm}/nvm.sh"
    if [ "$(node_major)" -lt 20 ] && [ -s "$nvm_sh" ]; then
        set +u # nvm.sh isn't safe under set -u
        . "$nvm_sh"
        nvm use --silent "$1" >/dev/null 2>&1 || true
        set -u
    fi
}

# check_keys_stamp KEYS MODULE KEYGEN STAMP TARGET — fails (returns 1) on
# keys built from other circuit sources than the working tree's: they give
# proofs that fail on-chain, or a gnark panic in Prove, minutes in. make
# TARGET stamps the keys in STAMP's dir with scripts/keys_src.sh MODULE
# KEYGEN (the sources' content), so a rebase, which moves file times only,
# passes. MODULE and STAMP are repo-relative; the message names the keys as
# "the KEYS in" STAMP's dir within its project (enygma_dvp, enygma_payments).
# It fails, naming TARGET, on a missing (or empty) stamp and on another one,
# told apart; and on sources keys_src.sh cannot fingerprint, naming no
# target, since TARGET's stamp would fail the same way.
check_keys_stamp() {
    [ $# -eq 5 ] || { echo "usage: check_keys_stamp <keys> <module dir> <keygen package> <stamp file> <make target>" >&2; exit 2; }
    local keys=$1 module=$2 keygen=$3 stamp=$4 target=$5 lib_dir src at
    lib_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    at=$(dirname "${stamp#*/}")
    stamp="$lib_dir/../$stamp"
    if ! src=$("$lib_dir/keys_src.sh" "$module" "$keygen"); then
        say "scripts/keys_src.sh could not fingerprint the circuit sources (above), so the $keys cannot be checked" >&2
        return 1
    elif [ "$(cat "$stamp" 2>/dev/null)" != "$src" ]; then
        if [ -s "$stamp" ]; then
            say "the $keys in $at were built from other circuit sources than the working tree's: make $target" >&2
        else
            say "the $keys in $at have no record of the circuit sources they were built from ($(basename "$stamp")): make $target" >&2
        fi
        return 1
    fi
}

# require_free_ports PORT… — fails if any PORT is taken: every run needs a fresh chain.
require_free_ports() {
    local port
    for port in "$@"; do
        if listening "$port"; then
            say "port $port is already in use — stop whatever is running there first (every run needs a fresh chain)." >&2
            exit 1
        fi
    done
}

PIDS=() NAMES=()
# cleanup — stops every service start started. Quiet before the first one, so
# a preflight or port-check failure ends on its own message.
cleanup() {
    trap - EXIT INT TERM
    [ -n "${PIDS[*]+x}" ] || return 0
    echo "" # after a ^C, on a line of its own
    say "stopping services…"
    # ${PIDS[@]+…}: bash 3.2 (macOS) calls an empty array unbound under set -u.
    for pid in ${PIDS[@]+"${PIDS[@]}"}; do
        # Each service runs in its own process group; kill the whole group so
        # children (npx → node) go too.
        kill -TERM -- "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
    done
    wait 2>/dev/null || true
    say "stopped. Logs: $LOG_DIR"
}
trap cleanup EXIT
# A signal exits, which runs cleanup through the EXIT trap: a trap that only
# returned would put the script back in its wait loop, to find the services
# it just stopped gone and report them dead.
trap 'exit 130' INT
trap 'exit 143' TERM

# start NAME DIR CMD… — runs CMD in DIR in its own process group, logging to $LOG_DIR/NAME.log.
start() {
    local name=$1 dir=$2
    shift 2
    # perl's setsid: stock macOS has no setsid(1).
    (cd "$dir" && exec perl -e 'use POSIX qw(setsid); setsid(); exec @ARGV or die "exec: $!"' -- "$@" >"$LOG_DIR/$name.log" 2>&1) &
    PIDS+=($!)
    NAMES+=("$name")
}

# died NAME — NAME's service exited: shows its log's tail and fails.
# scripts/stack_stop.sh fails a stop whose log holds this line: keep them in step.
died() {
    say "$1 exited — last log lines:" >&2
    tail -20 "$LOG_DIR/$1.log" >&2
    exit 1
}

# wait_port NAME PORT — waits for service NAME to listen on PORT, failing at
# once if it exits.
wait_port() {
    local name=$1 port=$2 i pid=
    for i in "${!NAMES[@]}"; do [ "${NAMES[$i]}" != "$name" ] || pid=${PIDS[$i]}; done
    [ -n "$pid" ] || { say "wait_port: no service named $name was started" >&2; exit 1; }
    for _ in $(seq 1 600); do
        listening "$port" && { say "$name is up on :$port"; return 0; }
        kill -0 "$pid" 2>/dev/null || died "$name"
        sleep 0.5
    done
    say "$name did not come up on :$port — last log lines:" >&2
    tail -20 "$LOG_DIR/$name.log" >&2
    exit 1
}

# step NAME DIR CMD… — runs CMD in DIR, logging to $LOG_DIR/NAME.log; on failure shows the log's tail.
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

# watch_services — returns never: up while every service runs; when one
# exits, fails with its log, and the trap stops the rest, so the stack never
# looks up with a service gone. (Polled: macOS's bash 3.2 has no wait -n.)
watch_services() {
    local i
    while :; do
        for i in "${!PIDS[@]}"; do
            kill -0 "${PIDS[$i]}" 2>/dev/null || died "${NAMES[$i]}"
        done
        sleep 1
    done
}
