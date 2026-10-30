# Hand-checking a shield demo page change

The demo page has no automated UI tests, so a page ticket's acceptance is checked by hand in Chrome against the `run.sh` stack. A scenario run takes 3–5 minutes, so each wasted run is expensive.

## Steps

1. **Check last.** Run the hand check after `/code-review` and its fixes, on the code you will commit. A run started earlier tests code the review may still change. Done when the working tree holds the reviewed change.
2. **Serve the current page on a live stack.** The demo server can outlive the chain, gnark server and relayer; a run against it then fails at its first step. Check all four:
   ```sh
   for p in 8545 8082 8090 9092; do lsof -iTCP:$p -sTCP:LISTEN -nP >/dev/null && echo "$p up" || echo "$p DOWN"; done
   ```
   Any port down: stop the leftover demo (`pkill -f 'shield_demo -root'`) and start the whole stack afresh with `bash run.sh` (in the background; it prints `demo is up on :9092` when ready). `run.sh` starts the demo with `-dev`, so the page is read from disk on every request. A stack started before `-dev` existed (check `ps ax | grep '[s]hield_demo -root'` for the flag) needs the demo rebuilt once and relaunched detached with the same `-root`/`-logs` args plus `-dev`; the chain, gnark server and relayer keep running. Done when all four ports are up and `curl -s localhost:9092/` shows your edit.
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
// Every hop played adds a hop line to the narration card.
new MutationObserver(recs => { for (const r of recs) if (r.addedNodes.length) c.shown++; })
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
$('runBtn').click(); 'started'
```

Read it back with:

```js
const c = window.__chk;
({ hiddenMs: Math.round(c.hiddenNow()), hidden: document.hidden, hops: c.hops, shown: c.shown, maxQueued: c.maxQueued,
   lag: c.drained - c.done, shownAtEnd: c.shownAtEnd,
   early: Object.keys(c.shownAtFinish).filter(s => c.shownAtFinish[s] < (c.lastHop[s] ?? 0)),
   maxStepLag: Math.max(0, ...Object.values(c.stepLag)), status: document.getElementById('statusText').textContent })
```

- `lag` is how long the page kept playing hops after the engine's run ended (about one hold when it had kept up). `drained` is unset while it is still playing.
- `early` lists steps that showed finished before their last hop played; it should be empty.
- `shownAtEnd` should equal `hops`.
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
3. **Long pause.** Hold one gate two minutes, then Next. The hops after it play at their normal hold (about 600 ms, 1.5 s with a hop detail), not at the 120 ms minimum: time them with the `#now` observer.
4. **Space.** `c.space()` releases a gate. With focus in the send form's amount box (`$('sendAmount').focus()`, then a keydown dispatched on it), Space does not.
5. **Pausing off.** Unchecking the toggle while playback waits releases it, and the rest of the run plays straight through; a reload keeps it off (localStorage), and a run with it off has no gates.
6. **?fast** stops at gates too.

A run on a stack `run.sh` has only just started can lose its first steps' events (the stepper stays empty and the card starts mid-run), seen once on 2026-10-29. Most likely the demo is still tailing the setup's log backlog, and the broker drops events for a page that falls behind. Run again once the logs are quiet.
