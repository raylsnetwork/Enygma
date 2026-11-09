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
  // The values a step shows inside its formulas, not only beside them.
  const inMath = new Set([...sec.querySelectorAll('math [data-v]')].map(el => el.getAttribute('data-v')));
  if (step === 'payment') {
    for (const k of ['commitmentIn', 'nullifier', 'globalIndex', 'commitmentOut0', 'commitmentOut1', 'valueIn', 'valueOut0', 'valueOut1', 'root']) assert.ok(inMath.has(k), `payment shows ${k} inside a formula`);
  }
  if (step === 'proof') {
    for (const k of ['L', 'x1', 'x2', 'x3', 'x4', 'x5', 'x6', 'x7', 'n', 'omega', 'nbWires', 'nbPublic', 'nbPrivate', 'verifyProof']) assert.ok(inMath.has(k), `proof shows ${k} inside a formula`);
  }
  if (step === 'discovery') {
    for (const k of ['pkSpend', 'salt', 'saltBytes', 'amount', 'token', 'commitment', 'scanned', 'opened']) assert.ok(inMath.has(k), `discovery shows ${k} inside a formula`);
  }
  if (step === 'withdrawal') {
    for (const k of ['message', 'recipientField', 'amount', 'token', 'commitment0', 'pkSpend', 'saltChange', 'change', 'commitmentChange', 'skSpend', 'nullifier', 'globalIndex', 'valueIn']) assert.ok(inMath.has(k), `withdrawal shows ${k} inside a formula`);
  }
  if (step === 'verdict') {
    for (const k of ['payer', 'withdrawer', 'shieldAmount', 'withdrawAmount', 'poolLeft']) assert.ok(inMath.has(k), `verdict shows ${k} inside a formula`);
  }
  if (step === 'audit') {
    for (const k of ['pkSpend', 'salt', 'amount', 'token', 'commitment', 'changeAmount', 'withdrawn']) assert.ok(inMath.has(k), `audit shows ${k} inside a formula`);
  }
  if (step === 'shield') {
    for (const k of ['pkSpend', 'salt', 'amount', 'token', 'commitment']) assert.ok(inMath.has(k), `shield shows ${k} inside the commitment's formula`);
  }

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
  assert.deepStrictEqual(steps(doc), ['keys', 'auditor', 'registration', 'shield', 'payment', 'proof', 'discovery', 'withdrawal', 'audit', 'verdict']);
  assert.deepStrictEqual(Object.keys(DATA.steps).sort(), steps(doc).slice().sort());
  assert.ok(DATA.steps.registration.ledger.some(i => i.msgSender), 'the registrations name their msg.sender');
});

test('the shield\'s ledger entries are the deposit\'s public calldata, its commitment and its msg.sender', () => {
  const v = DATA.steps.shield.values, ledger = DATA.steps.shield.ledger;
  const field = f => ledger.filter(i => i.field === f).map(i => i.value);
  assert.deepStrictEqual(field('msg.sender'), [v.msgSender], 'the deposit names its msg.sender');
  assert.ok(ledger.find(i => i.field === 'msg.sender').msgSender, 'marked amber');
  assert.deepStrictEqual(field('amount'), [v.amount], 'the entry amount is public');
  for (const f of ['pkSpend', 'salt', 'tokenId']) assert.strictEqual(field(f).length, 1, `the calldata's ${f}`);
  assert.deepStrictEqual(field('commitment'), [v.commitment], 'the Commitment event\'s leaf');
  assert.strictEqual(ledger.filter(i => i.msgSender).length, 1, 'only msg.sender is amber');
  const { doc } = open();
  const ul = doc.getElementById('ledger');
  for (let i = 0; i < 3; i++) doc.getElementById('next').click();
  const amber = [...ul.querySelectorAll('li.msg-sender')].map(li => text(li));
  assert.ok(amber.some(t => t.includes('depositV2') && t.includes(v.msgSender)), 'the deposit\'s msg.sender shows amber in the ledger');
  // The step names the bound the tree puts on the crowd: 2^8 leaf slots.
  assert.strictEqual(v.depth, '8');
  assert.ok(text(section(doc, 'shield')).includes('256'), 'the shield names the tree\'s 256 leaf slots');
});

// GATES are the deployed Payment circuit's constraints (gnark_circuits/
// templates/Payment.go), each a gate of the payment step's circuit diagram
// with a card naming what it stops.
const GATES = ['message', 'range', 'ownership', 'enable', 'commitmentIn', 'merkle', 'nullifier', 'commitmentsOut', 'changeOwner', 'conservation', 'withdrawSalt', 'contractAddress'];
// PUBLIC are the Payment statement's public inputs, in the layout's order
// (enygma_retail_payments/layout: PaymentLen = 7).
const PUBLIC = ['Message', 'TreeNumber', 'MerkleRoot', 'Nullifier', 'CommitmentOut[0]', 'CommitmentOut[1]', 'ContractAddress'];

test('the payment\'s circuit diagram: the witness enters private, every constraint is a gate with a card, only the public inputs leave to the Verifier', () => {
  const { doc } = open();
  const sec = section(doc, 'payment');
  const svg = sec.querySelector('svg#circuit');
  assert.ok(svg, 'the payment step has a circuit diagram');
  const drawn = [...svg.querySelectorAll('.gate')].map(g => g.dataset.gate);
  assert.deepStrictEqual(drawn, GATES, 'each deployed constraint is drawn as a gate, in the circuit\'s order');
  const cards = [...sec.querySelectorAll('.gate-card')].map(c => c.dataset.gate);
  assert.deepStrictEqual(cards, GATES, 'each gate has a card');
  for (const c of sec.querySelectorAll('.gate-card')) {
    assert.ok(c.querySelector('math'), `${c.dataset.gate}: its constraint as MathML`);
    assert.ok(text(c.querySelector('.stops')).trim().length > 20, `${c.dataset.gate}: one line on what it stops`);
  }
  const out = [...svg.querySelectorAll('.pubin')].map(n => n.dataset.name);
  assert.deepStrictEqual(out, PUBLIC, 'the public inputs, and only they, leave the circuit');
  assert.ok(svg.querySelectorAll('.witness').length >= 8, 'the witness enters on the private side');
  // Wires run witness → gate → public input → Verifier: none from the
  // witness past the gates.
  for (const w of svg.querySelectorAll('.wire')) {
    assert.ok(['wg', 'gp', 'pv'].includes(w.dataset.kind), `a wire is witness→gate, gate→public or public→Verifier (${w.dataset.kind})`);
  }
  // Every gate is wired in: it reads the witness, fixes a public input, or
  // both (G1 and G12 constrain public inputs alone).
  for (const g of GATES) assert.ok(svg.querySelector(`.wire[data-gate="${g}"]`), `${g} is wired in`);
  for (const g of ['range', 'ownership', 'commitmentIn', 'merkle', 'nullifier', 'commitmentsOut', 'changeOwner', 'conservation']) assert.ok(svg.querySelector(`.wire[data-kind="wg"][data-gate="${g}"]`), `${g} reads the witness`);
  for (const p of PUBLIC) assert.ok(svg.querySelector(`.wire[data-kind="gp"][data-to="${p}"]`), `a gate fixes ${p}`);
  assert.strictEqual(svg.querySelectorAll('.wire[data-kind="pv"]').length, PUBLIC.length, 'each public input goes to the Verifier');
  const cardText = text(sec);
  for (const s of ['VULN-3', '10', 'another vault']) assert.ok(cardText.includes(s), `the cards name ${s}`);
});

test('the payment\'s ledger entries are its public inputs, the nullifier, the two new commitments and the Sender\'s msg.sender', () => {
  const v = DATA.steps.payment.values, ledger = DATA.steps.payment.ledger;
  const field = f => ledger.filter(i => i.field === f).map(i => i.value);
  assert.deepStrictEqual(field('msg.sender'), [v.msgSender], 'the payment names its msg.sender');
  assert.strictEqual(ledger.filter(i => i.msgSender).length, 1, 'only msg.sender is amber');
  assert.strictEqual(v.msgSender, DATA.steps.shield.values.msgSender, 'the Sender sent it, from the account that shielded');
  const pubs = ledger.filter(i => i.tx === 'payment · public inputs');
  assert.deepStrictEqual(pubs.map(i => i.field), PUBLIC, 'the seven public inputs, in order');
  assert.deepStrictEqual(pubs.map(i => i.value), [v.message, v.tree, v.root, v.nullifier, v.commitmentOut0, v.commitmentOut1, v.contractAddress]);
  assert.deepStrictEqual(field('nullifier'), [v.nullifier], 'the Nullifier event');
  assert.deepStrictEqual(field('commitment'), [v.commitmentOut0, v.commitmentOut1], 'the two Commitment events');
  // No private value reaches the ledger.
  const all = ledger.map(i => i.value).join(' ');
  for (const k of ['skSpend', 'saltIn', 'saltOut0', 'saltOut1', 'anaPk']) assert.ok(!all.includes(v[k]), `${k} is not public`);
  for (const k of ['valueIn', 'valueOut0', 'valueOut1']) assert.ok(!ledger.some(i => i.value === v[k]), `${k} is not public`);
  // The values hold: conservation and range, as the run checked.
  assert.strictEqual(BigInt(v.valueIn), BigInt(v.valueOut0) + BigInt(v.valueOut1), 'Σin = Σout');
  for (const k of ['valueIn', 'valueOut0', 'valueOut1']) assert.ok(BigInt(v[k]) < 10n ** 36n, `${k} < 10^36`);
  assert.strictEqual(v.valueIn, DATA.steps.shield.values.amount, 'the spent note is the shielded one');
  assert.strictEqual(v.commitmentIn, DATA.steps.shield.values.commitment);
  assert.strictEqual(BigInt(v.globalIndex), BigInt(v.tree) * 256n + BigInt(v.leafIndex), 'tree·2^8 + leaf index');
  assert.strictEqual(v['path7.node'], v.root, 'the eighth level is the root');
  const { doc } = open();
  for (let i = 0; i < 4; i++) doc.getElementById('next').click();
  const amber = [...doc.querySelectorAll('#ledger li.msg-sender')].map(li => text(li));
  assert.ok(amber.some(t => t.includes('payment · Sender') && t.includes(v.msgSender)), 'the payment\'s msg.sender shows amber in the ledger');
});

test('the proof step folds the payment\'s public inputs into L, and the Verifier accepted the payment\'s proof and refused it with a tampered input', () => {
  const v = DATA.steps.proof.values, pay = DATA.steps.payment;
  // Verifying is a view call: nothing new on the ledger.
  assert.deepStrictEqual(DATA.steps.proof.ledger, [], 'verifying publishes nothing');
  // x₁…x₇ are the payment's public inputs, in the statement's order.
  const pubs = pay.ledger.filter(i => i.tx === 'payment · public inputs');
  PUBLIC.forEach((name, i) => {
    assert.strictEqual(v[`x${i + 1}.name`], name);
    assert.strictEqual(v[`x${i + 1}`], pubs[i].value, `x${i + 1} is the payment's ${name}`);
  });
  // The verifying key: IC₀ and one IC point per public input.
  assert.strictEqual(v.nbPublic, String(PUBLIC.length));
  for (let i = 0; i <= PUBLIC.length; i++) assert.match(v[`ic${i}`], /^\(\d+, \d+\)$/, `IC${i} is a G1 point`);
  assert.ok(!(`ic${PUBLIC.length + 1}` in v), 'no IC point past the public inputs');
  for (const k of ['alpha', 'L', 'A', 'C']) assert.match(v[k], /^\(\d+, \d+\)$/, `${k} is a G1 point`);
  for (const k of ['beta', 'gamma', 'delta', 'B']) assert.match(v[k], /^\(\d+ \+ \d+·u, \d+ \+ \d+·u\)$/, `${k} is a G2 point`);
  // The QAP's domain: n = 2^log2n points, holding between n/2 and n constraints.
  assert.strictEqual(BigInt(v.n), 1n << BigInt(v.log2n));
  assert.strictEqual(BigInt(v.halfN) * 2n, BigInt(v.n));
  assert.strictEqual(Number(v.nbWires), 1 + Number(v.nbPublic) + Number(v.nbPrivate), 'm = 1 + public + private');
  // The Verifier's answers: true for the settled proof, false with Nullifier + 1.
  assert.strictEqual(v.verifyProof, 'true');
  assert.strictEqual(v.tampered, 'false');
  assert.strictEqual(v.vkSlot, '0');
  const { doc } = open();
  const body = text(section(doc, 'proof'));
  for (const s of ['R1CS', 'QAP', 'toxic waste', 'single', 'ceremony', 'one honest participant', 'gnark server', 'generic group', 'zero-knowledge']) assert.ok(body.includes(s), `the proof step names ${s}`);
});

test('Ana\'s discovery opens the Payment event addressed to her, its opening hashes to her commitment, and her scan publishes nothing', () => {
  const v = DATA.steps.discovery.values, pay = DATA.steps.payment.values;
  // The scan is local: it reads the chain and adds nothing to the ledger.
  assert.deepStrictEqual(DATA.steps.discovery.ledger, [], 'the scan publishes nothing');
  // The ledger has the capsule and the sealed payload beside the commitment,
  // in the payment's Payment event: the ciphertext Ana's key opens.
  const event = DATA.steps.payment.ledger.filter(i => i.tx === 'Payment event');
  assert.deepStrictEqual(event.map(i => [i.field, i.value]), [['commitmentOut', v.commitment], ['ctxt', v.capsule], ['encTxData', v.encTxData]]);
  // What she opened is what the Sender paid her, to her keys.
  assert.strictEqual(v.commitment, pay.commitmentOut0, 'her note is output 0');
  assert.strictEqual(v.amount, pay.valueOut0);
  assert.strictEqual(v.salt, pay.saltOut0, 'HKDF(ss, "note salt") mod p is the Sender\'s salt_0');
  assert.strictEqual(v.pkSpend, DATA.steps.keys.values['ana.pkSpend']);
  assert.strictEqual(v.anaEk, DATA.steps.keys.values['ana.viewEk']);
  // The sizes the page names: a 1088-byte capsule, a 32-byte secret, salt and
  // key, and nonce ‖ ct ‖ tag = 12 + 64 + 16 bytes over token ‖ amount.
  const bytes = h => (h.length - 2) / 2;
  assert.deepStrictEqual([v.capsule, v.ss, v.saltBytes, v.key, v.nonce, v.sealed, v.plaintext].map(bytes), [1088, 32, 32, 32, 12, 80, 64]);
  assert.strictEqual(v.encTxData, v.nonce + v.sealed.slice(2), 'encTxData is nonce ‖ ct ‖ tag');
  assert.strictEqual(BigInt(v.plaintext), (BigInt(v.token) << 256n) + BigInt(v.amount), 'the plaintext is token ‖ amount, 32 bytes each');
  assert.strictEqual(BigInt(v.salt), BigInt(v.saltBytes) % BigInt(DATA.steps.keys.values.p), 'salt = HKDF output mod p');
  // Exactly one of the events scanned opened; a forged amount was refused.
  assert.strictEqual(v.opened, '1');
  assert.ok(Number(v.scanned) >= 2, 'the scan covered another Payment event: refresh the block with make primer-refresh');
  assert.notStrictEqual(v.forged, v.amount);
  const { doc } = open();
  const body = text(section(doc, 'discovery'));
  for (const s of ['module-LWE', 'quantum', 'AES-256-GCM', 'HKDF-SHA256']) assert.ok(body.includes(s), `discovery names ${s}`);
});

test('Ana\'s partial withdrawal is the Payment circuit with Message = 1: amount and recipient public, her change a private note, her msg.sender amber', () => {
  const v = DATA.steps.withdrawal.values, ledger = DATA.steps.withdrawal.ledger, found = DATA.steps.discovery.values;
  const field = (tx, f) => ledger.filter(i => i.tx === tx && i.field === f).map(i => i.value);
  const call = 'partialWithdraw · Ana';
  // The note she spends is the one she discovered.
  assert.strictEqual(v.commitmentIn, found.commitment);
  assert.strictEqual(v.valueIn, found.amount);
  assert.strictEqual(v.saltIn, found.salt);
  assert.strictEqual(v.pkSpend, found.pkSpend);
  // The same statement as the payment's, with Message = 1.
  assert.strictEqual(v.message, '1');
  const pubs = ledger.filter(i => i.tx === 'partialWithdraw · public inputs');
  assert.deepStrictEqual(pubs.map(i => i.field), PUBLIC, 'the same seven public inputs, in order');
  assert.deepStrictEqual(pubs.map(i => i.value), [v.message, v.tree, v.root, v.nullifier, v.commitment0, v.commitmentChange, v.contractAddress]);
  assert.strictEqual(v.vkSlot, DATA.steps.proof.values.vkSlot, 'the same verifying key as the payment\'s');
  assert.strictEqual(v.verifyProof, 'true');
  // The exit is public, as the shield's amount was.
  assert.deepStrictEqual(field(call, 'amount'), [v.amount], 'the amount is calldata');
  assert.deepStrictEqual(field(call, 'recipient'), [v.recipient], 'the recipient is calldata');
  assert.deepStrictEqual(field('PartialWithdrawal event', 'amount'), [v.amount]);
  assert.deepStrictEqual(field('PartialWithdrawal event', 'recipient'), [v.recipient]);
  assert.strictEqual(BigInt(v.recipientField), BigInt(v.recipient), 'output 0\'s owner is the recipient\'s address as a number');
  // Ana's change stays private: its commitment is public, its value and salt are not.
  assert.strictEqual(BigInt(v.valueIn), BigInt(v.amount) + BigInt(v.change), 'Σin = amount + change');
  assert.ok(BigInt(v.change) > 0n, 'a partial withdrawal leaves change');
  assert.deepStrictEqual(field('Commitment event', 'commitment'), [v.commitmentChange], 'only the change is inserted, never output 0');
  assert.deepStrictEqual(field('PartialWithdrawal event', 'changeCommitment'), [v.commitmentChange]);
  assert.deepStrictEqual(field('Nullifier event', 'nullifier'), [v.nullifier]);
  for (const k of ['skSpend', 'saltIn', 'saltChange']) assert.ok(!ledger.some(i => i.value === v[k]), `${k} is not public`);
  for (const k of ['valueIn', 'change']) assert.ok(!ledger.some(i => i.value === v[k]), `${k} is not public`);
  assert.strictEqual(BigInt(v.globalIndex), BigInt(v.tree) * 256n + BigInt(v.leafIndex), 'tree·2^8 + leaf index');
  // The recipient's public balance grew by the amount.
  assert.strictEqual(BigInt(v.balanceAfter) - BigInt(v.balanceBefore), BigInt(v.amount));
  // Ana's msg.sender, and only it, is amber.
  assert.deepStrictEqual(ledger.filter(i => i.msgSender).map(i => [i.tx, i.value]), [[call, v.msgSender]]);
  assert.strictEqual(v.msgSender, DATA.steps.registration.values['ana.address'], 'Ana sent it from her registered account');
  const { doc } = open();
  for (let i = 0; i < 7; i++) doc.getElementById('next').click();
  const amber = [...doc.querySelectorAll('#ledger li.msg-sender')].map(li => text(li));
  assert.ok(amber.some(t => t.includes(call) && t.includes(v.msgSender)), 'her msg.sender shows amber in the ledger');
  const body = text(section(doc, 'withdrawal'));
  for (const s of ['Message = 1', 'mirror', 'G11', 'partialWithdrawWithRelayerFee']) assert.ok(body.includes(s), `the withdrawal names ${s}`);
});

test('the audit recovers Ana\'s view key from what she registered, opens her notes with it, and publishes nothing', () => {
  const v = DATA.steps.audit.values, reg = DATA.steps.registration.values, keys = DATA.steps.keys.values;
  const found = DATA.steps.discovery.values, exit = DATA.steps.withdrawal;
  assert.deepStrictEqual(DATA.steps.audit.ledger, [], 'the audit only reads the chain');
  // The ciphertexts are the ones Ana registered, and they open to her view key.
  assert.strictEqual(v.capsule, reg['ana.capsule']);
  assert.strictEqual(v.aesCt, reg['ana.aesCt']);
  assert.strictEqual(v.aesCt, v.nonce + v.sealed.slice(2), 'aesAuditCt is nonce ‖ ct ‖ tag');
  assert.strictEqual(v.seed, keys['ana.viewSeed'], 'the recovered seed is her view key\'s');
  assert.strictEqual(v.ek, keys['ana.viewEk'], 'the recovered key is the one she registered');
  assert.strictEqual(v.otherAuditor, 'refused', 'another auditor\'s key opens nothing');
  // Her note, opened from the Payment event on the ledger.
  assert.strictEqual(v.noteCapsule, found.capsule);
  assert.strictEqual(v.noteEncTxData, found.encTxData);
  assert.strictEqual(v.commitment, found.commitment);
  assert.strictEqual(v.amount, found.amount);
  assert.strictEqual(v.salt, found.salt);
  assert.strictEqual(v.pkSpend, keys['ana.pkSpend'], 'the commitment is checked against her registered key');
  // Her change, opened from the EncryptedNote event, agrees with the public amount.
  const field = f => exit.ledger.filter(i => i.tx === 'EncryptedNote event' && i.field === f).map(i => i.value);
  assert.deepStrictEqual(field('ciphertextI'), [v.changeCapsule]);
  assert.deepStrictEqual(field('ciphertextII'), [v.changeSealed]);
  assert.strictEqual(v.changeAmount, exit.values.change);
  assert.strictEqual(v.withdrawn, exit.values.amount);
  assert.strictEqual(BigInt(v.amount) - BigInt(v.withdrawn), BigInt(v.changeAmount), 'her note less the public amount is her change');
  // The Sender's change has no ciphertext: their recovered key opens no Payment event.
  assert.strictEqual(v.senderOpened, '0');
  assert.ok(Number(v.scanned) >= 2, 'the scan covered another Payment event: refresh the block with make primer-refresh');
  const { doc } = open();
  const body = text(section(doc, 'audit'));
  for (const s of ['sk_spend', 'cannot spend', 'nullifier', 'never published']) assert.ok(body.includes(s), `the audit names ${s}`);
});

test('the last step\'s ledger shows who spent, but holds no amount besides the shield\'s and the withdrawal\'s, and never names Ana on the payment', () => {
  const v = DATA.steps.verdict.values;
  const { payment: pay, shield, withdrawal: exit, audit, keys, registration: reg } = DATA.steps;
  assert.deepStrictEqual(DATA.steps.verdict.ledger, [], 'the verdict adds nothing: it reads the ledger');
  const { doc } = open();
  const names = steps(doc);
  assert.strictEqual(names[names.length - 1], 'verdict', 'the verdict is the last step');
  const all = names.flatMap(n => DATA.steps[n].ledger);
  // Every amount on the ledger: the shield's, and the withdrawal's in its
  // calldata and its event.
  assert.deepStrictEqual(all.filter(i => /amount|value/i.test(i.field)).map(i => [i.tx, i.field, i.value]), [
    ['depositV2 · Sender', 'amount', shield.values.amount],
    ['partialWithdraw · Ana', 'amount', exit.values.amount],
    ['PartialWithdrawal event', 'amount', exit.values.amount],
  ], 'the only amounts on the ledger are the public two');
  // No private amount sits under another field's name, in decimal or as a
  // 32-byte word, and no note's opening does.
  const values = new Set(all.map(i => i.value));
  const word = x => BigInt(x).toString(16).padStart(64, '0');
  for (const [what, x] of Object.entries({ 'Ana\'s note': pay.values.valueOut0, 'the Sender\'s change': pay.values.valueOut1, 'Ana\'s change': exit.values.change, 'the audited change': audit.values.changeAmount })) {
    assert.ok(!values.has(x), `${what} (${x}) is not on the ledger`);
    assert.ok(![...values].some(l => l.includes(word(x))), `${what} is not on the ledger as a 32-byte word`);
  }
  for (const x of [pay.values.saltOut0, pay.values.saltOut1, exit.values.saltIn, exit.values.saltChange, DATA.steps.discovery.values.plaintext]) {
    assert.ok(![...values].some(l => l.includes(x)), `the opening ${x.slice(0, 20)}… is not on the ledger`);
  }
  // Who spent: the payment and the withdrawal each name their msg.sender.
  assert.strictEqual(v.payer, pay.values.msgSender);
  assert.strictEqual(v.payer, reg.values['sender.address'], 'the payer is the Sender');
  assert.strictEqual(v.withdrawer, exit.values.msgSender);
  assert.strictEqual(v.withdrawer, reg.values['ana.address'], 'the withdrawer is Ana');
  assert.strictEqual(v.recipient, exit.values.recipient);
  // To whom: nothing the payment published names Ana.
  const payment = pay.ledger.map(i => i.value);
  for (const x of [reg.values['ana.address'], keys.values['ana.pkSpend'], keys.values['ana.viewEk']]) {
    assert.ok(!payment.some(l => l.includes(x)), `the payment does not name Ana (${x.slice(0, 20)}…)`);
  }
  // The public amounts the page reads off, and the pool left between them.
  assert.strictEqual(v.shieldAmount, shield.values.amount);
  assert.strictEqual(v.withdrawAmount, exit.values.amount);
  assert.strictEqual(BigInt(v.poolLeft), BigInt(v.shieldAmount) - BigInt(v.withdrawAmount));
  // The nullifiers and the note commitments the page points at are the ledger's.
  for (const k of ['payNullifier', 'withdrawNullifier', 'commitmentOut0', 'commitmentOut1']) assert.ok(values.has(v[k]), `${k} is on the ledger`);
  // The last step's page shows that ledger, whole.
  for (let i = 1; i < names.length; i++) doc.getElementById('next').click();
  assert.strictEqual(doc.querySelectorAll('#ledger li').length, all.length, 'the last step lists every public item');
  assert.strictEqual([...doc.querySelectorAll('#ledger li .lf')].filter(f => f.textContent === 'amount').length, 3, 'three amount rows');
  const body = text(section(doc, 'verdict'));
  for (const s of ['who spent', 'to whom', 'how much', 'relayer', 'msg.sender', 'USDr', 'tags']) assert.ok(body.includes(s), `the verdict names ${s}`);
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

test('the primer\'s hand check names the page\'s steps', () => {
  const { doc } = open();
  const hand = fs.readFileSync(path.join(DEMO, 'HAND_CHECK.md'), 'utf8').split('\n## Protocol primer')[1]?.split('\n## ')[0];
  assert.ok(hand, 'HAND_CHECK.md has a Protocol primer section');
  const names = steps(doc);
  assert.ok(hand.includes(`(N = ${names.length})`), `its step count is N = ${names.length}`);
  names.forEach((step, i) => {
    const title = section(doc, step).querySelector('h2').lastChild.textContent.trim();
    assert.ok(hand.includes(`${i + 1} ${title}`), `its list of steps has ${i + 1} ${title}`);
    // Keys is the page's opening step, with no hash.
    if (i >= 1) assert.ok(hand.includes(`/protocol#${step}`), `it opens /protocol#${step}`);
  });
});

test('the primer\'s hand check counts each step\'s amber asides', () => {
  const { doc } = open();
  const hand = fs.readFileSync(path.join(DEMO, 'HAND_CHECK.md'), 'utf8').split('\n## Protocol primer')[1]?.split('\n## ')[0];
  const words = { two: 2, three: 3, four: 4 };
  const claims = [...hand.matchAll(/[Ss]tep (\d+)'s (?:(two|three|four) )?amber[^,.]*?asides?\b/g)];
  assert.ok(claims.length > 0, 'the section names the amber asides it checks');
  const names = steps(doc);
  claims.forEach(([claim, n, word]) => {
    const onPage = section(doc, names[n - 1]).querySelectorAll('aside.warn').length;
    assert.strictEqual(onPage, word ? words[word] : 1, `"${claim}": step ${n} has ${onPage} on the page`);
  });
});

test('the demo\'s header links to the primer', () => {
  const demo = new JSDOM(fs.readFileSync(path.join(DEMO, 'index.html'), 'utf8')).window.document;
  assert.ok(demo.querySelector('header a[href="/protocol"]'), 'the demo header has a link to /protocol');
});
