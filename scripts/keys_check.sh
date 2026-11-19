#!/usr/bin/env bash
# make retail-keys-check and make payments-keys-check: fail when a module's
# committed Groth16 keys (in LFS) were built from other circuit sources than
# the working tree's. Their stamp, keys.src beside them, is committed too:
# the module's keygen target writes it with scripts/keys_src.sh (the
# sources' content, not file times), and this compares it with the working
# tree's fingerprint (check_keys_stamp, scripts/stack_lib.sh). It never
# opens a .key, so it needs neither the LFS objects nor a chain. Without it,
# a PR changing a circuit or enygma_gnark_shared stays green until an
# on-chain suite fails on a proof.
#   retail    enygma_retail_payments/gnark_circuits/scripts/keys/*.key, from
#             make retail-keygen (the retail and DvP suites: DvP copies
#             retail's PrivateMint keys)
#   payments  enygma_payments/gnark-server/keys/{,zkdvp/}*.key, from make
#             payments-keygen (the payments suite)
#
#   scripts/keys_check.sh retail|payments   (make <module>-keys-check, in make check)
#
# Tested by scripts/keys_check.test.js.
set -euo pipefail
. "$(dirname "$0")/stack_lib.sh"
module=${1:-}
case "$module" in
    retail)
        dir=enygma_retail_payments/gnark_circuits keygen=./cmd/keygen stamp=scripts/keys/keys.src
        next_steps="after make retail-keygen, re-export what is built from the keys as enygma_retail_payments/setup.sh step 4/5 does (export_vk, export_verifier and the PrivateMintVerifier ABI); then make dvp-keygen, which copies the PrivateMint keys" ;;
    payments)
        dir=enygma_payments/gnark-server keygen=./keygen stamp=keys/keys.src
        next_steps="after make payments-keygen, copy each regenerated verifier into the contracts: enygma_payments/gnark-server/keys/EnygmaVerifier.sol to enygma_payments/contracts/enygma/contracts/EnygmaVerifier.sol, and likewise EnygmaFeeVerifier.sol and UsdrVerifier.sol. keys/zkdvp's DepositVerifier.sol and WithdrawVerifier6.sol have no copy to update: nothing deploys them (the suites register mock Deposit and Withdraw verifiers)" ;;
    *)
        echo "usage: scripts/keys_check.sh retail|payments" >&2
        exit 2 ;;
esac
if ! check_keys_stamp "$module keys" "$dir" "$keygen" "$dir/$stamp" "$module-keygen"; then
    say "$next_steps" >&2
    exit 1
fi
say "$module keys: built from the working tree's circuit sources"
