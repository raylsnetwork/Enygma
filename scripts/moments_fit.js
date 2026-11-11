// moments_fit.js checks, in headless Chrome, that every concept moment of
// the shield demo (enygma_retail_payments/shield_demo/index.html) fits the
// story column above the drawer: what jsdom, with no layout, cannot see.
//
// It opens index.html from disk with no stack (/defaults answered from the
// default run fixture, /events stubbed), at the viewport a 1200 × 785
// browser window gives, VIEWPORT. For each run a page can play (a full run
// and a walkthrough, each with the USDr fee and tags on or off) it feeds the
// page the fixture's reset and proto events, its step list cut as those
// options cut it, then each step's start, and measures each moment as the
// page shows it, every beat lit. It exits 1 when a moment's NEXT ends past
// the drawer's top or the card scrolls sideways, or a run shows a different
// number of moments than the narration block gives its steps, and prints
// each moment's height against the room the story column has. No hops are
// fed, so Hidden on chain's commitment and leaf show as … (a short hex and
// the same 24 leaves as a real run's).
//
//   (cd scripts && npm ci)   # once: ws comes with jsdom, make page-check's dependency
//   node scripts/moments_fit.js   (make moments-fit)

const fs = require('fs');
const path = require('path');
const withChrome = require('./headless_chrome');

const DEMO = path.join(__dirname, '..', 'enygma_retail_payments', 'shield_demo');
const fixture = name => JSON.parse(fs.readFileSync(path.join(DEMO, 'testdata', name), 'utf8'));
// VIEWPORT is what a 1200 × 785 Chrome or Brave window leaves the page: HAND_CHECK's layout size.
const VIEWPORT = { width: 1159, height: 688 };

// cut is fixture f's steps as a run with opts announces them, as
// engine/plan.go cuts them: the Tag channels step only with tags (a
// walkthrough's fixture has none to cut), USDr fee credit only with the
// fee in a walkthrough (a full run always credits it).
function cut(f, opts) {
  const all = JSON.parse(f.events.find(e => e.kind === 'proto' && e.field === 'steps').value);
  return all.filter(s => (s.id !== 'channels' || opts.tags) && (s.id !== 'usdr' || opts.usdrFee || !opts.walkthrough));
}

// RUNS are the runs a page plays moments in, each with its options and steps.
const RUNS = [];
for (const [name, f] of [['full run', fixture('default_run.events.json')], ['walkthrough', fixture('walkthrough.events.json')]]) {
  for (const usdrFee of [true, false]) for (const tags of [true, false]) {
    const base = f.run || f.defaults;
    const opts = { shieldTotal: base.shieldTotal, payouts: base.payouts, script: f.run ? [] : base.script, usdrFee, tags, walkthrough: !!f.run };
    const protos = f.events.filter(e => e.kind === 'proto').map(e => e.field === 'steps' ? { ...e, value: JSON.stringify(cut(f, opts)) } : e);
    RUNS.push({ name: `${name}, fee ${usdrFee ? 'on' : 'off'}, tags ${tags ? 'on' : 'off'}`, opts, protos, steps: cut(f, opts).map(s => s.id) });
  }
}

// PLAY runs one run in the page and returns each moment it showed, measured:
// it resets the page with the moments on, sets the run's options, feeds the
// protos, then starts each step and, while a moment shows, measures it and
// presses its NEXT. Pausing is on, so a moment holds until then.
const PLAY = run => `(async () => {
  const $ = id => document.getElementById(id), sleep = ms => new Promise(r => setTimeout(r, ms));
  const run = ${JSON.stringify(run)};
  $('optMoments').checked = true;
  onEvent({ kind: 'reset' });
  runOpts = run.opts;
  for (const e of run.protos) onEvent(e);
  const out = [], story = $('moment').parentElement;
  const want = run.steps.flatMap(s => NARRATION.moments[s] || []).length;
  for (const step of run.steps) {
    onEvent({ kind: 'step_started', step, label: step, msg: '' });
    for (;;) {
      let waited = 0;
      while ($('moment').hidden && waited < 400) { await sleep(20); waited += 20; }
      if ($('moment').hidden) break;
      $('moment').querySelectorAll('.beat').forEach(b => b.classList.add('on'));
      const m = $('moment'), next = $('momentNext').getBoundingClientRect(), drawer = $('drawer').getBoundingClientRect().top;
      out.push({ step, kicker: m.querySelector('.kicker').textContent, top: Math.round(m.getBoundingClientRect().top),
        height: Math.round(m.getBoundingClientRect().height), next: Math.round(next.bottom), drawer: Math.round(drawer),
        room: Math.round(drawer - m.getBoundingClientRect().top), sideways: m.scrollWidth > m.clientWidth || story.scrollWidth > story.clientWidth });
      $('momentNext').click();
      await sleep(50);
    }
  }
  return { want, out };
})()`;

withChrome(async ({ send, evaluate, events, sleep }) => {
  for (const domain of ['Page', 'Runtime']) await send(`${domain}.enable`);
  await send('Emulation.setDeviceMetricsOverride', { ...VIEWPORT, deviceScaleFactor: 1, mobile: false });
  const defaults = fixture('default_run.events.json').defaults;
  await send('Page.addScriptToEvaluateOnNewDocument', { source: `
    localStorage.setItem('enygmaRetailShieldPause', 'on');
    window.EventSource = class { close() {} };
    window.fetch = async url => {
      if (url !== '/defaults') throw new Error('moments_fit: unexpected request to ' + url);
      return { ok: true, status: 200, json: async () => (${JSON.stringify(defaults)}) };
    };` });
  await send('Page.navigate', { url: 'file://' + path.join(DEMO, 'index.html') });
  for (let i = 0; i < 50 && !events.some(e => e.method === 'Page.loadEventFired'); i++) await sleep(100);
  await sleep(500);

  const faults = [];
  for (const run of RUNS) {
    const { want, out: shown } = await evaluate(PLAY(run)) || {};
    if (!shown) faults.push(`${run.name}: the page threw before showing a moment`);
    else if (shown.length !== want) faults.push(`${run.name}: ${shown.length} moments showed, want ${want}`);
    console.log(`${run.name}:`);
    for (const m of shown || []) {
      const fits = m.next <= m.drawer && !m.sideways;
      console.log(`  ${fits ? 'ok  ' : 'FAIL'} ${m.kicker.padEnd(40)} ${String(m.height).padStart(4)} px in ${m.room} px, NEXT ends at ${m.next}, drawer at ${m.drawer}${m.sideways ? ', scrolls sideways' : ''}`);
      if (!fits) faults.push(`${run.name}: ${m.kicker} ${m.sideways ? 'scrolls sideways' : `NEXT ends at ${m.next} px, past the drawer at ${m.drawer}`}`);
    }
  }
  const thrown = events.filter(e => e.method === 'Runtime.exceptionThrown').map(e => e.params.exceptionDetails.exception?.description || e.params.exceptionDetails.text);
  for (const t of thrown) faults.push(`the page threw: ${t.split('\n')[0]}`);
  if (faults.length) {
    console.error(`\nat ${VIEWPORT.width} × ${VIEWPORT.height}, ${faults.length} moment fault(s):\n  ${faults.join('\n  ')}`);
    process.exitCode = 1;
  }
}).catch(e => { console.error(e); process.exit(1); });
