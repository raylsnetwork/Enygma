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
//   (cd scripts && npm ci)   # once: ws comes with jsdom, make page-check's dependency
//   node scripts/primer_from_disk.js [outdir]
//
// CHROME names the Chrome binary (default: macOS's Google Chrome).

const { spawn } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const WebSocket = require('ws');

const CHROME = process.env.CHROME || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const PRIMER = path.join(__dirname, '..', 'enygma_retail_payments', 'shield_demo', 'protocol.html');
const OUT = process.argv[2] || os.tmpdir();
fs.mkdirSync(OUT, { recursive: true });
const PORT = 9333;
const sleep = ms => new Promise(r => setTimeout(r, ms));

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

(async () => {
  const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'primer-chrome-'));
  const chrome = spawn(CHROME, ['--headless=new', `--remote-debugging-port=${PORT}`, `--user-data-dir=${profile}`,
    '--window-size=1200,800', '--no-first-run', '--no-default-browser-check', 'about:blank'], { stdio: 'ignore' });
  try {
    let page;
    for (let i = 0; i < 50 && !page; i++) {
      try { page = (await (await fetch(`http://127.0.0.1:${PORT}/json/list`)).json()).find(t => t.type === 'page'); } catch (_) {}
      if (!page) await sleep(200);
    }
    if (!page) throw new Error(`Chrome did not start (CHROME=${CHROME})`);
    const ws = new WebSocket(page.webSocketDebuggerUrl);
    await new Promise((ok, fail) => { ws.on('open', ok); ws.on('error', fail); });
    let id = 0;
    const pending = new Map(), events = [];
    ws.on('message', m => {
      const d = JSON.parse(m);
      if (d.id && pending.has(d.id)) { pending.get(d.id)(d); pending.delete(d.id); } else events.push(d);
    });
    const send = (method, params = {}) => new Promise(r => { pending.set(++id, r); ws.send(JSON.stringify({ id, method, params })); });
    const evaluate = async expr => (await send('Runtime.evaluate', { expression: expr, returnByValue: true })).result?.result?.value;
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

    result.network = events.filter(e => e.method === 'Network.requestWillBeSent').map(e => e.params.request.url.slice(0, 120));
    result.console = events.filter(e => e.method === 'Runtime.consoleAPICalled')
      .map(e => `${e.params.type}: ${e.params.args.map(a => a.value ?? a.description).join(' ')}`);
    result.exceptions = events.filter(e => e.method === 'Runtime.exceptionThrown')
      .map(e => e.params.exceptionDetails.exception?.description || e.params.exceptionDetails.text);
    result.logErrors = events.filter(e => e.method === 'Log.entryAdded' && e.params.entry.level === 'error').map(e => e.params.entry.text);
    console.log(JSON.stringify(result, null, 1));
    ws.close();
  } finally {
    chrome.kill();
    await sleep(500);
    fs.rmSync(profile, { recursive: true, force: true });
  }
})().catch(e => { console.error(e); process.exit(1); });
