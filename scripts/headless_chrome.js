// headless_chrome.js starts a headless Chrome with a fresh profile, opens a
// DevTools protocol connection to its page, runs fn with it, and always
// stops Chrome and removes the profile. The page checks that need a real
// browser's layout (primer_from_disk.js, moments_fit.js) run on it.
//
//   const withChrome = require('./headless_chrome');
//   await withChrome(async ({ send, evaluate, events, sleep }) => { … });
//
// send(method, params) sends a DevTools command and resolves to its reply;
// evaluate(expr) evaluates expr in the page, awaiting a promise, and returns
// its value (undefined if it threw); events holds every DevTools event
// received so far.
// CHROME names the Chrome binary (default: macOS's Google Chrome).

const { spawn } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const WebSocket = require('ws');

const CHROME = process.env.CHROME || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const sleep = ms => new Promise(r => setTimeout(r, ms));

// withChrome runs fn against a fresh headless Chrome. Chrome picks its own
// DevTools port and writes it to the profile's DevToolsActivePort, so two
// checks at once don't share one.
async function withChrome(fn) {
  const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'page-check-chrome-'));
  const chrome = spawn(CHROME, ['--headless=new', '--remote-debugging-port=0', `--user-data-dir=${profile}`,
    '--window-size=1200,800', '--no-first-run', '--no-default-browser-check', 'about:blank'], { stdio: 'ignore' });
  try {
    let page;
    for (let i = 0; i < 50 && !page; i++) {
      try {
        const port = fs.readFileSync(path.join(profile, 'DevToolsActivePort'), 'utf8').split('\n')[0];
        page = (await (await fetch(`http://127.0.0.1:${port}/json/list`)).json()).find(t => t.type === 'page');
      } catch (_) {}
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
    const evaluate = async expr => (await send('Runtime.evaluate', { expression: expr, returnByValue: true, awaitPromise: true })).result?.result?.value;
    try { return await fn({ send, evaluate, events, sleep }); } finally { ws.close(); }
  } finally {
    chrome.kill();
    await sleep(500);
    fs.rmSync(profile, { recursive: true, force: true });
  }
}

module.exports = withChrome;
