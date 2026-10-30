#!/usr/bin/env bash
# PROTOTYPE — privacy story variants (wallet zones, data chips, open wallet,
# Shield concept moment). No chain needed: the page replays a recorded
# default run (prototype/events.jsonl). Opens ?variant=A; flip with the
# bottom bar or ← →. ?live connects to a real engine instead.
cd "$(dirname "$0")" && (sleep 3; open "http://localhost:9092/?variant=A") & exec go run . -root .. -dev
