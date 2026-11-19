// make errcheck-skip-check (scripts/errcheck_skip_check.sh), on a scratch
// repo with the repo's Makefile and three Go modules: an
// ERRCHECK_SKIP_MODULES entry must be a tracked module that errcheck still
// finds dropped errors in. errcheck itself is a stand-in (ERRCHECK=) that
// prints what errcheck v1.30.0 prints through go run: the pinned errcheck
// needs a newer go than the page-check job has.
//
//   node --test scripts/errcheck_skip_check.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');
const { GIT_ENV, git, makeEnv, scratchRepo } = require('./test_git');

const ROOT = path.join(__dirname, '..');
const at = (dir, f) => path.join(dir, f);

// The stand-in errcheck, run in a module with ./...: in a module holding
// FINDINGS it prints a finding and exits 1; in one holding BROKEN, the
// error errcheck prints on a build break, exiting 1 too (go run's exit for
// both); else nothing, exit 0. Any other arguments exit 3.
const FAKE_ERRCHECK = `#!/usr/bin/env bash
[ "$*" = "-exclude x ./..." ] || { echo "fake errcheck: args $*"; exit 3; }
if [ -f FINDINGS ]; then printf 'p.go:3:13:\\tfunc G() { F() }\\nexit status 1\\n'; exit 1; fi
if [ -f BROKEN ]; then printf 'error: failed to check packages: errors while loading package ex/broken: [-: # ex/broken\\n./p.go:2:12: undefined: undefined]\\nexit status 2\\n'; exit 1; fi
exit 0
`;

// Module dirs clean (errcheck-clean), dirty (with findings), broken (does
// not build), and dirty/nested inside dirty, clean.
const FILES = {
  'clean/go.mod': 'module ex/clean\n\ngo 1.21\n',
  'dirty/go.mod': 'module ex/dirty\n\ngo 1.21\n',
  'dirty/FINDINGS': '',
  'dirty/nested/go.mod': 'module ex/dirty/nested\n\ngo 1.21\n',
  'broken/go.mod': 'module ex/broken\n\ngo 1.21\n',
  'broken/BROKEN': '',
};

// make runs make with args in a fresh scratch repo holding FILES, the
// Makefile, the script and the stand-in errcheck, then removes it; it
// returns the exit code and the output.
function make(...args) {
  const dir = scratchRepo('errcheck-skip-check-test');
  try {
    Object.entries(FILES).forEach(([f, c]) => {
      fs.mkdirSync(path.dirname(at(dir, f)), { recursive: true });
      fs.writeFileSync(at(dir, f), c);
    });
    fs.cpSync(at(ROOT, 'Makefile'), at(dir, 'Makefile'));
    // Copied, not rewritten, so it keeps its mode.
    fs.cpSync(at(__dirname, 'errcheck_skip_check.sh'), at(dir, 'scripts/errcheck_skip_check.sh'));
    fs.writeFileSync(at(dir, 'fake_errcheck.sh'), FAKE_ERRCHECK, { mode: 0o755 });
    git(dir, 'add', '-A');
    git(dir, 'commit', '-qm', 'init');
    const r = spawnSync('make', ['--no-print-directory', ...args], { cwd: dir, encoding: 'utf8', env: makeEnv });
    return { code: r.status, out: r.stdout + r.stderr };
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

const check = (mods, ...args) => make('errcheck-skip-check', `ERRCHECK_SKIP_MODULES=${mods}`, 'ERRCHECK=$(CURDIR)/fake_errcheck.sh -exclude x', ...args);

const passes = (r) => assert.strictEqual(r.code, 0, r.out);
const fails = (r, re) => {
  assert.notStrictEqual(r.code, 0, r.out);
  assert.match(r.out, re);
};

test('passes on an entry errcheck still finds dropped errors in', () => {
  const r = check('dirty');
  passes(r);
  assert.match(r.out, /errcheck skip: dirty still has 1 errcheck findings/);
});

test('fails on an errcheck-clean entry, naming it to delete from the Makefile', () => {
  fails(check('clean'), /ERRCHECK_SKIP_MODULES entry clean is errcheck-clean: delete it from the Makefile/);
});

test('with a dirty and a clean entry, fails naming the clean one only', () => {
  const r = check('dirty clean');
  fails(r, /entry clean is errcheck-clean/);
  assert.doesNotMatch(r.out, /entry dirty is errcheck-clean/);
});

test('fails on an entry errcheck could not check (a build break): its findings are not known', () => {
  fails(check('broken'), /entry broken: errcheck did not check it[^]*undefined: undefined/);
});

test('fails on an entry that is not a tracked module (one renamed since)', () => {
  fails(check('enygma_dvp/gone'), /ERRCHECK_SKIP_MODULES entry enygma_dvp\/gone is not a tracked Go module/);
});

test("fails on an entry that is a tracked module's dir only as a prefix", () => {
  fails(check('dirty/nest'), /entry dirty\/nest is not a tracked Go module/);
});

test("runs a nested module's entry in that module, not the enclosing one", () => {
  fails(check('dirty/nested'), /entry dirty\/nested is errcheck-clean/);
});

test("with MODULE, runs only that module's entry", () => {
  const r = check('clean', 'MODULE=dirty');
  passes(r);
  assert.match(r.out, /errcheck skip: clean not run here/);
});

test('with MODULE, still fails on an entry that is not a tracked module', () => {
  fails(check('gone', 'MODULE=dirty'), /entry gone is not a tracked Go module/);
});

test('passes, saying so, with no entries', () => {
  const r = check('');
  passes(r);
  assert.match(r.out, /errcheck skip: no ERRCHECK_SKIP_MODULES entries to check/);
});

test('runs the errcheck make errcheck runs', () => {
  const r = make('-n', 'errcheck-skip-check', 'ERRCHECK_SKIP_MODULES=dirty');
  passes(r);
  const mk = fs.readFileSync(at(ROOT, 'Makefile'), 'utf8');
  assert.match(mk, /^errcheck:[^\n]*\n\t[^\n]*\$\(ERRCHECK\) \.\/\.\.\./m, 'make errcheck no longer runs $(ERRCHECK) ./...');
  assert.match(r.out, /errcheck_skip_check\.sh 'go run github\.com\/kisielk\/errcheck@v[\d.]+ -exclude \S+\/\.errcheck-exclude' /);
});

test('the script, given no module list, fails printing its usage', () => {
  const r = spawnSync('bash', [at(__dirname, 'errcheck_skip_check.sh'), 'errcheck'], { encoding: 'utf8', env: GIT_ENV });
  assert.strictEqual(r.status, 2, r.stdout + r.stderr);
  assert.match(r.stderr, /usage: scripts\/errcheck_skip_check\.sh/);
});

test('make check runs errcheck-skip-check', () => {
  const mk = fs.readFileSync(at(ROOT, 'Makefile'), 'utf8');
  const deps = /^check:([^#\n]*)/m.exec(mk);
  assert.ok(deps, 'the Makefile has no check target');
  assert.ok(deps[1].split(/\s+/).includes('errcheck-skip-check'), `check: ${deps[1]}`);
});

// Each go matrix leg checks its module's entry, with its go.mod's go.
test("CI's go matrix runs make errcheck-skip-check MODULE=", () => {
  const yml = fs.readFileSync(at(ROOT, '.github/workflows/ci.yml'), 'utf8');
  const job = /^  go:\n[^]*?(?=^  \S|(?![^]))/m.exec(yml);
  assert.ok(job, 'ci.yml has no go job');
  assert.match(job[0], /make [^\n]*\berrcheck-skip-check\b[^\n]*MODULE="\$MODULE"/);
});
