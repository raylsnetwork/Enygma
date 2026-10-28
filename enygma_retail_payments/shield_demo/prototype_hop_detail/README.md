# PROTOTYPE — retail shield demo hop detail (#131)

Throwaway. It answers one question for #129: **what should the hop detail panel and the tree view look like?**
It is not the demo and is kept only on the `prototype/shield-demo-hop-detail` branch.

**Open it:** double-click `index.html` (no server, no stack needed) and press **RUN**. It opens on variant A; ← / → switch, and the URL keeps the choice (`index.html?variant=B`) for a reload or a bookmark.

The page is a copy of `../index.html` (variant B of #124, as shipped in #123) with a hop detail panel next to the diagram. `?variant=A|B|C` (or ← / → and the bottom bar) switches the panel:

- **A · Inspector**: a 420 px sidebar. The plain-language sentence, then the vault the hop touches in front (pruned tree over a 4 × 64 leaf strip), the other vault behind as one thin strip, and the real values folded under "Real values".
- **B · Tree stage**: half the width. Both vaults side by side as big trees, the one the hop doesn't touch dimmed; "values on the tree" writes the root and sibling hashes on the nodes. A proof's public inputs sit beside what stays private.
- **C · Feed**: a column left of the diagram. A dock with a sideways tree and 16 × 16 leaf tiles for both vaults, then a feed of detail cards, the newest open, older ones one line each (click to open).

All three have the "demo's view: show owners" toggle, short hex with the full value on hover and copy on click, full trees as chips, and the stepper: click a finished step to see its hop details again (‹ › between them, "back to live" to follow the run).

Other parameters: `?from=transfer_6` jumps to a step (the Payment2in merge), `?speed=2|4`, `?owners`, `?revisit=<step>`, `?autoplay`. ⏭ jumps to the end. Hops with a hop detail hold 1.5 s on the diagram instead of 600 ms; the page's catch-up rule is unchanged.

**The data.** `recording_124.js` is #124's recording of a real default-mode run (USDr fee + tags). `make_recording.py` turns it into `recording.js`, adding by hand what #129's engine will send: the Hop events the engine emits today, hop details on the hops that change protocol state (`commitment`, `merklePath`, `publicInputs`, `settleChecks`, `leavesIn`, shaped as in #129), and a tree snapshot of both vaults at run start. The trees are modelled for real (depth 8, numbered trees, a new tree when a transaction's batch would fill the current one), hashed with sha256 instead of Poseidon; commitments, siblings and roots are made up, except the recorded hex prefixes. The USDr vault is made up near the end of tree 1, so the run rolls it into tree 2 and later spends notes from the full tree 1. Re-run `python3 make_recording.py > recording.js` after changing it.

**Questions this raised for #129:**

- A prove hop needs two details (the Merkle path and the public inputs), so `hopDetail` is a list here.
- The `commitment` detail carries a `rootAfter`, so the tree view's root stays current after a shield or grant.
- The panel at the diagram's height is short: every variant scrolls on a 1000 px-tall window.
