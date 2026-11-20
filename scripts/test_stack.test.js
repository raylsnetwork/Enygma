// scripts/test_stack.js, the stack scripts' tests' fake-stack helpers: its
// one service-list reader yields only pids it is safe to signal as a group,
// and its killGroup signals none that is not. Neither test can kill what
// runs it: the reader's only reads, and killGroup's runs in a detached
// child, so a killGroup that signals group 0 kills that child alone.
//
//   node --test scripts/test_stack.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');
const { servicePid } = require('./test_stack');

// Signalling group 0 reaches the test's own group, 1 (as -1) every process
// the user owns; the rest are not pids at all.
test('the service-list reader never yields a pid that is not an integer above 1', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'test-stack-test-'));
  try {
    const bad = { zero: '0', one: '1', negative: '-5', word: 'abc', fraction: '1.5', hex: '0x10', none: '' };
    const lines = Object.entries(bad).map(([n, p]) => `${n} ${p}`);
    fs.writeFileSync(path.join(dir, 'services.pids'), [...lines, 'web 4242', 'bare', ''].join('\n'));
    for (const name of [...Object.keys(bad), 'bare']) assert.strictEqual(servicePid(dir, name), undefined, `${name}'s pid was read`);
    assert.strictEqual(servicePid(dir, 'web'), 4242);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

// A missing or garbled pid file reads as false or NaN, and -false is group
// 0: the caller's own. 1 and -1 are left out: a killGroup that signalled
// them would reach every process the user owns, the child's group or not.
test('killGroup signals no group for a pid that is not an integer above 1', () => {
  const lib = JSON.stringify(path.join(__dirname, 'test_stack.js'));
  const r = spawnSync(process.execPath, ['-e',
    `const { killGroup } = require(${lib});\nfor (const p of [0, -0, false, NaN, 1.5, undefined]) killGroup(p);\nconsole.log('survived');`],
  { detached: true, encoding: 'utf8', timeout: 10000 });
  assert.strictEqual(r.signal, null, `killGroup killed its own group: ${r.stderr}`);
  assert.strictEqual(r.stdout.trim(), 'survived', r.stderr);
});
