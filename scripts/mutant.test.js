// scripts/mutant.sh, run on a scratch file with small shell commands as the
// test command: its verdict and exit code, and the file restored after.
//
//   node --test scripts/mutant.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const MUTANT = path.join(__dirname, 'mutant.sh');

// run applies the mutant "ok" → "bad" to a scratch file holding "ok", with
// the shell command cmd as the test command (the file's path is $F in it),
// and returns mutant.sh's exit code, its output and the file afterwards.
function run(cmd, { expect } = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mutant-test-'));
  const file = path.join(dir, 'code.txt');
  fs.writeFileSync(file, 'ok\n');
  const args = [...(expect ? ['--expect', expect] : []), file, 'ok', 'bad', '--', 'bash', '-c', cmd];
  const r = spawnSync('bash', [MUTANT, ...args], { encoding: 'utf8', env: { ...process.env, F: file } });
  const after = fs.readFileSync(file, 'utf8');
  fs.rmSync(dir, { recursive: true });
  return { code: r.status, out: r.stdout + r.stderr, after };
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

test('every verdict is the last line, so a tail of the output keeps it', () => {
  const cases = {
    red: [CHECK, {}],
    survived: ['true', {}],
    'already red': ['echo "--- FAIL: TestCode/broken"; exit 1', {}],
    'did not run': ['grep -q ok "$F" || no-such-command-for-mutant-test', {}],
    'red elsewhere': [CHECK, { expect: 'TestCode/other_check' }],
  };
  for (const [name, [cmd, opts]] of Object.entries(cases)) {
    const last = run(cmd, opts).out.trimEnd().split('\n').at(-1);
    assert.match(last, /^(RED|SURVIVED|BAD INPUT):/, `${name}: last line ${JSON.stringify(last)}`);
  }
});
