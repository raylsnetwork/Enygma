// scripts/stack_lib.sh, the helpers the three local stack scripts source, on
// small stacks of fake services: how a stack stops on a signal, on a service
// dying, and on a failed step; that its stop also stops a service a make
// restart target relaunched or added (scripts/stack_service.sh), and only
// what it or they started; and its keys-stamp check on a scratch repo.
//
//   node --test scripts/stack_lib.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const net = require('net');
const os = require('os');
const path = require('path');
const { spawn, spawnSync } = require('child_process');
const { GIT_ENV, git, scratchRepo } = require('./test_git');
const { freePort, listening, servicePid, killServices } = require('./test_stack');

const LIB = path.join(__dirname, 'stack_lib.sh');

// server is a service command that listens on port.
const server = (port) => `python3 -m http.server ${port} --bind 127.0.0.1`;

// stack runs body as a stack script that sources the lib, in a scratch dir
// that is also its LOG_DIR, for test context t. onUp, if given, runs once
// the script prints "stack is up", with the child and that dir; when it
// throws (or rejects), the stack is killed and the promise rejects with
// that error. When t times out (t.signal aborts), it kills the stack script
// and the service groups its lists name (killServices): a stop that never
// ends then fails the test at its timeout. Left alive, the stack would keep
// node running, which mutant.sh reports as HUNG, never RED. It resolves with
// the exit code, the signal, the output, the scratch dir and the time the
// output closed (closedAt), once onUp has settled and that dir is removed:
// a test that SIGKILLs the stack awaits its reaper's stop in onUp, while the
// list the reaper reads is still there.
function stack(t, body, onUp) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'stack-lib-test-'));
  const script = path.join(dir, 'stack.sh');
  fs.writeFileSync(script, `set -euo pipefail\n. ${JSON.stringify(LIB)}\nuse_log_dir stack_lib_test\n${body}\n`);
  const child = spawn('bash', [script], { cwd: dir, env: { ...process.env, LOG_DIR: dir } });
  let out = '';
  let up = false;
  let upErr = null;
  let upDone = Promise.resolve();
  const collect = (d) => {
    out += d;
    if (!up && onUp && out.includes('stack is up')) {
      up = true;
      upDone = (async () => onUp(child, dir))().catch((e) => { upErr = e; child.kill('SIGKILL'); });
    }
  };
  child.stdout.on('data', collect);
  child.stderr.on('data', collect);
  const abort = () => { killServices(dir); child.kill('SIGKILL'); };
  t.signal.addEventListener('abort', abort, { once: true });
  return new Promise((resolve, reject) => {
    child.on('close', async (code, signal) => {
      const closedAt = Date.now();
      await upDone;
      t.signal.removeEventListener('abort', abort);
      fs.rmSync(dir, { recursive: true, force: true });
      if (upErr) reject(upErr); else resolve({ code, signal, out, dir, closedAt });
    });
  });
}

// reaping says whether a reaper (scripts/stack_reaper.sh) watches the stack
// script at pid.
function reaping(pid) {
  const ps = spawnSync('ps', ['-axo', 'command='], { encoding: 'utf8' });
  return ps.stdout.split('\n').some((c) => c.includes(`stack_reaper.sh ${pid} `));
}

// A stack's cleanup stops its reaper before its services, and waits for it:
// a stopped stack leaves none behind, and prints what it printed before it
// had one. The stack's EXIT trap lists the processes right after cleanup
// returns, while the stack script still runs, so its reaper can't have
// left on its own.
for (const [sig, code] of [['SIGTERM', 143], ['SIGINT', 130]]) {
  test(`${sig} stops the services and exits ${code}, reporting none of them dead, and leaves no reaper`, { timeout: 30000 }, async (t) => {
    const port = await freePort();
    let pid = 0;
    let watched = false;
    let after = '';
    const r = await stack(t, `trap 'cleanup; ps -axo command= >"$LOG_DIR/after-cleanup.ps"' EXIT\n` +
      `start web . ${server(port)}\nwait_port web ${port}\nsay "stack is up"\nwatch_services`, async (child, dir) => {
      pid = child.pid;
      watched = reaping(pid);
      const exited = new Promise((res) => child.once('exit', res));
      child.kill(sig);
      await exited;
      after = fs.readFileSync(path.join(dir, 'after-cleanup.ps'), 'utf8');
    });
    assert.strictEqual(r.code, code, r.out);
    assert.strictEqual(r.out, `[stack.sh] web is up on :${port}\n[stack.sh] stack is up\n\n` +
      `[stack.sh] stopping services…\n[stack.sh] stopped. Logs: ${r.dir}\n`);
    assert.ok(watched, 'no reaper watched the stack while it was up');
    assert.strictEqual(after.split('\n').some((c) => c.includes(`stack_reaper.sh ${pid} `)), false,
      'the reaper still ran when cleanup returned');
    assert.strictEqual(await listening(port), false, 'the service still listens');
  });
}

// The one reader of a service list's pids in shell (scripts/stack_groups.sh,
// which cleanup and the reaper stop from) skips a pid that isn't an integer
// above 1, as test_stack.js's does: kill -- -0 signals the stopper's own
// group, and kill -- -1 every process the user owns. It only reads, so no
// line here is ever signalled.
test("a service list's shell reader skips any pid that isn't an integer above 1", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'stack-lib-test-'));
  try {
    const list = path.join(dir, 'services.pids');
    fs.writeFileSync(list, ['init 1', 'zero 0', 'all -1', 'word x', 'half 2.5', 'bare', 'web 4242', 'demo 4343', ''].join('\n'));
    const r = spawnSync('bash', ['-c', '. "$1"; listed_pids "$2"', 'listed', path.join(__dirname, 'stack_groups.sh'), list],
      { encoding: 'utf8' });
    assert.strictEqual(r.status, 0, r.stdout + r.stderr);
    assert.strictEqual(r.stdout, '4242\n4343\n');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

const OWN_GROUP = path.join(__dirname, 'own_group.sh');
const STACK_SERVICE = path.join(__dirname, 'stack_service.sh');

// slowServer listens on port, as server does, but its group takes a second
// to exit after a TERM: a stop must wait for it, not only signal it.
const slowServer = (port) => `bash -c 'trap "sleep 1; exit 0" TERM; ${server(port)} & wait'`;

// relaunch starts NAME as make shield-demo-restart and shield-gnark-restart
// do under a running stack logging to logDir: first killing the service
// that holds its port, if pid is given, then a slowServer detached in its
// own group, recorded with stack_service.sh. It returns the new pid.
async function relaunch(logDir, name, port, pid) {
  if (pid) {
    process.kill(-pid, 'SIGTERM');
    await untilListening(port, false);
  }
  const r = spawnSync('bash', ['-c',
    `(exec ${OWN_GROUP} ${slowServer(port)} >>"$1/$2.log" 2>&1) &\n${STACK_SERVICE} "$1" "$2" $! && echo $!`,
    'relaunch', logDir, name], { encoding: 'utf8' });
  assert.strictEqual(r.status, 0, r.stdout + r.stderr);
  const newPid = Number(r.stdout.trim());
  await untilListening(port, true);
  return newPid;
}

// untilListening waits for listening(port) to be want (each test's timeout bounds it).
async function untilListening(port, want) {
  while ((await listening(port)) !== want) await new Promise((r) => setTimeout(r, 100));
}

// goneElseKill says whether no process of pid's group runs, killing what
// does so a failed test leaks nothing.
function goneElseKill(pid) {
  try { process.kill(-pid, 0); } catch { return true; }
  process.kill(-pid, 'SIGKILL');
  return false;
}

// As run.sh waits once up: a restart target replaces a service under it.
test('a service relaunched under a running stack stops with it', { timeout: 30000 }, async (t) => {
  const [port, other] = [await freePort(), await freePort()];
  let replaced = 0;
  const r = await stack(t, `start web . ${server(port)}\nwait_port web ${port}\nstart other . ${server(other)}\n` +
    `wait_port other ${other}\nsay "stack is up"\nwait`, async (child, dir) => {
    try {
      replaced = await relaunch(dir, 'web', port, servicePid(dir, 'web'));
      assert.strictEqual(servicePid(dir, 'web'), replaced, 'the service list still names the old web');
    } finally {
      child.kill('SIGTERM');
    }
  });
  assert.strictEqual(r.code, 143, r.out);
  assert.doesNotMatch(r.out, /exited/);
  assert.ok(replaced, r.out);
  assert.ok(goneElseKill(replaced), 'the relaunched service outlived the stack');
  assert.strictEqual(await listening(port), false, 'the relaunched service still listens');
  assert.strictEqual(await listening(other), false, 'the other service still listens');
});

// As make shield-demo-restart starts the demo on a run.sh --no-demo stack.
test('a service started under a running stack that had none stops with it', { timeout: 30000 }, async (t) => {
  const [port, other] = [await freePort(), await freePort()];
  let added = 0;
  const r = await stack(t, `start other . ${server(other)}\nwait_port other ${other}\nsay "stack is up"\nwait`,
    async (child, dir) => {
      try {
        added = await relaunch(dir, 'demo', port);
      } finally {
        child.kill('SIGTERM');
      }
    });
  assert.strictEqual(r.code, 143, r.out);
  assert.ok(added, r.out);
  assert.ok(goneElseKill(added), 'the added service outlived the stack');
  assert.strictEqual(await listening(port), false, 'the added service still listens');
  assert.strictEqual(await listening(other), false, 'the other service still listens');
});

// A stack reusing a LOG_DIR (RETAIL_LOG_DIR=…) must not take a past
// stack's list for its own: its pids may be anyone's by now.
test("a stop leaves alone what a past stack's service list in its LOG_DIR names", { timeout: 30000 }, async (t) => {
  const port = await freePort();
  const bystander = spawn(OWN_GROUP, ['sleep', '60'], { stdio: 'ignore' });
  try {
    const r = await stack(t, `echo "web ${bystander.pid}" >"$LOG_DIR/services.pids"\n` +
      `start web . ${server(port)}\nwait_port web ${port}\nsay "stack is up"\nwait`, (child) => child.kill('SIGTERM'));
    assert.strictEqual(r.code, 143, r.out);
    assert.strictEqual(await listening(port), false, 'the service still listens');
    assert.doesNotThrow(() => process.kill(-bystander.pid, 0), "the stop killed a past stack's pid");
  } finally {
    try { process.kill(-bystander.pid, 'SIGKILL'); } catch { /* gone */ }
  }
});

// survivors resolves with those of the groups pids that still run after
// about 15 s (the reaper's poll and its 10 s stop), killing them so a
// failed test leaks nothing.
async function survivors(pids) {
  assert.ok(pids.length && pids.every((pid) => Number.isInteger(pid) && pid > 1), `not a list of service pids: ${pids}`);
  const running = (pid) => { try { process.kill(-pid, 0); return true; } catch { return false; } };
  for (let i = 0; i < 150 && pids.some(running); i++) await new Promise((r) => setTimeout(r, 100));
  const left = pids.filter(running);
  left.forEach((pid) => process.kill(-pid, 'SIGKILL'));
  return left;
}

// A SIGKILL runs no trap: the stack's reaper stops what its list names.
test('a SIGKILLed stack still stops its services', { timeout: 30000 }, async (t) => {
  const [port, other] = [await freePort(), await freePort()];
  let left = null;
  const r = await stack(t, `start web . ${server(port)}\nwait_port web ${port}\nstart other . ${slowServer(other)}\n` +
    `wait_port other ${other}\nsay "stack is up"\nwatch_services`, async (child, dir) => {
    const pids = [servicePid(dir, 'web'), servicePid(dir, 'other')];
    child.kill('SIGKILL');
    left = await survivors(pids);
  });
  assert.strictEqual(r.signal, 'SIGKILL', r.out);
  assert.deepStrictEqual(left, [], 'these service groups outlived the SIGKILLed stack');
});

// The way a stuck stack is often killed: by its script's name, with every
// process whose command line holds it. Its reaper's must not.
test('a stack SIGKILLed by its script\'s name (pkill -f) still stops its services', { timeout: 30000 }, async (t) => {
  const port = await freePort();
  let left = null;
  let killed = null;
  const r = await stack(t, `start web . ${server(port)}\nwait_port web ${port}\nsay "stack is up"\nwatch_services`, async (child, dir) => {
    const pids = [servicePid(dir, 'web')];
    killed = spawnSync('pkill', ['-KILL', '-f', path.join(dir, 'stack.sh')], { encoding: 'utf8' });
    left = await survivors(pids);
  });
  assert.strictEqual(killed.status, 0, `pkill matched nothing: ${killed.stderr}`);
  assert.strictEqual(r.signal, 'SIGKILL', r.out);
  assert.deepStrictEqual(left, [], 'the service outlived the stack, its reaper killed with it');
});

// As run.sh waits once up: a restart target relaunched web and added demo.
test('a SIGKILLed stack still stops a service relaunched under it, and one added', { timeout: 30000 }, async (t) => {
  const [port, other, demo] = [await freePort(), await freePort(), await freePort()];
  let left = null;
  const r = await stack(t, `start web . ${server(port)}\nwait_port web ${port}\nstart other . ${server(other)}\n` +
    `wait_port other ${other}\nsay "stack is up"\nwait`, async (child, dir) => {
    const pids = [servicePid(dir, 'other')];
    pids.push(await relaunch(dir, 'web', port, servicePid(dir, 'web')));
    pids.push(await relaunch(dir, 'demo', demo));
    child.kill('SIGKILL');
    left = await survivors(pids);
  });
  assert.strictEqual(r.signal, 'SIGKILL', r.out);
  assert.deepStrictEqual(left, [], 'these service groups outlived the SIGKILLed stack');
});

// A reused pid runs another command line: here the stack script's own pid,
// exec'd into a perl loop, which runs no trap. The reaper takes it for gone.
test('a stack whose pid comes to run another command has its services stopped, as on a reused pid', { timeout: 30000 }, async (t) => {
  const port = await freePort();
  let left = null;
  // A ps that takes 2 s stands in for a slow reaper (CI's): the stack execs
  // before the reaper could read its command line, so the reaper must be
  // handed the line the stack ran, not read it once it is up.
  const slowPs = `real=$(command -v ps)\nmkdir shim\nprintf '#!/bin/sh\\nsleep 2\\nexec %s "$@"\\n' "$real" >shim/ps\n` +
    `chmod +x shim/ps\nPATH=$PWD/shim:$PATH\n`;
  // The new command reaps the stack's children, as init does once a stack
  // whose pid is later reused has died: a stopped service left a zombie
  // would still count as running (Linux's kill -0 reaches a zombie).
  const reaps = `exec perl -e 'while (1) { waitpid(-1, 0) > 0 or sleep 1 }'`;
  await stack(t, `${slowPs}start web . ${server(port)}\nwait_port web ${port}\nsay "stack is up"\n${reaps}`, async (child, dir) => {
    try {
      left = await survivors([servicePid(dir, 'web')]);
    } finally {
      child.kill('SIGKILL');
    }
  });
  assert.deepStrictEqual(left, [], 'the service outlived the stack script');
});

// A caller reading the stack's output (this test, a pipe) sees it end with
// the stack, though its reaper lives on: here for about 10 s, stopping a
// service that ignores TERM.
test("a SIGKILLed stack's output closes at once, its reaper holding none of it", { timeout: 30000 }, async (t) => {
  let killedAt = 0;
  let stubborn = 0;
  const r = await stack(t, `start stubborn . bash -c 'trap "" TERM; while :; do sleep 1; done'\n` +
    `say "stack is up"\nwatch_services`, (child, dir) => {
    stubborn = servicePid(dir, 'stubborn');
    killedAt = Date.now();
    child.kill('SIGKILL');
  });
  try {
    assert.strictEqual(r.signal, 'SIGKILL', r.out);
    assert.ok(r.closedAt - killedAt < 3000, `the output closed ${r.closedAt - killedAt} ms after the SIGKILL`);
  } finally {
    if (stubborn) try { process.kill(-stubborn, 'SIGKILL'); } catch { /* gone */ }
  }
});

// The reaper exits only once the groups it stopped have (or about 10 s on):
// here a service that takes 3 s to exit on TERM.
test("a SIGKILLed stack's reaper exits only once the services it stopped have", { timeout: 30000 }, async (t) => {
  let watched = false;
  let gone = false;
  let left = false;
  const r = await stack(t, `start slow . bash -c 'trap "sleep 3; exit 0" TERM; while :; do sleep 0.2; done'\n` +
    `say "stack is up"\nwatch_services`, async (child, dir) => {
    const [pid, slow] = [child.pid, servicePid(dir, 'slow')];
    child.kill('SIGKILL');
    watched = reaping(pid);
    for (let i = 0; i < 150 && reaping(pid); i++) await new Promise((res) => setTimeout(res, 50));
    gone = !reaping(pid);
    try { process.kill(-slow, 0); left = true; process.kill(-slow, 'SIGKILL'); } catch { /* gone */ }
  });
  assert.strictEqual(r.signal, 'SIGKILL', r.out);
  assert.ok(watched && gone, `the reaper was ${watched ? 'never gone' : 'never seen'}`);
  assert.strictEqual(left, false, 'the reaper exited before the service it stopped');
});

// A listed pid that leads no group (here a child of this test's, in its
// group) gets the TERM itself, as from cleanup.
test("a SIGKILLed stack's reaper TERMs a listed pid that leads no group", { timeout: 30000 }, async (t) => {
  const loner = spawn('sleep', ['60'], { stdio: 'ignore' });
  const ended = new Promise((res) => loner.once('exit', (code, signal) => res(signal)));
  try {
    let signal = null;
    const r = await stack(t, `start web . sleep 60\necho "loner ${loner.pid}" >>"$LOG_DIR/services.pids"\n` +
      `say "stack is up"\nwatch_services`, async (child, dir) => {
      const web = servicePid(dir, 'web');
      child.kill('SIGKILL');
      signal = await Promise.race([ended, new Promise((res) => setTimeout(() => res('none in 5 s'), 5000))]);
      await survivors([web]);
    });
    assert.strictEqual(r.signal, 'SIGKILL', r.out);
    assert.strictEqual(signal, 'SIGTERM', 'the pid that leads no group was not TERMed');
  } finally {
    loner.kill('SIGKILL');
  }
});

// The reaper reads the list the stack's first start emptied, as cleanup does.
test("a SIGKILLed stack's reaper leaves alone what a past stack's service list in its LOG_DIR names", { timeout: 30000 }, async (t) => {
  const port = await freePort();
  const bystander = spawn(OWN_GROUP, ['sleep', '60'], { stdio: 'ignore' });
  let left = null;
  try {
    const r = await stack(t, `echo "web ${bystander.pid}" >"$LOG_DIR/services.pids"\n` +
      `start web . ${server(port)}\nwait_port web ${port}\nsay "stack is up"\nwait`, async (child, dir) => {
      // The stack's own web: its line, not the past stack's.
      const pids = fs.readFileSync(path.join(dir, 'services.pids'), 'utf8').trim().split('\n')
        .map((l) => Number(l.split(' ')[1])).filter((pid) => pid !== bystander.pid);
      child.kill('SIGKILL');
      left = await survivors(pids);
    });
    assert.strictEqual(r.signal, 'SIGKILL', r.out);
    assert.deepStrictEqual(left, [], 'the service outlived the SIGKILLed stack');
    assert.doesNotThrow(() => process.kill(-bystander.pid, 0), "the reaper killed a past stack's pid");
  } finally {
    try { process.kill(-bystander.pid, 'SIGKILL'); } catch { /* gone */ }
  }
});

// With its LOG_DIR removed under it, a stack still stops what it started.
// The list is kept aside as kept.pids, so a stop that kills nothing (and so
// waits on its service forever) is still cleaned up at the test's timeout.
test('a stop whose service list is gone stops the services start started', { timeout: 30000 }, async (t) => {
  const port = await freePort();
  const r = await stack(t, `start web . ${server(port)}\nwait_port web ${port}\n` +
    `mv "$LOG_DIR/services.pids" "$LOG_DIR/kept.pids"\nsay "stack is up"\nwait`, (child) => child.kill('SIGTERM'));
  assert.strictEqual(r.code, 143, r.out);
  assert.strictEqual(await listening(port), false, 'the service still listens with its list gone');
});

// make shield-demo-restart and shield-gnark-restart may run at once.
test('stack_service.sh keeps every line when restarts record at once', { timeout: 30000 }, async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'stack-service-test-'));
  try {
    const names = Array.from({ length: 12 }, (_, i) => `svc${i}`);
    fs.writeFileSync(path.join(dir, 'services.pids'), names.map((n) => `${n} 1`).join('\n') + '\n');
    const runs = names.map((n, i) => new Promise((resolve) => {
      const c = spawn(STACK_SERVICE, [dir, n, String(1000 + i)], { stdio: ['ignore', 'ignore', 'pipe'] });
      let err = '';
      c.stderr.on('data', (d) => { err += d; });
      c.on('close', (code) => resolve({ code, err }));
    }));
    for (const { code, err } of await Promise.all(runs)) assert.strictEqual(code, 0, err);
    const lines = fs.readFileSync(path.join(dir, 'services.pids'), 'utf8').trim().split('\n').sort();
    assert.deepStrictEqual(lines, names.map((n, i) => `${n} ${1000 + i}`).sort(), 'a concurrent record lost or kept a line');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('stack_service.sh fails, naming its usage, on the wrong number of arguments', () => {
  const r = spawnSync(STACK_SERVICE, [os.tmpdir(), 'demo'], { encoding: 'utf8' });
  assert.strictEqual(r.status, 2, r.stdout + r.stderr);
  assert.match(r.stderr, /usage: stack_service\.sh <log dir> <name> <pid>/);
});

test('a service exiting stops the stack with its log tail, and stops the others', { timeout: 30000 }, async (t) => {
  const port = await freePort();
  const r = await stack(t, `start web . ${server(port)}\nwait_port web ${port}\n` +
    `start brief . sh -c 'echo its-last-words; sleep 1'\nsay "stack is up"\nwatch_services`);
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /brief exited — last log lines:\nits-last-words\n/);
  assert.strictEqual(await listening(port), false, 'the other service still listens');
});

// run.sh waits once up: its wait ends when every service has exited, the
// reaper being no child of the stack script's to wait on.
test("a stack's wait returns once its services exit, never waiting on its reaper", { timeout: 30000 }, async (t) => {
  const r = await stack(t, `start brief . sleep 1\nsay "stack is up"\nwait\nsay "every service exited"`);
  assert.strictEqual(r.code, 0, r.out);
  assert.match(r.out, /every service exited/);
});

test('wait_port fails at once with the log tail when its service exits before listening', { timeout: 30000 }, async (t) => {
  const port = await freePort();
  const t0 = Date.now();
  const r = await stack(t, `start web . sh -c 'echo cannot-bind; exit 1'\nwait_port web ${port}\nsay "stack is up"`);
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /web exited — last log lines:\ncannot-bind\n/);
  assert.ok(Date.now() - t0 < 10000, 'wait_port waited on a dead service');
  assert.doesNotMatch(r.out, /stack is up/);
});

test('a failed step stops the stack with its log tail', { timeout: 30000 }, async (t) => {
  const r = await stack(t, `step build . sh -c 'echo compile-error; exit 2'\nsay "stack is up"`);
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /build failed — last log lines:\ncompile-error\n/);
  assert.doesNotMatch(r.out, /stack is up/);
});

test('a busy port fails with its own message, not a stop of services never started', { timeout: 30000 }, async (t) => {
  const busy = net.createServer();
  await new Promise((resolve) => busy.listen(0, '127.0.0.1', resolve));
  const { port } = busy.address();
  const r = await stack(t, `require_free_ports ${port}\nsay "stack is up"`);
  busy.close();
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, new RegExp(`port ${port} is already in use`));
  assert.doesNotMatch(r.out, /stopping services/);
});

// check_keys_stamp on a scratch repo holding the two keygen modules, with
// the repo's stack_lib.sh, the stack_groups.sh it sources and keys_src.sh:
// the preflight check that the keys' stamp names the working tree's circuit sources.
const STAMP_FILES = {
  'enygma_dvp/gnark_circuits/go.mod': 'module enygma_dvp/gnark_circuits\n\ngo 1.21\n',
  'enygma_dvp/gnark_circuits/go.sum': '',
  'enygma_dvp/gnark_circuits/cmd/keygen/main.go':
    'package main\n\nimport "enygma_dvp/gnark_circuits/templates"\n\nfunc main() { templates.Define() }\n',
  'enygma_dvp/gnark_circuits/templates/circuit.go': 'package templates\n\nfunc Define() {}\n',
  'enygma_payments/gnark-server/go.mod': 'module gnark-server\n\ngo 1.21\n',
  'enygma_payments/gnark-server/go.sum': '',
  'enygma_payments/gnark-server/keygen/generate_keys.go': 'package main\n\nfunc main() {}\n',
};
// What each stack passes, and the dir it runs from (enygma_dvp/scripts/stack.sh
// is run from enygma_dvp; enygma_payments/run_scripts/stack.sh from the root).
const DVP = { keys: 'keys', module: 'enygma_dvp/gnark_circuits', keygen: './cmd/keygen',
  stamp: 'enygma_dvp/gnark_circuits/scripts/keys/keys.src', target: 'dvp-keygen', cwd: 'enygma_dvp' };
const PAYMENTS = { keys: 'Burn keys', module: 'enygma_payments/gnark-server', keygen: './keygen',
  stamp: 'enygma_payments/gnark-server/keys/burn.src', target: 'payments-burn-keygen', cwd: '.' };
const goEnv = { ...GIT_ENV, GOFLAGS: '', GOWORK: 'off', GOTOOLCHAIN: 'local' };

// stampRepo writes STAMP_FILES, the lib and keys_src.sh to a scratch git
// repo with one commit, and stamps both modules' keys as their keygen
// targets do; it returns the repo's root.
function stampRepo() {
  const dir = scratchRepo('stack-lib-stamp-test');
  Object.entries(STAMP_FILES).forEach(([f, c]) => {
    fs.mkdirSync(path.dirname(path.join(dir, f)), { recursive: true });
    fs.writeFileSync(path.join(dir, f), c);
  });
  // Copied, not rewritten: the lib runs keys_src.sh, so it keeps its mode.
  for (const f of ['stack_lib.sh', 'stack_groups.sh', 'keys_src.sh']) fs.cpSync(path.join(__dirname, f), path.join(dir, 'scripts', f));
  git(dir, 'add', '-A');
  git(dir, 'commit', '-qm', 'init');
  for (const { module, keygen, stamp } of [DVP, PAYMENTS]) {
    const src = spawnSync('bash', [path.join(dir, 'scripts/keys_src.sh'), module, keygen], { encoding: 'utf8', env: goEnv });
    assert.strictEqual(src.status, 0, src.stderr);
    fs.mkdirSync(path.dirname(path.join(dir, stamp)), { recursive: true });
    fs.writeFileSync(path.join(dir, stamp), src.stdout);
  }
  return dir;
}

// checkStamp runs check_keys_stamp with args (a stack's, DvP's unless given;
// args.argv, if set, in place of its fields) from the stack's dir, in a
// script that sources the scratch repo's lib, after edit (on the repo's
// root); it returns the exit code and the output.
function checkStamp(edit = () => {}, args = DVP) {
  const dir = stampRepo();
  try {
    edit(dir);
    const argv = args.argv || [args.keys, args.module, args.keygen, args.stamp, args.target];
    const lib = path.relative(path.join(dir, args.cwd), path.join(dir, 'scripts/stack_lib.sh'));
    const script = `set -euo pipefail\n. ${lib}\ncheck_keys_stamp ${argv.map((a) => `'${a}'`).join(' ')}\n`;
    const r = spawnSync('bash', ['-c', script], { cwd: path.join(dir, args.cwd), encoding: 'utf8', env: goEnv });
    return { code: r.status, out: r.stdout + r.stderr };
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

test("check_keys_stamp passes, quietly, on keys stamped with the working tree's sources", () => {
  for (const args of [DVP, PAYMENTS]) {
    const r = checkStamp(undefined, args);
    assert.strictEqual(r.code, 0, r.out);
    assert.strictEqual(r.out, '');
  }
});

test('check_keys_stamp fails, naming the keygen target, on keys built from other sources', () => {
  const r = checkStamp((d) => fs.appendFileSync(path.join(d, 'enygma_dvp/gnark_circuits/templates/circuit.go'), '// x\n'));
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /the keys in gnark_circuits\/scripts\/keys were built from other circuit sources than the working tree's: make dvp-keygen\n/);
  assert.doesNotMatch(r.out, /no record/);
});

for (const [what, edit] of [
  ['no stamp', (d) => fs.rmSync(path.join(d, DVP.stamp))],
  ['an empty stamp', (d) => fs.writeFileSync(path.join(d, DVP.stamp), '')],
]) {
  test(`check_keys_stamp fails, naming the keygen target, on keys with ${what}`, () => {
    const r = checkStamp(edit);
    assert.strictEqual(r.code, 1, r.out);
    assert.match(r.out, /the keys in gnark_circuits\/scripts\/keys have no record of the circuit sources they were built from \(keys\.src\): make dvp-keygen\n/);
    assert.doesNotMatch(r.out, /other circuit sources/);
  });
}

// Keygen would fail the same way, so this message names no target.
test('check_keys_stamp fails on sources keys_src.sh cannot fingerprint, rather than pass the stamp', () => {
  const r = checkStamp((d) => fs.rmSync(path.join(d, 'enygma_dvp/gnark_circuits/templates/circuit.go')));
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /scripts\/keys_src\.sh could not fingerprint the circuit sources \(above\), so the keys cannot be checked\n/);
  assert.doesNotMatch(r.out, /make dvp-keygen/);
});

test("check_keys_stamp names payments' Burn keys as its stack does", () => {
  const r = checkStamp((d) => fs.appendFileSync(path.join(d, 'enygma_payments/gnark-server/keygen/generate_keys.go'), '// x\n'),
    PAYMENTS);
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /the Burn keys in gnark-server\/keys were built from other circuit sources than the working tree's: make payments-burn-keygen\n/);
  const missing = checkStamp((d) => fs.rmSync(path.join(d, PAYMENTS.stamp)), PAYMENTS);
  assert.strictEqual(missing.code, 1, missing.out);
  assert.match(missing.out, /the Burn keys in gnark-server\/keys have no record of the circuit sources they were built from \(burn\.src\): make payments-burn-keygen\n/);
});

test('check_keys_stamp fails, naming its usage, on the wrong number of arguments', () => {
  const r = checkStamp(undefined, { ...DVP, argv: [DVP.keys, DVP.module] });
  assert.strictEqual(r.code, 2, r.out);
  assert.match(r.out, /usage: check_keys_stamp <keys> <module dir> <keygen package> <stamp file> <make target>/);
});
