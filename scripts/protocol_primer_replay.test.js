// The Protocol primer (enygma_retail_payments/shield_demo/protocol.html),
// stepped through in jsdom on the run its data block holds.
//
// The data block is a JSON <script> inside the page, which a passing
// TestProtocolPrimer (enygma_retail_payments/test/15_…) rewrites from one
// live run. Each test opens the page, as the demo server serves it and as a
// file opened from disk, with every request the page could make failing the
// test: it makes none. It steps through the page with Next to the last step
// and back to the first with Back, and on every step checks that:
//   - the step's section shows each of the step's values digit for digit,
//     each where its name is, and has a MathML formula;
//   - the observer's ledger holds exactly the public items of the steps up
//     to this one, in order, digit for digit, with each msg.sender marked;
//   - the diagram plays this step;
//   - Back restores what the previous step showed;
// and that every step's text stays in the document, never hidden.
//
//   node --test scripts/protocol_primer_replay.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { JSDOM, VirtualConsole } = require('jsdom');

const DEMO = path.join(__dirname, '..', 'enygma_retail_payments', 'shield_demo');
const PRIMER_FILE = path.join(DEMO, 'protocol.html');
const PRIMER = fs.readFileSync(PRIMER_FILE, 'utf8');
const BLOCK = /(<script type="application\/json" id="primer-data">\n)([\s\S]*?)(\n<\/script>)/;
const DATA = JSON.parse(PRIMER.match(BLOCK)?.[2] ?? 'null');

// open loads page (the primer by default) at url. A request of any kind, an
// exception the page throws or a console.error is recorded in errors.
function open({ page = PRIMER, url = 'http://localhost:9092/protocol' } = {}) {
  const errors = [];
  const vc = new VirtualConsole();
  vc.on('jsdomError', err => errors.push(err));
  vc.on('error', (...a) => errors.push(new Error('console.error: ' + a.join(' '))));
  const refuse = what => function () { errors.push(new Error(`the primer made a request: ${what} ${[...arguments][0]}`)); throw new Error('no requests'); };
  const dom = new JSDOM(page, {
    url,
    runScripts: 'dangerously',
    pretendToBeVisual: true,
    virtualConsole: vc,
    beforeParse(win) {
      win.fetch = refuse('fetch');
      win.EventSource = refuse('EventSource');
      win.WebSocket = refuse('WebSocket');
      win.XMLHttpRequest.prototype.open = refuse('XMLHttpRequest');
      win.navigator.sendBeacon = refuse('sendBeacon');
      win.Element.prototype.scrollIntoView = () => {};
    },
  });
  test.after(() => dom.window.close());
  return { win: dom.window, doc: dom.window.document, errors };
}

const text = el => el.textContent.replace(/\s+/g, ' ');
const section = (doc, step) => doc.querySelector(`section.step[data-step="${step}"]`);
const steps = doc => [...doc.querySelectorAll('section.step')].map(s => s.dataset.step);

// shown is what the stepper shows: the current step, the ledger's items and
// the diagram's step.
function shown(doc) {
  const current = doc.querySelectorAll('section.step.current');
  assert.strictEqual(current.length, 1, 'one step is current');
  return {
    step: current[0].dataset.step,
    ledger: [...doc.querySelectorAll('#ledger li')].map(li => ({ text: text(li), msgSender: li.classList.contains('msg-sender') })),
    diagram: doc.getElementById('flow').dataset.step,
  };
}

// OWNERS names the person a value's key prefix belongs to.
const OWNERS = { sender: 'Sender', ana: 'Ana', auditor: 'Auditor' };

// ownerOf is the person whose value el shows, as the page labels it: the
// owner its formula row names, or the heading over its rows of values.
function ownerOf(el) {
  const row = el.closest('mtr');
  if (row) return Object.values(OWNERS).find(o => row.textContent.includes(o));
  for (let n = el.previousElementSibling; n; n = n.previousElementSibling) {
    if (n.classList.contains('who')) return n.textContent.trim();
  }
}

// checkStep checks that the page shows step i of the data block's run.
function checkStep(doc, i) {
  const names = steps(doc);
  const step = names[i];
  const s = shown(doc);
  assert.strictEqual(s.step, step, `step ${i + 1} is current`);
  assert.strictEqual(s.diagram, step, `the diagram plays ${step}`);
  assert.match(text(doc.getElementById('stepCount')), new RegExp(`${i + 1}\\s*/\\s*${names.length}`));

  const sec = section(doc, step), body = text(sec);
  assert.ok(sec.querySelector('math'), `${step} has a MathML formula`);
  const values = DATA.steps[step].values;
  for (const [k, v] of Object.entries(values)) {
    assert.ok(body.includes(v), `${step} shows ${k} digit for digit (${v.slice(0, 20)}…)`);
  }
  // Each value sits where its name is: every element bound to a value shows
  // that value, so swapping two (the Sender's and Ana's) fails.
  for (const el of sec.querySelectorAll('[data-v]')) {
    const k = el.getAttribute('data-v');
    assert.ok(k in values, `${step} binds ${k}, which the data block does not hold`);
    assert.strictEqual(el.textContent, values[k], `${step}: ${k} shows its own value`);
    const owner = OWNERS[k.split('.')[0]];
    if (owner) assert.strictEqual(ownerOf(el), owner, `${step}: ${k} is shown under ${owner}`);
  }
  const mathValues = [...sec.querySelectorAll('math [data-v]')].length;
  assert.ok(step !== 'keys' || mathValues >= 4, `keys shows its values inside its formula (${mathValues} bound)`);

  // The ledger: every public item so far, in order.
  const want = names.slice(0, i + 1).flatMap(n => DATA.steps[n].ledger);
  assert.strictEqual(s.ledger.length, want.length, `${step}: ledger holds ${want.length} items, has ${s.ledger.length}`);
  want.forEach((item, j) => {
    const got = s.ledger[j];
    for (const part of [item.tx, item.field, item.value]) assert.ok(got.text.includes(part), `${step}: ledger item ${j} shows ${part.slice(0, 30)}`);
    assert.strictEqual(got.msgSender, !!item.msgSender, `${step}: ledger item ${j} (${item.field}) ${item.msgSender ? 'is' : 'is not'} marked as msg.sender`);
  });
}

// stepThrough steps the page with Next to the end and Back to the start,
// checking each step, and that Back restores the step before.
function stepThrough({ doc, errors }) {
  const names = steps(doc);
  const next = doc.getElementById('next'), back = doc.getElementById('back');
  const seen = [];
  assert.deepStrictEqual(errors, [], 'the page opened without an error');
  checkStep(doc, 0);
  assert.ok(back.disabled, 'Back is disabled on the first step');
  seen.push(shown(doc));
  for (let i = 1; i < names.length; i++) {
    next.click();
    checkStep(doc, i);
    seen.push(shown(doc));
  }
  assert.ok(next.disabled, 'Next is disabled on the last step');
  for (let i = names.length - 2; i >= 0; i--) {
    back.click();
    assert.deepStrictEqual(shown(doc), seen[i], `Back restores step ${i + 1} (${names[i]})`);
    checkStep(doc, i);
  }
  // All the text stays in the document, whatever step is current.
  for (const s of doc.querySelectorAll('section.step')) {
    assert.ok(!s.closest('[hidden]'), `${s.dataset.step} is never hidden`);
  }
  assert.deepStrictEqual(errors, []);
}

test('the data block holds an entry for each of the primer\'s steps, and nothing else', () => {
  assert.ok(DATA, 'the primer has a data block');
  const { doc } = open();
  assert.deepStrictEqual(steps(doc), ['keys', 'auditor', 'registration']);
  assert.deepStrictEqual(Object.keys(DATA.steps).sort(), steps(doc).slice().sort());
  assert.ok(DATA.steps.registration.ledger.some(i => i.msgSender), 'the registrations name their msg.sender');
});

test('served by the demo, the primer steps through its run and links back to the demo', () => {
  const p = open();
  stepThrough(p);
  assert.strictEqual(p.doc.getElementById('demoLink').href, 'http://localhost:9092/');
});

test('opened from disk, the primer steps through its run, makes no requests, and links to the demo on its default port', () => {
  const p = open({ url: 'file://' + PRIMER_FILE });
  stepThrough(p);
  assert.strictEqual(p.doc.getElementById('demoLink').href, 'http://localhost:9092/');
  // Nothing to load: no external script, stylesheet, image or frame.
  const external = [...p.doc.querySelectorAll('[src], link[href]')].map(el => el.outerHTML.slice(0, 80));
  assert.deepStrictEqual(external, []);
});

test('the page shows the data block\'s values, not values of its own', () => {
  const v = DATA.steps.keys.values;
  const edited = structuredClone(DATA);
  edited.steps.keys.values['sender.pkSpend'] = '1234567890123456789';
  edited.steps.registration.ledger[0].value = '0x00000000000000000000000000000000000000aa';
  const page = PRIMER.replace(BLOCK, (_, a, _b, c) => a + JSON.stringify(edited) + c);
  const { doc, errors } = open({ page });
  assert.ok(text(section(doc, 'keys')).includes('1234567890123456789'));
  assert.ok(!text(section(doc, 'keys')).includes(v['sender.pkSpend']));
  doc.getElementById('next').click();
  doc.getElementById('next').click();
  assert.ok(text(doc.getElementById('ledger')).includes('0x00000000000000000000000000000000000000aa'));
  assert.deepStrictEqual(errors, []);
});

test('the demo\'s header links to the primer', () => {
  const demo = new JSDOM(fs.readFileSync(path.join(DEMO, 'index.html'), 'utf8')).window.document;
  assert.ok(demo.querySelector('header a[href="/protocol"]'), 'the demo header has a link to /protocol');
});
