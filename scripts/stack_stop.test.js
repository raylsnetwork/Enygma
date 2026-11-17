// scripts/stack_stop.sh, the stop behind make's *-stack-stop, on a small
// stack script of fake services built on scripts/stack_lib.sh, started the
// way make's *-stack targets start theirs.
//
//   node --test scripts/stack_stop.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const net = require('net');
const os = require('os');
const path = require('path');
const { spawn } = require('child_process');

const STOP = path.join(__dirname, 'stack_stop.sh');
const LIB = path.join(__dirname, 'stack_lib.sh');

// freePort asks the OS for a port nothing listens on.
function freePort() {
  return new Promise((resolve) => {
    const s = net.createServer().listen(0, '127.0.0.1', () => {
      const { port } = s.address();
      s.close(() => resolve(port));
    });
  });
}

// listening says whether something accepts connections on port.
function listening(port) {
  return new Promise((resolve) => {
    const c = net.connect(port, '127.0.0.1');
    c.on('connect', () => { c.destroy(); resolve(true); });
    c.on('error', () => resolve(false));
  });
}

// alive says whether process pid still runs.
function alive(pid) {
  try { process.kill(pid, 0); return true; } catch { return false; }
}

// fakeStack starts a stack script (fake_stack.sh, sourcing the lib, extra
// lines first) with one web service on port, detached with its output in
// stack.log, as make's *-stack targets do, and records its pid and log dir
// in pid file and pid file.logs. It resolves once the stack is up.
async function fakeStack(port, extra = '') {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'stack-stop-test-'));
  const script = path.join(dir, 'fake_stack.sh');
  fs.writeFileSync(script, `set -euo pipefail\n. ${JSON.stringify(LIB)}\nuse_log_dir stack_stop_test\n${extra}\n` +
    `start web . python3 -m http.server ${port} --bind 127.0.0.1\nwait_port web ${port}\nsay "stack is up"\nwatch_services\n`);
  const log = fs.openSync(path.join(dir, 'stack.log'), 'w');
  const child = spawn('bash', [script], { cwd: dir, env: { ...process.env, LOG_DIR: dir }, detached: true, stdio: ['ignore', log, log] });
  fs.closeSync(log);
  child.unref();
  const pidFile = path.join(dir, 'stack.pid');
  fs.writeFileSync(pidFile, `${child.pid}\n`);
  fs.writeFileSync(`${pidFile}.logs`, `${dir}\n`);
  for (let i = 0; i < 300 && !fs.readFileSync(path.join(dir, 'stack.log'), 'utf8').includes('stack is up'); i++) {
    await new Promise((r) => setTimeout(r, 100));
  }
  return { dir, pid: child.pid, pidFile };
}

// stop runs stack_stop.sh on the fake stack, resolving with its exit status
// and output. Not spawnSync: node is the stack script's parent here, and a
// blocked event loop never reaps it, so it would wait as a zombie.
function stop(s, port, match = 'fake_stack.sh') {
  const child = spawn('bash', [STOP, s.pidFile, match, 'stack.log', 'fake-stack', String(port)]);
  let out = '';
  child.stdout.on('data', (d) => { out += d; });
  child.stderr.on('data', (d) => { out += d; });
  return new Promise((resolve) => child.on('close', (status) => resolve({ status, out })));
}

// done kills what a test left running and removes its dir.
function done(s) {
  try { process.kill(-s.pid, 'SIGKILL'); } catch { /* already gone */ }
  fs.rmSync(s.dir, { recursive: true, force: true });
}

test('a clean stop frees the ports and removes the pid files', { timeout: 60000 }, async () => {
  const port = await freePort();
  const s = await fakeStack(port);
  try {
    const r = await stop(s, port);
    assert.strictEqual(r.status, 0, r.out);
    assert.match(r.out, /stack stopped/);
    assert.strictEqual(await listening(port), false, 'the service still listens');
    assert.strictEqual(alive(s.pid), false, 'the stack script still runs');
    assert.strictEqual(fs.existsSync(s.pidFile), false, 'the pid file is left');
    assert.strictEqual(fs.existsSync(`${s.pidFile}.logs`), false, 'the logs file is left');
  } finally { done(s); }
});

// Before #301 a signal's trap returned, putting the stack script back in
// watch_services to find the services it had just stopped and report them
// dead: a stop then passed while the stack's log said a service died. Here
// the report comes a second after the ports free, so a stop that read the
// log before the stack script exited would miss it.
test('a stop fails when the stack reports a service dead after the signal', { timeout: 60000 }, async () => {
  const port = await freePort();
  const s = await fakeStack(port, "trap 'cleanup; sleep 1' INT TERM");
  try {
    const r = await stop(s, port);
    assert.strictEqual(r.status, 1, r.out);
    assert.match(r.out, /web exited — last log lines:/);
    assert.strictEqual(await listening(port), false, 'the service still listens');
  } finally { done(s); }
});

// A stale pid file may name a pid since reused by another process.
test('a pid file whose process is not the stack is refused, and nothing is signalled', { timeout: 60000 }, async () => {
  const port = await freePort();
  const s = await fakeStack(port);
  try {
    const r = await stop(s, port, 'some_other_stack.sh');
    assert.strictEqual(r.status, 1, r.out);
    assert.match(r.out, /no stack started by make fake-stack/);
    assert.strictEqual(alive(s.pid), true, 'the stack was signalled');
    assert.strictEqual(await listening(port), true, 'the service was stopped');
  } finally { done(s); }
});

// Without the stack's log there is nothing to show it stopped cleanly.
test('a stop with no stack log to read stops the stack but fails', { timeout: 60000 }, async () => {
  const port = await freePort();
  const s = await fakeStack(port);
  fs.rmSync(`${s.pidFile}.logs`);
  try {
    const r = await stop(s, port);
    assert.strictEqual(r.status, 1, r.out);
    assert.match(r.out, /no stack log/);
    assert.strictEqual(await listening(port), false, 'the service still listens');
  } finally { done(s); }
});
