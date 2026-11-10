#!/usr/bin/env bash
# PROTOTYPE (tag channel): serve the shield demo and the primer from disk, no chain needed.
#   demo:   http://localhost:9092/?replay&variant=A   (A Arcs, B Keyring, C Matrix + sealed route)
#   primer: http://localhost:9092/protocol?variant=A#channel   (A On the stage, B Sequence, C Key tree)
# Flip variants with the bottom bar (or ← → on the demo, [ ] on the primer, whose arrows step).
cd "$(dirname "$0")" && CC=/usr/bin/clang exec go run . -root .. -dev
