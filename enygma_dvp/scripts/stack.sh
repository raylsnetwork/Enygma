#!/usr/bin/env bash
# enygma_dvp's on-chain test stack in one command: a fresh Hardhat node, the
# main deployment and the dedicated Payment-family deployment (both deployed
# and initialised), the gnark server on :8081, and the two relayers: :8091 on
# the main deployment (test 10) and :8092 on the Payment deployment (tests 12,
# 15, 26, 27; RELAYER_MIN_FEE=1). With all of it up, no integration test
# skips for a missing service. Ctrl-C (or any failure) stops what it started.
#
# Usage:
#   cd enygma_dvp && bash scripts/stack.sh
#   make dvp-stack       # the same, detached: returns once the stack is up
#   make dvp-test        # then the suite, once per fresh stack
#   make dvp-stack-stop
#
# Keys: make dvp-keygen once, and after a circuit change. VK slots whose
# legacy key file is missing get unprovable placeholders (export_vk_init
# -placeholder); no integration test uses them. Logs, built binaries and
# $LOG_DIR/test.env (what the suite needs from the relayers) go to $LOG_DIR.
set -euo pipefail
DVP_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REPO_ROOT="$(cd "$DVP_ROOT/.." && pwd)"
. "$REPO_ROOT/scripts/stack_lib.sh"
use_log_dir dvp_stack
KEYS="$DVP_ROOT/gnark_circuits/scripts/keys"
use_node "$(cat "$DVP_ROOT/.nvmrc")"

# preflight — fails in a second, with the fix, on what otherwise fails
# minutes in.
preflight() {
    local ok=1 k newest
    if [ "$(node_major)" -lt 20 ]; then
        say "Node $(node -v 2>/dev/null || echo missing) is too old for Hardhat; use 20 or later: cd $DVP_ROOT && nvm use" >&2
        ok=0
    fi
    if [ ! -x "$DVP_ROOT/node_modules/.bin/hardhat" ]; then
        say "enygma_dvp's npm packages are not installed: cd $DVP_ROOT && npm ci" >&2
        ok=0
    fi
    # The keys every test proves with: the ones keygen makes.
    for k in PrivateMint DvPInitiator DvPDestination Payment Payment2in PaymentFee PaymentRelayerFeePublic UsdrFee; do
        if [ ! -s "$KEYS/${k}PK.key" ] || [ ! -s "$KEYS/${k}VK.key" ]; then
            say "no ${k} keys in gnark_circuits/scripts/keys: make dvp-keygen" >&2
            ok=0
        fi
    done
    # PrivateMintVerifier.sol is committed for retail's PrivateMint keys.
    if [ -s "$KEYS/PrivateMintVK.key" ] &&
        ! cmp -s "$KEYS/PrivateMintVK.key" "$REPO_ROOT/enygma_retail_payments/gnark_circuits/scripts/keys/PrivateMintVK.key"; then
        say "PrivateMintVK.key is not retail's, so it does not match PrivateMintVerifier.sol: make dvp-keygen copies it" >&2
        ok=0
    fi
    [ "$ok" = 1 ] || exit 1
    # Keys older than the last circuit change are stale: proofs from them fail
    # on-chain verification. A warning, since a rebase also moves the time.
    newest=$(git -C "$REPO_ROOT" log -1 --format=%ct -- enygma_dvp/gnark_circuits/templates enygma_gnark_shared 2>/dev/null || echo 0)
    if [ "$(stat -c %Y "$KEYS/PaymentVK.key" 2>/dev/null || stat -f %m "$KEYS/PaymentVK.key")" -lt "$newest" ]; then
        say "WARNING: the keys predate the last circuit change ($(git -C "$REPO_ROOT" log -1 --format='%h %s' -- enygma_dvp/gnark_circuits/templates enygma_gnark_shared)); if proofs fail on-chain: make dvp-keygen" >&2
    fi
}
preflight

require_free_ports 8545 8081 8091 8092
say "logs: $LOG_DIR"

step compile "$DVP_ROOT" bash -c 'npx hardhat compile && node scripts/regen_poseidon.js'
step build "$DVP_ROOT" bash -c "cd scripts && for c in deploy init deploy_payment init_payment; do go build -o '$BIN_DIR'/\$c ./cmd/\$c || exit 1; done &&
    cd ../gnark_circuits && go build -o '$BIN_DIR'/gnark_server ./cmd/server && cd ../relayer && go build -o '$BIN_DIR'/relayer ."

start hardhat "$DVP_ROOT" npx hardhat node
wait_port hardhat 8545

step deploy "$DVP_ROOT" "$BIN_DIR/deploy"
step export-vk "$DVP_ROOT" bash -c 'cd gnark_circuits && go run ./cmd/export_vk_init -placeholder ../build'
step init "$DVP_ROOT" "$BIN_DIR/init"
step deploy-payment "$DVP_ROOT" "$BIN_DIR/deploy_payment"
step export-vk-payment "$DVP_ROOT" bash -c 'cd gnark_circuits && go run ./cmd/export_vk_payment ../build'
step init-payment "$DVP_ROOT" "$BIN_DIR/init_payment"

start gnark "$DVP_ROOT/gnark_circuits" "$BIN_DIR/gnark_server"
wait_port gnark 8081

# accounts[2] relays for the main deployment (init registers it), accounts[3]
# for the Payment one, so their nonces stay apart.
read -r KEY2 KEY3 DVP_ADDR PAYMENT_DVP_ADDR < <(cd "$DVP_ROOT" && node -e '
  const a = require("./enygmadvp.config.json").network.accounts;
  const addr = (r) => typeof r === "string" ? r : r.contractAddress;
  console.log(a[2].private, a[3].private,
    addr(require("./build/receipts.json").EnygmaDvp),
    addr(require("./build/payment_receipts.json").EnygmaDvp));')
FEE_SPEND_KEY=123456789
FEE_NOTES_8091="$LOG_DIR/fee_notes_8091.jsonl"
FEE_NOTES_8092="$LOG_DIR/fee_notes_8092.jsonl"
start relayer-8091 "$DVP_ROOT/relayer" env RELAYER_PRIVATE_KEY="$KEY2" RELAYER_DVP_ADDR="$DVP_ADDR" \
    RELAYER_API_KEY=test-api-key-dev-only RELAYER_PORT=8091 RELAYER_FEE_NOTES_PATH="$FEE_NOTES_8091" "$BIN_DIR/relayer"
start relayer-8092 "$DVP_ROOT/relayer" env RELAYER_PRIVATE_KEY="$KEY3" RELAYER_DVP_ADDR="$PAYMENT_DVP_ADDR" \
    RELAYER_RECEIPTS_PATH=../build/payment_receipts.json RELAYER_MIN_FEE=1 RELAYER_FEE_SPEND_PRIVATE_KEY="$FEE_SPEND_KEY" \
    RELAYER_API_KEY=test-api-key-dev-only RELAYER_PORT=8092 RELAYER_FEE_NOTES_PATH="$FEE_NOTES_8092" "$BIN_DIR/relayer"
wait_port relayer-8091 8091
wait_port relayer-8092 8092

# Test 12's fee-note spends and test 25 read the :8092 relayer's key and store.
cat >"$LOG_DIR/test.env" <<EOF
RELAYER_FEE_SPEND_PRIVATE_KEY=$FEE_SPEND_KEY
RELAYER_FEE_NOTES_PATH=$FEE_NOTES_8092
EOF
say "stack is up (logs $LOG_DIR)"
watch_services
