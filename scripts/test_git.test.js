// scripts/test_git.js, the scripts' tests' git: its scratch repos read none
// of the machine's git config, and every test under scripts/ runs git
// through it.
//
//   node --test scripts/test_git.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

// A global config whose pre-commit hook refuses every commit, set before
// test_git.js is loaded, as a developer's ~/.gitconfig would be.
const home = fs.mkdtempSync(path.join(os.tmpdir(), 'test-git-home-'));
fs.mkdirSync(path.join(home, 'hooks'));
fs.writeFileSync(path.join(home, 'hooks', 'pre-commit'), '#!/bin/sh\nexit 1\n', { mode: 0o755 });
fs.writeFileSync(path.join(home, 'gitconfig'), `[core]\n\thooksPath = ${path.join(home, 'hooks')}\n`);
process.env.GIT_CONFIG_GLOBAL = path.join(home, 'gitconfig');
const { git, scratchRepo } = require('./test_git');
test.after(() => fs.rmSync(home, { recursive: true, force: true }));

test("a scratch repo's commit runs no hook from the global git config", () => {
  const dir = scratchRepo('test-git');
  try {
    fs.writeFileSync(path.join(dir, 'f'), 'x\n');
    git(dir, 'add', 'f');
    // The hook really refuses a commit that reads the global config, so the
    // pass below is the helper's doing.
    const plain = spawnSync('git', ['-c', 'user.name=t', '-c', 'user.email=t@t', 'commit', '-qm', 'plain'], { cwd: dir, encoding: 'utf8' });
    assert.notStrictEqual(plain.status, 0, 'the global pre-commit hook let a plain git commit through');
    git(dir, 'commit', '-qm', 'helper');
    assert.strictEqual(git(dir, 'log', '--format=%s'), 'helper');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

// A direct call of git from node: spawn/spawnSync/execFile/execFileSync with
// 'git' (or "git") as the command, or exec/execSync of a string starting git.
const DIRECT_GIT = /\b(?:spawn|spawnSync|execFile|execFileSync|exec|execSync)\(\s*['"`]git\b/;

test('the direct-git reader finds a direct call of git', () => {
  // A reader that found none would make the next test pass vacuously.
  for (const s of ["spawnSync('git', ['init'])", 'spawn("git", a)', 'execSync(`git log`)', "execFileSync( 'git', [])"]) {
    assert.match(s, DIRECT_GIT);
  }
  assert.doesNotMatch("spawnSync('bash', [SCRIPT])", DIRECT_GIT);
});

test("every test under scripts/ runs git through test_git.js", () => {
  const files = fs.readdirSync(__dirname).filter((f) => f.endsWith('.test.js'));
  assert.ok(files.length > 1, files);
  const direct = files.filter((f) => f !== 'test_git.test.js' && DIRECT_GIT.test(fs.readFileSync(path.join(__dirname, f), 'utf8')));
  assert.deepStrictEqual(direct, [], 'these tests run git themselves: use git, scratchRepo or GIT_ENV from test_git.js');
});
