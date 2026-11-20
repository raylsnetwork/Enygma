// make's *-stack targets (scripts/stack_start.sh), on a small stack script
// of fake services built on scripts/stack_lib.sh: the pid files exist from
// the moment the stack script starts, not only once it is up, so
// scripts/mutant.sh's guard and make's *-stack-stop see a starting stack.
//
//   node --test scripts/stack_start.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawn, execFileSync } = require('child_process');
const { makeEnv } = require('./test_git');
const { freePort, alive, run, killStack, testNoLeftovers } = require('./test_stack');

const ROOT = path.join(__dirname, '..');
const LIB = path.join(__dirname, 'stack_lib.sh');
const STOP = path.join(__dirname, 'stack_stop.sh');

// Each make *-stack target, with the variables naming its command, pid file
// and log dir.
const TARGETS = [
  ['dvp-stack', 'DVP_STACK_CMD', 'DVP_STACK_PID', 'DVP_LOG_DIR', 'stack.log'],
  ['payments-stack', 'PAYMENTS_STACK_CMD', 'PAYMENTS_STACK_PID', 'PAYMENTS_LOG_DIR', 'stack.log'],
  ['shield-stack', 'SHIELD_STACK_CMD', 'SHIELD_STACK_PID', 'RETAIL_LOG_DIR', 'run.log'],
];

// sleep resolves after ms.
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// until polls fn every 100 ms, tries times (about 10 s), resolving with its first truthy value.
async function until(fn, tries = 100) {
  for (let i = 0; i < tries; i++) {
    const v = fn();
    if (v) return v;
    await sleep(100);
  }
  return fn();
}

// fakeStack writes, in a fresh dir, a stack script that starts one web
// service on port, then waits for the file `go` in its dir, then runs
// `then` (by default: says the stack is up and watches its services).
function fakeStack(port, then = 'say "stack is up"\nwatch_services') {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'stack-start-test-'));
  const script = path.join(dir, 'fake_stack.sh');
  fs.writeFileSync(script, `set -euo pipefail\n. ${JSON.stringify(LIB)}\nuse_log_dir stack_start_test\n` +
    `start web . python3 -m http.server ${port} --bind 127.0.0.1\nwait_port web ${port}\n` +
    `until [ -f ${JSON.stringify(path.join(dir, 'go'))} ]; do sleep 0.1; done\n${then}\n`);
  return {
    dir,
    cmd: `bash ${script}`,
    pidFile: path.join(dir, 'stack.pid'),
    logs: path.join(dir, 'logs'),
    go: () => fs.writeFileSync(path.join(dir, 'go'), ''),
  };
}

// makeStart runs make's <target> on the fake stack s, with env added, for
// test context ctx. stack_start.sh waits as long as the stack script runs,
// so when ctx times out (its signal aborts) it kills the stack script and
// its services (abortStack): make then returns, and done runs.
function makeStart(ctx, [target, cmdVar, pidVar, logVar], s, env = {}) {
  ctx.signal.addEventListener('abort', () => abortStack(s), { once: true });
  return run(spawn('make', ['--no-print-directory', '-C', ROOT, target,
    `${cmdVar}=${s.cmd}`, `${pidVar}=${s.pidFile}`, `${logVar}=${s.logs}`], { env: { ...makeEnv, ...env } }));
}

// abortStack kills fake stack s's services and its stack script, as its pid
// file names it.
function abortStack(s) {
  killStack(s.logs, fs.existsSync(s.pidFile) && Number(fs.readFileSync(s.pidFile, 'utf8')), s.pid);
}

// done lets the fake stack past its wait, so make returns, then kills what
// a test left running (the stack script, and its services, each in its own
// group) and removes its dir.
async function done(s, started) {
  s.go();
  await started;
  abortStack(s);
  fs.rmSync(s.dir, { recursive: true, force: true });
}

// readPid resolves with the pid in s's pid file once it is there, and keeps
// it on s for done.
async function readPid(s) {
  const pid = await until(() => fs.existsSync(s.pidFile) && Number(fs.readFileSync(s.pidFile, 'utf8')));
  s.pid = pid;
  return pid;
}

for (const t of TARGETS) {
  const [target, , , , stackLog] = t;

  // #312's mutant of a wiring file ran while a stack was sourcing
  // stack_lib.sh, before it was up: the pid file must exist then.
  test(`make ${target} writes its pid files while the stack is starting`, { timeout: 60000 }, async (ctx) => {
    const s = fakeStack(await freePort());
    const started = makeStart(ctx, t, s);
    try {
      const pid = await readPid(s);
      assert.ok(pid, 'no pid file while the stack starts');
      assert.strictEqual(alive(pid), true, 'the pid file names no live process');
      assert.doesNotMatch(fs.readFileSync(path.join(s.logs, stackLog), 'utf8'), /stack is up/);
      assert.strictEqual(fs.readFileSync(`${s.pidFile}.logs`, 'utf8').trim(), s.logs);
      s.go();
      const r = await started;
      assert.strictEqual(r.status, 0, r.out);
      assert.match(r.out, /stack is up \(logs /);
      assert.strictEqual(Number(fs.readFileSync(s.pidFile, 'utf8')), pid, 'the pid file changed once up');
    } finally { await done(s, started); }
  });
}

// A stack that fails to start (a busy port, a failed build step) must not
// leave a pid file behind: it would be stale, and block mutant.sh until a
// stop removed it.
test('a stack script that exits before it is up leaves no pid files', { timeout: 60000 }, async (ctx) => {
  const s = fakeStack(await freePort(), 'echo cannot-start; exit 3');
  const started = makeStart(ctx, TARGETS[2], s);
  try {
    const pid = await readPid(s);
    assert.ok(pid, 'no pid file while the stack starts');
    s.go();
    const r = await started;
    assert.strictEqual(r.status, 2, r.out);
    assert.match(r.out, /bash \S+fake_stack\.sh exited before the stack came up:\n[^]*cannot-start/);
    assert.strictEqual(fs.existsSync(s.pidFile), false, 'the pid file is left');
    assert.strictEqual(fs.existsSync(`${s.pidFile}.logs`), false, 'the logs file is left');
  } finally { await done(s, started); }
});

// make *-stack-stop on a stack still starting stops it as one that is up:
// stack_stop.sh matches the stack script's command line, which is already
// the stack's while it starts. make *-stack then reports the exit, without
// a pid file to leave behind.
test('stack_stop.sh stops a stack that is still starting', { timeout: 60000 }, async (ctx) => {
  const port = await freePort();
  const s = fakeStack(port);
  const started = makeStart(ctx, TARGETS[2], s);
  try {
    const pid = await readPid(s);
    const r = await run(spawn('bash', [STOP, s.pidFile, s.cmd, 'run.log', 'shield-stack', String(port)]));
    assert.strictEqual(r.status, 0, r.out);
    assert.match(r.out, /stack stopped/);
    assert.strictEqual(alive(pid), false, 'the stack script still runs');
    const st = await started;
    assert.notStrictEqual(st.status, 0, st.out);
    assert.match(st.out, /exited before the stack came up/);
    assert.strictEqual(fs.existsSync(s.pidFile), false, 'the pid file is left');
  } finally { await done(s, started); }
});

// A second start on a running stack would overwrite its pid file, fail on
// the busy ports, and remove the file: the running stack would be left with
// none, unguarded and unstoppable by make.
test('a start while the stack runs is refused, and leaves its pid files alone', { timeout: 60000 }, async (ctx) => {
  const s = fakeStack(await freePort());
  const started = makeStart(ctx, TARGETS[2], s);
  let second;
  try {
    const pid = await readPid(s);
    // Unrefused, the second start would wait on the fake stack's `go` too.
    second = makeStart(ctx, TARGETS[2], s);
    const r = await Promise.race([second, sleep(5000).then(() => ({ status: 0, out: 'the second start was not refused' }))]);
    assert.notStrictEqual(r.status, 0, r.out);
    assert.match(r.out, new RegExp(`the stack make shield-stack started \\(pid ${pid}\\) still runs: stop it first \\(make shield-stack-stop\\)`));
    assert.strictEqual(Number(fs.readFileSync(s.pidFile, 'utf8')), pid, 'the pid file changed');
    assert.strictEqual(fs.readFileSync(`${s.pidFile}.logs`, 'utf8').trim(), s.logs, 'the logs file changed');
    assert.strictEqual(alive(pid), true, 'the running stack stopped');
  } finally { s.go(); await second; await done(s, started); }
});

// Between the fork and perl's exec of the stack script, ps shows another
// command line, and stack_stop.sh takes the pid file for stale. A perl that
// takes a second to exec holds that window open.
test('the pid file names the stack script from the start, never the process before its exec', { timeout: 60000 }, async (ctx) => {
  const s = fakeStack(await freePort());
  const bin = path.join(s.dir, 'bin');
  fs.mkdirSync(bin);
  const perl = execFileSync('sh', ['-c', 'command -v perl'], { encoding: 'utf8' }).trim();
  fs.writeFileSync(path.join(bin, 'perl'), `#!/bin/sh\nsleep 1\nexec ${perl} "$@"\n`, { mode: 0o755 });
  const started = makeStart(ctx, TARGETS[2], s, { PATH: `${bin}:${process.env.PATH}` });
  try {
    const pid = await readPid(s);
    const cmd = execFileSync('ps', ['-o', 'command=', '-p', String(pid)], { encoding: 'utf8' }).trim();
    assert.strictEqual(cmd, s.cmd, 'the pid file names a process not yet the stack script');
  } finally { await done(s, started); }
});

testNoLeftovers(test);
