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
const DEFAULT_CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const sleep = ms => new Promise(r => setTimeout(r, ms));

// Chrome receives DevTools messages on fd 3 and sends replies on fd 4.
// Each message is JSON terminated by a NUL byte.
async function withChrome(fn) {
  const executable = process.env.CHROME || DEFAULT_CHROME;
  const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'page-check-chrome-'));
  const chrome = spawn(executable, ['--headless=new', '--remote-debugging-pipe', `--user-data-dir=${profile}`,
    '--window-size=1200,800', '--no-first-run', '--no-default-browser-check'],
  { stdio: ['ignore', 'ignore', 'ignore', 'pipe', 'pipe'] });
  const [, , , input, output] = chrome.stdio;
  const closed = new Promise(resolve => chrome.once('close', resolve));
  const pending = new Map();
  const events = [];
  let id = 0;
  let sessionId;
  let failure;
  let remaining = '';

  function fail(err) {
    if (failure) return;
    failure = err;
    for (const { reject, timer } of pending.values()) {
      clearTimeout(timer);
      reject(err);
    }
    pending.clear();
  }
  chrome.once('error', fail);
  chrome.once('close', (code, signal) => fail(new Error(`Chrome exited (${signal || code})`)));
  input.on('error', fail);
  output.on('error', fail);
  output.on('end', () => fail(new Error('Chrome closed the DevTools pipe')));
  output.setEncoding('utf8');
  output.on('data', chunk => {
    const frames = (remaining + chunk).split('\0');
    remaining = frames.pop();
    for (const frame of frames) {
      if (!frame) continue;
      let message;
      try { message = JSON.parse(frame); } catch (err) { fail(err); return; }
      const command = pending.get(message.id);
      if (command) {
        pending.delete(message.id);
        clearTimeout(command.timer);
        command.resolve(message);
      } else if (sessionId && message.sessionId === sessionId && message.method) {
        events.push(message);
      }
    }
  });

  function command(method, params, targetSession) {
    if (failure) return Promise.reject(failure);
    return new Promise((resolve, reject) => {
      const commandId = ++id;
      const timer = setTimeout(() => {
        pending.delete(commandId);
        reject(new Error(`Chrome did not answer ${method} (CHROME=${executable})`));
      }, 30000);
      pending.set(commandId, { resolve, reject, timer });
      input.write(JSON.stringify({ id: commandId, method, params, sessionId: targetSession }) + '\0');
    });
  }

  try {
    const page = await command('Target.createTarget', { url: 'about:blank' });
    if (!page.result?.targetId) throw new Error(`Chrome could not create a page: ${page.error?.message}`);
    const attached = await command('Target.attachToTarget', { targetId: page.result.targetId, flatten: true });
    sessionId = attached.result?.sessionId;
    if (!sessionId) throw new Error(`Chrome could not attach to the page: ${attached.error?.message}`);
    const send = (method, params = {}) => command(method, params, sessionId);
    const evaluate = async expr => (await send('Runtime.evaluate', { expression: expr, returnByValue: true, awaitPromise: true })).result?.result?.value;
    return await fn({ send, evaluate, events, sleep });
  } finally {
    chrome.kill();
    input.destroy();
    output.destroy();
    const timeout = setTimeout(() => chrome.kill('SIGKILL'), 2000);
    try { await closed; } finally {
      clearTimeout(timeout);
      fs.rmSync(profile, { recursive: true, force: true });
    }
  }
}

module.exports = withChrome;
