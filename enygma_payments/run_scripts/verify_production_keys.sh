#!/usr/bin/env bash
# Verify the installed keys and reviewed main verifier before a remote deploy.
set -euo pipefail
: "${CEREMONY_SRS:?set CEREMONY_SRS to the absolute path of the approved Phase 1 SRS}"
: "${CEREMONY_PHASE1_SEAL_SHA256:?set CEREMONY_PHASE1_SEAL_SHA256 to the approved Phase 1 seal digest}"
: "${CEREMONY_PHASE2_SEAL_SHA256:?set CEREMONY_PHASE2_SEAL_SHA256 to the approved payments Phase 2 seal digest}"

root="$(cd "$(dirname "$0")/../.." && pwd)"
gnark="$root/enygma_payments/gnark-server"
r1cs="$(mktemp -d)"
trap 'rm -rf "$r1cs"' EXIT

go -C "$gnark" run ./cmd/export_r1cs -out "$r1cs"
go -C "$root/enygma_ceremony" run ./cmd/ceremony verify-keys \
    -ceremony enygma_payments -srs "$CEREMONY_SRS" -r1cs "$r1cs" -keys "$gnark" \
    -phase1-seal-sha256 "$CEREMONY_PHASE1_SEAL_SHA256" -phase2-seal-sha256 "$CEREMONY_PHASE2_SEAL_SHA256"
go -C "$gnark" run ./cmd/verify_deployed_verifier -vk keys/EnygmaVk.key -artifact "$1"
