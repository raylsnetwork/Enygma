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

test('--expect: a red elsewhere names the first failure in the verdict, so a near-miss regex shows in a tail', () => {
  const r = run(CHECK, { expect: 'TestCode/other_check' });
  const last = r.out.trimEnd().split('\n').at(-1);
  assert.match(last, /^SURVIVED: .*\(\/TestCode\/other_check\/\); first failure: --- FAIL: TestCode\/ok_check$/);
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
// applies old → repl (default "ok" → "bad") to rel with cmd as the test
// command (the file's path is $F in it, and $P a file it may write to) and
// returns mutant.sh's exit code, its output, the file afterwards and what
// cmd wrote to $P. inspect(root, wrote) runs before the repo is removed.
function inRepo(rel, cmd, { pids = {}, old = 'ok', repl = 'bad', inspect } = {}) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'mutant-repo-'));
  const tmp = path.join(root, 'tmp');
  fs.mkdirSync(path.join(root, 'scripts'));
  fs.mkdirSync(tmp);
  for (const s of ['mutant.sh', 'suite_changes.sh']) fs.copyFileSync(path.join(__dirname, s), path.join(root, 'scripts', s));
  for (const f of ['Makefile', 'other.txt']) fs.writeFileSync(path.join(root, f), 'ok\n');
  for (const [stack, pid] of Object.entries(pids)) fs.writeFileSync(path.join(tmp, `enygma_${stack}_stack.pid`), `${pid}\n`);
  const file = path.join(root, rel);
  const note = path.join(root, 'note');
  const r = spawnSync('bash', [path.join(root, 'scripts', 'mutant.sh'), file, old, repl, '--', 'bash', '-c', cmd],
    { encoding: 'utf8', env: { ...process.env, F: file, P: note, TMPDIR: tmp }, timeout: 15000 });
  const after = fs.readFileSync(file, 'utf8');
  const wrote = fs.existsSync(note) ? fs.readFileSync(note, 'utf8') : undefined;
  if (inspect) inspect(root, wrote);
  fs.rmSync(root, { recursive: true });
  return { code: r.status, out: r.stdout + r.stderr, after, tmp, wrote };
}

test('a wiring file of a live stack is bad input: the mutant would run inside the stack', () => {
  // This test's own pid is live.
  const r = inRepo('Makefile', CHECK, { pids: { shield: process.pid } });
  assert.strictEqual(r.code, 2, r.out);
  const last = r.out.trimEnd().split('\n').at(-1);
  assert.match(last, /^BAD INPUT:/, r.out);
  assert.ok(last.includes(path.join(r.tmp, 'enygma_shield_stack.pid')), `names the pid file: ${last}`);
  assert.match(last, /make shield-stack-stop/);
  assert.strictEqual(r.after, 'ok\n', 'the file is untouched');
});

test("a stale pid file, whose process is gone, doesn't block a wiring file's mutant", () => {
  const gone = spawnSync('true').pid;
  const r = inRepo('Makefile', CHECK, { pids: { shield: gone } });
  assert.strictEqual(r.code, 0, r.out);
  assert.match(r.out, /^RED/m);
  assert.strictEqual(r.after, 'ok\n', 'the file is restored');
});

test('a file outside the wiring mutates as before with a stack up', () => {
  const r = inRepo('other.txt', CHECK, { pids: { dvp: process.pid } });
  assert.strictEqual(r.code, 0, r.out);
  assert.match(r.out, /^RED/m);
});

test('a wiring file mutates as before with no stack up', () => {
  const r = inRepo('Makefile', CHECK);
  assert.strictEqual(r.code, 0, r.out);
  assert.match(r.out, /^RED/m);
});

test('a mutant of mutant.sh itself runs from a copy, not the script bash is running', () => {
  // The mutant changes the RED verdict's text. bash reads a script as it
  // goes, so a mutant.sh running the mutated file would print the mutated
  // verdict. The test command writes to $P the script its parent, the
  // running mutant.sh, was started on, then what it sees of the env vars
  // that tell the copy where the real repo is.
  const verdict = 'echo "RED: the command failed';
  const cmd = `{ ps -o args= -p $PPID | awk '{ print $2 }'; echo "\${MUTANT_SH_ROOT-unset} \${MUTANT_SH_COPY_DIR-unset}"; } >"$P"; `
    + `grep -qF '${verdict}' "$F" || exit 1`;
  let real, copyLeft;
  const r = inRepo('scripts/mutant.sh', cmd, {
    old: verdict,
    repl: 'echo "MUTATED: the command failed',
    inspect: (root, wrote) => {
      real = path.join(root, 'scripts', 'mutant.sh');
      copyLeft = fs.existsSync(wrote.split('\n')[0]);
    },
  });
  assert.strictEqual(r.code, 0, r.out);
  assert.match(r.out.trimEnd().split('\n').at(-1), /^RED:/, r.out);
  assert.strictEqual(r.after, fs.readFileSync(MUTANT, 'utf8'), 'the file is restored');
  const [running, env] = r.wrote.trimEnd().split('\n');
  assert.ok(running.endsWith('/mutant.sh'), `the test command named the running script: ${running}`);
  assert.notStrictEqual(running, real, 'the running script is not the mutated file');
  assert.ok(!copyLeft, `the copy ${running} is removed on exit`);
  assert.strictEqual(env, 'unset unset', 'the test command does not inherit where the copy came from');
});

test('a file that is not there is left not there', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mutant-test-'));
  const file = path.join(dir, 'missing.txt');
  const r = spawnSync('bash', [MUTANT, file, 'ok', 'bad', '--', 'true'], { encoding: 'utf8', timeout: 15000 });
  const left = fs.existsSync(file);
  fs.rmSync(dir, { recursive: true });
  assert.notStrictEqual(r.status, 0, r.stdout + r.stderr);
  assert.ok(!left, 'no file is restored from a backup never made');
});

// batch runs mutant.sh --batch on entries (a list of { expect, old, new,
// cmd }, or raw JSON text), each applied to its own scratch file holding
// text (default "ok\n"; the entry's file is that path unless it names one)
// with the shell command cmd as its test command (the file's path is $F in
// it). It runs in that scratch dir, which also holds a.txt and b.txt for
// raw JSON to name. It returns mutant.sh's exit code, its output, its RED
// lines (the lines from "- `" on) and each file afterwards.
function batch(entries, { text = 'ok\n' } = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mutant-batch-'));
  const files = [];
  let json = entries;
  if (typeof entries !== 'string') {
    json = JSON.stringify(entries.map((e, i) => {
      const file = path.join(dir, `code${i}.txt`);
      fs.writeFileSync(file, text);
      files.push(file);
      const { cmd, ...rest } = e;
      return { file, ...rest, cmd: ['bash', '-c', `F=${file}; ${cmd}`] };
    }), null, 2);
  }
  for (const f of ['a.txt', 'b.txt']) fs.writeFileSync(path.join(dir, f), text);
  const list = path.join(dir, 'mutants.json');
  fs.writeFileSync(list, json);
  const r = spawnSync('bash', [MUTANT, '--batch', list], { cwd: dir, encoding: 'utf8', timeout: 30000 });
  const after = files.map((f) => fs.readFileSync(f, 'utf8'));
  fs.rmSync(dir, { recursive: true });
  const out = r.stdout + r.stderr;
  return { code: r.status, out, reds: out.split('\n').filter((l) => l.startsWith('- `')), after };
}

// CHECK2 is a second check, on the line "two".
const CHECK2 = 'grep -q two "$F" || { echo "--- FAIL: TestCode/two_check"; exit 1; }';

test('--batch: two reds print both commit lines and exit 0', () => {
  const r = batch([
    { expect: 'TestCode/ok_check', old: 'ok', new: 'bad', cmd: CHECK },
    { expect: 'TestCode/two_check', old: 'two', new: 'six', cmd: CHECK2 },
  ], { text: 'ok\ntwo\n' });
  assert.strictEqual(r.code, 0, r.out);
  assert.deepStrictEqual(r.reds, [
    '- `ok` → `bad`: RED: "TestCode/ok_check"',
    '- `two` → `six`: RED: "TestCode/two_check"',
  ], r.out);
  assert.deepStrictEqual(r.after, ['ok\ntwo\n', 'ok\ntwo\n'], 'each file is restored');
});

test('--batch: a survivor among reds exits 1, is named, and gets no RED line', () => {
  const r = batch([
    { expect: 'TestCode/ok_check', old: 'ok', new: 'bad', cmd: CHECK },
    { expect: 'TestCode/two_check', old: 'two', new: 'six', cmd: 'true' },
    { expect: 'TestCode/two_check', old: 'two', new: 'ten', cmd: CHECK2 },
  ], { text: 'ok\ntwo\n' });
  assert.strictEqual(r.code, 1, r.out);
  assert.deepStrictEqual(r.reds, [
    '- `ok` → `bad`: RED: "TestCode/ok_check"',
    '- `two` → `ten`: RED: "TestCode/two_check"',
  ], r.out);
  assert.match(r.out, /^  mutant 2, `two` → `six` \(\/TestCode\/two_check\/\): SURVIVED:/m, 'the survivor is named');
  assert.match(r.out.trimEnd().split('\n').at(-1), /^SURVIVED: 1 of 3/, r.out);
  assert.deepStrictEqual(r.after, ['ok\ntwo\n', 'ok\ntwo\n', 'ok\ntwo\n'], 'each file is restored');
});

test('--batch: a multi-line old text is applied whole, and quoted on one line', () => {
  const r = batch([
    { expect: 'TestCode/ok_check', old: 'ok\ntwo\n', new: 'two\n', cmd: CHECK },
  ], { text: 'ok\ntwo\n' });
  assert.strictEqual(r.code, 0, r.out);
  assert.deepStrictEqual(r.reds, ['- `ok two` → `two`: RED: "TestCode/ok_check"'], r.out);
  assert.deepStrictEqual(r.after, ['ok\ntwo\n'], 'the file is restored');
});

test('--batch: a long old or new text is cut in its RED line', () => {
  const long = 'x'.repeat(80);
  const r = batch([{ expect: 'TestCode/ok_check', old: 'ok', new: long, cmd: CHECK }]);
  assert.strictEqual(r.code, 0, r.out);
  assert.deepStrictEqual(r.reds, [`- \`ok\` → \`${'x'.repeat(59)}…\`: RED: "TestCode/ok_check"`], r.out);
});

test('--batch: a change past the cut is shown in a window around it, so the two sides differ', () => {
  // Cut at 60 characters, both sides would read 59 x's and "…".
  const x = 'x'.repeat(70), y = 'y'.repeat(70);
  const r = batch([
    { expect: 'TestCode/ok_check', old: `${x}ok`, new: `${x}no`, cmd: CHECK },
    { expect: 'TestCode/ok_check', old: `${x}ok${y}`, new: `${x}no${y}`, cmd: CHECK },
    { expect: 'TestCode/ok_check', old: `${x}ok`, new: `${x}`, cmd: CHECK },
  ], { text: `${x}ok${y}\n` });
  assert.strictEqual(r.code, 0, r.out);
  const x20 = 'x'.repeat(20), y36 = 'y'.repeat(36);
  assert.deepStrictEqual(r.reds, [
    `- \`…${x20}ok\` → \`…${x20}no\`: RED: "TestCode/ok_check"`,
    `- \`…${x20}ok${y36}…\` → \`…${x20}no${y36}…\`: RED: "TestCode/ok_check"`,
    `- \`…${x20}ok\` → \`…${x20}\`: RED: "TestCode/ok_check"`,
  ], r.out);
});

test('--batch: an entry it cannot read is bad input, named, and nothing runs', () => {
  const ok = { expect: 'TestCode/ok_check', file: 'a.txt', old: 'ok', new: 'bad', cmd: ['true'] };
  const cases = {
    'no cmd': [[ok, { expect: 'x', file: 'b.txt', old: 'ok', new: 'bad' }], /entry 2 \(b\.txt\): no cmd/],
    'unknown key': [[{ ...ok, exepct: 'x' }], /entry 1 \(a\.txt\): unknown exepct/],
    'cmd as one string': [[{ ...ok, cmd: 'node --test x.js' }], /entry 1 \(a\.txt\): cmd is not a non-empty list/],
    'empty old': [[{ ...ok, old: '' }], /entry 1 \(a\.txt\): old is empty/],
    'not an object': [[ok, 'ok'], /entry 2 is not an object/],
    'not a list': [ok, /want a non-empty JSON list/],
    'an empty list': [[], /want a non-empty JSON list/],
    'expect not a string': [[{ ...ok, expect: 1 }], /entry 1 \(a\.txt\): expect is not a string/],
    'a NUL in old': [[{ ...ok, old: 'o\u0000k' }], /entry 1 \(a\.txt\): holds a NUL/],
    'no such file': [[{ ...ok, file: 'no-such.txt' }], /entry 1 \(no-such\.txt\): no such file/],
  };
  for (const [name, [entries, want]] of Object.entries(cases)) {
    const r = batch(JSON.stringify(entries));
    assert.strictEqual(r.code, 2, `${name}: ${r.out}`);
    assert.match(r.out, want, name);
    assert.doesNotMatch(r.out, /== mutant/, `${name}: nothing runs`);
  }
  const r = batch('[{"expect": "x",');
  assert.strictEqual(r.code, 2, r.out);
  assert.match(r.out, /BAD INPUT: .*not a JSON list of mutants/);
});

test('--batch takes no mutant of its own and no --expect', () => {
  for (const args of [['--batch', 'l.json', '$D/code.txt', 'ok', 'bad', '--', 'true'], ['--expect', 'x', '--batch', 'l.json'], ['--batch']]) {
    const r = raw(args);
    assert.strictEqual(r.code, 2, `${args.join(' ')}: ${r.out}`);
    assert.match(r.out, /^usage:/m, args.join(' '));
  }
});

test('--batch: an entry may mutate mutant.sh itself, and the batch reads on unharmed', () => {
  // A scratch repo's mutant.sh runs the batch; the entry mutates that same
  // file's single-mutant RED verdict, which the test command checks for.
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'mutant-repo-'));
  fs.mkdirSync(path.join(root, 'scripts'));
  for (const s of ['mutant.sh', 'suite_changes.sh']) fs.copyFileSync(path.join(__dirname, s), path.join(root, 'scripts', s));
  const self = path.join(root, 'scripts', 'mutant.sh');
  const verdict = 'echo "RED: the command failed';
  const list = path.join(root, 'mutants.json');
  fs.writeFileSync(list, JSON.stringify([{
    expect: 'verdict_check', file: self, old: verdict, new: 'echo "MUTATED: the command failed',
    cmd: ['bash', '-c', `grep -qF '${verdict}' '${self}' || { echo "--- FAIL: verdict_check"; exit 1; }`],
  }]));
  const r = spawnSync('bash', [self, '--batch', list], { encoding: 'utf8', timeout: 30000 });
  const after = fs.readFileSync(self, 'utf8');
  fs.rmSync(root, { recursive: true });
  const out = r.stdout + r.stderr;
  assert.strictEqual(r.status, 0, out);
  assert.match(out, /^RED: the command failed with the mutant in place$/m, 'the entry ran from a copy, not the mutated file');
  assert.match(out, /^- `echo "RED: the command failed` → `echo "MUTATED: the command failed`: RED: "verdict_check"$/m, out);
  assert.match(out.trimEnd().split('\n').at(-1), /^RED: all 1 mutants$/, out);
  assert.strictEqual(after, fs.readFileSync(MUTANT, 'utf8'), 'the file is restored');
});

test('--batch: an entry run on bad input makes the batch bad input, exit 2', () => {
  const r = batch([
    { expect: 'TestCode/ok_check', old: 'ok', new: 'bad', cmd: CHECK },
    { expect: 'TestCode/broken', old: 'ok', new: 'bad', cmd: 'echo "--- FAIL: TestCode/broken"; exit 1' },
  ]);
  assert.strictEqual(r.code, 2, r.out);
  assert.match(r.out, /^  mutant 2, .*: BAD INPUT:/m, 'the entry is named');
  assert.match(r.out.trimEnd().split('\n').at(-1), /^BAD INPUT: 1 of 2/, r.out);
});

test('--batch: a batch with no red prints no RED line and exits 1', () => {
  const r = batch([
    { expect: 'TestCode/ok_check', old: 'ok', new: 'bad', cmd: 'true' },
    { expect: 'TestCode/ok_check', old: 'ok', new: 'odd', cmd: 'true' },
  ]);
  assert.strictEqual(r.code, 1, r.out);
  assert.deepStrictEqual(r.reds, [], r.out);
  assert.doesNotMatch(r.out, /RED lines/);
  assert.match(r.out.trimEnd().split('\n').at(-1), /^SURVIVED: 2 of 2/, r.out);
});
