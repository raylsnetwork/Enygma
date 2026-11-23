#!/usr/bin/env bash
# Which on-chain suites an event runs, for the onchain workflow's `changes`
# job: prints payments=, dvp= and retail=, each true or false. Reads the
# changed files, one per line, on stdin, and runs a suite when one of them
# matches that suite's paths. With --base <commit> (a pull request) it reads
# them from git instead: every file HEAD changes since its merge base with
# commit, a rename as both its old and its new path, so a file moved out of
# a suite's paths runs that suite. With --all (a push to main) it runs every
# suite and reads nothing. With --wiring it prints the suites' shared wiring
# files (common below), one per line: mutant.sh refuses to mutate one while a
# stack is up.
#
#   bash scripts/suite_changes.sh --base "$BASE"
#   git diff --name-only --no-renames "$BASE...HEAD" | bash scripts/suite_changes.sh
#   bash scripts/suite_changes.sh --all
#   bash scripts/suite_changes.sh --wiring
set -euo pipefail

# Each suite's paths (globs; * also matches /): what its stack builds,
# deploys or copies in, plus the suites' shared wiring: the workflow, the
# root Makefile's stack and suite targets, this script, the suites' verdict
# and every stack script with the helpers they share, so a change to how CI
# runs one suite is tested on all three. suite_changes.test.js checks that
# stacks holds every stack script and that onchain.yml's paths cover common.
stacks="enygma_payments/run_scripts/stack.sh enygma_dvp/scripts/stack.sh enygma_retail_payments/shield_demo/run.sh scripts/stack_lib.sh scripts/stack_start.sh scripts/stack_stop.sh scripts/stack_service.sh scripts/stack_reaper.sh scripts/stack_groups.sh scripts/own_group.sh"
common="Makefile .github/workflows/onchain.yml scripts/suite_changes.sh scripts/suite_result.sh $stacks"
# make payments-burn-keygen and make dvp-keygen stamp their keys (and CI
# keys their caches) with keys_src.sh's fingerprint.
# The payments stack compiles enygma_dvp's contracts too: the suite deploys
# the real EnygmaErc20CoinVault and Poseidon from them.
dvp_for_payments="enygma_dvp/contracts/* enygma_dvp/hardhat.config.js enygma_dvp/package.json enygma_dvp/package-lock.json enygma_dvp/scripts/regen_poseidon.js enygma_dvp/regen_poseidon.mjs"
paths_payments="$common enygma_payments/* enygma_gnark_shared/* scripts/keys_src.sh $dvp_for_payments"
# make dvp-keygen also copies retail's PrivateMint keys.
paths_dvp="$common enygma_dvp/* enygma_gnark_shared/* enygma_retail_payments/gnark_circuits/scripts/keys/PrivateMint* scripts/keys_src.sh"
# The retail stack runs enygma_dvp's Hardhat node and deploys its contracts;
# the retail test module imports enygma_dvp's src and layout.
dvp_for_retail="enygma_dvp/contracts/* enygma_dvp/src/* enygma_dvp/layout/* enygma_dvp/hardhat.config.js enygma_dvp/package.json enygma_dvp/package-lock.json enygma_dvp/regen_poseidon.mjs enygma_dvp/.nvmrc"
paths_retail="$common enygma_retail_payments/* enygma_gnark_shared/* $dvp_for_retail"

if [ "${1:-}" = --wiring ]; then
  printf '%s\n' $common
  exit 0
fi

all=false changed=
case ${1:-} in
  --all) all=true ;;
  --base) changed=$(git diff --name-only --no-renames "$2...HEAD") ;;
  *) changed=$(cat) ;;
esac

set -f # the globs are patterns for [[ ]], not for the working dir
for suite in payments dvp retail; do
  paths_var=paths_$suite
  run=$all
  while IFS= read -r f; do
    # Docs and the demo pages: no suite reads a .md or .html file.
    [[ -n $f && $f != *.md && $f != *.html ]] || continue
    for p in ${!paths_var}; do
      if [[ $f == $p ]]; then run=true; fi
    done
  done <<<"$changed"
  echo "$suite=$run"
done
