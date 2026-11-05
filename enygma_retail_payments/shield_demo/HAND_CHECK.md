# Hand-checking a shield demo page change

A page ticket's acceptance is checked by hand in Chrome against the `run.sh` stack. A scenario run takes 3–5 minutes, so each wasted run is expensive.

## What the replay test covers

`make page-check` (and CI) runs `scripts/shield_demo_replay.test.js`. It loads `index.html` in jsdom as `/?fast` with pausing off and plays two real runs on a virtual clock: a default run (`testdata/default_run.events.json`, which a passing `TestShieldDemoScenario` rewrites, `make shield-test` too) and a walkthrough (`testdata/walkthrough.events.json`, which a passing `TestShieldDemoWalkthrough` rewrites: `cd test && CC=/usr/bin/clang go test -run 'TestShieldDemoWalkthrough$' -v`, under a minute). Commit both with an engine change. The default run's events are fed as `/events` sends them. The walkthrough is one the page starts: its Walkthrough button sends the fixture's run options, the test presses Next at every gate and feeds each release's events only after it, and `/release` and the on-chain proof check's `eth_call` are answered as the fixture's run answered them. After each run the test clicks each finished step and steps through all its hop details with the Real values and Request and response / Receipt folds open. It fails on:

- any exception the page throws while it plays or replays a run (an empty receipt's missing `events` list, say);
- a step not shown as succeeded at the end, or a step whose replay has a different number of hop details than the run sent;
- a field element (a run of 16+ digits) of an HTTP request or response body that the panel's body does not show digit for digit, proofs aside; and a body with a proof not shown in full behind "show in full";
- in the walkthrough, Next pressed (or `/release` sent) a different number of times than the run had gates; payment 1's columns, the replay's refusal or the on-chain proof check card never showing; and the check not showing `verifyProof → true`, then `false` once tampered.

jsdom does no layout, timing or theming, and the fixtures are a default run and a walkthrough with the form's amounts. So the hand check is still needed for layout, themes, sizes and collapsed cards, timing and pacing, hidden tabs, pausing on, the walkthrough's Space key, handover and leaving it, and full runs with fees or tags off; it need not look for console exceptions on a default run or a walkthrough, or check bodies digit for digit.

## Steps

1. **Check last.** Run the hand check after `/code-review` and its fixes, on the code you will commit. A run started earlier tests code the review may still change. Done when the working tree holds the reviewed change.
2. **Serve the current page on a live stack.** The demo server can outlive the chain, gnark server and relayer; a run against it then fails at its first step. Check all four:
   ```sh
   for p in 8545 8082 8090 9092; do lsof -iTCP:$p -sTCP:LISTEN -nP >/dev/null && echo "$p up" || echo "$p DOWN"; done
   ```
   Any port down: stop the leftover demo (`pkill -f 'shield_demo -root'`) and start the whole stack afresh with `bash run.sh` (in the background; it prints `demo is up on :9092` when ready). `run.sh` starts the demo with `-dev`, so the page is read from disk on every request. A stack started before `-dev` existed (check `ps ax | grep '[s]hield_demo -root'` for the flag) needs the demo rebuilt once and relaunched detached with the same `-root`/`-logs` args plus `-dev`; the chain, gnark server and relayer keep running. `-dev` covers page edits only: after an engine (Go) change, `make shield-demo-restart` rebuilds the demo and relaunches it on the running stack. Done when all four ports are up and `curl -s localhost:9092/` shows your edit.
3. **Open the page with the `navigate` tool**, as `http://localhost:9092/?fast` when the check needs the end of a run. Reloading from inside `javascript_tool` (`location.reload()`) kills the evaluation mid-call.
4. **Install the probe, then start the run** (below). It records what the check needs without depending on the page's timers.
5. **Wait in the background** (`sleep` with `run_in_background`), then read `window.__chk` in one short `javascript_tool` call. `javascript_tool` times out at 45 s; never wait inside it.
6. **Check with the tab hidden as well as visible.** Chrome throttles a hidden tab's timers to about once a second (once a minute after ~5 minutes hidden), which is exactly when the hop queue falls behind. The tools cannot bring Chrome's window to the front: while it is behind other windows the tab is hidden, which gives the hidden case. For the visible case, ask the user to bring Chrome to the front before you start the run. The probe's `hiddenMs` says how long the tab was hidden; a case counts when it held for most of the run. Done when both cases have been read, or the report names the case that was not checked.

## The probe

Paste into `javascript_tool` on a freshly loaded page. It uses `MutationObserver`s and its own `EventSource` on `/events`, which keep firing in a hidden tab, unlike `setInterval`. Besides the queue and the run's end, it records, for each scenario step, how many hops the diagram had played when the step showed finished.

```js
const c = window.__chk = { maxQueued: 0, hiddenMs: 0, hops: 0, shown: 0, lastHop: {}, endedAt: {}, shownAtFinish: {}, stepLag: {} };
let hidAt = document.hidden ? performance.now() : null;
document.addEventListener('visibilitychange', () => {
  if (document.hidden) hidAt = performance.now();
  else if (hidAt != null) { c.hiddenMs += performance.now() - hidAt; hidAt = null; }
});
c.hiddenNow = () => c.hiddenMs + (hidAt != null ? performance.now() - hidAt : 0);
const $ = id => document.getElementById(id);
new MutationObserver(() => { c.maxQueued = Math.max(c.maxQueued, +($('queued').textContent.match(/\d+/) || [0])[0]); })
  .observe($('queued'), { childList: true, characterData: true, subtree: true });
// Every hop played adds a hop line to the narration card, or is one of the repeats a line stands for ("…and N more").
new MutationObserver(recs => { for (const r of recs) for (const n of r.addedNodes) c.shown += +n.dataset.hops || 1; })
  .observe($('now'), { childList: true });
new MutationObserver(recs => {
  for (const { target: el } of recs) {
    const id = el.id.slice('step-'.length);
    if (!el.matches('.st.success, .st.error') || id in c.shownAtFinish) continue;
    c.shownAtFinish[id] = c.shown;
    if (c.endedAt[id] != null) c.stepLag[id] = Math.round(performance.now() - c.endedAt[id]);
  }
}).observe($('steps'), { attributes: true, attributeFilter: ['class'], subtree: true });
// The page's run is over (its last hop played) when the status chip shows it.
new MutationObserver(() => { if (/done|failed/.test($('statusText').textContent) && !c.drained) { c.drained = performance.now(); c.shownAtEnd = c.shown; } })
  .observe($('statusText'), { childList: true, characterData: true, subtree: true });
new EventSource('/events').onmessage = m => {
  const e = JSON.parse(m.data);
  if (e.kind === 'hop') { c.hops++; if (e.step) c.lastHop[e.step] = c.hops; }
  if (e.kind === 'step_succeeded' || e.kind === 'step_failed') c.endedAt[e.step] = performance.now();
  if (e.kind === 'done' || e.kind === 'failed') c.done = performance.now();
};
// Elements in the hop panel or a story card that hold text and are rendered, yet drawn 0 px tall
// (a scrolling flex child shrunk to nothing), each the outermost such; their text is in the DOM either way.
c.collapsed = () => [...document.querySelectorAll('#hopPanel *, .card *')]
  .filter(el => el.checkVisibility() && el.innerText?.trim() && el.getBoundingClientRect().height === 0 && el.parentElement.getBoundingClientRect().height > 0)
  .map(el => `${el.tagName.toLowerCase()}.${el.className}: ${el.innerText.trim().slice(0, 40)}`);
$('runBtn').click(); 'started'
```

Read it back with:

```js
const c = window.__chk;
({ hiddenMs: Math.round(c.hiddenNow()), hidden: document.hidden, hops: c.hops, shown: c.shown, maxQueued: c.maxQueued,
   lag: c.drained - c.done, shownAtEnd: c.shownAtEnd,
   early: Object.keys(c.shownAtFinish).filter(s => c.shownAtFinish[s] < (c.lastHop[s] ?? 0)),
   maxStepLag: Math.max(0, ...Object.values(c.stepLag)), status: document.getElementById('statusText').textContent, collapsed: c.collapsed() })
```

- `lag` is how long the page kept playing hops after the engine's run ended (about one hold when it had kept up). `drained` is unset while it is still playing.
- `early` lists steps that showed finished before their last hop played; it should be empty.
- `shownAtEnd` should equal `hops`.
- `collapsed` should be empty. It reads the page as it is now, so a check of a card or panel calls `c.collapsed()` while that card shows (each replayed step's hop detail too, with the Proof and Merkle tree details fold open: a closed fold renders nothing, so nothing in it counts): a card's text being in the DOM does not mean it is drawn. #172's decryption card passed every test and review at 0 px.
- The engine log in the terminal shows why a run stopped early.

Extend the probe for what a ticket checks (a panel's content, say), keeping to observers and the event stream.

## Layout checks

The Side story layout (#150): a step strip on top, the diagram beside a story column (the narration card, public vs private, the hop detail fold), every other panel in one fold below, and the terminal drawer collapsed. Check it with `resize_window` at 1280×828 and again at 1600×1000, in both themes (the ◐ button), on a freshly loaded page and during a run. `resize_window` can fail to change the page's size when the screen is smaller (seen 2026-10-29: `innerWidth` stayed 1280 at 1600 and at 900), so read `innerWidth` and `innerHeight` after it, and leave a size it didn't reach to the user.

1. **Above the drawer at 1280×828.** On load and at each step: the step strip, the whole diagram and the whole narration card show without scrolling, and none is under the drawer. Read it with
   ```js
   const dr = document.getElementById('drawer').getBoundingClientRect().top, b = s => document.querySelector(s).getBoundingClientRect().bottom;
   ({ drawerTop: dr, strip: b('#stepper'), diagram: b('section.diagram'), card: b('.narr'), sides: b('#narrSides'), scrollY })
   ```
   Every bottom but `sides` is at most `drawerTop`, with `scrollY` 0. On a fan-out step, `sides` may run under the drawer; the criteria do not cover it, so note it.
2. **The diagram fills its column.** No empty band under it: its bottom is within a few pixels of `drawerTop - 12`, and its rows spread to fill it, with the nodes and labels at their usual size. Its size stays the same from step to step.
3. **Folds start closed.** On load, the hop detail fold, the fold below and the drawer are all closed: `[...document.querySelectorAll('details.fold')].map(d => d.open)` is all `false`, and the drawer shows only its tab bar. Each opens with one click. Opened mid-run, the ledger, the auditor's view, the run totals and the fan-out and transfers tables are up to date, and they keep updating.
4. **Step strip.** Done steps show ✓, the running one its number and label, and a step's tooltip its message. All 22 markers fit on one line at 1280 wide, or the strip scrolls to keep the running one in view.
5. **Replay.** Clicking a finished step that has hop details (Shield) unfolds the hop detail fold and replays them there; clicking one without (Check prerequisites) does nothing.
6. **Themes.** Both themes look right: the cards, the public vs private boxes, the folds' summaries and the strip's markers take the theme's colours, with nothing left light in dark or dark in light.
7. **Narrow window.** Under 1000px wide the story column drops under the diagram, and hovering a diagram node still puts its tooltip beside the node.
8. **Zones** (#154). At both sizes and in both themes, the diagram shows three zones, labelled WALLETS · PRIVATE, NETWORK and CHAIN · PUBLIC (19px in viewBox units), with the dashed privacy line between the first two and its caption at the line's foot; there is no column header above the diagram. The users and the Auditor are one column in the wallet zone, with the Wallet's prover at its bottom and its line "a separate service in this demo, part of the wallet in production" readable; the gas funder and the Relayer are in the network zone, and every contract is in the chain zone. The zones span the canvas's full height after every refit (resize the window, open and close the drawer). Hops still light their links over the zones, a failed hop is still red (feed one as in narration check 5), node hover cards still show, and the hop detail fold still fills and replays. Read the zones with
   ```js
   const z = [...document.querySelectorAll('#diagram .zone')].map(r => r.getBBox()), vb = document.getElementById('diagram').viewBox.baseVal;
   ({ zones: z.map(b => [b.x, b.width, b.height]), viewBoxH: vb.height, labels: [...document.querySelectorAll('#diagram .zl')].map(t => t.textContent) })
   ```
   Each zone's height is the viewBox's height less 4.

## Narration checks

The step narration card at the head of the story column (#149). Install the probe extended with a recorder of what the card shows for each step (paste it before the probe's last line, which starts the run), and read `window.__chk.narr` after the run:

```js
// One entry per narration shown, in order; the card rewrites its kicker on each.
c.narr = [];
new MutationObserver(() => c.narr.push({ kicker: $('narrKicker').textContent, head: $('narrHead').textContent,
  body: $('narrBody').textContent, sides: $('narrSides').innerText, hops: [] }))
  .observe($('narrKicker'), { childList: true, characterData: true, subtree: true });
new MutationObserver(recs => { for (const r of recs) for (const n of r.addedNodes) c.narr.at(-1)?.hops.push((n.className ? '[' + n.className + '] ' : '') + n.textContent); })
  .observe($('now'), { childList: true });
```

1. **Default run** (fees and tags on). Every step's kicker, headline and body read right for it; every scenario step but Check prerequisites and Totals has both lists; each hop adds one plain line (no `from → to: label` fallback). The audit step says the auditor names every payer.
2. **Fees-off run.** `JSON.stringify(c.narr)` has no `fee` or `earned`. The hop labels are a subset of the narration's (`payment`, not `paymentWithUsdrFee`).
3. **Tags-off run.** No Tag channels step; the discover step tries every note; the audit step says the auditor can't name the payer, and nothing claims it can.
4. **Manual transfer**, after a run: the card shows "After the run · Your payment", the from, to and amount sent, and its hop lines.
5. **Failed hop and replay.** A failed hop's line is red with its error, the newest too, and a failed self-hop has no arrow. No run fails on demand, so feed the page one with `onEvent({ kind: 'hop', step: 'manual_9', from: 'actor2', to: 'relayer', label: 'relay', detail: 'connection refused', failed: true })` after a run and read `$('now').lastElementChild`. Clicking a finished step with hop details (Shield) still replays them in the hop detail panel.

## Open wallet checks

The acting user's wallet card on the diagram (#156). Install the probe with pausing on, extended with this recorder (paste it before the probe's last line). It folds the run's actor events as they arrive and keeps a copy of the fold at each step's end. At each gate, playback has just finished a step while the engine has usually run ahead, so the card must match that step's fold and not a later one:

```js
// The engine's actor fields, folded as they arrive, and a copy at each step's end.
c.fold = {}; c.foldAt = {}; c.cards = [];
new EventSource('/events').onmessage = m => {
  const e = JSON.parse(m.data);
  if (e.kind === 'actor') (c.fold[e.actor] ||= {})[e.field] = e.value;
  if (e.kind === 'step_succeeded') c.foldAt[e.step] = JSON.parse(JSON.stringify(c.fold));
};
c.card = () => { const w = $('wallet'); return w.hidden ? null : { head: w.querySelector('.wh b').textContent,
  keys: [...w.querySelectorAll('.wkey code')].map(k => k.textContent),
  notes: [...w.querySelectorAll('.wnote')].map(n => n.textContent), pub: w.querySelector('.wpub b').textContent }; };
// At each gate: the step last finished, the card, and that step's fold for the user it names.
new MutationObserver(() => {
  if ($('nextBtn').disabled) return;
  const step = [...document.querySelectorAll('#steps .st.success')].at(-1)?.id.slice('step-'.length), card = c.card();
  const i = card && ACTORS.findIndex(([n]) => card.head.startsWith(n)), f = c.foldAt[step]?.[i] || {};
  const amt = v => v ? v.split(' ')[0] : null, key = v => v ? v.slice(0, 6) + '…' + v.slice(-4) : '—';
  const want = { keys: [key(f.pkSpend), key(f.pkView)], notes: [amt(f.balance) && amt(f.balance) !== '0' && amt(f.balance) + 'RaylsERC20', amt(f.usdr) && amt(f.usdr) !== '0' && amt(f.usdr) + 'USDr'].filter(Boolean), pub: amt(f.public) || '—' };
  c.cards.push({ step, card, ok: !!card && JSON.stringify([card.keys, card.notes, card.pub]) === JSON.stringify([want.keys, want.notes, want.pub]) });
}).observe($('nextBtn'), { attributes: true, attributeFilter: ['disabled'] });
```

1. **Follows and names the acting user.** On a default run, release each gate and read `c.cards.map(x => x.step + ': ' + x.card?.head)`. From Make keys on, every gate shows a card naming the user at either end of the step's last user hop (the Sender through the fan-out, then each payer), never "—’s wallet" and never the gas funder's or the auditor's. Watched during USDr credit, the card opens on the issuer for its grants.
2. **Matches the fold, never ahead.** Every entry of `c.cards` that has a card has `ok: true`: its keys, notes and public balance are the fold at the end of the step played, even when `+N queued` shows the engine ahead. Spot-check against the ledger fold below the stage after the run: the last card's notes and public balance equal that user's RaylsERC20, USDr and Public cells.
3. **A note shows when its hop plays.** During a fan-out step, watch the card through the step's hops with pausing on: the payer's RaylsERC20 amount changes only after the step's last hop has played, not when the step starts.
4. **Layout.** At 1280×828 in both themes, and during a run: the card sits right of the acting user's row inside the wallet zone (its right edge left of the privacy line) and its bottom above the Wallet's prover; nothing in it is cut off or overflows. Read it with
   ```js
   const r = s => document.querySelector(s).getBoundingClientRect(), w = r('#wallet'), el = $('wallet');
   ({ cardRight: w.right, line: r('#diagram .pline').left, cardBottom: w.bottom, prover: r('#diagram .node.service rect').top, overflow: [el.scrollWidth - el.clientWidth, el.scrollHeight - el.clientHeight] })
   ```
   `cardRight` is less than `line`, `cardBottom` less than `prover`, and both overflows 0.
5. **Reset.** Starting a new run closes the card (`$('wallet').hidden` is `true` straight after RUN), and the next card opened shows only that run's values, with no notes until the run's own hops play.

## Data chip checks

The chip each hop carries on the diagram (#155), from its label in the narration block's `chips`. Chips ride above their packet in their own drawing (`#chips`), over the open wallet. Count the chips the diagram draws against the hops it plays (paste before the probe's last line, or on its own with pausing off and `/?fast`):

```js
c.chipTally = { labels: {}, noChip: {}, chips: 0, packets: 0 };
new MutationObserver(rs => rs.forEach(r => r.addedNodes.forEach(n => n.classList?.contains('chip') && c.chipTally.chips++))).observe($('chips'), { childList: true });
new MutationObserver(rs => rs.forEach(r => r.addedNodes.forEach(n => n.classList?.contains('packet') && c.chipTally.packets++))).observe($('diagram'), { childList: true, subtree: true });
new EventSource('/events').onmessage = m => { const e = JSON.parse(m.data);
  if (e.kind === 'hop') { c.chipTally.labels[e.label] = (c.chipTally.labels[e.label] || 0) + 1; if (!NARRATION.chips[e.label]) c.chipTally.noChip[e.label] = 1; } };
```

To look at one chip without a run, close the page's stream (`events.close()`) and feed a hop: `onEvent({ kind: 'reset' })`, `onEvent({ kind: 'step_started', step: 'setup', label: 'Setup' })`, then `onEvent({ kind: 'hop', step: 'setup', from, to, label, detail: 'x' })`. To freeze it for a screenshot, stub `setTimeout` for delays of 300 ms and more just before the hop, and `pauseAnimations()` both `#diagram` and `#chips` about 450 ms after it.

1. **Every hop has a chip.** On a default, a fees-off and a tags-off run, `c.chipTally.noChip` is empty and, once the queue drains, `chips` equals `packets`. Between them the three runs see every label in `chips` but `settle` (a failed settlement only); `payment` only on the fees-off run.
2. **Private stays in the wallet.** The private chips (🔒, the user's colour) are `spend key`, `view key` and `audit ciphertexts`, which loop on the user's own row, and the `prove …` hops, from the user to the Wallet's prover: none crosses the privacy line. Every chip on a link that crosses it, or lies right of it, is public (the contracts' colour).
3. **Readable.** At 1280×828 in both themes, chip text is 19px in viewBox units, readable without zooming: a self-loop chip (`spend key`) over the open wallet, a chip on a link that bends right of the contracts (`notes in`, `fee note in`) inside the drawing's right edge, and a top-row chip (the Issuer's) inside its top.
4. **Failed hop.** A failed hop (feed one with `failed: true`, as in narration check 5, e.g. `relayer` → `EnygmaDvp`, `settle`) shows its chip with a red outline, riding its red link.

## Concept moment checks

The five concept moments heading the story column (#157, #158). Install the probe with the pausing recorder below, extended with this one (paste it before the probe's last line). It records each moment shown, its beats lit over time, and the Shield hop's hop detail to compare with:

```js
// One entry per moment shown: its kicker, when each beat lit (ms after it showed), and its text when hidden.
c.moments = [];
new MutationObserver(() => {
  const m = $('moment');
  if (m.hidden) { const x = c.moments.at(-1); if (x && x.hiddenAt == null) { x.hiddenAt = performance.now(); x.text = x.last; } return; }
  // Proof and Relayer show back to back, so a new kicker starts a new entry too.
  let x = c.moments.at(-1); const kicker = m.querySelector('.kicker')?.textContent;
  if (x && x.hiddenAt == null && x.kicker !== kicker) { x.hiddenAt = performance.now(); x.text = x.last; }
  if (!x || x.hiddenAt != null) c.moments.push(x = { at: performance.now(), kicker, lit: [] });
  x.last = m.innerText;
  while (x.lit.length < m.querySelectorAll('.beat.on').length) x.lit.push(Math.round(performance.now() - x.at));
}).observe($('moment'), { attributes: true, childList: true, subtree: true, characterData: true });
// The Shield hop's commitment and leaf, as the hop detail panel shows them.
new EventSource('/events').onmessage = m => {
  const e = JSON.parse(m.data), d = e.kind === 'hop' && e.step === 'shield' && e.hopDetail?.find(d => d.kind === 'commitment');
  if (d) c.shieldLeaf = { commitment: shortHex(d.commitment), leaf: d.leaves[0].index, tree: d.tree };
};
// Hops the diagram played while a moment showed.
new MutationObserver(recs => { if (!$('moment').hidden) for (const r of recs) c.hopsInMoment = (c.hopsInMoment || 0) + r.addedNodes.length; })
  .observe($('now'), { childList: true });
```

Read the moment itself with `$('moment').innerText` while it holds.

1. **Each holds before its step, pausing on.** On a default run, release each gate as it comes. The moments show in this order, each with its step's narration under it and none of that step's hops played (`c.hopsInMoment` stays unset):
   - "Idea 1 of 5 · Hidden on chain" after USDr credit's gate, before Shield;
   - "Idea 2 of 5 · Proof", then "Idea 3 of 5 · Relayer", after Shield's gate, before Pay Ana (Fan-out 1);
   - "Idea 4 of 5 · Finding your notes" before Discover;
   - "Idea 5 of 5 · Auditor" before Audit.

   Each one's three beats light in turn, about 1.8 s apart (`lit` about `[0, 1800, 3600]`). The run ends after 26 Nexts: `c.gates.length` 26, five of them with a moment showing.
2. **Every way out.** On three runs (or by re-checking the toggle between runs), release the moment with its own NEXT (`$('momentNext').click()`), the card's NEXT and `c.space()`: each releases it (Proof's release shows Relayer; the others' hide the card and their step's hops play). One default run can cover all three by rotating them over its five moments.
3. **Wording.** Read each moment's text with all its beats lit (`c.moments[i].text`):
   - Proof names Ana and Pay Ana's amount (the payouts' first) and the Sender's shield total.
   - On a default run, Proof says a second proof pays the relayer's USDr fee, Relayer's "paid" row shows the fixed fee once ("1 USDr, from a private USDr note", not "USDr USDr") and says it posts Ana's tag, Finding your notes says Ana's tag points her to it, and Auditor's "payers" row names every payment's payer through the tag channels.
   - On a fees-off run nothing in any moment mentions a fee: Relayer's "paid" row says the relayer is free. `TestNoConceptMomentMentionsAFeeOnAFeesOffRun` guards this too.
   - On a tags-off run nothing claims a payer is named (`payment()` publishes only output 0's ciphertext): no tag in Relayer or Finding your notes, Bruno tries the note and it doesn't open, and Auditor's "payers" row says payers stay hidden even from the auditor, with no balances rebuilt.

4. **The run's values.** While Hidden on chain holds, its text shows the run's shield total, `c.shieldLeaf.commitment` and leaf `#c.shieldLeaf.leaf` in tree `c.shieldLeaf.tree`, the same as the hop detail fold's Shield entry (commitment row and `tree N #L`). A `…` shows only until the Shield hop arrives.
5. **Rerun.** After the run, the "concept moments" toggle is off. A second RUN shows no moment (`c.moments` empty, 21 gates). Check the toggle and RUN again: the moments hold again. A reload checks it again.
6. **Pausing off.** With pausing off, each moment still shows, its hint saying it plays on by itself, and hides about 8 s later (`hiddenAt - at` about 8000) with no Next; the run plays straight through. Unchecking pausing while the moment holds changes its hint and releases it about 8 s later; unchecking it at an earlier step's gate, with the moment already queued, still shows the moment for about 8 s. Checking pausing again while it waits to release itself makes it hold until Next.
7. **?fast** stops at the moments too; starting a new run while it holds hides it (`$('moment').hidden` straight after RUN).
8. **Layout.** At 1280×828 in both themes, during each moment (Proof is the tallest): it heads the story column, the public vs private lists are hidden, its NEXT is above the drawer, and nothing in it is cut off.

## Pausing checks

Presenter pacing (#151): with "pause after each step" on, playback holds at a gate after each step but the last until Next or Space. Extend the probe with a recorder of the gates (paste it before the probe's last line), and release gates from `javascript_tool` with `document.getElementById('nextBtn').click()` or a dispatched Space:

```js
// One entry per gate: when playback reached it and was released, the step last shown finished, and the hops queued behind it.
c.gates = [];
new MutationObserver(() => {
  if ($('nextBtn').disabled) { const g = c.gates.at(-1); if (g && g.releasedAt == null) g.releasedAt = performance.now(); return; }
  c.gates.push({ at: performance.now(), step: [...document.querySelectorAll('#steps .st.success')].at(-1)?.id, queued: $('queued').textContent });
}).observe($('nextBtn'), { attributes: true, attributeFilter: ['disabled'] });
c.space = () => document.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }));
```

1. **Default run, pausing on.** Release each gate as it comes (poll for `!$('nextBtn').disabled`). The run ends after 21 Nexts, with `c.gates.length` 21, none at the last step; the probe's `early` is empty and `shownAtEnd` equals `hops`.
2. **The run keeps going.** At a gate, the engine's events keep arriving (`c.hops` grows, `+N queued` grows) and its steps finish in the terminal; Next plays the next step's hops at once.
3. **Long pause.** Hold one gate two minutes, then Next. The hops after it play at the full hold for pausing on (1.5 s, 3 s with a hop detail; see the pace checks), not at the 120 ms minimum.
4. **Space.** `c.space()` releases a gate. With focus in the send form's amount box (`$('sendAmount').focus()`, then a keydown dispatched on it), Space does not.
5. **Pausing off.** Unchecking the toggle while playback waits releases it, and the rest of the run plays straight through at the pausing-off pace; a reload keeps it off (localStorage), and a run with it off has no step gates (a concept moment still holds about 8 s).
6. **?fast** stops at gates too.

A run on a stack `run.sh` has only just started can lose its first steps' events (the stepper stays empty and the card starts mid-run), seen once on 2026-10-29. Most likely the demo is still tailing the setup's log backlog, and the broker drops events for a page that falls behind. Run again once the logs are quiet.

## Pace checks

The hop pace (#159): with pausing on, every hop holds 1.5 s, or 3 s with a hop detail, with no catch-up, and, in the steps the narration block lists in `collapse` (Audit keys, Tag channels, USDr credit), queued hops repeating a hop, or a pair of hops, in a row within its step (`open channel`, `openChannel`, `open channel`, …) play together after the first pair at the 120 ms minimum, under the first hop's line, which ends "…and N more" (N more of that hop). With pausing off, the pace is as before: 600 ms (1.5 s with a hop detail), shrinking once playback is more than 3 s behind. Timing checks need the tab visible: ask the user to bring Chrome to the front first. Install the probe with the pausing recorder, extended with this one (paste it before the probe's last line). It times each hop line until the next, outside the gates:

```js
// Whether each hop, in arrival order, has a hop detail other than http and receipt (those don't lengthen its hold); the diagram plays them in that order.
c.hasDetail = [];
new EventSource('/events').onmessage = m => { const e = JSON.parse(m.data); if (e.kind === 'hop') c.hasDetail.push(!!e.hopDetail?.some(d => d.kind !== 'http' && d.kind !== 'receipt')); };
// One entry per hop line: its text, how many hops it stands for, whether its first has a hop detail, whether the tab was hidden, and how long until the next line (unset across a gate).
c.lines = []; c.played = 0;
new MutationObserver(recs => { for (const r of recs) for (const n of r.addedNodes) {
  const now = performance.now(), x = c.lines.at(-1), hops = +n.dataset.hops || 1;
  if (x && x.ms == null && !x.gated) x.ms = Math.round(now - x.at);
  c.lines.push({ at: now, text: n.textContent, hops, detail: c.hasDetail[c.played], hidden: document.hidden });
  c.played += hops;
} }).observe($('now'), { childList: true });
// A gate between two lines leaves the first untimed.
new MutationObserver(() => { if (!$('nextBtn').disabled && c.lines.at(-1)) c.lines.at(-1).gated = true; })
  .observe($('nextBtn'), { attributes: true, attributeFilter: ['disabled'] });
// Timed lines that held less than their hold (1500, or 3000 with a hop detail) less 100.
c.want = x => x.detail ? 3000 : 1500;
c.offPace = () => c.lines.filter(x => x.ms != null && x.ms < c.want(x) - 100);
// How many timed lines held their hold, give or take 100 ms (220 over, for the 120 ms of repeats), of how many timed.
c.onPace = () => [c.lines.filter(x => x.ms != null && x.ms - c.want(x) >= -100 && x.ms - c.want(x) <= 220).length, c.lines.filter(x => x.ms != null).length];
```

The 120 ms the repeats take falls on the line before them: a collapsed line's own, or, for a pair, the pair's second line. `detail` lines up with the hops only while `shown` keeps pace with `hops` (no hop dropped for an unknown node). A line can last longer than its hold when the queue ran dry behind it (the engine had not sent the next hop yet), so only short lines count against the pace; with gates released a few seconds after they hold, `c.onPace()` should be nearly all of them.

1. **Pausing on, in a step.** On a default run, release each gate a few seconds after it holds. `c.offPace()` is empty and `c.onPace()` is nearly all lines: every timed line held 1.5 s, or 3 s when its hop has a hop detail. Lines timed with the tab hidden (each line's `hidden`) run long, to whole seconds.
2. **After a two-minute pause.** Hold one gate (a fan-out's) two minutes, so the engine's backlog is a whole step or more, then Next. The step's lines still hold 1.5 s / 3 s each, with no catch-up: `c.offPace()` stays empty.
3. **After a long backlog.** Hold USDr credit's gate (or an earlier one) until `+N queued` shows 100 or more, then release it and the rest. The lines after it hold 1.5 s / 3 s each, from the first to the last: `c.offPace()` stays empty.
4. **"…and N more".** On a default run, Tag channels shows two lines: the first `open channel` ending "…and N more", with `hops` 2N + 1 (it and the N later pairs; N is 11 when the run opens 12 channels), and the first `openChannel`. Audit keys and USDr credit collapse their runs of single hops too. Make keys, Register and Discover play every hop, one line each, so the open wallet steps through each person in turn. Collapsing never spans a gate or another step, and a failed hop is never collapsed. The probe's `shownAtEnd` equals `hops` and `early` is empty.
5. **Pausing off.** With pausing off, lines hold about 600 ms (1.5 s with a hop detail) and none ends "…and N more"; a burst more than 3 s behind plays faster, down to 120 ms. `?fast` still holds 120 ms.

## Walkthrough checks

The walkthrough (#170): the header offers WALKTHROUGH or RUN. A walkthrough plays the Sender's path with no USDr fee and no tags, and the engine waits before each action until Next or Space releases it (`POST /release`). Keygen, Register and Discover release one person per Next, the first payment one hop per Next (proof request, proof back, relay, settle, since #173), and Auditor keys, Shield, each later payment and Audit one Next each. `TestShieldDemoWalkthrough` checks the engine side (one action per release, nothing while the gate is closed); these check the page. Extend the probe with the pausing recorder, and start with `$('walkBtn').click()` instead of `$('runBtn').click()`.

Measured 2026-11-03, updated for #173 and #175: a walkthrough is **35 engine releases** (auditor 1, keys 8, register 7, auditor keys 1, shield 1, payment 1's proof request, proof back, relay and settle 4, payments 2–6 5, Replay payment 1 1, scans 6, audit 1), plus **5 concept moments** when they play: **40 Nexts** on a page's first walkthrough. Each Payment proof took about a second on the gnark server.

1. **Walkthrough or Run.** On a fresh page both buttons show; RUN plays the full run exactly as before (pausing checks 1–2 still hold, 21 Nexts). WALKTHROUGH's stepper lists Generate keys, Register, Auditor keys, Shield, Pay Ana … Pay Felipe, Discover, Audit (12 steps), and the kicker starts "Walkthrough".
2. **The engine waits.** At each gate the card shows an "On Next" paragraph naming the action and the person, Next pulses, the terminal shows `⏸ waiting for Next`, and no new hop arrives (`c.hops` stays put). After Next the paragraph reads "Now, for real" and that action's hops play: one person's spend key, view key and gas; one registration; …
3. **One per click.** Payment 1 takes four Nexts: `prove Payment`, then `proof` (gnark server → Sender), then `relay`, then `payment` with its notes in; payments 2–6 one each, and Pay Bruno one more for Replay payment 1 (see below). The count of gates (`c.gates.length`) matches the measured count above.
4. **Moments still gate.** The Shield, Proof, Relayer, Finding your notes and Auditor moments each hold before their step's first action; their Next releases only the moment, not the engine (the terminal shows no action released).
5. **Pausing off.** Unchecking "pause after each step" during a walkthrough keeps every action gate (the engine still waits); only the moments play on by themselves.
6. **Audit.** The Audit step's text says the auditor infers balances by conservation, and the auditor's view (in the fold) shows a balance for every user, each matching.
7. **Handover.** When the walkthrough's last hop has played, RUN THE FULL DEMO shows beside Next. It starts a normal run with the default options (USDr fee and tags on, the script) and new actors, without restarting `run.sh`.
8. **Leaving a walkthrough.** At any gate, RUN or WALKTHROUGH starts a new run: the old walkthrough's step fails ("context canceled") and the new one resets the page. A reload mid-walkthrough leaves the engine waiting; starting again from the reloaded page ends it.
9. **A failure.** Stop the relayer during a walkthrough, then Next on Pay Ana's relay: the relay hop shows red with its error, the walkthrough ends, and WALKTHROUGH or RUN starts again once the relayer is back.
10. **Both themes at 1280×828.** The "On Next" paragraph and the RUN THE FULL DEMO button show above the drawer, in both themes.

Check 9 run 2026-11-04: with the relayer stopped at Pay Ana’s relay gate, Next showed the relay hop red with `connection refused`, step 5 ✗ and the walkthrough failed. Restarted with run.sh’s command and env, a new walkthrough ran to the end. Stopping only the relayer leaves run.sh and the other services up.

## Wire and receipt checks

Since #171, each proof request and relayer call hop carries an http hop detail (the request the engine sent and the response it got back, bodies as sent), and each hop that mines a transaction carries a receipt hop detail (its tx hash, block, gas and logged events). They show in the hop detail panel under **Request and response** or **Receipt**, one click below the hop's sentence. An http or receipt detail alone doesn't lengthen a hop's hold: a hop whose only detail is one of them (fund gas, relay, open channel, tag, openChannel, publishTag) holds as a hop without details. Register holds longer since #172: it carries a registration detail too.

1. **Proof request.** On a default run, replay Pay Ana (click its step): its `prove Payment` entry's **Request and response** shows `POST http://localhost:8082/proof/payment → 200`, the request body with its `st…` and `wt…` fields, and the response body with the proof shortened (each element `12345678…123456`). **proof shortened · show the body in full** opens the response exactly as sent. The `prove UsdrFee` entry shows `/proof/usdrFee` the same way. Real values' private rows say "only in the request to the gnark server".
2. **Relayer call.** The `relay` entry shows `POST http://localhost:8090/relay/payment_usdr_fee → 200` (`/relay/payment` on a fees-off run), the request with its shortened `proof` and `usdrProof`, and the response's `txHash`. Tag channels' `open channel` and a transfer's `tag` show `/relay/channel` and `/relay/tag`, with no proof to shorten (no "show in full").
3. **Receipt.** The `paymentWithUsdrFee` entry's **Receipt** shows the tx hash (the same as the relay response's `txHash`), its block and gas, and each logged event named by contract (`Erc20CoinVault.Commitment(…)`, `EnygmaDvp…`, …), none "unknown event". The tx hash copies on click; `cast receipt <hash> --rpc-url http://localhost:8545` (or `eth_getTransactionReceipt`) shows the same block, gas and as many logs. Fund gas, Register, Shield, USDr credit's grants, `openChannel` and `publishTag` carry a receipt too. Fund gas is a plain ETH transfer: its receipt reads "0 logged events" and its sentence "Its receipt logged no events.", with no exception in the console (the engine omits an empty events list).
4. **Walkthrough.** In a walkthrough, payment 1's prove click and relay click each show their request and response, and its settle click's hop its receipt.
5. **Pace unchanged.** The pace checks still pass with the probe above (its `hasDetail` ignores http and receipt details).
6. **Both themes at 1280×828.** The request line, the bodies (scrolling inside their box, never widening the panel) and the event lines read in both themes.

Check 4 run 2026-11-04: in a walkthrough, payment 1’s prove and relay hops each carried their `POST … → 200` http detail, and its settle hop its receipt.

## Witness and public columns

Since #173, payment 1's proof request hop in a walkthrough carries a witness hop detail: the private inputs the Sender's wallet sent its prover (the spend key, the spent note's amount, salt and Merkle path, the token id, and the two new notes' amounts, salts and owners' spend public keys). A card under the narration shows it beside the hop's publicInputs detail. `TestShieldDemoWalkthrough` checks the witness against the request body sent to the gnark server, and that the new notes' openings hash to the public `CommitmentsOut`, the commitments the vault logged.

1. **The columns.** In a walkthrough, when Pay Ana's `prove Payment` hop plays, a card "Payment 1: what stays with the Sender, what goes on chain" shows two columns side by side. **Stays with the Sender** has its narration, then the spend private key, the Sender's spent note's amount and salt, its Merkle path (leaf # and 8 siblings), Ana's new note and the Sender's change (each amount, salt and spend public key) and the token id: every value tagged "never leaves the Sender’s wallet", each label naming its circuit witness (`PrivateKeysIn`, `ValuesIn`, …). **Goes on chain** has its narration, then each public input labelled with its Payment statement field (`Message`, `TreeNumbers`, `MerkleRoots`, `Nullifiers`, `CommitmentsOut[0]`, `CommitmentsOut[1]`, `ContractAddress`) and tagged "Payment statement · <field>".
2. **Real values.** The new notes' amounts are Ana's payout and the Sender's shield less it (defaults: 120 and 880; the spent note is the 1000 shielded); hex values show the full value on hover and copy on click. `CommitmentsOut[0]` and `[1]` match the two `new leaf` values of Pay Ana's `notes in` entry in the hop detail panel, and the `Erc20CoinVault.Commitment` events of the `payment` hop's Receipt.
3. **How long it stays.** The card stays through payment 1's proof back, relay and settle clicks, and goes when Pay Bruno's hop plays. A new run or walkthrough takes it away; a full RUN never shows it.
4. **Both themes at 1280×828.** The two columns sit side by side in the story column, the siblings wrap inside their column, nothing widens the column or scrolls sideways, and the warn (stays) and accent (chain) tags read in both themes.

Run 2026-11-04 at 1280×707, all four passed: outputs 120 (Ana) and 880 (change) from the 1000 spent, `CommitmentsOut` equal to `notes in`’s two leaves, the card through proof, relay and settle and gone at Pay Bruno, no sideways scroll.

## On-chain proof check

Since #174, once payment 1 of a walkthrough settles, the engine sends its `Verifier.verifyProof(vkIndex, proof, inputs)` call (the `verifyProof` protocol field: slot, proof, public inputs and the ABI-encoded data, and the same with `CommitmentsOut[0]` plus one). `/defaults` names the node's RPC URL, the deployment's Verifier and the Payment verifying key's slot (0, Erc20CoinVault's `VK_ID_ERC20_JOINSPLIT`). `TestShieldDemoWalkthrough` sends the same `eth_call` and asserts true, then false with the tampered input.

**The call goes direct.** The page sends the `eth_call` straight to the Hardhat node (`http://127.0.0.1:8545`), not through the demo server: the node answers the page's cross-origin call (checked 2026-11-04 from `localhost:9092`), so there is no pass-through endpoint. The card says so under its buttons.

1. **True.** In a walkthrough, once Pay Ana's settle hops have played, a card under the narration shows "Check payment 1’s proof on chain", its narration, `verifyProof → true`, the proof (a, b, c), the public inputs, the JSON-RPC request body (`eth_call` to the Verifier, `"latest"`) and a curl command. The browser's network panel shows the POST going to `127.0.0.1:8545`, not `:9092`.
2. **Tamper.** TAMPER changes the heading, adds the Tamper narration, marks `CommitmentsOut[0]` red as `old → new`, and shows `verifyProof → false`. CHECK THE PROOF goes back to true.
3. **curl.** COPY copies the command; run in a terminal, it prints `"result":"0x…01"` for the genuine call and `"0x…00"` after Tamper (copy again after pressing TAMPER).
4. **Stays until the end.** The card and both buttons stay through payments 2–6, Discover and Audit, and after the walkthrough's last hop; a new run or walkthrough takes it away. A full RUN never shows it.
5. **Both themes at 1280×828**, and under 1000px wide: the body and curl boxes scroll inside the card, never widening the story column.

Run 2026-11-04, all five passed: the eth_call went to `127.0.0.1:8545`; Tamper changed exactly one calldata digit; COPY then the terminal gave `…01` genuine, `…00` tampered; the card stayed to the end. Run the copied command from the clipboard (`pbpaste`): the browser tool can’t return the long calldata.

## Replay payment 1

Since #175, once payment 2 of a walkthrough has settled, the engine waits at one more gate in Pay Bruno: Replay payment 1. Released, it resubmits payment 1's exact `/relay/payment` request through the relayer. The relayer refuses it (`400`, "nullifier already spent": payment 1 published that nullifier), and the `replay payment 1` hop (Sender → Relayer) carries a refusal hop detail: who refused it, the reason, the nullifier, payment 1's transaction, and both vaults' trees and the vault's, the relayer's and every actor's RaylsERC20 balance, read before and after the replay. The engine fails the walkthrough if the replay is accepted or changes any of them. `TestShieldDemoWalkthrough` checks the request is payment 1's byte for byte, the refusal, the unchanged state, and that no block was mined between payments 2 and 3.

1. **Offered after payment 2, not before.** Through Pay Ana and Pay Bruno's payment click, Next reads NEXT ▸. Once Pay Bruno's `notes in` has played, Next reads **REPLAY PAYMENT 1 ▸** and the card's "On Next" paragraph explains the replay, naming Ana. After the click it reads NEXT ▸ again.
2. **The refusal.** The `replay payment 1` hop line reads "refused by the relayer: nullifier already spent". A card "Payment 1, sent again: refused" shows the refusal narration, ✗ refused by **Relayer**: nullifier already spent, and ✓ the nullifier (the same value as payment 1's `Nullifiers` public input in the columns card) spent by payment 1's tx.
3. **Nothing changed.** The card's second paragraph says nothing changed, then a ✓ line for each vault tree (`tree N: L leaves, root …, unchanged`) and each balance (Erc20CoinVault, Relayer, Issuer, Sender, Ana … Felipe), none with an arrow. The tree's leaf count matches the hop detail panel's after Pay Bruno's `notes in`.
4. **Raw artifacts.** The hop detail panel's entry for the replay shows its sentence, the refusal in Real values, and **Request and response**: `POST http://localhost:8090/relay/payment → 400` (the status in red), the request body identical to Pay Ana's relay request (compare the two), and the response `{"error":"nullifier already spent"}`.
5. **How long it stays.** The card stays until Pay Carla's hop plays; a new run or walkthrough takes it away, and a full RUN never shows it or the REPLAY label.
6. **Both themes at 1280×828.** The card's lines wrap inside the story column, and the ✗ (bad) and ✓ (ok) marks read in both themes.

Run 2026-11-04, all six passed: REPLAY PAYMENT 1 ▸ only after Pay Bruno’s `notes in`, refused by the relayer (`400`, `{"error":"nullifier already spent"}`) with payment 1’s nullifier and its relay request byte for byte, 78 leaves unchanged (Pay Bruno’s last leaf #77), the card gone at Pay Carla. The replay hop can take more than 10 s after Next to arrive.

## Key, registration and decryption values

Since #172, the key-material and decryption hops carry their real values as hop details, in a full run and in a walkthrough: each `spend key` and `view key` hop a key detail (its scheme, the public half in full, the private half shortened by the engine to `0x123456…abcd`; the walkthrough's `auditor key` hop too), each `register` hop a registration detail (pk_spend, pk_view and the two audit ciphertexts, as `UserRegistry.register` got them), each `recover view key` hop a viewKeyRecovery detail (the audit ciphertexts as the registry holds them and the view key they open to), each participant's discovery hop (`open note` with tags, `scan` without) a decryption detail (the ciphertext, the opening: amount, salt, token, and the commitment it hashes to), and the `audit` hop, now in a full run too, one decryption detail with every note the auditor opened (all but the USDr fee legs, which carry no ciphertext). A recipient's `scan` or `open note` in a pay step (a scripted or manual transfer) carries the same decryption detail as Discover's. Each is a state detail: it holds its hop as long as any other, so with pausing on Make keys, Register, Discover and Audit hold 3 s per hop (the pace checks' `hasDetail` counts them already), and Auditor keys' collapsed `recover view key` hops flash by: read each by replaying the step. `TestShieldDemoScenario` and `TestShieldDemoWalkthrough` check every value against the actors' keys, the UserRegistry, the chain's note and tag events, and each participant's payout.

Run 2026-11-04 in Chrome at 1280×707, all seven passed after one fix: the decryption card (`.opened`, a scrolling flex child) collapsed to zero height in the panel until it got `flex-shrink:0`. Check its height, not just its text: the text is in the DOM either way. A walkthrough with the concept moments off was 35 Nexts.

1. **Keys.** On a default run, replay Make keys (click its step): each person's `spend key` entry shows its sentence, a card `private sk_spend 0x……` → `public pk_spend 0x…`, and under Real values the public half (copy on click) and the private half shortened, in the warn colour, its tooltip saying the full key never reaches the page. `view key` shows `pk_view` as 1184 B in the card and in full (scrolling inside its box) under Real values. No private key appears in full anywhere: search the page's `/events` stream (network panel) for a `"private"` longer than 15 characters: none.
2. **Registration.** Replay Register: each `register` entry's card lists pk_spend, pk_view (1184 B), the audit capsule (1088 B) and the sealed view key (92 B: nonce, seed, tag); Real values shows them in full, and pk_view is the same value as that person's `view key` entry. Its Receipt is still one click below.
3. **Auditor keys.** Replay Auditor keys: each `recover view key` entry shows the two ciphertexts (the same values as that person's `register` entry) → auditor’s key → the person's view key, shortened, "= the registered pk_view ✓"; Real values' recovered pk_view is the person's pk_view.
4. **Discover.** Replay Discover: each participant's `open note` entry shows tag payload → channel key → amount, salt, token → Poseidon → commitment, the amount that participant's payout (defaults: Ana 120, Bruno 150, …, as in their Pay step). On a tags-off run (or a walkthrough's Discover), the `scan` entry shows the capsule and sealed amount → that person's view key → the opening. The commitment matches one `new leaf` of their Pay step's `notes in` entry.
5. **Audit.** The Audit step's `audit` entry, in a full run and a walkthrough, lists one line per note the auditor opened (the shield, each USDr grant, each payment), scrolling inside the card, with each note's owner, kind, amount and transaction in Real values.
6. **Walkthrough.** In a walkthrough, each Next on Generate keys shows that person's spend and view key entries (the first click the auditor's key), each Register click that person's registration, and each Discover click that participant's opening; Back to live works after replaying any of them.
7. **Both themes at 1280×828.** The cards wrap inside the panel, the long values scroll inside their boxes and never widen the panel, and the warn (private) and accent (public) tags read in both themes.

## Formula cards

Since #198, a formula card is laid out as a privacy argument (#191): the property the formula gives as its title, the equation in symbols, a "chain sees" line with the public tag, a "stays private" line with the private tag, and "why it holds" in plain words. A walkthrough's adds one line, "✓ checked against the chain with this run’s values" (✗ when not equal), naming the chain's value, and a closed "this run's values" fold with every input, the result and the chain's value in full. A full run's card has the argument only: the engine sends its formula detail by name, with no values, once per run on the formula's first hop. In a walkthrough, the "Proof and Merkle tree details" fold opens by itself when a hop with a formula card plays; a full run never opens it. `TestShieldDemoScenario` checks a full run's shield carries the one value-less commitment detail; `TestShieldDemoWalkthrough` recomputes the commitment with the client library's Poseidon and compares it with the card and the logged leaf; the replay test checks each card's argument, that the values fold is walkthrough-only and closed with every value digit for digit, and the fold opening by itself, live and on replay. The commitment is the first card laid out this way; the Merkle path and nullifier cards keep their title, equation and line until #192, their values in the same fold.

1. **The commitment card, walkthrough.** Start a walkthrough with the hop detail fold closed. When Shield's `shield` hop plays, the fold opens by itself and the panel shows, under the opening → commitment line, a card titled **Hiding** · the note commitment. Then, top to bottom: `commitment = Poseidon(pk_spend, salt, amount, token)`; `CHAIN SEES` (accent tag) the commitment, one leaf in the vault's tree; `STAYS PRIVATE` (warn tag) the owner's pk_spend, the salt, the amount, the token; "why it holds": Poseidon is one-way, and the random salt means even a guessed amount can't be checked; the line that the shield's opening is public on chain and privacy starts at the first spend; "✓ checked against the chain with this run’s values: equals leaf #N of tree T, the vault's Commitment event"; and a closed "this run’s values" fold.
2. **The values fold.** Open "this run’s values": pk_spend, salt, amount (the form's shield total, default 1000) and token (`0x000…0`) each in full, `= commitment` in full, then `on chain` (the leaf's name on hover) with the same value. The result equals the `commitment · public` value under Real values, whose opening rows say "in the formula below: public in this deposit’s calldata".
3. **Closing the fold.** Close "Proof and Merkle tree details": it stays closed through the hops that follow until the next hop with a formula card (Pay Ana's `proof`), which opens it again.
4. **The commitment card, full run.** In a full run with the fold closed, the shield plays without opening it. Open it while the shield's hop shows (or replay Shield): the same card, property to the public-opening line, with no ✓ line and no values fold, and the opening rows under Real values still read "not sent to this page".
5. **Replay.** Click the Shield step after later steps have played: the fold opens and the card comes back as it played, its values fold closed.
6. **Card height.** With the card showing, `c.collapsed()` (the probe's) is empty, with the values fold closed and open: the card has its full height, not 0 px.
7. **Both themes at 1280×828.** Checks 1 and 3 (the fold opening by itself, and closing) hold in both themes. The tags read in the accent and warn colours, the "why it holds" text and the 66-digit values in the open fold wrap inside the card, never widening the panel or scrolling sideways, the ✓ reads in the ok colour, in both themes.
8. **Presenter pace.** With pausing on, the shield hop holds as a state detail hop (3 s), long enough to point at the property and the ✓; the card stays until the next hop with a hop detail replaces it.

Since #187, payment 1 of a walkthrough carries two more formula details. The proof coming back (gnark server → Sender) carries the Merkle path: the spent leaf hashed with one sibling per level, `Poseidon(left, right)` ordered by the leaf index's bit, up the tree's 8 levels, each level's node shown, checked against the vault's `isValidRoot`. The settlement carries the nullifier: `Poseidon(sk_spend, tree · 2⁸ + leaf index)`, checked against the `Nullifier` event in the settlement's receipt. It goes on the settle hop rather than the proof hop because only the settlement has the receipt. `TestShieldDemoWalkthrough` recomputes every level and the nullifier with the client library's Poseidon, from the shield's logged leaf and the request sent to the prover, and checks the root against `isValidRoot` and the nullifier against the receipt's event.

9. **The Merkle path card.** In a walkthrough, when Pay Ana's `proof` hop plays (the click after `prove Payment`), the hop detail panel shows "The Merkle path · real values". The equation comes first, then its line, the ✓ line "checked against the chain with this run’s values: equals the root Sender's proof publishes, one the vault has had in tree T (isValidRoot → true)", and the values fold. Open it: `leaf #N · node₀` (equal to the shield card's commitment), then `sibling₀ · right` or `· left` (the side bit 0 of N says), `node₁`, and so on up to `sibling₇`. Then `= root` in full, then `on chain` with the same value. That value equals the `MerkleRoots` public input on `prove Payment`. The sibling values match the Merkle path rows on the `prove Payment` hop, one click earlier.
10. **The nullifier card.** When Pay Ana's settle hop (`payment`) plays, under its settle checks, the panel shows "The nullifier · real values": `nullifier = Poseidon(sk_spend, tree · 2⁸ + leaf index)`, its line (the inputs are private, so no one but the Sender can link the nullifier to the note), the ✓ line naming the vault's Nullifier event in the settlement, tx 0x…, and the values fold. In it come sk_spend (equal to the spend key in payment 1's "stays with the Sender" witness column), tree, leaf index (the Merkle path card's N), `tree · 2⁸ + leaf index`, `= nullifier` in full, then `on chain` with the same value, equal to the settle checks' published nullifier.
11. **Replay and height.** Click Pay Ana after later steps and step through its hop details: both cards come back with the same values. With each card showing, `c.collapsed()` is empty. The Merkle path card's 17 rows scroll inside the panel, and the panel never widens or scrolls sideways, in both themes.

Before #198, #186's commitment card (then checks 1–5, values shown inline) was run 2026-11-04 in Chrome at 683×713 (`resize_window` to 1280×828 did not take): checks 1–4 passed after one fix: the `= commitment` label wrapped onto two lines in a 76 px column, now 96 px. The card showed leaf #131 of tree 0 with pk_spend equal to the Sender's wallet's (`0x1405…8685`), 242 px tall, `collapsed` empty, no sideways scroll, in both themes; it came back on replaying Shield and was still live into Pay Ana's first click. Check 5 was not timed: the shield hop was already a state detail hop through its commitment detail, and a walkthrough holds it until Next anyway.

Checks 1–8 (#198) were run 2026-11-05 in Chrome, window 1280×828 (viewport 713 px tall). Checks 1–7 passed; check 8 was not timed (the full run used `?fast`, and a walkthrough holds the shield at its gate until Next anyway). In a walkthrough with the fold closed, the shield's hop opened it by itself. The card (325 px) read top to bottom: Hiding · the note commitment, the equation, CHAIN SEES, STAYS PRIVATE, why it holds, the public-opening line, "✓ checked against the chain with this run’s values: equals leaf #198 of tree 0, the vault's Commitment event", and a closed values fold. Opened (525 px), the fold held pk_spend, salt, amount 1000, token 0 and `= commitment` in full, and the result equalled `commitment · public` under Real values, whose opening rows said "in the formula below". One fix: the chain value's row label, the leaf's full name, wrapped onto five lines in the 96 px column, so it now reads `on chain` with the name on hover (the ✓ line names it). After the fold was closed, it stayed closed until Pay Ana's `proof` hop opened it for the Merkle path card. Replaying Shield opened the fold and showed the card with its values fold closed. In a full run the shield played with the fold left closed; replayed, its card had the argument with no ✓ line and no values fold, and the opening rows read "not sent to this page". `collapsed` was empty every time (values fold closed and open), with no sideways scroll, in both themes.

Checks 9–11 (then 6–8, #187, values shown inline before #198 folded them) were run 2026-11-04 in Chrome at 1280×707. They passed after one fix: on the nullifier card, the ✓ line's transaction hash didn't wrap and was cut off at the panel edge, so the card now sets `overflow-wrap:anywhere`. Payment 1 spent leaf #22 of tree 1 (0b10110). The card had sibling₀ on the right, siblings 1, 2 and 4 on the left and sibling₃ on the right, as the bits say. Its leaf equalled the shield card's commitment and its siblings equalled `prove Payment`'s Merkle path rows. The root's ✓ was true and the card was 762 px, scrolling inside the panel. The nullifier card (tree · 2⁸ + leaf index = 278) was 357 px, and its result equalled the settle checks' published nullifier. Its sk_spend equalled the witness column's. Both cards came back on replaying Pay Ana. `collapsed` was empty on every hop, with no sideways scroll, in both themes. The two-line labels (`sibling₀ · right`, `tree · 2⁸ + leaf index`) wrap inside their 96 px column and stay readable.
