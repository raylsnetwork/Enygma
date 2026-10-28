#!/usr/bin/env bash
# PROTOTYPE (#124) — records one default-mode run of the shield demo into
# recording.js. Needs the stack and the demo up (bash ../run.sh) and the
# launcher's log directory:
#
#   bash record.sh /var/folders/…/shield_demo.XXXX
set -euo pipefail

LOG_DIR=${1:?usage: record.sh <run.sh log directory>}
HERE="$(cd "$(dirname "$0")" && pwd)"
REC="$(mktemp -d -t shield_demo_rec)"

for f in hardhat gnark relayer; do wc -l <"$LOG_DIR/$f.log" >"$REC/$f.start"; done
curl -sN localhost:9092/events >"$REC/events.sse" &
CURL=$!
trap 'kill $CURL 2>/dev/null || true' EXIT
sleep 1
curl -s -X POST localhost:9092/run -d "$(curl -s localhost:9092/defaults)"
echo
until grep -q '"kind":"done"\|"kind":"failed"' "$REC/events.sse"; do sleep 2; done
sleep 1
for f in hardhat gnark relayer; do tail -n +$(($(cat "$REC/$f.start") + 1)) "$LOG_DIR/$f.log" >"$REC/$f.log"; done
sed -n 's/^data: //p' "$REC/events.sse" >"$REC/events.jsonl"
python3 "$HERE/make_recording.py" "$REC" >"$HERE/recording.js"
echo "recorded → $HERE/recording.js"
