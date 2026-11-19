// make known-red-check (scripts/known_red_check.sh), on a scratch repo with
// the repo's Makefile and two Go modules: a TEST_SKIP_PACKAGES entry must
// still fail its tests and name a package some tracked module builds. And
// make test-unit, with no entries, still runs every package.
//
//   node --test scripts/known_red_check.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');
const { GIT_ENV, git, makeEnv, scratchRepo } = require('./test_git');

const ROOT = path.join(__dirname, '..');
const GO_VARS = { GOFLAGS: '', GOWORK: 'off', GOTOOLCHAIN: 'local' };
const goEnv = { ...GIT_ENV, ...GO_VARS };

const testFile = (pkg, body) => `package ${pkg}\n\nimport "testing"\n\nfunc TestIt(t *testing.T) { ${body} }\n`;

// Module example/m (dir m) has a package whose test passes and one whose test
// fails; module example/m/nested (dir m/nested) sits inside it, and module
// other (dir other) beside it.
const FILES = {
  'm/go.mod': 'module example/m\n\ngo 1.21\n',
  'm/green/green_test.go': testFile('green', ''),
  'm/red/red_test.go': testFile('red', 't.Fatal("known red")'),
  'm/broken/broken_test.go': testFile('broken', 'undefined()'),
  'm/nested/go.mod': 'module example/m/nested\n\ngo 1.21\n',
  'm/nested/green/green_test.go': testFile('green', ''),
  'other/go.mod': 'module other\n\ngo 1.21\n',
  'other/green/green_test.go': testFile('green', ''),
};

const at = (dir, f) => path.join(dir, f);

// make runs make with args in a fresh scratch repo holding FILES, the
// Makefile and the script, then removes it; it returns the exit code and the
// output.
function make(...args) {
  const dir = scratchRepo('known-red-check-test');
  try {
    Object.entries(FILES).forEach(([f, c]) => {
      fs.mkdirSync(path.dirname(at(dir, f)), { recursive: true });
      fs.writeFileSync(at(dir, f), c);
    });
    fs.cpSync(at(ROOT, 'Makefile'), at(dir, 'Makefile'));
    // Copied, not rewritten, so it keeps its mode.
    fs.cpSync(at(__dirname, 'known_red_check.sh'), at(dir, 'scripts/known_red_check.sh'));
    git(dir, 'add', '-A');
    git(dir, 'commit', '-qm', 'init');
    const r = spawnSync('make', ['--no-print-directory', ...args], { cwd: dir, encoding: 'utf8', env: { ...makeEnv, ...GO_VARS } });
    return { code: r.status, out: r.stdout + r.stderr };
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

const check = (pkgs, ...args) => make('known-red-check', `TEST_SKIP_PACKAGES=${pkgs}`, ...args);

const passes = (r) => assert.strictEqual(r.code, 0, r.out);
const fails = (r, re) => {
  assert.notStrictEqual(r.code, 0, r.out);
  assert.match(r.out, re);
};

test('passes on an entry whose tests fail', () => {
  const r = check('example/m/red');
  passes(r);
  assert.match(r.out, /known red: example\/m\/red still fails its tests/);
  assert.match(r.out, /^ +--- FAIL: TestIt/m);
});

test('fails on an entry whose package does not build: its tests did not run, so are not known red', () => {
  fails(check('example/m/broken'), /entry example\/m\/broken did not run its tests \(in m\)[^]*undefined/);
});

test("fails on a pattern entry, which test-unit's filter never matches", () => {
  fails(check('example/m/...'), /entry example\/m\/\.\.\. names no package in m/);
});

test('fails on an entry whose tests pass, naming it to delete from the Makefile', () => {
  fails(check('example/m/green'), /entry example\/m\/green passes its tests \(in m\): delete it, and its comment, from the Makefile/);
});

test('with a red and a green entry, fails naming the green one only', () => {
  const r = check('example/m/red example/m/green');
  fails(r, /entry example\/m\/green passes its tests/);
  assert.doesNotMatch(r.out, /example\/m\/red passes/);
});

test('fails on an entry no tracked module owns (a module path renamed since)', () => {
  fails(check('enygma-server/pkg/circuits/withdraw'), /entry enygma-server\/pkg\/circuits\/withdraw names no package: no tracked module's path is a prefix of it/);
});

test('fails on an entry in a tracked module that names no package of it', () => {
  fails(check('example/m/gone'), /entry example\/m\/gone names no package in m \(go list prints no package/);
});

test('fails on a tracked module path that is a prefix of an entry only as a string', () => {
  fails(check('example/mx/red'), /entry example\/mx\/red names no package: no tracked module's path is a prefix/);
});

test("runs an entry from the nested module that owns it, not the enclosing one's", () => {
  fails(check('example/m/nested/green'), /entry example\/m\/nested\/green passes its tests \(in m\/nested\)/);
});

test("with MODULE, runs only that module's entries", () => {
  const r = check('example/m/green', 'MODULE=other');
  passes(r);
  assert.match(r.out, /known red: example\/m\/green is in m, not run here/);
});

test('with MODULE, still fails on an entry no tracked module owns', () => {
  fails(check('enygma-server/x', 'MODULE=other'), /entry enygma-server\/x names no package/);
});

test('passes, saying so, with no entries', () => {
  const r = check('');
  passes(r);
  assert.match(r.out, /known red: no TEST_SKIP_PACKAGES entries to check/);
});

test('the script, given no module list, fails printing its usage', () => {
  const r = spawnSync('bash', [at(__dirname, 'known_red_check.sh')], { encoding: 'utf8', env: goEnv });
  assert.strictEqual(r.status, 2, r.stdout + r.stderr);
  assert.match(r.stderr, /usage: scripts\/known_red_check\.sh/);
});

test('make test-unit with no entries runs every package (an empty skip list filters none out)', () => {
  const r = make('test-unit', 'TEST_SKIP_PACKAGES=', 'TEST_SKIP_MODULES=', 'MODULE=other');
  passes(r);
  assert.match(r.out, /^ok\s+other\/green\b/m);
});

test('make test-unit leaves out an entry, and runs the rest', () => {
  const r = make('test-unit', 'TEST_SKIP_PACKAGES=example/m/red example/m/broken', 'TEST_SKIP_MODULES=', 'MODULE=m');
  passes(r);
  assert.match(r.out, /^ok\s+example\/m\/green\b/m);
});

test('make check runs known-red-check', () => {
  const mk = fs.readFileSync(at(ROOT, 'Makefile'), 'utf8');
  const deps = /^check:([^#\n]*)/m.exec(mk);
  assert.ok(deps, 'the Makefile has no check target');
  assert.ok(deps[1].split(/\s+/).includes('known-red-check'), `check: ${deps[1]}`);
});

// Each go matrix leg checks its module's entries, with its LFS objects.
test("CI's go matrix runs make known-red-check MODULE=, checking out LFS", () => {
  const yml = fs.readFileSync(at(ROOT, '.github/workflows/ci.yml'), 'utf8');
  const job = /^  go:\n[^]*?(?=^  \S|(?![^]))/m.exec(yml);
  assert.ok(job, 'ci.yml has no go job');
  assert.match(job[0], /lfs: true/);
  assert.match(job[0], /make [^\n]*\bknown-red-check\b[^\n]*MODULE="\$MODULE"/);
});
