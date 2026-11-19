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

// server is a service command that listens on port.
const server = (port) => `python3 -m http.server ${port} --bind 127.0.0.1`;

// stack runs body as a stack script that sources the lib, in a scratch dir
// that is also its LOG_DIR. onUp, if given, runs once the script prints
// "stack is up", with the child and that dir; when it throws (or rejects),
// the stack is killed and the promise rejects with that error. It resolves
// with the exit code, the signal and the output, once the scratch dir is
// removed.
function stack(body, onUp) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'stack-lib-test-'));
  const script = path.join(dir, 'stack.sh');
  fs.writeFileSync(script, `set -euo pipefail\n. ${JSON.stringify(LIB)}\nuse_log_dir stack_lib_test\n${body}\n`);
  const child = spawn('bash', [script], { cwd: dir, env: { ...process.env, LOG_DIR: dir } });
  let out = '';
  let up = false;
  let upErr = null;
  const collect = (d) => {
    out += d;
    if (!up && onUp && out.includes('stack is up')) {
      up = true;
      (async () => onUp(child, dir))().catch((e) => { upErr = e; child.kill('SIGKILL'); });
    }
  };
  child.stdout.on('data', collect);
  child.stderr.on('data', collect);
  return new Promise((resolve, reject) => {
    child.on('close', (code, signal) => {
      fs.rmSync(dir, { recursive: true, force: true });
      if (upErr) reject(upErr); else resolve({ code, signal, out });
    });
  });
}

for (const [sig, code] of [['SIGTERM', 143], ['SIGINT', 130]]) {
  test(`${sig} stops the services and exits ${code}, reporting none of them dead`, { timeout: 30000 }, async () => {
    const port = await freePort();
    const r = await stack(`start web . ${server(port)}\nwait_port web ${port}\nsay "stack is up"\nwatch_services`,
      (child) => child.kill(sig));
    assert.strictEqual(r.code, code, r.out);
    assert.doesNotMatch(r.out, /exited/);
    assert.match(r.out, /stopped\. Logs: /);
    assert.strictEqual(await listening(port), false, 'the service still listens');
  });
}

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

// servicePid reads NAME's pid from the stack's service list (or list, in logDir).
function servicePid(logDir, name, list = 'services.pids') {
  const line = fs.readFileSync(path.join(logDir, list), 'utf8').split('\n').find((l) => l.startsWith(`${name} `));
  return line && Number(line.split(' ')[1]);
}

// goneElseKill says whether no process of pid's group runs, killing what
// does so a failed test leaks nothing.
function goneElseKill(pid) {
  try { process.kill(-pid, 0); } catch { return true; }
  process.kill(-pid, 'SIGKILL');
  return false;
}

// As run.sh waits once up: a restart target replaces a service under it.
test('a service relaunched under a running stack stops with it', { timeout: 30000 }, async () => {
  const [port, other] = [await freePort(), await freePort()];
  let replaced = 0;
  const r = await stack(`start web . ${server(port)}\nwait_port web ${port}\nstart other . ${server(other)}\n` +
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
test('a service started under a running stack that had none stops with it', { timeout: 30000 }, async () => {
  const [port, other] = [await freePort(), await freePort()];
  let added = 0;
  const r = await stack(`start other . ${server(other)}\nwait_port other ${other}\nsay "stack is up"\nwait`,
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
test("a stop leaves alone what a past stack's service list in its LOG_DIR names", { timeout: 30000 }, async () => {
  const port = await freePort();
  const bystander = spawn(OWN_GROUP, ['sleep', '60'], { stdio: 'ignore' });
  try {
    const r = await stack(`echo "web ${bystander.pid}" >"$LOG_DIR/services.pids"\n` +
      `start web . ${server(port)}\nwait_port web ${port}\nsay "stack is up"\nwait`, (child) => child.kill('SIGTERM'));
    assert.strictEqual(r.code, 143, r.out);
    assert.strictEqual(await listening(port), false, 'the service still listens');
    assert.doesNotThrow(() => process.kill(-bystander.pid, 0), "the stop killed a past stack's pid");
  } finally {
    try { process.kill(-bystander.pid, 'SIGKILL'); } catch { /* gone */ }
  }
});

// With its LOG_DIR removed under it, a stack still stops what it started.
// A stop that kills nothing waits on its service forever: after 10 s the
// test kills the service (its pid kept aside) and fails.
test('a stop whose service list is gone stops the services start started', { timeout: 30000 }, async () => {
  const port = await freePort();
  let forced = false;
  let timer;
  const r = await stack(`start web . ${server(port)}\nwait_port web ${port}\n` +
    `mv "$LOG_DIR/services.pids" "$LOG_DIR/kept.pids"\nsay "stack is up"\nwait`, (child, dir) => {
    child.kill('SIGTERM');
    timer = setTimeout(() => {
      forced = true;
      process.kill(-servicePid(dir, 'web', 'kept.pids'), 'SIGKILL');
    }, 10000);
  });
  clearTimeout(timer);
  assert.strictEqual(forced, false, 'the stop did not stop the service with its list gone');
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

test('a service exiting stops the stack with its log tail, and stops the others', { timeout: 30000 }, async () => {
  const port = await freePort();
  const r = await stack(`start web . ${server(port)}\nwait_port web ${port}\n` +
    `start brief . sh -c 'echo its-last-words; sleep 1'\nsay "stack is up"\nwatch_services`);
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /brief exited — last log lines:\nits-last-words\n/);
  assert.strictEqual(await listening(port), false, 'the other service still listens');
});

test('wait_port fails at once with the log tail when its service exits before listening', { timeout: 30000 }, async () => {
  const port = await freePort();
  const t0 = Date.now();
  const r = await stack(`start web . sh -c 'echo cannot-bind; exit 1'\nwait_port web ${port}\nsay "stack is up"`);
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /web exited — last log lines:\ncannot-bind\n/);
  assert.ok(Date.now() - t0 < 10000, 'wait_port waited on a dead service');
  assert.doesNotMatch(r.out, /stack is up/);
});

test('a failed step stops the stack with its log tail', { timeout: 30000 }, async () => {
  const r = await stack(`step build . sh -c 'echo compile-error; exit 2'\nsay "stack is up"`);
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /build failed — last log lines:\ncompile-error\n/);
  assert.doesNotMatch(r.out, /stack is up/);
});

test('a busy port fails with its own message, not a stop of services never started', { timeout: 30000 }, async () => {
  const busy = net.createServer();
  await new Promise((resolve) => busy.listen(0, '127.0.0.1', resolve));
  const { port } = busy.address();
  const r = await stack(`require_free_ports ${port}\nsay "stack is up"`);
  busy.close();
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, new RegExp(`port ${port} is already in use`));
  assert.doesNotMatch(r.out, /stopping services/);
});

// check_keys_stamp on a scratch repo holding the two keygen modules, with
// the repo's stack_lib.sh and keys_src.sh: the preflight check that the
// keys' stamp names the working tree's circuit sources.
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
  for (const f of ['stack_lib.sh', 'keys_src.sh']) fs.cpSync(path.join(__dirname, f), path.join(dir, 'scripts', f));
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
