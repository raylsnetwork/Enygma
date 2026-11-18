// scripts/mutant.sh, run on a scratch file with small shell commands as the
// test command: its verdict and exit code, and the file restored after.
//
//   node --test scripts/mutant.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawn, spawnSync } = require('child_process');

const MUTANT = path.join(__dirname, 'mutant.sh');

// run applies the mutant "ok" → "bad" to a scratch file holding "ok", with
// the shell command cmd as the test command (the file's path is $F in it,
// and $P a file it may write a pid to), and returns mutant.sh's exit code,
// its output, the file afterwards and the pid written to $P, if any.
function run(cmd, { expect, timeout } = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mutant-test-'));
  const file = path.join(dir, 'code.txt');
  const pidFile = path.join(dir, 'pid');
  fs.writeFileSync(file, 'ok\n');
  const args = [...(expect ? ['--expect', expect] : []), ...(timeout ? ['--timeout', String(timeout)] : []),
    file, 'ok', 'bad', '--', 'bash', '-c', cmd];
  // A mutant.sh that misses its own deadline is ended here (SIGTERM, which
  // its trap passes on to the run's group), and so fails the test.
  const r = spawnSync('bash', [MUTANT, ...args],
    { encoding: 'utf8', env: { ...process.env, F: file, P: pidFile }, timeout: 15000 });
  const after = fs.readFileSync(file, 'utf8');
  const pid = fs.existsSync(pidFile) ? Number(fs.readFileSync(pidFile, 'utf8')) : undefined;
  fs.rmSync(dir, { recursive: true });
  return { code: r.status, out: r.stdout + r.stderr, after, pid };
}

// alive says whether process pid still runs.
function alive(pid) {
  try { process.kill(pid, 0); return true; } catch { return false; }
}

// A check that fails on the mutant: grep finds "ok" in the file.
const CHECK = 'grep -q ok "$F" || { echo "--- FAIL: TestCode/ok_check"; exit 1; }';

test('a check green without the mutant and red with it is shown red', () => {
  const r = run(CHECK);
  assert.strictEqual(r.code, 0, r.out);
  assert.match(r.out, /^RED/m);
  assert.strictEqual(r.after, 'ok\n', 'the file is restored');
});

test('a check that passes with the mutant in place survives', () => {
  const r = run('true');
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /SURVIVED/);
});

test('a command already red without the mutant shows nothing: bad input', () => {
  const r = run('echo "--- FAIL: TestCode/broken"; exit 1');
  assert.strictEqual(r.code, 2, r.out);
  assert.match(r.out, /without the mutant/);
  assert.doesNotMatch(r.out, /^RED/m);
  assert.strictEqual(r.after, 'ok\n', 'the file is restored');
});

test('a command that does not run is bad input, not red', () => {
  // Green without the mutant, then "command not found" (exit 127) with it,
  // as a test command passed as one word fails.
  const r = run('grep -q ok "$F" || no-such-command-for-mutant-test');
  assert.strictEqual(r.code, 2, r.out);
  assert.match(r.out, /did not run/);
  assert.doesNotMatch(r.out, /^RED/m);
});

// raw runs mutant.sh with args as they are, in a scratch dir (its path is
// $D in the env), and returns its exit code and output.
function raw(args) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mutant-test-'));
  fs.writeFileSync(path.join(dir, 'code.txt'), 'ok\n');
  const r = spawnSync('bash', [MUTANT, ...args.map((a) => a.replaceAll('$D', dir))], { encoding: 'utf8', timeout: 15000 });
  fs.rmSync(dir, { recursive: true });
  return { code: r.status, out: r.stdout + r.stderr };
}

test('a command passed as one word does not run: bad input, not split by a shell', () => {
  // As the shell would, mutant.sh execs "echo ok" as one program's name.
  const r = raw(['$D/code.txt', 'ok', 'bad', '--', 'echo ok']);
  assert.strictEqual(r.code, 2, r.out);
  assert.match(r.out.trimEnd().split('\n').at(-1), /^BAD INPUT:/);
});

test('a mutant that leaves the command unable to start is bad input: did not run', () => {
  // The mutated file is the command: a mutant of its #! line can't exec.
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mutant-test-'));
  const script = path.join(dir, 'check.sh');
  fs.writeFileSync(script, '#!/bin/sh\nexit 0\n', { mode: 0o755 });
  const r = spawnSync('bash', [MUTANT, script, '#!/bin/sh', '#!/no/such/shell', '--', script], { encoding: 'utf8', timeout: 15000 });
  fs.rmSync(dir, { recursive: true });
  assert.strictEqual(r.status, 2, r.stdout + r.stderr);
  assert.match(r.stdout, /did not run with the mutant in place \(exit 12[67]\)/);
});

test('--timeout takes a whole number of seconds, above 0', () => {
  for (const t of [['--timeout', '0'], ['--timeout', 'abc'], ['--timeout', '1.5']]) {
    const r = raw([...t, '$D/code.txt', 'ok', 'bad', '--', 'true']);
    assert.strictEqual(r.code, 2, `${t.join(' ')}: ${r.out}`);
    assert.match(r.out, /^usage:/m, t.join(' '));
  }
  const bare = raw(['--timeout']);
  assert.strictEqual(bare.code, 2, bare.out);
  assert.match(bare.out, /^usage:/m);
});

// HANG starts a child that would outlive the command, writes its pid to $P
// and sleeps past the deadline. The child's output goes elsewhere, so a
// mutant.sh that leaves it running still returns (its pipes close).
const HANG = '{ sleep 30 >/dev/null 2>&1 & echo $! >"$P"; sleep 30; }';
// MUTANT_HANG passes as the code stands and hangs with the mutant.
const MUTANT_HANG = `grep -q ok "$F" || ${HANG}`;

test('a command that ends on time is killed with all it left running', () => {
  const r = run(`sleep 30 >/dev/null 2>&1 & echo $! >"$P"; ${CHECK}`);
  assert.strictEqual(r.code, 0, r.out);
  assert.match(r.out, /^RED/m);
  assert.ok(r.pid, 'the command wrote its child pid');
  assert.ok(!alive(r.pid), `the child ${r.pid} the mutant run left is killed`);
});

test('a green run past --timeout is killed with all it started: HUNG, bad input', () => {
  const r = run(HANG, { timeout: 1 });
  assert.strictEqual(r.code, 2, r.out);
  assert.strictEqual(r.out.trimEnd().split('\n').at(-1), 'HUNG: the green run passed 1 s');
  assert.strictEqual(r.after, 'ok\n', 'the file is restored');
  assert.ok(r.pid, 'the command wrote its child pid');
  assert.ok(!alive(r.pid), `the child ${r.pid} the command started is killed`);
});

test('a mutant run past --timeout is killed with all it started: HUNG, not red', () => {
  const r = run(MUTANT_HANG, { timeout: 1 });
  assert.strictEqual(r.code, 1, r.out);
  assert.strictEqual(r.out.trimEnd().split('\n').at(-1), 'HUNG: the mutant run passed 1 s');
  assert.strictEqual(r.after, 'ok\n', 'the file is restored');
  assert.ok(r.pid, 'the command wrote its child pid');
  assert.ok(!alive(r.pid), `the child ${r.pid} the command started is killed`);
});

test('mutant.sh stopped by Ctrl-C kills the run in progress and restores the file', async () => {
  // The command runs in its own process group, out of reach of the
  // terminal's Ctrl-C, so mutant.sh must kill it itself.
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mutant-test-'));
  const file = path.join(dir, 'code.txt');
  const pidFile = path.join(dir, 'pid');
  fs.writeFileSync(file, 'ok\n');
  // detached: mutant.sh leads its own group, as a terminal's foreground job.
  const p = spawn('bash', [MUTANT, file, 'ok', 'bad', '--', 'bash', '-c', MUTANT_HANG],
    { env: { ...process.env, F: file, P: pidFile }, detached: true, stdio: 'ignore' });
  const exited = new Promise((resolve) => p.on('exit', resolve));
  for (let i = 0; i < 100 && !fs.existsSync(pidFile); i++) await new Promise((r) => setTimeout(r, 100));
  assert.ok(fs.existsSync(pidFile), 'the mutant run started its child');
  assert.strictEqual(fs.readFileSync(file, 'utf8'), 'bad\n', 'the mutant is in place');
  const pid = Number(fs.readFileSync(pidFile, 'utf8'));
  process.kill(-p.pid, 'SIGINT');
  await exited;
  const after = fs.readFileSync(file, 'utf8');
  fs.rmSync(dir, { recursive: true });
  assert.strictEqual(after, 'ok\n', 'the file is restored');
  assert.ok(!alive(pid), `the child ${pid} the command started is killed`);
});

test('--expect: red from the named check is shown red', () => {
  const r = run(CHECK, { expect: 'TestCode/ok_check' });
  assert.strictEqual(r.code, 0, r.out);
  assert.match(r.out, /^RED/m);
});

test('--expect: red from another check is not the check shown red', () => {
  const r = run(CHECK, { expect: 'TestCode/other_check' });
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /not from the expected check/);
  assert.doesNotMatch(r.out, /^RED:/m);
});

test("a miss of --expect prints go test's assertion lines, so the red shows what failed", () => {
  // go test prints the test's name and its assertion on separate lines.
  const goFail = 'grep -q ok "$F" || { echo "--- FAIL: TestScan (0.00s)"; echo "    tags_test.go:328: vault query spans 25000 blocks"; echo FAIL; exit 1; }';
  const r = run(goFail, { expect: 'TestScan.*vault query' });
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /^ +tags_test\.go:328: vault query spans 25000 blocks$/m);
});

test('every verdict is the last line, so a tail of the output keeps it', () => {
  const cases = {
    red: [CHECK, {}],
    survived: ['true', {}],
    'already red': ['echo "--- FAIL: TestCode/broken"; exit 1', {}],
    'did not run': ['grep -q ok "$F" || no-such-command-for-mutant-test', {}],
    'red elsewhere': [CHECK, { expect: 'TestCode/other_check' }],
    'green hung': [HANG, { timeout: 1 }],
    'mutant hung': [MUTANT_HANG, { timeout: 1 }],
  };
  for (const [name, [cmd, opts]] of Object.entries(cases)) {
    const last = run(cmd, opts).out.trimEnd().split('\n').at(-1);
    assert.match(last, /^(RED|SURVIVED|BAD INPUT|HUNG):/, `${name}: last line ${JSON.stringify(last)}`);
  }
});

// inRepo runs mutant.sh from a scratch repo: a copy of mutant.sh and
// suite_changes.sh (whose stacks and common lists name the wiring files)
// under scripts/, a wiring file (Makefile) and another file, each holding
// "ok", and pid files named in pids ({ shield: pid }) in its own TMPDIR. It
// applies "ok" → "bad" to rel with cmd as the test command and returns
// mutant.sh's exit code, its output and the file afterwards.
function inRepo(rel, cmd, pids = {}) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'mutant-repo-'));
  const tmp = path.join(root, 'tmp');
  fs.mkdirSync(path.join(root, 'scripts'));
  fs.mkdirSync(tmp);
  for (const s of ['mutant.sh', 'suite_changes.sh']) fs.copyFileSync(path.join(__dirname, s), path.join(root, 'scripts', s));
  for (const f of ['Makefile', 'other.txt']) fs.writeFileSync(path.join(root, f), 'ok\n');
  for (const [stack, pid] of Object.entries(pids)) fs.writeFileSync(path.join(tmp, `enygma_${stack}_stack.pid`), `${pid}\n`);
  const file = path.join(root, rel);
  const r = spawnSync('bash', [path.join(root, 'scripts', 'mutant.sh'), file, 'ok', 'bad', '--', 'bash', '-c', cmd],
    { encoding: 'utf8', env: { ...process.env, F: file, TMPDIR: tmp } });
  const after = fs.readFileSync(file, 'utf8');
  fs.rmSync(root, { recursive: true });
  return { code: r.status, out: r.stdout + r.stderr, after, tmp };
}

test('a wiring file of a live stack is bad input: the mutant would run inside the stack', () => {
  // This test's own pid is live.
  const r = inRepo('Makefile', CHECK, { shield: process.pid });
  assert.strictEqual(r.code, 2, r.out);
  const last = r.out.trimEnd().split('\n').at(-1);
  assert.match(last, /^BAD INPUT:/, r.out);
  assert.ok(last.includes(path.join(r.tmp, 'enygma_shield_stack.pid')), `names the pid file: ${last}`);
  assert.match(last, /make shield-stack-stop/);
  assert.strictEqual(r.after, 'ok\n', 'the file is untouched');
});

test("a stale pid file, whose process is gone, doesn't block a wiring file's mutant", () => {
  const gone = spawnSync('true').pid;
  const r = inRepo('Makefile', CHECK, { shield: gone });
  assert.strictEqual(r.code, 0, r.out);
  assert.match(r.out, /^RED/m);
  assert.strictEqual(r.after, 'ok\n', 'the file is restored');
});

test('a file outside the wiring mutates as before with a stack up', () => {
  const r = inRepo('other.txt', CHECK, { dvp: process.pid });
  assert.strictEqual(r.code, 0, r.out);
  assert.match(r.out, /^RED/m);
});

test('a wiring file mutates as before with no stack up', () => {
  const r = inRepo('Makefile', CHECK);
  assert.strictEqual(r.code, 0, r.out);
  assert.match(r.out, /^RED/m);
});
