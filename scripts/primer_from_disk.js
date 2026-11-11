// primer_from_disk.js runs the Protocol primer's "From disk" hand check
// (enygma_retail_payments/shield_demo/HAND_CHECK.md) in headless Chrome,
// which Claude in Chrome cannot do: it cannot open file:// URLs.
//
// It opens protocol.html as a file:// URL, steps it with Next to the last
// step and Back to the first, and prints as JSON: the demo link, the page's
// scroll and width, each step's state (step, chips shown, ledger items, Back
// and Next), every network request, console message and exception. It saves a
// screenshot of each step in the output directory (made if missing; default:
// the system's temporary directory). Read the screenshots against the served page.
//
// At 1200 × 785, with the last step scrolled into view as Next leaves it, it
// exits 1 when the page or a diagram's panel scrolls sideways, the stage is
// clipped or ends past the window, the ledger is left under MIN_LEDGER_PX, or a
// formula's value runs past its grey box with no value over MAX_SCROLLING_VALUE
// characters in its row to excuse it: a 77-digit value may scroll, an account may not.
//
// Then, at a 390 px phone width, it measures each diagram's smallest rendered
// label (an SVG <text>'s font size times its <svg>'s scale) and exits 1 when
// one is under MIN_LABEL_PX: a diagram scaled down until it can't be read.
// With every step's chips drawn, it also exits 1 on a layout fault: two chips
// overlapping, a chip outside every wallet and contract box, a label past its
// chip or box, a MathML table cell drawn over the next one, or a table's
// first column not sitting right.
//
//   (cd scripts && npm ci)   # once: ws comes with jsdom, make page-check's dependency
//   node scripts/primer_from_disk.js [outdir]
//
// CHROME names the Chrome binary (default: macOS's Google Chrome; see headless_chrome.js).

const fs = require('fs');
const os = require('os');
const path = require('path');
const withChrome = require('./headless_chrome');

const PRIMER = path.join(__dirname, '..', 'enygma_retail_payments', 'shield_demo', 'protocol.html');
const OUT = process.argv[2] || os.tmpdir();
fs.mkdirSync(OUT, { recursive: true });
// MIN_LABEL_PX is the smallest label, in CSS px, a diagram may render at 390 px wide.
const MIN_LABEL_PX = 7;
// MIN_LEDGER_PX is the least height, at 1200 × 785, the diagram and stepper may leave the ledger.
const MIN_LEDGER_PX = 120;
// MAX_SCROLLING_VALUE is the longest value, in characters, that must still fit
// its formula's box (an account is 42); a longer one excuses its row.
const MAX_SCROLLING_VALUE = 44;

// LABELS is, per <svg> with an id, its smallest drawn label's rendered size
// in px, and that label's text.
const LABELS = `(() => Object.fromEntries([...document.querySelectorAll('svg[id]')].map(svg => {
  const scale = svg.getBoundingClientRect().width / svg.viewBox.baseVal.width;
  let min = { px: Infinity, text: '' };
  for (const t of svg.querySelectorAll('text')) {
    if (getComputedStyle(t).display === 'none' || t.closest('[style*="display: none"], .later')) continue;
    const px = parseFloat(getComputedStyle(t).fontSize) * scale;
    if (px < min.px) min = { px: Math.round(px * 10) / 10, text: t.textContent.slice(0, 30) };
  }
  return [svg.id, min];
})))()`;

// LAYOUT lists the page's layout faults, the chips measured in the SVG's own
// units so the animation and the width don't move them.
const LAYOUT = `(() => {
  const faults = [];
  const chips = [...document.querySelectorAll('#flow .chip')].map(c => {
    const m = c.transform.baseVal[0].matrix, b = c.querySelector('rect').getBBox();
    return { name: c.textContent + ' (' + c.dataset.step + ')', left: m.e, top: m.f, right: m.e + b.width, bottom: m.f + b.height };
  });
  const nodes = [...document.querySelectorAll('#flow .node rect')].map(n => n.getBBox());
  chips.forEach((chip, i) => {
    if (!nodes.some(n => chip.left >= n.x && chip.top >= n.y && chip.right <= n.x + n.width && chip.bottom <= n.y + n.height)) faults.push('chip ' + chip.name + ' is outside every box');
    for (const other of chips.slice(i + 1)) if (chip.left < other.right && other.left < chip.right && chip.top < other.bottom && other.top < chip.bottom) faults.push('chips ' + chip.name + ' and ' + other.name + ' overlap');
  });
  for (const g of document.querySelectorAll('#flow .chip, #flow .node, #circuit .witness, #circuit .gate, #circuit .pubin, #circuit .verifier')) {
    const box = g.querySelector('rect').getBBox();
    for (const t of g.querySelectorAll('text')) {
      const b = t.getBBox();
      if (b.x < box.x || b.x + b.width > box.x + box.width || b.y < box.y || b.y + b.height > box.y + box.height) faults.push('label ' + t.textContent.slice(0, 30) + ' is past its box');
    }
  }
  for (const tr of document.querySelectorAll('section.step mtr')) {
    [...tr.children].forEach((td, i) => {
      const next = tr.children[i + 1];
      if (!next) return;
      const right = Math.max(0, ...[...td.querySelectorAll('*')].map(e => e.getBoundingClientRect().right));
      if (right > next.getBoundingClientRect().left + 1) faults.push(tr.closest('section').id + ': a table cell is over the next (' + tr.textContent.slice(0, 40) + ')');
    });
  }
  for (const tr of document.querySelectorAll('mtable.align > mtr')) {
    const td = tr.children[0];
    if (tr.children.length < 2 || !td.childElementCount) continue;
    const contentRight = Math.max(...[...td.children].map(e => e.getBoundingClientRect().right));
    if (Math.abs(td.getBoundingClientRect().right - contentRight) > 1.5) faults.push(tr.closest('section').id + ': a first column does not sit right (' + tr.textContent.slice(0, 40) + ')');
  }
  return faults;
})()`;

// DESKTOP lists what is wrong with the stage at a desktop width, the current
// section scrolled to where Next leaves it. (At scrollY 0, as a deep link
// opens, the header sits above the stage and a full one ends past the window.)
const DESKTOP = `(() => {
  document.querySelector('section.step.current').scrollIntoView({ behavior: 'instant', block: 'start' });
  const faults = [], stage = document.querySelector('.stage'), ledger = stage.querySelector('.ledger');
  if (stage.getBoundingClientRect().bottom > innerHeight) faults.push('the stage ends at ' + Math.round(stage.getBoundingClientRect().bottom) + ' px, past the window');
  if (document.documentElement.scrollWidth > innerWidth) faults.push('the page scrolls sideways');
  if (stage.scrollHeight > stage.clientHeight) faults.push('the stage is clipped: ' + stage.scrollHeight + ' px in ' + stage.clientHeight);
  if (ledger.getBoundingClientRect().height < ${MIN_LEDGER_PX}) faults.push('the ledger is ' + Math.round(ledger.getBoundingClientRect().height) + ' px tall');
  for (const id of ['flow', 'circuit']) {
    const panel = document.getElementById(id).parentElement;
    if (panel.scrollWidth > panel.clientWidth) faults.push('the panel of #' + id + ' scrolls sideways');
  }
  for (const box of document.querySelectorAll('section.step .fm')) {
    const edge = box.getBoundingClientRect().right;
    for (const value of box.querySelectorAll('[data-v]')) {
      if (value.getBoundingClientRect().right <= edge + 1) continue;
      const row = value.closest('mtr') || box;
      if (![...row.querySelectorAll('[data-v]')].some(v => v.textContent.length > ${MAX_SCROLLING_VALUE})) faults.push(box.closest('section').id + ': the value ' + value.getAttribute('data-v') + ' is cut off at its box');
    }
  }
  return faults;
})()`;

// STATE is what the stepper shows, read in the page.
const STATE = `(() => ({
  step: document.querySelector('section.step.current')?.dataset.step,
  diagram: document.getElementById('flow').dataset.step,
  count: document.getElementById('stepCount').textContent.trim(),
  chips: [...document.querySelectorAll('#flow .chip')].filter(c => getComputedStyle(c).display !== 'none').length,
  ledger: document.querySelectorAll('#ledger li').length,
  back: !document.getElementById('back').disabled,
  next: !document.getElementById('next').disabled,
}))()`;

withChrome(async ({ send, evaluate, events, sleep }) => {
  const shot = async name => {
    const r = await send('Page.captureScreenshot', { format: 'png' });
    const f = path.join(OUT, name);
    fs.writeFileSync(f, Buffer.from(r.result.data, 'base64'));
    return f;
  };
  const click = async button => { await evaluate(`document.getElementById('${button}').click()`); await sleep(1500); };

  for (const domain of ['Page', 'Runtime', 'Log', 'Network']) await send(`${domain}.enable`);
  await send('Page.navigate', { url: 'file://' + PRIMER });
  for (let i = 0; i < 50 && !events.some(e => e.method === 'Page.loadEventFired'); i++) await sleep(100);
  await sleep(1500);

  const result = {
    url: await evaluate('location.href'),
    demoLink: await evaluate("document.getElementById('demoLink').href"),
    scroll: await evaluate('({ y: scrollY, scrollWidth: document.documentElement.scrollWidth, innerWidth })'),
    steps: [],
    screenshots: [],
  };
  const record = async after => {
    const s = await evaluate(STATE);
    result.steps.push({ after, ...s });
    result.screenshots.push(await shot(`primer-from-disk-${result.steps.length}-${s.step}.png`));
    return s;
  };
  let s = await record('open');
  while (s.next) { await click('next'); s = await record('next'); }
  while (s.back) { await click('back'); s = await record('back'); }

  // A desktop width, on the last step: the ledger at its longest.
  await send('Emulation.setDeviceMetricsOverride', { width: 1200, height: 785, deviceScaleFactor: 1, mobile: false });
  await evaluate('showStep(STEPS.length - 1, false)');
  await sleep(800);
  result.desktop = await evaluate(DESKTOP);

  // The phone width: the last step drawn, so every diagram has its labels.
  await send('Emulation.setDeviceMetricsOverride', { width: 390, height: 844, deviceScaleFactor: 1, mobile: true });
  await evaluate("location.hash = 'payment'; showStep(STEPS.length - 1, false)");
  await sleep(800);
  result.phone = { scrollWidth: await evaluate('document.documentElement.scrollWidth'), innerWidth: await evaluate('innerWidth'), labels: await evaluate(LABELS) };
  result.screenshots.push(await shot('primer-from-disk-phone.png'));
  result.tooSmall = Object.entries(result.phone.labels).filter(([, m]) => m.px < MIN_LABEL_PX).map(([svg, m]) => `${svg}: ${m.px} px ("${m.text}")`);
  result.layout = await evaluate(LAYOUT);

  result.network = events.filter(e => e.method === 'Network.requestWillBeSent').map(e => e.params.request.url.slice(0, 120));
  result.console = events.filter(e => e.method === 'Runtime.consoleAPICalled')
    .map(e => `${e.params.type}: ${e.params.args.map(a => a.value ?? a.description).join(' ')}`);
  result.exceptions = events.filter(e => e.method === 'Runtime.exceptionThrown')
    .map(e => e.params.exceptionDetails.exception?.description || e.params.exceptionDetails.text);
  result.logErrors = events.filter(e => e.method === 'Log.entryAdded' && e.params.entry.level === 'error').map(e => e.params.entry.text);
  console.log(JSON.stringify(result, null, 1));
  if (result.tooSmall.length) {
    console.error(`labels under ${MIN_LABEL_PX} px at 390 px wide: ${result.tooSmall.join('; ')}`);
    process.exitCode = 1;
  }
  if (result.desktop.length) {
    console.error(`at 1200 × 785: ${result.desktop.join('; ')}`);
    process.exitCode = 1;
  }
  if (result.layout.length) {
    console.error(`layout faults: ${result.layout.join('; ')}`);
    process.exitCode = 1;
  }
}).catch(e => { console.error(e); process.exit(1); });
