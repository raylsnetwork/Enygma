#!/usr/bin/env bash
# Print a fingerprint of the Go sources the shield demo binary is built
# from: the content of every non-test .go file, go.mod and go.sum in its
# module and in each local module it replaces (go list, so a new replace
# counts at once), committed or not.
# run.sh and make shield-demo-restart write it beside the binary they
# build (shield_demo.src); make shield-demo-stale compares it with the
# working tree's, so a demo built before an engine change is caught.
#
#   scripts/shield_demo_src.sh
set -euo pipefail
cd "$(git -C "$(dirname "$0")" rev-parse --show-toplevel)/enygma_retail_payments/shield_demo"
dirs=(. $(go list -m -f '{{with .Replace}}{{if not .Version}}{{.Path}}{{end}}{{end}}' all | sort -u))
git ls-files -co --exclude-standard -- "${dirs[@]}" |
  grep -E '(\.go|/go\.mod|/go\.sum)$' | grep -v '_test\.go$' | LC_ALL=C sort |
  while IFS= read -r f; do [ -f "$f" ] && shasum -a 256 "$f"; done |
  shasum -a 256 | cut -d' ' -f1
