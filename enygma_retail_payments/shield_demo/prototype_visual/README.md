# PROTOTYPE — retail shield demo visual (#124)

Throwaway. It answers one question for #123: **what should the new demo look like?**
It is not the demo and is kept only on the `prototype/shield-demo-visual` branch.

**Open it:** double-click `index.html` (no server, no stack needed). Press **RUN** to replay.

- `?variant=A|B|C` (or ← / → and the bottom bar) switches between the three light-theme variants:
  - **A · Lanes**: swimlanes (actors / services / contracts), each hop a straight arrow with a label pill, a vertical timeline and actor cards.
  - **B · Columns**: left-to-right columns (off-chain → services → on-chain), each hop a curved arrow with a travelling packet, a "now" caption, a numbered stepper and a ledger table.
  - **C · Sequence**: a sequence diagram (each node is a lifeline, hops stack as arrows), a narrative step feed and big balance tiles.
- `?autoplay` starts the replay on load; `?theme=dark` forces the dark theme (◐ toggles it and remembers it).
- ⏭ jumps to the end of the run; the speed select replays at 1×, 2× or 4×. Hop pacing (at least 600 ms per hop, with catch-up when more than 3 s behind) stays in real time.
- Hover a node for its address and protocol fields. The terminal drawer is shared by all variants. It can be resized (drag its top edge) and collapsed, and the hardhat tab has "hide RPC reads".

**The data** in `recording.js` comes from a real default-mode run (USDr fee + tags) on 2026-10-28: the demo's event stream plus the hardhat, gnark and relayer output written during the run. The engine doesn't emit hops yet, so `shared.js` (`hopsFor`) derives them from the recorded log lines. To keep the file small, the hardhat tab keeps every transaction but only 1 in 4 RPC reads. Re-record against a running `../run.sh` with `bash record.sh <its log dir>`.
