#!/usr/bin/env bash
# Copy the enygma_dvp artifacts retail deploys and calls into contracts/abis/.
# This is the one list of them: setup.sh, shield_demo/run.sh and the README's
# manual steps all run this script. Run it after enygma_dvp's
# `npx hardhat compile && node regen_poseidon.mjs`.
#
#   bash enygma_retail_payments/scripts/copy_dvp_abis.sh   (from any directory)

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SRC="$ROOT/../enygma_dvp/artifacts"
DST="$ROOT/contracts/abis"

# Each entry: the artifact's path under enygma_dvp/artifacts,
# <sourceName>/<contract>.json. enygma_dvp/test/hardhat/RetailArtifacts.test.js
# reads this list and checks each copy against the build of its source.
ARTIFACTS=(
    contracts/core/contracts/Poseidon.sol/PoseidonT3.json
    contracts/core/contracts/Poseidon.sol/PoseidonT5.json
    contracts/core/contracts/PoseidonWrapper.sol/PoseidonWrapper.json
    contracts/core/contracts/GenericGroth16Verifier.sol/GenericGroth16Verifier.json
    contracts/core/contracts/Verifier.sol/Verifier.json
    contracts/core/contracts/EnygmaDvp.sol/EnygmaDvp.json
    contracts/erc20/contracts/RaylsERC20.sol/RaylsERC20.json
    contracts/core/contracts/vaults/Erc20CoinVault.sol/Erc20CoinVault.json
)

for artifact in "${ARTIFACTS[@]}"; do
    cp "$SRC/$artifact" "$DST/$(basename "$artifact")"
done
