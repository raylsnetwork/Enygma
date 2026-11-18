#!/usr/bin/env bash
# make retail-keys-check: fail when retail's committed Groth16 keys
# (enygma_retail_payments/gnark_circuits/scripts/keys/*.key, in LFS) were
# built from other circuit sources than the working tree's. Their stamp,
# keys.src beside them, is committed too: make retail-keygen writes it with
# scripts/keys_src.sh (the sources' content, not file times), and this
# compares it with the working tree's fingerprint (check_keys_stamp,
# scripts/stack_lib.sh). It never opens a .key, so it needs neither the LFS
# objects nor a chain. Without it, a PR changing a retail circuit or
# enygma_gnark_shared stays green until the retail or DvP on-chain suite
# (DvP copies retail's PrivateMint keys) fails on a proof.
#
#   scripts/retail_keys_check.sh   (make retail-keys-check, in make check)
#
# Tested by scripts/retail_keys_check.test.js.
set -euo pipefail
. "$(dirname "$0")/stack_lib.sh"
if ! check_keys_stamp "retail keys" enygma_retail_payments/gnark_circuits ./cmd/keygen \
    enygma_retail_payments/gnark_circuits/scripts/keys/keys.src retail-keygen; then
    say "after make retail-keygen, re-export what is built from the keys as enygma_retail_payments/setup.sh step 4/5 does (export_vk, export_verifier and the PrivateMintVerifier ABI); then make dvp-keygen, which copies the PrivateMint keys" >&2
    exit 1
fi
say "retail keys: built from the working tree's circuit sources"
