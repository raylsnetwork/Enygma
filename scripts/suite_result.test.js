// scripts/suite_result.sh, run on a canned `go test -v` log: the counts it
// prints, and when it fails a suite (a failed test, a skip not on the
// suite's known-skip list, or go test's own non-zero exit).
//
//   node --test scripts/suite_result.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const SCRIPT = path.join(__dirname, 'suite_result.sh');

// run writes log to a scratch file and runs the checker on it with go test's
// exit status and the known skips; it returns the exit code and output.
function run(log, { status = 0, known = '', cwd } = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'suite-result-test-'));
  const file = path.join(dir, 'suite.log');
  fs.writeFileSync(file, log);
  const r = spawnSync('bash', [SCRIPT, file, String(status), known], { encoding: 'utf8', cwd });
  fs.rmSync(dir, { recursive: true });
  return { code: r.status, out: r.stdout + r.stderr };
}

const PASSING = [
  '=== RUN   TestA', '--- PASS: TestA (0.01s)',
  '=== RUN   TestB', '--- PASS: TestB (0.02s)',
  '    --- PASS: TestB/sub (0.01s)',
  'PASS', 'ok  \tenygma_test\t1.2s', '',
].join('\n');

test('a passing suite prints its top-level counts and passes', () => {
  const r = run(PASSING);
  assert.strictEqual(r.code, 0, r.out);
  assert.match(r.out, /^suite PASS: 2$/m);
  assert.match(r.out, /^suite FAIL: 0$/m);
  assert.match(r.out, /^suite SKIP: 0$/m);
});

test('a suite that ran no tests fails, though go test exited 0', () => {
  // A build tag, a renamed file or a TestMain that exits early: go test is happy.
  const r = run('testing: warning: no tests to run\nPASS\nok  \tenygma_test\t0.01s [no tests to run]\n');
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /^no test passed: the suite ran no tests$/m);
});

test('a failed test fails the suite, and is listed', () => {
  // go test's status is 0 here, so the FAIL line alone must fail the suite.
  const r = run('--- PASS: TestA (0s)\n--- FAIL: TestB (0s)\nFAIL\n');
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /^suite FAIL: 1$/m);
  assert.match(r.out, /^--- FAIL: TestB/m);
});

test('go test failing with no failed test (a build error) fails the suite', () => {
  const r = run('# enygma_test\n./x_test.go:3:1: syntax error\nFAIL\tenygma_test [build failed]\n', { status: 1 });
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /go test exited 1/);
});

test('a skip not on the known-skip list fails the suite, and is named', () => {
  const r = run('--- PASS: TestA (0s)\n--- SKIP: TestB (0s)\n    x_test.go:9: gnark server not reachable\n');
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /^suite SKIP: 1$/m);
  assert.match(r.out, /unexpected skip: TestB$/m);
});

test('a skip on the known-skip list passes', () => {
  const r = run('--- PASS: TestA (0s)\n--- SKIP: TestB (0s)\n--- SKIP: TestC (0s)\n', { known: 'TestC TestB' });
  assert.strictEqual(r.code, 0, r.out);
  assert.match(r.out, /^suite SKIP: 2$/m);
});

test('a known skip matches the whole test name, not a prefix', () => {
  const r = run('--- SKIP: TestBravo (0s)\n', { known: 'TestB' });
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /unexpected skip: TestBravo$/m);
});

test('a skipped subtest counts against the list by its full name', () => {
  const log = '--- PASS: TestA (0s)\n    --- SKIP: TestA/flake (0s)\n';
  assert.strictEqual(run(log).code, 1);
  assert.strictEqual(run(log, { known: 'TestA/flake' }).code, 0);
});

test('a skipped subtest name with glob characters is matched as written', () => {
  // Go keeps * and [ ] in subtest names; run where they match a file.
  const cwd = fs.mkdtempSync(path.join(os.tmpdir(), 'suite-result-glob-'));
  fs.mkdirSync(path.join(cwd, 'TestF'));
  fs.writeFileSync(path.join(cwd, 'TestF', 'amount_1'), '');
  const log = '--- PASS: TestF (0s)\n    --- SKIP: TestF/amount_* (0s)\n';
  const known = run(log, { known: 'TestF/amount_*', cwd });
  const unknown = run(log, { cwd });
  fs.rmSync(cwd, { recursive: true });
  assert.strictEqual(known.code, 0, known.out);
  assert.match(unknown.out, /^unexpected skip: TestF\/amount_\*$/m);
});
