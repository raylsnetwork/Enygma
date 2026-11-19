// make test-name-skip-check (scripts/test_name_skip_check.sh), on a scratch
// repo with the repo's Makefile: each entry of a suite's test-name list
// (PRIMER_REFRESH_SKIP and the *_RUN lists, go test -skip and -run regexes,
// and the *_KNOWN_SKIPS allowed skips) must name a func TestX( in a tracked
// _test.go of the list's module (the *_SUITE dir beside it), outside a
// module nested in it.
// The lists and dirs are set on make's command line.
//
//   node --test scripts/test_name_skip_check.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');
const { GIT_ENV, git, makeEnv, scratchRepo } = require('./test_git');

const ROOT = path.join(__dirname, '..');
const at = (dir, f) => path.join(dir, f);
const MK = fs.readFileSync(at(ROOT, 'Makefile'), 'utf8');

// A suite module, suite: TestReal and TestOther at its root, TestInSub in a
// package below it; TestNested in a module nested in it; TestNotATest in a
// file that is not a _test.go; and other, a second module.
const FILES = {
  'suite/go.mod': 'module ex/suite\n\ngo 1.21\n',
  'suite/a_test.go': 'package suite\n\nimport "testing"\n\nfunc TestReal(t *testing.T) {}\n\nfunc TestOther(t *testing.T) {}\n\nfunc helperTest(t *testing.T) {}\n',
  'suite/sub/b_test.go': 'package sub\n\nimport "testing"\n\nfunc TestInSub(t *testing.T) {}\n',
  'suite/nested/go.mod': 'module ex/suite/nested\n\ngo 1.21\n',
  'suite/nested/c_test.go': 'package nested\n\nimport "testing"\n\nfunc TestNested(t *testing.T) {}\n',
  'suite/p.go': 'package suite\n\nimport "testing"\n\nfunc TestNotATest(t *testing.T) {}\n',
  'other/go.mod': 'module ex/other\n\ngo 1.21\n',
  'other/o_test.go': 'package other\n\nimport "testing"\n\nfunc TestInOther(t *testing.T) {}\n',
};
// Written after the commit: on disk, not tracked.
const UNTRACKED = { 'suite/u_test.go': 'package suite\n\nimport "testing"\n\nfunc TestUntracked(t *testing.T) {}\n' };

// Every list empty, each in module suite: a test sets the one it checks.
const LISTS = {
  DVP_SUITE: 'suite', DVP_KNOWN_SKIPS: '',
  PAYMENTS_SUITE: 'suite', PAYMENTS_KNOWN_SKIPS: '',
  RETAIL_SUITE: 'suite', RETAIL_KNOWN_SKIPS: '', PRIMER_REFRESH_SKIP: '',
  PRIMER_REFRESH_RUN: '', SHIELD_FIXTURES_RUN: '', SHIELD_TEST_RUN: '',
};

// check runs make test-name-skip-check in a fresh scratch repo holding
// FILES, UNTRACKED, the Makefile and the script, with LISTS and vars set on
// its command line, then removes it; it returns the exit code and output.
function check(vars) {
  const dir = scratchRepo('test-name-skip-check-test');
  try {
    const write = (files) => Object.entries(files).forEach(([f, c]) => {
      fs.mkdirSync(path.dirname(at(dir, f)), { recursive: true });
      fs.writeFileSync(at(dir, f), c);
    });
    write(FILES);
    fs.cpSync(at(ROOT, 'Makefile'), at(dir, 'Makefile'));
    // Copied, not rewritten, so it keeps its mode.
    fs.cpSync(at(__dirname, 'test_name_skip_check.sh'), at(dir, 'scripts/test_name_skip_check.sh'));
    git(dir, 'add', '-A');
    git(dir, 'commit', '-qm', 'init');
    write(UNTRACKED);
    const args = Object.entries({ ...LISTS, ...vars }).map(([k, v]) => `${k}=${v}`);
    const r = spawnSync('make', ['--no-print-directory', 'test-name-skip-check', ...args], { cwd: dir, encoding: 'utf8', env: makeEnv });
    return { code: r.status, out: r.stdout + r.stderr };
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

const passes = (r) => assert.strictEqual(r.code, 0, r.out);
const fails = (r, re) => {
  assert.notStrictEqual(r.code, 0, r.out);
  assert.match(r.out, re);
};

test('passes on allowed skips that each name a test, at the module root or in a package below it', () => {
  const r = check({ DVP_KNOWN_SKIPS: 'TestReal TestInSub' });
  passes(r);
  assert.match(r.out, /test names: DVP_KNOWN_SKIPS: 2 entries, each a test in suite/);
});

for (const list of ['DVP_KNOWN_SKIPS', 'PAYMENTS_KNOWN_SKIPS', 'RETAIL_KNOWN_SKIPS']) {
  test(`fails on a ${list} entry that names no test, naming the entry and the list`, () => {
    const r = check({ [list]: 'TestReal TestGone' });
    fails(r, new RegExp(`${list} entry TestGone names no test in suite`));
    assert.doesNotMatch(r.out, /entry TestReal names no test/);
  });
}

test("checks each list against its own suite's module", () => {
  passes(check({ PAYMENTS_SUITE: 'other', PAYMENTS_KNOWN_SKIPS: 'TestInOther' }));
  fails(check({ PAYMENTS_SUITE: 'other', PAYMENTS_KNOWN_SKIPS: 'TestReal' }), /PAYMENTS_KNOWN_SKIPS entry TestReal names no test in other/);
});

test('passes on a -skip regex whose alternatives each name a test, a trailing $ dropped', () => {
  const r = check({ PRIMER_REFRESH_SKIP: 'TestReal|TestOther$$' });
  passes(r);
  assert.match(r.out, /test names: PRIMER_REFRESH_SKIP: 2 entries/);
});

test('fails on a -skip alternative that names no test (a renamed test)', () => {
  const r = check({ PRIMER_REFRESH_SKIP: 'TestReal|TestGone|TestOther' });
  fails(r, /PRIMER_REFRESH_SKIP entry TestGone names no test in suite/);
  assert.doesNotMatch(r.out, /entry Test(Real|Other) names no test/);
});

for (const list of ['PRIMER_REFRESH_RUN', 'SHIELD_FIXTURES_RUN', 'SHIELD_TEST_RUN']) {
  test(`fails on a ${list} -run alternative that names no test (a renamed test runs nothing)`, () => {
    passes(check({ [list]: 'TestReal$$|TestOther$$' }));
    const r = check({ [list]: 'TestReal$$|TestGone$$' });
    fails(r, new RegExp(`${list} entry TestGone names no test in suite`));
    assert.doesNotMatch(r.out, /entry TestReal names no test/);
  });
}

test('fails on a -skip alternative that is a pattern, not a test name', () => {
  fails(check({ PRIMER_REFRESH_SKIP: 'TestRe.*' }), /PRIMER_REFRESH_SKIP entry TestRe\.\* names no test/);
});

test('fails on an empty alternative, which matches every test', () => {
  for (const re of ['|TestReal', 'TestReal|', 'TestReal||TestOther']) {
    fails(check({ PRIMER_REFRESH_SKIP: re }), /PRIMER_REFRESH_SKIP has an empty alternative, which matches every test/);
    fails(check({ SHIELD_FIXTURES_RUN: re }), /SHIELD_FIXTURES_RUN has an empty alternative, which matches every test/);
  }
});

test('fails on a name that is a func only outside a _test.go, in a nested module, or in an untracked file', () => {
  for (const name of ['TestNotATest', 'TestNested', 'TestUntracked', 'helperTest']) {
    fails(check({ RETAIL_KNOWN_SKIPS: name }), new RegExp(`RETAIL_KNOWN_SKIPS entry ${name} names no test in suite`));
  }
});

test('fails on a list whose module is not a tracked module', () => {
  for (const dir of ['gone', 'suit', 'suite/sub']) {
    fails(check({ DVP_SUITE: dir, DVP_KNOWN_SKIPS: 'TestReal' }), new RegExp(`DVP_KNOWN_SKIPS's module ${dir} is not a tracked Go module`));
  }
});

test('passes, saying so, on empty lists', () => {
  const r = check({});
  passes(r);
  assert.match(r.out, /test names: PAYMENTS_KNOWN_SKIPS: no entries/);
  assert.match(r.out, /test names: PRIMER_REFRESH_SKIP: no entries/);
});

test('the script fails printing its usage on arguments not in fours, or an unknown kind', () => {
  for (const args of [[], ['names', 'L', 'suite'], ['glob', 'L', 'suite', 'TestReal']]) {
    const r = spawnSync('bash', [at(__dirname, 'test_name_skip_check.sh'), ...args], { encoding: 'utf8', env: GIT_ENV });
    assert.strictEqual(r.status, 2, r.stdout + r.stderr);
    assert.match(r.stderr, /usage: scripts\/test_name_skip_check\.sh/);
  }
});

// goTestFilters returns each go test -skip or -run (or -test.skip,
// -test.run) a recipe of mk passes, as { target, flag, list, dir }: list is
// the LIST of its '$(LIST)' value (a -run may add subtests after a /), dir
// the last cd $(DIR) before it in the same recipe, or undefined. A recipe is
// the tab lines after a non-tab line, and the lines a \ continues. Comments
// are skipped. It throws on a literal value, and on a flag it cannot place
// in a recipe (in a define body, a variable, or a $(call ...)'s argument),
// so a filter in a shape it does not read fails rather than goes unchecked.
function goTestFilters(mk) {
  const out = [];
  let target, dir, continued = false;
  for (const raw of mk.split('\n')) {
    const recipe = raw.startsWith('\t') || continued;
    continued = recipe && raw.endsWith('\\');
    if (!recipe) {
      target = /^([\w.-]+):/.exec(raw)?.[1];
      dir = undefined;
    }
    const line = recipe ? (/^\s*@?#/.test(raw) ? '' : raw) : raw.replace(/#.*/, '');
    for (const m of line.matchAll(/cd \$\((\w+)\)|(?<=^|[\s,;'"(])(-(?:test\.)?(?:run|skip))\b(.*)/g)) {
      if (m[1]) { dir = m[1]; continue; }
      const [, , flag, rest] = m;
      if (!recipe || /,$/.test(line.slice(0, m.index))) {
        throw new Error(`cannot place ${flag} in a recipe: ${raw.trim()}`);
      }
      const v = /^[ =]'\$\((\w+)\)(?:'|\/)/.exec(rest);
      if (!v) {
        throw new Error(`${target} passes ${flag}${rest.split(/\s/, 2).join(' ')}, a literal regex: set it in a list beside its suite, check it in test-name-skip-check, pass ${flag} '$(LIST)'`);
      }
      out.push({ target, flag, list: v[1], dir });
    }
  }
  return out;
}

test('goTestFilters reads each filter\'s list and its own recipe\'s cd, through a continued line', () => {
  const mk = "a: ## the -run regex\n\t@# go test -run 'TestX'\n\tcd $(D1); \\\n  go test -run '$(R1)/($(T))' .\nb:\n\tgo test -count=1 -skip '$(S1)' .\n";
  assert.deepStrictEqual(goTestFilters(mk), [
    { target: 'a', flag: '-run', list: 'R1', dir: 'D1' },
    { target: 'b', flag: '-skip', list: 'S1', dir: undefined },
  ]);
});

test('goTestFilters throws on a literal -run or -skip value', () => {
  for (const v of ["-run 'TestA$$'", '-run=TestA', '-skip "$(S)"', "-run '$(R)TestExtra'", "-test.run '^TestA$$'"]) {
    assert.throws(() => goTestFilters(`a:\n\tgo test ${v} .\n`), /a literal regex/, v);
  }
});

test('goTestFilters throws on a filter outside a recipe it can read', () => {
  for (const mk of ["define d\nx; go test -skip '$(S)' .\nendef\n", 'GOFLAGS := -run TestA\n', "a:\n\t$(call suite_test,p,d,n,1s,,-skip '$(S)',)\n"]) {
    assert.throws(() => goTestFilters(mk), /cannot place -(run|skip) in a recipe/, mk);
  }
});

// The repo's own lists: every *_KNOWN_SKIPS and every -skip or -run regex a
// recipe passes is checked, each against the dir its suite runs in.
test("checks every list of test names the Makefile has, with its suite's dir", () => {
  const recipe = /^test-name-skip-check:[^\n]*\n((?:\t[^\n]*\n)+)/m.exec(MK);
  assert.ok(recipe, 'the Makefile has no test-name-skip-check recipe');
  const checked = new Map([...recipe[1].matchAll(/\b(names|regex) (\w+) '\$\((\w+)\)' '\$\((\w+)\)'/g)]
    .map(([, kind, list, dir, val]) => (assert.strictEqual(val, list), [list, { kind, dir }])));
  const lists = new Set([...MK.matchAll(/^(\w+_KNOWN_SKIPS) *:?=/gm)].map((m) => m[1]));
  // A recipe's -skip or -run is a checked list, '$(LIST)' (a -run may add
  // subtests after a /), run in the dir the list is checked against.
  const filters = goTestFilters(MK);
  assert.ok(filters.length >= 4, `go test filters: ${JSON.stringify(filters)}`);
  for (const f of filters) {
    lists.add(f.list);
    // Before the dir read below, which needs the entry.
    assert.ok(checked.has(f.list), `test-name-skip-check does not check ${f.list}, which ${f.target} passes ${f.flag}`);
    assert.strictEqual(checked.get(f.list).dir, f.dir, `${f.list} is checked against $(${checked.get(f.list).dir}), ${f.target} runs it in $(${f.dir})`);
  }
  assert.ok(lists.size >= 7, `lists: ${[...lists]}`);
  for (const l of lists) {
    assert.ok(checked.has(l), `test-name-skip-check does not check ${l}`);
    assert.strictEqual(checked.get(l).kind, /_KNOWN_SKIPS$/.test(l) ? 'names' : 'regex', l);
  }
  // Each suite runs in the dir its list is checked against.
  const suites = [...MK.matchAll(/\$\(call suite_test,[^,]*,\$\((\w+)\),(?:[^,]*,){4}\$\((\w+)\)/g)];
  assert.strictEqual(suites.length, 3, 'suite_test calls');
  for (const [, dir, skips] of suites) {
    assert.strictEqual(checked.get(skips).dir, dir, `${skips} is checked against $(${checked.get(skips).dir}), its suite runs in $(${dir})`);
  }
});

test("passes on the repo's own lists", () => {
  const r = spawnSync('make', ['--no-print-directory', 'test-name-skip-check'], { cwd: ROOT, encoding: 'utf8', env: makeEnv });
  passes({ code: r.status, out: r.stdout + r.stderr });
});

test('make check runs test-name-skip-check', () => {
  const deps = /^check:([^#\n]*)/m.exec(MK);
  assert.ok(deps, 'the Makefile has no check target');
  assert.ok(deps[1].split(/\s+/).includes('test-name-skip-check'), `check: ${deps[1]}`);
});
