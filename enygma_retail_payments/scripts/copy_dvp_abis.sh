#!/usr/bin/env bash
# Copy the enygma_dvp artifacts retail deploys and calls into contracts/abis/.
# This is the one list of them: setup.sh, shield_demo/run.sh and the README's
# manual steps all run this script. Run it after enygma_dvp's
# `npx hardhat compile && node regen_poseidon.mjs`.
#
#   bash enygma_retail_payments/scripts/copy_dvp_abis.sh   (from any directory)

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SRC="$ROOT/../enygma_dvp/artifacts/contracts/core/contracts"
DST="$ROOT/contracts/abis"

# Each entry: the artifact's path under enygma_dvp/artifacts/contracts/core/contracts.
ARTIFACTS=(
    vaults/Erc20CoinVault.sol/Erc20CoinVault.json
    EnygmaDvp.sol/EnygmaDvp.json
    Verifier.sol/Verifier.json
)

for artifact in "${ARTIFACTS[@]}"; do
    cp "$SRC/$artifact" "$DST/$(basename "$artifact")"
done
