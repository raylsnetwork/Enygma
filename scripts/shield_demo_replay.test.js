// The shield demo page, played and replayed in jsdom on real runs' events.
//
// The fixtures are the events of TestShieldDemoScenario's default run and of
// TestShieldDemoWalkthrough's walkthrough (enygma_retail_payments/test/13_…
// and 14_…, which write them on every passing live run). Each test opens
// index.html as /?fast with pausing off, feeds each event to the page as
// /events delivers it, plays the run out on a virtual clock, then replays
// every finished step's hop details, opening the hop detail panel's folds on
// each. The walkthrough is one this page started: its Walkthrough button
// sends the fixture's run options, and the test presses Next at every gate,
// feeding the events that followed each release only once it has; /release
// and the on-chain proof check's eth_call are answered from the fixture's
// run. A test fails on any exception the page throws; on a step not shown as
// succeeded at the end; on a step's replay stepping through a different
// number of hop details than the run sent; on a field element in an HTTP
// body that the panel does not show digit for digit; on a body with a proof
// not shown in full behind "show in full"; and, for the walkthrough, on Next
// pressed a different number of times than the run had gates, or a
// walkthrough card (payment 1's columns, the replay's refusal, the on-chain
// proof check, a formula card) that never showed while it played, a formula
// card's value not shown digit for digit, and on the check not
// showing verifyProof → true, then false once tampered. The cards show only
// as their hops play; replaying a step shows the same hop details (the
// witness, the refusal) in the hop detail panel. Two first tests check the
// fixtures still hold the cases this is there for.
//
//   node --test scripts/shield_demo_replay.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { JSDOM, VirtualConsole } = require('jsdom');

const DEMO = path.join(__dirname, '..', 'enygma_retail_payments', 'shield_demo');
const PAGE = fs.readFileSync(path.join(DEMO, 'index.html'), 'utf8');
// fixture reads testdata/name: { defaults, events } (the walkthrough's adds run and ethCall).
const fixture = name => JSON.parse(fs.readFileSync(path.join(DEMO, 'testdata', name), 'utf8'));
const DEFAULT_RUN = fixture('default_run.events.json');
const WALKTHROUGH = fixture('walkthrough.events.json');

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

// open loads the page as /?fast with pausing off, on fixture f. /defaults
// answers with the fixture's; with walk, /run, /release and the on-chain
// check's eth_call to the fixture's node answer as the fixture's run did,
// each recorded in requests. Any other request is an error: a full run the
// page did not start sends none.
async function open(f, { walk = false } = {}) {
  const errors = [];
  const vc = new VirtualConsole();
  vc.on('jsdomError', err => errors.push(err));
  vc.on('error', (...a) => errors.push(new Error('console.error: ' + a.join(' '))));
  let source = null, timers = null;
  const requests = { run: [], release: 0, ethCall: [] };
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
      // answer is a 200 response with JSON value v, parsed in the page's realm.
      const answer = v => { const body = JSON.stringify(v); return { ok: true, status: 200, json: async () => win.JSON.parse(body), text: async () => body }; };
      win.fetch = async (url, init = {}) => {
        if (url === '/defaults') return answer(f.defaults);
        if (walk && url === '/run') { requests.run.push(JSON.parse(init.body)); return answer({ status: 'started' }); }
        if (walk && url === '/release') { requests.release++; return answer({ status: 'released' }); }
        if (walk && url === f.defaults.chain?.rpcUrl) {
          const { method, params: [{ to, data }] } = JSON.parse(init.body);
          requests.ethCall.push(data);
          if (method !== 'eth_call' || to !== f.defaults.chain.verifier) throw new Error(`unexpected ${method} to ${to}`);
          if (!(data in f.ethCall)) throw new Error(`an eth_call the run never made: ${data.slice(0, 74)}…`);
          return answer({ jsonrpc: '2.0', id: 1, result: f.ethCall[data] });
        }
        throw new Error(`unexpected request to ${url}`);
      };
    },
  });
  const onRejection = err => errors.push(err);
  process.on('unhandledRejection', onRejection);
  test.after(() => { process.off('unhandledRejection', onRejection); dom.window.close(); });
  await tick();
  assert.ok(source, 'the page opened no EventSource');
  source.onopen?.();
  return { win: dom.window, doc: dom.window.document, source, timers, errors, requests };
}

// feed sends events to the page as /events delivers them, each a few
// virtual ms after the last; an exception names its event's index in the
// fixture, the first being at.
function feed({ source, timers, errors }, events, at = 0) {
  events.forEach((e, i) => {
    try { source.onmessage({ data: JSON.stringify(e) }); } catch (err) { err.message = `event ${at + i} (${e.kind}): ${err.message}`; errors.push(err); }
    timers.run(5);
  });
}

// playOut plays what the page has queued until no timer is left: the run's
// end, or a gate nothing releases by itself. Moments with pausing off
// release themselves after 8 s; it gives the queue all the time it needs.
async function playOut({ timers }) {
  await tick();
  for (let i = 0; i < 1000 && timers.pending(); i++) { timers.run(1000); await tick(); }
}

// finished fails unless the page shows its run done with every step
// succeeded, and returns the steps.
function finished({ doc }) {
  assert.strictEqual(doc.getElementById('statusText').textContent, 'done', 'the run did not play to its end');
  const all = [...doc.querySelectorAll('#steps .st')];
  assert.ok(all.length > 0, 'no steps');
  const unfinished = all.filter(el => !el.classList.contains('success')).map(el => el.id);
  assert.deepStrictEqual(unfinished, [], 'steps not shown as succeeded');
  return all;
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

// checkFormulas fails unless the hop detail panel shows one formula card
// per formula detail of hop h, each with every input, its result and the
// chain's value digit for digit, its ✓ (or ✗), and, for the commitment, the
// public-opening line; it returns how many values it checked.
function checkFormulas(doc, h, where) {
  const fs = (h.hopDetail || []).filter(d => d.kind === 'formula');
  const shown = [...doc.querySelectorAll('#hopPanel .formula')];
  assert.strictEqual(shown.length, fs.length, `${where}: formula cards shown`);
  let n = 0;
  fs.forEach(({ formula: f }, i) => {
    const text = shown[i].textContent, values = [...shown[i].querySelectorAll('.fv')].map(v => v.textContent);
    // In card order: each input, the result, then the chain's value.
    const want = [...f.inputs.map(x => x.value), f.result, f.chain.value];
    assert.deepStrictEqual(values, want, `${where}, ${f.name} formula: its values, digit for digit, in order`);
    n += want.length;
    for (const x of f.inputs) assert.ok(text.includes(x.name), `${where}, ${f.name} formula: input ${x.name} unnamed`);
    assert.ok(text.includes(f.ok ? '✓' : '✗'), `${where}, ${f.name} formula: no ${f.ok ? '✓' : '✗'}`);
    if (f.name === 'commitment') assert.match(text, /opening is public on chain[^.]*\. Privacy starts at the first spend/, `${where}: the public-opening line`);
    if (f.name === 'nullifier') assert.match(text, /inputs are private[^.]*no one but the Sender can link this nullifier to the note/, `${where}: the unlinkable line`);
  });
  return n;
}

// detailHopsOf is fixture f's hops with a hop detail, per step, in the order the panel keeps them.
function detailHopsOf(f) {
  const out = {};
  for (const e of f.events) if (e.kind === 'hop' && e.hopDetail && e.step) (out[e.step] ||= []).push(e);
  return out;
}

// replayAll clicks each finished step of run f and steps through all its hop
// details with the hop detail panel's folds open, failing on an exception,
// on a hop detail count other than the run's, and on a field element of an
// HTTP body not shown digit for digit or a body with a proof not in full.
async function replayAll(page, f, all) {
  const { doc, timers, errors } = page;
  const $ = id => doc.getElementById(id);
  const detailHops = detailHopsOf(f);
  let replayed = 0, checkedElements = 0, fullProofs = 0, formulaValues = 0;
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
      formulaValues += checkFormulas(doc, hops[i], `${step} hop detail ${i + 1}`);
      replayed++;
      const next = doc.querySelector('#hopPanel [data-replay="next"]');
      if (i < hops.length - 1) { assert.ok(!next.disabled, `${step}: › disabled at ${i + 1} of ${hops.length}`); next.click(); await tick(); }
    }
    failOn(errors, `replaying ${step}`);
  }
  assert.strictEqual(replayed, Object.values(detailHops).flat().length, 'hop details replayed');
  assert.ok(checkedElements > 0 && fullProofs > 0, `checked ${checkedElements} field elements and ${fullProofs} full bodies`);
  return { formulaValues };
}

test('the default run fixture is a real default run, with the cases the replay is there for', () => {
  const kinds = DEFAULT_RUN.events.map(e => e.kind);
  assert.strictEqual(kinds[0], 'reset');
  assert.strictEqual(kinds.at(-1), 'done', 'the run did not finish');
  assert.ok(!DEFAULT_RUN.defaults.walkthrough && !kinds.includes('action'), 'a walkthrough, not a default run');
  const details = DEFAULT_RUN.events.flatMap(e => e.hopDetail || []);
  assert.ok(details.some(d => d.kind === 'receipt' && !d.events), 'no receipt that logged no events (a gas funding)');
  const http = details.filter(d => d.kind === 'http');
  assert.ok(http.some(d => hasProof(d.response || '')), 'no proof in an HTTP response');
  assert.ok(http.some(d => /[:[,]\s*\d{16,}\s*[,\]}]/.test(stripProofs(d.request + d.response))),
    'no bare field element outside a proof: the digit-for-digit check would check nothing');
});

test('the page plays the whole default run and replays every step’s hop details without an exception, showing every field element digit for digit', async () => {
  const page = await open(DEFAULT_RUN);
  feed(page, DEFAULT_RUN.events);
  await playOut(page);
  failOn(page.errors, 'while playing the run');
  const all = finished(page);
  await replayAll(page, DEFAULT_RUN, all);
  failOn(page.errors, 'at the end');
});

// The walkthrough's gates: one action event per release.
const GATES = WALKTHROUGH.events.filter(e => e.kind === 'action').length;
// verifyProofCall is the walkthrough's on-chain proof check call, from its verifyProof field.
const verifyProofCall = () => JSON.parse(WALKTHROUGH.events.find(e => e.kind === 'proto' && e.field === 'verifyProof').value);

test('the walkthrough fixture is a real walkthrough, with the cards the replay is there for', () => {
  const kinds = WALKTHROUGH.events.map(e => e.kind);
  assert.strictEqual(kinds[0], 'reset');
  assert.strictEqual(kinds.at(-1), 'done', 'the walkthrough did not finish');
  assert.ok(WALKTHROUGH.run.walkthrough && GATES > 0, 'not a walkthrough');
  const details = WALKTHROUGH.events.flatMap(e => e.hopDetail || []);
  for (const kind of ['witness', 'publicInputs', 'refusal']) assert.ok(details.some(d => d.kind === kind), `no ${kind} hop detail`);
  assert.ok(details.some(d => d.kind === 'formula' && d.formula.name === 'commitment'), 'no commitment formula on the shield');
  for (const name of ['merklePath', 'nullifier'])
    assert.ok(WALKTHROUGH.events.some(e => e.step === 'fanout_1' && (e.hopDetail || []).some(d => d.kind === 'formula' && d.formula.name === name)), `no ${name} formula on payment 1`);
  const chain = WALKTHROUGH.defaults.chain;
  assert.ok(chain?.rpcUrl && chain.verifier, '/defaults names no Verifier: the on-chain check would not call it');
  const call = verifyProofCall(), one = '0x' + '0'.repeat(63) + '1', zero = '0x' + '0'.repeat(64);
  assert.deepStrictEqual([WALKTHROUGH.ethCall[call.data], WALKTHROUGH.ethCall[call.tamper.data]], [one, zero],
    'the node’s answers to the genuine and the tampered verifyProof calls');
});

test('the page plays a walkthrough it started, Next pressed at every gate, and replays every step’s hop details without an exception', async () => {
  const page = await open(WALKTHROUGH, { walk: true });
  const { doc, errors, requests } = page;
  const $ = id => doc.getElementById(id);

  // The Walkthrough button sends the fixture's run options, as the form holds /defaults.
  await tick();
  $('walkBtn').click();
  await tick();
  assert.strictEqual(requests.run.length, 1, 'the Walkthrough button sent no /run');
  for (const [k, v] of Object.entries(requests.run[0])) assert.deepStrictEqual(v, WALKTHROUGH.run[k], `/run ${k}`);

  // Each release's events reach the page only once Next has released it.
  const cards = { columns: false, refusal: false, verify: false };
  let formulaLive = false;
  const evs = WALKTHROUGH.events;
  let nexts = 0;
  for (let at = 0; at < evs.length;) {
    const gate = evs.findIndex((e, i) => i >= at && e.kind === 'action');
    const end = gate < 0 ? evs.length : gate + 1;
    feed(page, evs.slice(at, end), at);
    at = end;
    await playOut(page);
    failOn(errors, `playing up to gate ${nexts + 1}`);
    for (const id of Object.keys(cards)) if (!$(id).hidden && $(id).innerHTML) cards[id] = true;
    // A formula card shows in the hop detail panel as its hop plays.
    const latest = evs.slice(0, at).findLast(e => e.kind === 'hop' && e.hopDetail);
    if (latest?.hopDetail.some(d => d.kind === 'formula')) { checkFormulas(doc, latest, `gate ${nexts + 1}, live`); formulaLive = true; }
    if (gate < 0) break;
    const { action, step } = evs[gate];
    assert.ok(!$('nextBtn').disabled, `gate ${nexts + 1} (${step}/${action}): Next is off`);
    $('nextBtn').click();
    nexts++;
    await tick();
    assert.strictEqual(requests.release, nexts, `gate ${nexts} (${step}/${action}): /release requests`);
  }
  failOn(errors, 'while playing the walkthrough');
  assert.strictEqual(nexts, GATES, 'Next pressed once per gate');
  assert.ok($('nextBtn').disabled, 'Next still on after the last gate');
  for (const [id, seen] of Object.entries(cards)) assert.ok(seen, `the ${id} card never showed`);
  assert.ok(formulaLive, 'no formula card showed while its hop played');
  const all = finished(page);

  // The on-chain proof check asked the fixture's node, and shows its answers.
  const call = verifyProofCall(), verdict = () => $('verify').querySelector('.result')?.textContent;
  assert.deepStrictEqual(requests.ethCall, [call.data], 'the check’s eth_call');
  assert.strictEqual(verdict(), 'verifyProof → true');
  $('verify').querySelector('[data-v="tamper"]').click();
  await tick(); await tick();
  assert.strictEqual(verdict(), 'verifyProof → false', 'the tampered call');
  failOn(errors, 'checking the proof on chain');

  const { formulaValues } = await replayAll(page, WALKTHROUGH, all);
  assert.ok(formulaValues > 0, 'no formula card value checked');
  failOn(errors, 'at the end');
});
