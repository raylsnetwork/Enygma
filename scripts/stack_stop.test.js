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

// fakeStack starts a stack script (at script, relative to a fresh dir it
// runs in, sourcing the lib, extra lines first) with one web service on
// port, as `bash <script> <args>`, detached with its output in stack.log, as make's
// *-stack targets do, and records its pid and log dir in pid file and pid
// file.logs. It resolves once the stack is up.
async function fakeStack(port, extra = '', script = 'fake_stack.sh', args = []) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'stack-stop-test-'));
  fs.mkdirSync(path.dirname(path.join(dir, script)), { recursive: true });
  fs.writeFileSync(path.join(dir, script), `set -euo pipefail\n. ${JSON.stringify(LIB)}\nuse_log_dir stack_stop_test\n${extra}\n` +
    `start web . python3 -m http.server ${port} --bind 127.0.0.1\nwait_port web ${port}\nsay "stack is up"\nwatch_services\n`);
  const log = fs.openSync(path.join(dir, 'stack.log'), 'w');
  const child = spawn('bash', [script, ...args], { cwd: dir, env: { ...process.env, LOG_DIR: dir }, detached: true, stdio: ['ignore', log, log] });
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
function stop(s, port, cmdline = 'bash fake_stack.sh') {
  return run(spawn('bash', [STOP, s.pidFile, cmdline, 'stack.log', 'fake-stack', String(port)]));
}

// makeStop runs make's <target> on the stack whose pid file is <pid var>'s,
// so the stop gets the Makefile's own command line.
function makeStop(target, pidVar, s) {
  return run(spawn('make', ['--no-print-directory', '-C', path.join(__dirname, '..'), target, `${pidVar}=${s.pidFile}`]));
}

// run resolves with child's exit status and output.
function run(child) {
  let out = '';
  child.stdout.on('data', (d) => { out += d; });
  child.stderr.on('data', (d) => { out += d; });
  return new Promise((resolve) => child.on('close', (status) => resolve({ status, out })));
}

// waitGone resolves once process pid has exited, or after about 10 s.
async function waitGone(pid) {
  for (let i = 0; i < 100 && alive(pid); i++) await new Promise((r) => setTimeout(r, 100));
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
test('a pid file whose process is not the stack is stale: removed, and nothing is signalled', { timeout: 60000 }, async () => {
  const port = await freePort();
  const s = await fakeStack(port);
  try {
    const r = await stop(s, port, 'bash some_other_stack.sh');
    assert.strictEqual(r.status, 1, r.out);
    assert.match(r.out, /stale pid file .*signalled nothing/);
    assert.strictEqual(alive(s.pid), true, 'the stack was signalled');
    assert.strictEqual(await listening(port), true, 'the service was stopped');
    assert.strictEqual(fs.existsSync(s.pidFile), false, 'the stale pid file is left');
    assert.strictEqual(fs.existsSync(`${s.pidFile}.logs`), false, 'the stale logs file is left');
  } finally { done(s); }
});

// A stack that exits by itself (its watch_services loop ends it when a
// service dies) leaves its pid files behind.
test('a pid file left by a stack that exited by itself is stale, and says so', { timeout: 60000 }, async () => {
  const port = await freePort();
  const s = await fakeStack(port);
  try {
    process.kill(-s.pid, 'SIGKILL');
    await waitGone(s.pid);
    const r = await stop(s, port);
    assert.strictEqual(r.status, 1, r.out);
    assert.match(r.out, /stale pid file .*has exited/);
    assert.strictEqual(fs.existsSync(s.pidFile), false, 'the stale pid file is left');
  } finally { done(s); }
});

// With no pid file, no make *-stack started a stack.
test('a stop with no pid file says no stack was started', { timeout: 60000 }, async () => {
  const port = await freePort();
  const s = await fakeStack(port);
  fs.rmSync(s.pidFile);
  try {
    const r = await stop(s, port);
    assert.strictEqual(r.status, 1, r.out);
    assert.match(r.out, /no stack started by make fake-stack/);
    assert.strictEqual(alive(s.pid), true, 'the stack was signalled');
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

// The DvP stack runs `bash scripts/stack.sh`, the payments stack `bash
// enygma_payments/run_scripts/stack.sh`: a stop matching a substring of the
// command line would take one for the other.
test("dvp-stack-stop leaves a payments stack alone when the DvP pid file names its pid", { timeout: 60000 }, async () => {
  const port = await freePort();
  const s = await fakeStack(port, '', 'enygma_payments/run_scripts/stack.sh');
  try {
    const r = await makeStop('dvp-stack-stop', 'DVP_STACK_PID', s);
    assert.notStrictEqual(r.status, 0, r.out);
    assert.strictEqual(alive(s.pid), true, 'the payments stack was signalled');
    assert.strictEqual(await listening(port), true, 'the payments service was stopped');
  } finally { done(s); }
});

// Each make *-stack-stop takes the stack its make *-stack starts: the
// command line it matches (the Makefile's *_STACK_CMD) runs the stack
// script at its path, from the dir that target runs it in. Only the signal
// is checked: the stop also waits on the stack's real ports, which a running
// stack may hold.
for (const [target, pidVar, script, args] of [
  ['dvp-stack-stop', 'DVP_STACK_PID', 'scripts/stack.sh', []],
  ['payments-stack-stop', 'PAYMENTS_STACK_PID', 'enygma_payments/run_scripts/stack.sh', []],
  ['shield-stack-stop', 'SHIELD_STACK_PID', 'run.sh', ['--no-demo']],
]) {
  test(`make ${target} signals the stack its start target runs`, { timeout: 60000 }, async () => {
    const port = await freePort();
    const s = await fakeStack(port, '', script, args);
    try {
      const r = await makeStop(target, pidVar, s);
      await waitGone(s.pid);
      assert.strictEqual(alive(s.pid), false, `the stack was not signalled:\n${r.out}`);
    } finally { done(s); }
  });
}
