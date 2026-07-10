#!/usr/bin/env bash
# run.sh — build and start the Enygma payments relayer.
#
# Usage:
#   cp .env.example .env          # fill in RELAYER_PRIVATE_KEY and RELAYER_API_KEY
#   source .env && ./run.sh
#
# Or pass variables inline:
#   RELAYER_PRIVATE_KEY=abc123 RELAYER_API_KEY=secret ./run.sh

set -euo pipefail

# Guard: require the two secrets that have no safe defaults.
if [[ -z "${RELAYER_PRIVATE_KEY:-}" ]]; then
  echo "ERROR: RELAYER_PRIVATE_KEY is not set." >&2
  echo "  Copy .env.example to .env, fill in the value, then: source .env && ./run.sh" >&2
  exit 1
fi
if [[ -z "${RELAYER_API_KEY:-}" ]]; then
  echo "ERROR: RELAYER_API_KEY is not set." >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="${SCRIPT_DIR}/relayer_bin"

echo "Building relayer..."
CGO_ENABLED=0 go build -o "$BIN" .

echo "Starting relayer on port ${RELAYER_PORT:-8082}..."
exec "$BIN"
