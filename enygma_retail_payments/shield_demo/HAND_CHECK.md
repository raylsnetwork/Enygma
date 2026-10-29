# Hand-checking a shield demo page change

The demo page has no automated UI tests, so a page ticket's acceptance is checked by hand in Chrome against the `run.sh` stack. A scenario run takes 3–5 minutes, so each wasted run is expensive.

## Steps

1. **Check last.** Run the hand check after `/code-review` and its fixes, on the code you will commit. A run started earlier tests code the review may still change. Done when the working tree holds the reviewed change.
2. **Serve the current page.** `run.sh` starts the demo with `-dev`, so the page is read from disk on every request. A stack started before `-dev` existed (check `ps ax | grep '[s]hield_demo -root'` for the flag) needs the demo rebuilt once and relaunched detached with the same `-root`/`-logs` args plus `-dev`; the chain, gnark server and relayer keep running. Done when `curl -s localhost:9092/` shows your edit.
3. **Open the page with the `navigate` tool**, as `http://localhost:9092/?fast` when the check needs the end of a run. Reloading from inside `javascript_tool` (`location.reload()`) kills the evaluation mid-call.
4. **Install the probe, then start the run** (below). It records what the check needs without depending on the page's timers.
5. **Wait in the background** (`sleep` with `run_in_background`), then read `window.__chk` in one short `javascript_tool` call. `javascript_tool` times out at 45 s; never wait inside it.
6. **Check with the tab hidden as well as visible.** Chrome throttles a hidden tab's timers to about once a second (once a minute after ~5 minutes hidden), which is exactly when the hop queue falls behind. The probe's `hiddenMs` says how long the tab was hidden; a check that matters for a backlog needs a run where it is most of the run. Done when both cases have been read.

## The probe

Paste into `javascript_tool` on a freshly loaded page. It observes `#queued` with a `MutationObserver` and the event stream with its own `EventSource`, both of which keep firing in a hidden tab, unlike `setInterval`.

```js
const c = window.__chk = { maxQueued: 0, hiddenMs: 0 };
let hidAt = document.hidden ? performance.now() : null;
document.addEventListener('visibilitychange', () => {
  if (document.hidden) hidAt = performance.now();
  else if (hidAt != null) { c.hiddenMs += performance.now() - hidAt; hidAt = null; }
});
const qEl = document.getElementById('queued');
new MutationObserver(() => {
  const q = qEl.textContent;
  c.maxQueued = Math.max(c.maxQueued, +(q.match(/\d+/) || [0])[0]);
  if (c.done && !q && !c.drained) c.drained = performance.now();
}).observe(qEl, { childList: true, characterData: true, subtree: true });
new EventSource('/events').onmessage = m => {
  const e = JSON.parse(m.data);
  if (e.kind === 'done') { c.done = performance.now(); if (!qEl.textContent) c.drained = c.done; }
};
document.getElementById('runBtn').click(); c
```

Read it back with:

```js
({ ...window.__chk, lag: window.__chk.drained - window.__chk.done, hidden: document.hidden })
```

`lag` is how long the hop queue kept playing after the run's last event (0: it had kept up). `drained` is unset while the queue is still playing. Extend the probe for what a ticket checks (a step's finished state, a panel's content), keeping to observers and the event stream.
