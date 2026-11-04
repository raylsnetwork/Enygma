// The shield demo page, played and replayed in jsdom on a real run's events.
//
// The fixture is the events of TestShieldDemoScenario's default run
// (enygma_retail_payments/test/13_shield_demo_scenario_test.go writes it on
// every live run). The test opens index.html as /?fast with pausing off,
// feeds each event to the page as /events delivers it, plays the run out on
// a virtual clock, then replays every finished step's hop details, opening
// the hop detail panel's folds on each. It fails on any exception the page
// throws; on a step not shown as succeeded at the end; on a step's replay
// stepping through a different number of hop details than the run sent; on
// a field element in an HTTP body that the panel does not show digit for
// digit; and on a body with a proof not shown in full behind "show in full".
// A first test checks the fixture still holds the cases this is there for.
//
//   node --test scripts/shield_demo_replay.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { JSDOM, VirtualConsole } = require('jsdom');

const DEMO = path.join(__dirname, '..', 'enygma_retail_payments', 'shield_demo');
const PAGE = fs.readFileSync(path.join(DEMO, 'index.html'), 'utf8');
const FIXTURE = JSON.parse(fs.readFileSync(path.join(DEMO, 'testdata', 'default_run.events.json'), 'utf8'));

// tick lets the page's pending promises and jsdom's queued tasks run.
const tick = () => new Promise(r => setImmediate(r));

// clock replaces a window's timers and performance.now with a virtual
// clock that only moves when run says so. A timer callback's exception is
// recorded in errors, not thrown.
function clock(win, errors) {
  let now = 0, seq = 0;
  const timers = new Map();
  win.setTimeout = (fn, ms = 0, ...args) => {
    timers.set(++seq, { at: now + Math.max(0, Number(ms) || 0), seq, fn, args });
    return seq;
  };
  win.clearTimeout = id => { timers.delete(id); };
  Object.defineProperty(win.performance, 'now', { value: () => now });
  return {
    // run fires every timer due within ms of now, in order, the time
    // jumping to each; it fails past limit timers (a timer loop).
    run(ms, limit = 1e6) {
      const end = now + ms;
      for (let n = 0; ; n++) {
        let next = null;
        for (const t of timers.values()) if (t.at <= end && (!next || t.at < next.at || (t.at === next.at && t.seq < next.seq))) next = t;
        if (!next) break;
        if (n > limit) throw new Error(`more than ${limit} timers: a timer loop?`);
        timers.delete(next.seq);
        now = next.at;
        try { next.fn(...next.args); } catch (err) { errors.push(err); }
      }
      now = end;
    },
    pending: () => timers.size,
  };
}

// open loads the page as /?fast with pausing off. /defaults answers with
// the fixture's options; any other request is an error, since a full run
// sends none.
async function open() {
  const errors = [];
  const vc = new VirtualConsole();
  vc.on('jsdomError', err => errors.push(err));
  vc.on('error', (...a) => errors.push(new Error('console.error: ' + a.join(' '))));
  let source = null, timers = null;
  const dom = new JSDOM(PAGE, {
    url: 'http://localhost:9092/?fast',
    runScripts: 'dangerously',
    pretendToBeVisual: true,
    virtualConsole: vc,
    beforeParse(win) {
      timers = clock(win, errors);
      win.localStorage.setItem('enygmaRetailShieldPause', 'off');
      // What a browser has and jsdom lacks; layout is all zeros here.
      win.ResizeObserver = class { observe() {} unobserve() {} disconnect() {} };
      win.SVGElement.prototype.getBBox = () => ({ x: 0, y: 0, width: 0, height: 0 });
      win.SVGElement.prototype.beginElement = () => {};
      win.structuredClone ||= v => win.JSON.parse(win.JSON.stringify(v));
      win.EventSource = class { constructor(url) { assert.strictEqual(url, '/events'); source = this; } close() {} };
      win.fetch = async url => {
        if (url !== '/defaults') throw new Error(`unexpected request to ${url}`);
        const body = JSON.stringify(FIXTURE.defaults);
        return { ok: true, status: 200, json: async () => win.JSON.parse(body), text: async () => body };
      };
    },
  });
  const onRejection = err => errors.push(err);
  process.on('unhandledRejection', onRejection);
  test.after(() => { process.off('unhandledRejection', onRejection); dom.window.close(); });
  await tick();
  assert.ok(source, 'the page opened no EventSource');
  source.onopen?.();
  return { win: dom.window, doc: dom.window.document, source, timers, errors };
}

// failOn fails with every exception the page has thrown so far.
function failOn(errors, when) {
  if (errors.length) assert.fail(`${errors.length} exception(s) ${when}:\n` + errors.map(e => e.stack || e).join('\n\n'));
}

// stripProofs drops a body's proof arrays, the one value the panel shortens:
// the gnark server's "proof" and the relayer requests' "usdrProof" (the
// page's proofKey), each a flat array of numbers.
const stripProofs = raw => raw.replace(/"(usdr)?proof"\s*:\s*\[[^\]]*\]/gi, '');
// fieldElements are a body's runs of 16 or more digits outside its proofs:
// field elements, bare or quoted, too long for a JS number.
const fieldElements = raw => stripProofs(raw).match(/\d{16,}/g) || [];
const hasProof = raw => stripProofs(raw) !== raw;

// The hops with a hop detail, per step, in the order the panel keeps them.
const detailHops = {};
for (const e of FIXTURE.events) if (e.kind === 'hop' && e.hopDetail && e.step) (detailHops[e.step] ||= []).push(e);

test('the fixture is a real default run, with the cases the replay is there for', () => {
  const kinds = FIXTURE.events.map(e => e.kind);
  assert.strictEqual(kinds[0], 'reset');
  assert.strictEqual(kinds.at(-1), 'done', 'the run did not finish');
  const details = FIXTURE.events.flatMap(e => e.hopDetail || []);
  assert.ok(details.some(d => d.kind === 'receipt' && !d.events), 'no receipt that logged no events (a gas funding)');
  const http = details.filter(d => d.kind === 'http');
  assert.ok(http.some(d => hasProof(d.response || '')), 'no proof in an HTTP response');
  assert.ok(http.some(d => /[:[,]\s*\d{16,}\s*[,\]}]/.test(stripProofs(d.request + d.response))),
    'no bare field element outside a proof: the digit-for-digit check would check nothing');
});

test('the page plays the whole run and replays every step’s hop details without an exception, showing every field element digit for digit', async () => {
  const { doc, source, timers, errors } = await open();
  const $ = id => doc.getElementById(id);

  FIXTURE.events.forEach((e, i) => {
    try { source.onmessage({ data: JSON.stringify(e) }); } catch (err) { err.message = `event ${i} (${e.kind}): ${err.message}`; errors.push(err); }
    timers.run(5);
  });
  // Moments with pausing off release themselves after 8 s; give the queue all the time it needs.
  for (let i = 0; i < 1000 && timers.pending(); i++) { timers.run(1000); await tick(); }
  failOn(errors, 'while playing the run');
  assert.strictEqual($('statusText').textContent, 'done', 'the run did not play to its end');
  const all = [...doc.querySelectorAll('#steps .st')];
  assert.ok(all.length > 0, 'no steps');
  const unfinished = all.filter(el => !el.classList.contains('success')).map(el => el.id);
  assert.deepStrictEqual(unfinished, [], 'steps not shown as succeeded');

  let replayed = 0, checkedElements = 0, fullProofs = 0;
  for (const el of all) {
    const step = el.id.slice('step-'.length), hops = detailHops[step] || [];
    // Back to live first: a step with no hop details leaves the panel as it was.
    doc.querySelector('#hopPanel [data-replay="live"]')?.click();
    el.click();
    await tick();
    if (!hops.length) { assert.ok(!doc.querySelector('#hopPanel .replay'), `${step} has no hop details but replays`); continue; }
    for (let i = 0; i < hops.length; i++) {
      const bar = doc.querySelector('#hopPanel .replay');
      assert.ok(bar, `${step}: not replaying`);
      assert.match(bar.textContent, new RegExp(`hop detail ${i + 1} of ${hops.length}\\b`), `${step}: the replay bar`);
      for (const id of ['hopValues', 'hopWire']) if ($(id) && !$(id).open) { $(id).open = true; await tick(); }
      timers.run(0);
      failOn(errors, `opening the folds of ${step}'s hop detail ${i + 1}`);

      // Every field element of each body shows as it was sent in the shortened body;
      // a body with a proof shows in full behind "show in full".
      // Each body is checked in its own <pre>: a request often repeats its response's values.
      const http = hops[i].hopDetail.filter(d => d.kind === 'http');
      const wires = [...doc.querySelectorAll('#hopWire .wire')].filter(w => w.querySelector('.lbl')?.textContent === 'request body');
      assert.strictEqual(wires.length, http.length, `${step} hop detail ${i + 1}: HTTP exchanges shown`);
      http.forEach((d, j) => {
        const shown = [...wires[j].querySelectorAll(':scope > pre')].map(p => p.textContent);
        const full = [...wires[j].querySelectorAll(':scope > details pre')].map(p => p.textContent);
        assert.strictEqual(shown.length, 2, `${step} hop detail ${i + 1}, ${d.url}: a request and a response body`);
        [['request', d.request || ''], ['response', d.response || '']].forEach(([part, raw], k) => {
          for (const n of fieldElements(raw)) {
            assert.ok(shown[k].includes(n), `${step} hop detail ${i + 1}, ${d.url} ${part}: ${n} is not shown digit for digit`);
            checkedElements++;
          }
          if (hasProof(raw)) { assert.ok(full.includes(raw), `${step} hop detail ${i + 1}, ${d.url} ${part}: the body in full`); fullProofs++; }
        });
      });
      replayed++;
      const next = doc.querySelector('#hopPanel [data-replay="next"]');
      if (i < hops.length - 1) { assert.ok(!next.disabled, `${step}: › disabled at ${i + 1} of ${hops.length}`); next.click(); await tick(); }
    }
    failOn(errors, `replaying ${step}`);
  }
  assert.strictEqual(replayed, Object.values(detailHops).flat().length, 'hop details replayed');
  assert.ok(checkedElements > 0 && fullProofs > 0, `checked ${checkedElements} field elements and ${fullProofs} full bodies`);
  failOn(errors, 'at the end');
});
