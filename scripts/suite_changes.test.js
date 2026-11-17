// scripts/suite_changes.sh, which the onchain workflow asks which on-chain
// suites an event runs: on a pull request, those whose paths its changed
// files touch; on a push to main (--all), every suite.
//
//   node --test scripts/suite_changes.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const path = require('path');
const { spawnSync } = require('child_process');

const SCRIPT = path.join(__dirname, 'suite_changes.sh');
// Every suite, in the order the script prints them.
const ALL = ['payments', 'dvp', 'retail'];

// suites runs the script on the changed files (one per line on stdin) and
// returns the suites it runs, by name, in the order it prints them.
function suites(files, args = []) {
  const r = spawnSync('bash', [SCRIPT, ...args], { encoding: 'utf8', input: files.map((f) => f + '\n').join('') });
  assert.strictEqual(r.status, 0, r.stdout + r.stderr);
  const out = {};
  for (const line of r.stdout.trim().split('\n')) {
    const [k, v] = line.split('=');
    out[k] = v;
  }
  assert.deepStrictEqual(Object.keys(out), ALL, r.stdout);
  return Object.keys(out).filter((k) => out[k] === 'true');
}

test('a docs-only change runs no suite', () => {
  assert.deepStrictEqual(suites([
    'README.md',
    'docs/adr/0007-onchain-suites.md',
    'enygma_dvp/docs/flows/03_dvp.md',
    'enygma_payments/README.md',
    'enygma_retail_payments/shield_demo/HAND_CHECK.md',
  ]), []);
});

test('a demo page change runs no suite', () => {
  assert.deepStrictEqual(suites([
    'enygma_retail_payments/shield_demo/index.html',
    'enygma_retail_payments/shield_demo/protocol.html',
    'enygma_payments/demo/bank.html',
    'enygma_dvp/demo/index.html',
  ]), []);
});

test("a change to enygma_dvp's contracts runs the DvP and retail suites, not payments", () => {
  assert.deepStrictEqual(suites(['enygma_dvp/contracts/EnygmaDvp.sol']), ['dvp', 'retail']);
});

test('a change to enygma_dvp outside what the retail stack uses runs the DvP suite only', () => {
  assert.deepStrictEqual(suites(['enygma_dvp/relayer/main.go']), ['dvp']);
});

test('a change to enygma_payments runs the payments suite only', () => {
  assert.deepStrictEqual(suites(['enygma_payments/contracts/enygma/contracts/Enygma.sol']), ['payments']);
});

test('a change to the onchain workflow runs every suite', () => {
  assert.deepStrictEqual(suites(['.github/workflows/onchain.yml']), ALL);
});

test("a change to the root Makefile's stack or suite targets runs every suite", () => {
  assert.deepStrictEqual(suites(['Makefile']), ALL);
});

test('a push to main runs every suite, whatever it changes', () => {
  assert.deepStrictEqual(suites([], ['--all']), ALL);
});

for (const script of [
  'enygma_payments/run_scripts/stack.sh',
  'enygma_dvp/scripts/stack.sh',
  'enygma_retail_payments/shield_demo/run.sh',
]) {
  test(`a change to the stack script ${script} runs every suite`, () => {
    assert.deepStrictEqual(suites([script]), ALL);
  });
}

for (const script of ['scripts/suite_result.sh', 'scripts/suite_changes.sh', 'scripts/stack_lib.sh', 'scripts/stack_stop.sh']) {
  test(`a change to the suites' wiring ${script} runs every suite`, () => {
    assert.deepStrictEqual(suites([script]), ALL);
  });
}
