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

## Narration checks

The step narration card under the diagram (#149). Install the probe extended with a recorder of what the card shows for each step (paste it before the probe's last line, which starts the run), and read `window.__chk.narr` after the run:

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

A run on a stack `run.sh` has only just started can lose its first steps' events (the stepper stays empty and the card starts mid-run), seen once on 2026-10-29. Most likely the demo is still tailing the setup's log backlog, and the broker drops events for a page that falls behind. Run again once the logs are quiet.
