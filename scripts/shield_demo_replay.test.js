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
// showing verifyProof → true, then false once tampered. In both, a formula
// card must show its privacy argument, and its values fold only with a
// walkthrough's values; the hop detail fold must open by itself for a
// formula card in the walkthrough, and never in the full run. The cards show only
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

// ARGUMENT is each formula card's privacy argument, as checkFormulas expects
// it: its property (the title), its equation, what the chain sees, what stays
// private, why it holds, its line (note), for a card whose ✓ says what it
// checked rather than "equals" the chain's value, what it says (check) and,
// for a card whose ✓ (or ✗) is not the engine's, whether it shows ✓ (ok). A
// part that depends on the formula detail d is a function of it.
const ARGUMENT = {
  spendKey: {
    property: 'Only the owner can spend', eq: 'pk_spend = Poseidon(sk_spend)',
    sees: /pk_spend, in the UserRegistry and inside every commitment to the Sender/, private: /^stays private\s*sk_spend$/,
    why: /Poseidon is one-way, so pk_spend reveals nothing about sk_spend\..*needs sk_spend inside the proof.*nullifier’s key.*only its holder can spend the notes sent to pk_spend/,
    // A walkthrough's says its fold holds a throwaway demo key; a full run's has no fold to say it of.
    note: d => d.formula.chain ? /throwaway/ : /^$/,
  },
  commitment: {
    property: 'Hiding', eq: 'commitment = Poseidon(pk_spend, salt, amount, token)',
    sees: /the commitment/, private: /pk_spend.*salt.*amount.*token/, why: /Poseidon is one-way.*random salt.*guessed amount can’t be checked/,
    note: /opening is public on chain[^.]*\. Privacy starts at the first spend/,
  },
  conservation: {
    property: 'Amounts hidden, yet balanced', eq: 'Σin = Σout: in₀ = out₀ + out₁, with no fee in the circuit; and every value 0 ≤ v < 10³⁶',
    sees: /only commitments: the spent note’s, behind its nullifier, and the two new ones/, private: /^stays private\s*every amount: the spent note’s, the payment’s, the change$/,
    why: /amounts sit inside the commitments, hidden as the commitment card’s Hiding says\..*circuit proves Σin = Σout over these hidden amounts.*range check is why no one can mint.*“negative” amount.*0 ≤ v < 10³⁶ rules that out/,
    // A walkthrough's ✓ is the amounts summing, in range, and the Verifier accepting the proof.
    check: d => d.formula.ok ? /: the amounts sum, each is in range, and the chain answers true to the Verifier.s verifyProof/ : /: not all hold of the amounts summing/,
  },
  noteEncryption: {
    property: 'Only the recipient (and the auditor) can read it',
    eq: '(capsule, ss) = ML-KEM-768.Encaps(view key) → salt, k = HKDF(ss) → sealed = AES-GCM_k(token, amount); the recipient: ss = Decaps(view key, capsule), the same salt and k, opens sealed, and Poseidon(pk_spend, salt, amount, token) = commitment',
    sees: /the capsule and the sealed payload/, private: /^stays private\s*the shared secret ss, the salt, the amount, the token$/,
    why: /Only the view key’s holder can decapsulate the capsule to the shared secret.*ML-KEM-768 is post-quantum.*AES-GCM fails to open under any other key.*The auditor reads the note too: it recovers every user’s view key/,
  },
  groth16: {
    property: 'Zero-knowledge and soundness', eq: 'on BN254: e(A, B) = e(α, β) · e(L, γ) · e(C, δ), with L = IC₀ + Σ xᵢ · ICᵢ',
    sees: /^chain sees\s*the proof \(A, B, C\) and the public inputs$/, private: /the whole witness/,
    why: /^why it holdsZero-knowledge: the proof reveals nothing beyond its public inputs.*Soundness: no one can make a proof that checks without a valid witness, so the chain can trust a proof it can’t look inside\.$/,
    note: /A, B and C are the proof.*L is the public inputs xᵢ folded into one point/,
    // A walkthrough's ✓ is the chain's answer to the page's on-chain check: ✗ once TAMPER changed an input.
    ok: d => d.formula.ok && !tampered,
    check: d => tampered
      ? new RegExp(`: TAMPER changed one public input, ${verifyProofCall().tamper.name.replace(/[[\]]/g, '\\$&')} 0x.* → 0x.*, and the chain answers false to the Verifier.s verifyProof`)
      : /: the chain answers true to the Verifier.s verifyProof/,
  },
  merklePath: {
    property: 'Anonymity', eq: 'nodeᵢ₊₁ = Poseidon(left, right): nodeᵢ and siblingᵢ, ordered by bit i of the leaf index, from node₀ = leaf up 8 levels to root = node₈',
    sees: d => new RegExp(`the root and the tree number, tree ${d.tree}$`), private: /the leaf and its position/,
    // A walkthrough's names the crowd: the leaves its tree held when the root was taken.
    why: d => new RegExp(`the note is one of the ${d.formula.leaves ? `${d.formula.leaves} ` : ''}leaves of tree ${d.tree}, not which\\. A deeper tree means a larger crowd to hide in`),
  },
  nullifier: {
    property: 'Unlinkable, spend-once', eq: 'nullifier = Poseidon(sk_spend, tree · 2⁸ + leaf index)',
    sees: /the nullifier/, private: /sk_spend.*position/,
    why: /Linking a nullifier to its note needs sk_spend.*same note always gives the same nullifier, so the vault refuses a second spend/,
    // A walkthrough's points at its own replay; a full run's at a walkthrough's.
    note: d => d.formula.chain ? /^The replay shows that refusal: after payment 2/ : /^A walkthrough’s replay shows that refusal: after payment 2/,
  },
};

// tampered says whether the walkthrough's on-chain proof check was last
// sent tampered: the Groth16 card's ✓ follows it.
let tampered = false;

// checkFormulas fails unless the hop detail panel shows one formula card
// per formula detail of hop h, each with its privacy argument (ARGUMENT),
// and, only when the detail carries values (a walkthrough's), one line with
// its ✓ (or ✗) naming the chain's value and a closed "this run's values"
// fold with every input, the result and the chain's value digit for digit,
// in order. A value-less detail (a full run's) shows no value, check or
// fold. It returns how many values it checked.
function checkFormulas(doc, h, where) {
  const fs = (h.hopDetail || []).filter(d => d.kind === 'formula');
  const shown = [...doc.querySelectorAll('#hopPanel .formula')];
  assert.strictEqual(shown.length, fs.length, `${where}: formula cards shown`);
  let n = 0;
  fs.forEach(({ formula: f }, i) => {
    const card = shown[i], at = `${where}, ${f.name} formula`;
    const a = ARGUMENT[f.name], part = v => typeof v === 'function' ? v(fs[i]) : v;
    assert.ok(a, `${at}: a formula the replay test has no argument for`);
    assert.strictEqual(card.querySelector('.fprop')?.textContent, a.property, `${at}: its property as the title`);
    assert.strictEqual(card.querySelector('.feq')?.textContent, a.eq, `${at}: its equation`);
    const sees = card.querySelector('.farg.sees'), priv = card.querySelector('.farg.priv');
    assert.ok(sees?.querySelector('.vis.pub'), `${at}: a chain-sees line with the public tag`);
    assert.match(sees.textContent, part(a.sees), `${at}: what the chain sees`);
    assert.ok(priv?.querySelector('.vis.priv'), `${at}: a stays-private line with the private tag`);
    assert.match(priv.textContent, part(a.private), `${at}: what stays private`);
    assert.match(card.querySelector('.fwhy')?.textContent || '', part(a.why), `${at}: why it holds`);
    if (a.note) assert.match(card.querySelector('.fnote')?.textContent || '', part(a.note), `${at}: its line`);
    const fold = card.querySelector('details.fvals'), chk = card.querySelector('.fchk');
    if (!f.chain) {
      assert.ok(!fold && !chk && !card.querySelector('.fv'), `${at}: a value-less (full run) card shows a value, check or values fold`);
      return;
    }
    assert.ok(chk, `${at}: no check line`);
    const ok = a.ok ? part(a.ok) : f.ok;
    assert.match(chk.textContent, new RegExp(`^${ok ? '✓' : '✗'} checked against the chain with this run’s values`), `${at}: the check line`);
    assert.ok(chk.textContent.includes(f.chain.name), `${at}: the check line names the chain's value`);
    assert.match(chk.textContent, a.check ? part(a.check) : new RegExp(`: ${ok ? 'equals' : 'differs from'} `), `${at}: what the check line says it checked`);
    assert.ok(fold, `${at}: no "this run's values" fold`);
    assert.strictEqual(fold.querySelector('summary')?.textContent, 'this run’s values', `${at}: the values fold's summary`);
    assert.ok(!fold.open, `${at}: the values fold is open, want it closed`);
    const values = [...card.querySelectorAll('.fv')];
    assert.ok(values.every(v => fold.contains(v)), `${at}: a value outside the values fold`);
    // In card order: each input, the result, then the chain's value.
    const want = [...f.inputs.map(x => x.value), f.result, f.chain.value];
    assert.deepStrictEqual(values.map(v => v.textContent), want, `${at}: its values, digit for digit, in order`);
    n += want.length;
    for (const x of f.inputs) assert.ok(fold.textContent.includes(x.name), `${at}: input ${x.name} unnamed`);
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
  // A full run's formula details carry only their name, each once on its
  // first hop: the spend key on the Sender's registration, the commitment on
  // the shield, the Merkle path and value conservation on payment 1's proof
  // request, the nullifier on its settlement, and, tags on, note encryption on
  // the audit: each participant opens their note with a tag channel's key.
  const formulas = DEFAULT_RUN.events.filter(e => e.kind === 'hop').flatMap(hop => (hop.hopDetail || []).filter(d => d.kind === 'formula').map(detail => ({ hop, detail })));
  assert.deepStrictEqual(formulas.map(({ hop, detail }) => [hop.step, `${hop.from} → ${hop.to}`, detail.formula.name]),
    [['register', 'actor1 → UserRegistry', 'spendKey'], ['shield', 'actor1 → Erc20CoinVault', 'commitment'], ['fanout_1', 'actor1 → gnark', 'merklePath'], ['fanout_1', 'actor1 → gnark', 'conservation'], ['fanout_1', 'relayer → EnygmaDvp', 'nullifier'], ['fanout_1', 'relayer → EnygmaDvp', 'groth16'], ['audit', 'Erc20CoinVault → auditor', 'noteEncryption']],
    'formula details: the spend key on the Sender’s registration, the commitment on the shield, the Merkle path, value conservation, the nullifier and the Groth16 check on payment 1, note encryption on the audit');
  for (const { detail } of formulas) assert.deepStrictEqual(Object.keys(detail.formula), ['name'], `a full run’s ${detail.formula.name} formula detail carries values`);
});

test('the page plays the whole default run and replays every step’s hop details without an exception, showing every field element digit for digit', async () => {
  const page = await open(DEFAULT_RUN);
  // Up to each hop with a formula: its value-less card shows as it plays.
  let fed = 0;
  for (const [at, e] of DEFAULT_RUN.events.entries()) {
    if (e.kind !== 'hop' || !(e.hopDetail || []).some(d => d.kind === 'formula')) continue;
    feed(page, DEFAULT_RUN.events.slice(fed, at + 1), fed);
    fed = at + 1;
    await playOut(page);
    checkFormulas(page.doc, e, `${e.step} ${e.label}, live`);
  }
  assert.ok(fed > 0, 'no formula card played');
  feed(page, DEFAULT_RUN.events.slice(fed), fed);
  await playOut(page);
  failOn(page.errors, 'while playing the run');
  assert.ok(!page.doc.getElementById('hopFold').open, 'a full run opened the hop detail fold by itself');
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
  assert.ok(WALKTHROUGH.events.some(e => e.step === 'register' && e.label === 'register' && e.from === 'actor1' && (e.hopDetail || []).some(d => d.kind === 'formula' && d.formula.name === 'spendKey' && d.formula.chain)),
    'no spend key formula, with its values, on the Sender’s registration');
  assert.ok(WALKTHROUGH.events.some(e => e.step === 'fanout_1' && e.label === 'prove Payment' && (e.hopDetail || []).some(d => d.kind === 'formula' && d.formula.name === 'conservation' && d.formula.chain)),
    'no value conservation formula, with its values, on payment 1’s proof request');
  // Payment 1's settlement carries the Groth16 card with the proof and public inputs the on-chain check sends.
  const check = verifyProofCall();
  const groth16 = WALKTHROUGH.events.filter(e => e.step === 'fanout_1' && e.from === 'relayer' && e.to === 'EnygmaDvp').flatMap(e => (e.hopDetail || []).filter(d => d.kind === 'formula' && d.formula.name === 'groth16'));
  assert.strictEqual(groth16.length, 1, 'no Groth16 formula on payment 1’s settlement');
  assert.deepStrictEqual(groth16[0].formula.inputs.map(x => x.value), [...check.proof, ...check.inputs.map(x => x.value)],
    'the Groth16 card’s values: the proof and the public inputs the on-chain check sends');
  // Tags off, the first participant's scan is the first hop a view key opens a note's ciphertext by.
  assert.ok(WALKTHROUGH.events.some(e => e.step === 'discover' && e.label === 'scan' && e.from === 'Erc20CoinVault' && e.to === 'actor2' && (e.hopDetail || []).some(d => d.kind === 'formula' && d.formula.name === 'noteEncryption' && d.formula.chain)),
    'no note encryption formula, with its values, on the first participant’s scan');
  for (const name of ['merklePath', 'nullifier'])
    assert.ok(WALKTHROUGH.events.some(e => e.step === 'fanout_1' && (e.hopDetail || []).some(d => d.kind === 'formula' && d.formula.name === name)), `no ${name} formula on payment 1`);
  // The Merkle path card names its crowd: at least the leaves up to the spent one.
  for (const { formula: f } of details.filter(d => d.kind === 'formula' && d.formula.name === 'merklePath'))
    assert.ok(f.leaves > Number(f.inputs[0].name.match(/#(\d+)/)[1]), `the Merkle path's crowd is ${f.leaves} leaves, want more than its leaf index`);
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
    if (latest?.hopDetail.some(d => d.kind === 'formula')) {
      checkFormulas(doc, latest, `gate ${nexts + 1}, live`);
      formulaLive = true;
      // The hop detail fold opened by itself; the presenter closes it, and it stays closed until the next formula hop.
      assert.ok($('hopFold').open, `gate ${nexts + 1}: the hop detail fold did not open by itself for ${latest.label}'s formula card`);
      $('hopFold').open = false;
      await tick();
      assert.ok(!$('hopFold').open, `gate ${nexts + 1}: the presenter could not close the hop detail fold`);
    }
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
  // The Groth16 card on payment 1's settlement follows the check: ✗ once tampered, ✓ again once checked.
  const settle = WALKTHROUGH.events.find(e => e.kind === 'hop' && e.step === 'fanout_1' && e.from === 'relayer' && e.to === 'EnygmaDvp');
  const groth16Mark = () => [...doc.querySelectorAll('#hopPanel .formula')].find(c => c.querySelector('.fprop')?.textContent === 'Zero-knowledge and soundness')?.querySelector('.fchk')?.textContent.slice(0, 1);
  $('step-fanout_1').click();
  await tick();
  for (let i = 0; i < 12 && !groth16Mark(); i++) { doc.querySelector('#hopPanel [data-replay="next"]').click(); await tick(); }
  assert.strictEqual(groth16Mark(), '✓', 'the Groth16 card on payment 1’s settlement, checked');
  checkFormulas(doc, settle, 'payment 1’s settlement, checked');
  $('verify').querySelector('[data-v="tamper"]').click();
  await tick(); await tick();
  assert.strictEqual(verdict(), 'verifyProof → false', 'the tampered call');
  tampered = true;
  assert.strictEqual(groth16Mark(), '✗', 'the Groth16 card did not turn ✗ under TAMPER');
  checkFormulas(doc, settle, 'payment 1’s settlement, tampered');
  $('verify').querySelector('[data-v="check"]').click();
  await tick(); await tick();
  tampered = false;
  assert.strictEqual(groth16Mark(), '✓', 'the Groth16 card did not turn ✓ again once checked');
  $('verify').querySelector('[data-v="tamper"]').click();
  await tick(); await tick();
  tampered = true;
  failOn(errors, 'checking the proof on chain');

  const { formulaValues } = await replayAll(page, WALKTHROUGH, all);
  assert.ok(formulaValues > 0, 'no formula card value checked');
  failOn(errors, 'at the end');
});
