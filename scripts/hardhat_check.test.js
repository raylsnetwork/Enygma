// Exercise make hardhat-check's test-count rules with the commands replaced
// by shims, so page-check needs neither npm installs nor a Solidity compiler.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { makeEnv } = require('./test_git');

const PROJECT = 'enygma_dvp';
const EMPTY = '  0 passing (0ms)';
const TESTED = '  3 passing (1s)';

function hardhatCheck(output, { compileOnly = '', testStatus = 0 } = {}) {
  const bin = fs.mkdtempSync(path.join(os.tmpdir(), 'hardhat-check-'));
  try {
    fs.writeFileSync(path.join(bin, 'npm'), '#!/bin/sh\nexit 0\n', { mode: 0o755 });
    fs.writeFileSync(path.join(bin, 'npx'), `#!/bin/sh
case "$*" in
  "hardhat compile") exit 0 ;;
  "hardhat test") printf '%s\\n' "$TEST_HARDHAT_OUTPUT"; exit "$TEST_HARDHAT_STATUS" ;;
  *) exit 2 ;;
esac
`, { mode: 0o755 });
    const r = spawnSync('make', ['--no-print-directory', 'hardhat-check',
      `HARDHAT_PROJECTS=${PROJECT}`, `HARDHAT_COMPILE_ONLY=${compileOnly}`], {
      cwd: path.join(__dirname, '..'),
      encoding: 'utf8',
      timeout: 10000,
      env: {
        ...makeEnv,
        PATH: `${bin}:${process.env.PATH}`,
        TEST_HARDHAT_OUTPUT: output,
        TEST_HARDHAT_STATUS: String(testStatus),
      },
    });
    assert.ifError(r.error);
    return { status: r.status, output: r.stdout + r.stderr };
  } finally {
    fs.rmSync(bin, { recursive: true, force: true });
  }
}

test('hardhat-check passes when a project runs tests', () => {
  const r = hardhatCheck(TESTED);
  assert.equal(r.status, 0, r.output);
  assert.match(r.output, /3 passing \(1s\)/);
});

test('hardhat-check fails when a project unexpectedly runs no tests', () => {
  const r = hardhatCheck(EMPTY);
  assert.notEqual(r.status, 0, r.output);
  assert.match(r.output, /enygma_dvp: Hardhat ran no tests/);
});

test('hardhat-check allows an empty test run for a compile-only project', () => {
  const r = hardhatCheck(EMPTY, { compileOnly: PROJECT });
  assert.equal(r.status, 0, r.output);
  assert.match(r.output, /0 passing \(0ms\)/);
});

test('hardhat-check refuses a compile-only entry that has passing tests', () => {
  const r = hardhatCheck(TESTED, { compileOnly: PROJECT });
  assert.notEqual(r.status, 0, r.output);
  assert.match(r.output, /enygma_dvp: has Hardhat tests, remove it from HARDHAT_COMPILE_ONLY/);
});

test('hardhat-check refuses a compile-only entry that has pending tests', () => {
  const r = hardhatCheck(`${EMPTY}\n  1 pending`, { compileOnly: PROJECT });
  assert.notEqual(r.status, 0, r.output);
  assert.match(r.output, /enygma_dvp: has Hardhat tests, remove it from HARDHAT_COMPILE_ONLY/);
});

test('hardhat-check rejects an unknown compile-only entry', () => {
  const r = hardhatCheck(TESTED, { compileOnly: 'missing_project' });
  assert.notEqual(r.status, 0, r.output);
  assert.match(r.output, /HARDHAT_COMPILE_ONLY names projects outside HARDHAT_PROJECTS: missing_project/);
  assert.doesNotMatch(r.output, /==>/);
});

test('hardhat-check rejects a missing or ambiguous Hardhat test summary', () => {
  for (const output of ['nothing to run', `${EMPTY}\n${EMPTY}`, `${EMPTY}\n  1 pending\n  2 pending`]) {
    const r = hardhatCheck(output, { compileOnly: PROJECT });
    assert.notEqual(r.status, 0, r.output);
    assert.match(r.output, /enygma_dvp: cannot read the Hardhat test summary/);
  }
});

test('hardhat-check propagates a failing Hardhat test command', () => {
  const r = hardhatCheck(TESTED, { testStatus: 1 });
  assert.notEqual(r.status, 0, r.output);
  assert.match(r.output, /FAILED: enygma_dvp/);
});
