// Every target make check runs is run by some ci.yml job too. CI never runs
// make check as one target: each prerequisite has a job of its own (or the
// per-module go job's), so a new prerequisite runs only locally until a job
// names it. A target whose help text says "(skipped in CI)" is exempt
// (hooks-check: CI's checkout has no hooks to check).
//
//   node --test scripts/check_in_ci.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');

// checkTargets returns the prerequisites of the Makefile's check target, less
// those whose own help text says "(skipped in CI)".
function checkTargets(makefile) {
  const m = /^check:([^#\n]*)/m.exec(makefile);
  assert.ok(m, 'the Makefile has no check target');
  return m[1].split(/\s+/).filter(Boolean).filter((t) => {
    const rule = new RegExp(`^${t.replace(/[-]/g, '\\-')}:[^\\n]*##[^\\n]*\\(skipped in CI\\)`, 'm');
    return !rule.test(makefile);
  });
}

// ciTargets returns every make target a ci.yml command names: each word
// after "make" up to the first that is not a target (a VAR=value, a flag,
// a quote, the end of the line). Comment lines name no command.
function ciTargets(yml) {
  const out = new Set();
  const code = yml.split('\n').filter((l) => !/^\s*#/.test(l)).join('\n');
  for (const [, words] of code.matchAll(/\bmake((?: +[a-z][a-z0-9-]*\b(?!=))+)/g)) {
    words.trim().split(/ +/).forEach((w) => out.add(w));
  }
  return out;
}

// missing returns the check targets no CI command names.
function missing(makefile, yml) {
  const ci = ciTargets(yml);
  return checkTargets(makefile).filter((t) => !ci.has(t));
}

const MK = [
  'check: hooks-check fmt-check page-check build ## Static checks only',
  'hooks-check: ## Fail on a missing hook (skipped in CI)',
  'fmt-check: ## gofmt',
  'page-check: ## pages',
  'build: ## go build',
  '',
].join('\n');

test('passes when each check target is in some CI command, inline or in a run block', () => {
  const yml = '      - run: make fmt-check\n      - run: make page-check\n        run: |\n          make build vet MODULE="$MODULE"\n';
  assert.deepStrictEqual(missing(MK, yml), []);
});

test('names a check target no CI command runs', () => {
  assert.deepStrictEqual(missing(MK, '      - run: make fmt-check page-check\n'), ['build']);
});

test('a VAR=value after make ends its targets', () => {
  assert.deepStrictEqual([...ciTargets('make MODULE=x build\nmake vet MODULE=x\nmake lint v=1\n')], ['vet', 'lint']);
});

test('a ci.yml comment naming a target does not count as CI running it', () => {
  assert.deepStrictEqual(missing(MK, '# make build locally\n      - run: make fmt-check page-check\n'), ['build']);
});

test('a target marked "(skipped in CI)" need not be in CI; others must', () => {
  const yml = '      - run: make fmt-check page-check build\n';
  assert.deepStrictEqual(missing(MK, yml), []);
  assert.deepStrictEqual(missing(MK.replace(' (skipped in CI)', ''), yml), ['hooks-check']);
});

test("every target make check runs, CI runs too", () => {
  const mk = fs.readFileSync(path.join(ROOT, 'Makefile'), 'utf8');
  const yml = fs.readFileSync(path.join(ROOT, '.github/workflows/ci.yml'), 'utf8');
  assert.ok(checkTargets(mk).length > 3, `check targets: ${checkTargets(mk)}`);
  assert.deepStrictEqual(missing(mk, yml), [],
    'make check runs these, but no ci.yml job does: add a job or a step that runs them');
});
