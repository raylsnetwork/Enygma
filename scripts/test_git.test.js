// scripts/test_git.js, the scripts' tests' git and make: its scratch repos
// read none of the machine's git config, its make environment none of a
// parent make's flags and variables, and every test under scripts/ runs git
// through it and make with its makeEnv.
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
// And what a parent make run as make check MODULE=<dir> 'A=x B=y' C:=z hands
// down (make 3.81's MAKEFLAGS, as on macOS): each command-line variable both
// in MAKEFLAGS and exported. B, set only in the environment, is no make
// variable of the parent's command line.
Object.assign(process.env, {
  MAKEFLAGS: 'A=x\\ B=y C:=z MODULE=parent', MFLAGS: '', MAKELEVEL: '1',
  A: 'x B=y', C: 'z', MODULE: 'parent', B: 'kept',
});
const { git, makeEnv, scratchRepo } = require('./test_git');
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

test("a make run with makeEnv sees none of a parent make's variables", () => {
  const dir = scratchRepo('test-git-make');
  try {
    fs.writeFileSync(path.join(dir, 'Makefile'), 'all:\n\t@echo "[$(MODULE)]"\n');
    // A make that inherits process.env really sees the parent's MODULE, so
    // the pass below is the helper's doing.
    const plain = spawnSync('make', ['--no-print-directory'], { cwd: dir, encoding: 'utf8' });
    assert.strictEqual(plain.stdout, '[parent]\n', plain.stdout + plain.stderr);
    const r = spawnSync('make', ['--no-print-directory'], { cwd: dir, encoding: 'utf8', env: makeEnv });
    assert.strictEqual(r.stdout, '[]\n', r.stdout + r.stderr);
    for (const v of ['MAKEFLAGS', 'MFLAGS', 'MAKELEVEL', 'MODULE', 'A', 'C']) assert.ok(!(v in makeEnv), v);
    assert.strictEqual(makeEnv.B, 'kept');
    assert.strictEqual(makeEnv.GIT_CONFIG_GLOBAL, '/dev/null');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

// directMakeCalls returns the text of each direct call of make in src:
// spawn/spawnSync/execFile/execFileSync with 'make' as the command, or
// exec/execSync of a string starting make, up to its closing parenthesis
// (counting every bracket, in strings too). It throws on a call that never
// closes, rather than taking the rest of the file as the call.
function directMakeCalls(src) {
  const calls = [];
  const start = /\b(?:spawn|spawnSync|execFile|execFileSync|exec|execSync)\(\s*['"`]make\b/g;
  for (let m; (m = start.exec(src));) {
    let depth = 0;
    let i = m.index + m[0].indexOf('(');
    for (; i < src.length; i++) {
      if (src[i] === '(') depth++;
      else if (src[i] === ')' && --depth === 0) break;
    }
    if (i === src.length) throw new Error(`a call of make never closes: ${src.slice(m.index, m.index + 60)}`);
    calls.push(src.slice(m.index, i + 1));
  }
  return calls;
}

test('the direct-make reader finds each direct call of make, whole', () => {
  // A reader that found none, or cut a call short of its env, would make the
  // next test pass vacuously.
  const src = "spawnSync('make', ['a', f(x)], { env: GIT_ENV });\nspawn(\n  \"make\", [], {\n    env: { ...makeEnv, A: g() },\n  });\nexecSync(`make b`); spawnSync('bash', [MAKE])";
  assert.deepStrictEqual(directMakeCalls(src), [
    "spawnSync('make', ['a', f(x)], { env: GIT_ENV })",
    'spawn(\n  "make", [], {\n    env: { ...makeEnv, A: g() },\n  })',
    'execSync(`make b`)',
  ]);
  assert.throws(() => directMakeCalls("spawnSync('make', [], {\n  env: GIT_ENV,\n// makeEnv"), /never closes/);
});

test('every test under scripts/ runs make with makeEnv from test_git.js', () => {
  const files = fs.readdirSync(__dirname).filter((f) => f.endsWith('.test.js') && f !== 'test_git.test.js');
  const calls = Object.fromEntries(files.map((f) => [f, directMakeCalls(fs.readFileSync(path.join(__dirname, f), 'utf8'))]));
  const withMake = files.filter((f) => calls[f].length > 0);
  assert.ok(withMake.length > 1, withMake);
  const bare = withMake.filter((f) => calls[f].some((c) => !/\bmakeEnv\b/.test(c)));
  assert.deepStrictEqual(bare, [], "these tests run make without makeEnv, so a parent make's MODULE reaches theirs: pass env: makeEnv (or { ...makeEnv, ... }) from test_git.js");
});
