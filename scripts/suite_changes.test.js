// scripts/suite_changes.sh, which the onchain workflow asks which on-chain
// suites an event runs: on a pull request, those whose paths its changed
// files touch; on a push to main (--all), every suite. And that onchain.yml's
// paths start the workflow on every wiring file (--wiring), and that the
// wiring holds every stack script.
//
//   node --test scripts/suite_changes.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');
const { GIT_ENV, git, scratchRepo } = require('./test_git');

const ROOT = path.join(__dirname, '..');
const SCRIPT = path.join(__dirname, 'suite_changes.sh');
// Every suite, in the order the script prints them.
const ALL = ['payments', 'dvp', 'retail'];

// suites runs the script on the changed files (one per line on stdin) and
// returns the suites it runs, by name, in the order it prints them. It runs
// in cwd (the repo root by default) with env (the test's by default).
function suites(files, { args = [], cwd = ROOT, env = process.env } = {}) {
  const r = spawnSync('bash', [SCRIPT, ...args], { cwd, env, encoding: 'utf8', input: files.map((f) => f + '\n').join('') });
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

test("a change to enygma_dvp's contracts runs the DvP, retail and payments suites", () => {
  assert.deepStrictEqual(suites(['enygma_dvp/contracts/EnygmaDvp.sol']), ['payments', 'dvp', 'retail']);
});

test("a change to enygma_dvp's Poseidon regeneration runs the payments and retail suites", () => {
  // hardhat.config.js's compile task runs it, and the retail stack compiles.
  assert.deepStrictEqual(suites(['enygma_dvp/scripts/regen_poseidon.js']), ['payments', 'dvp', 'retail']);
});

test('a change to enygma_dvp outside what the retail stack uses runs the DvP suite only', () => {
  assert.deepStrictEqual(suites(['enygma_dvp/relayer/main.go']), ['dvp']);
});

test('a change to enygma_payments runs the payments suite only', () => {
  assert.deepStrictEqual(suites(['enygma_payments/contracts/enygma/contracts/Enygma.sol']), ['payments']);
});

// prSuites makes a scratch repo holding from, commits, moves it to to (git
// mv) and commits again, then runs the script with --base the first commit,
// as the workflow runs it on a PR, and returns the suites it runs. Git
// reads no machine config (test_git.js), so a diff.renames=false there
// can't hide the renames under test.
function prSuites(from, to) {
  const dir = scratchRepo('suite-changes');
  try {
    fs.mkdirSync(path.join(dir, path.dirname(from)), { recursive: true });
    fs.writeFileSync(path.join(dir, from), 'package main\n');
    git(dir, 'add', '.');
    git(dir, 'commit', '-qm', 'base');
    const base = git(dir, 'rev-parse', 'HEAD');
    fs.mkdirSync(path.join(dir, path.dirname(to)), { recursive: true });
    git(dir, 'mv', from, to);
    git(dir, 'commit', '-qm', 'move');
    return suites([], { args: ['--base', base], cwd: dir, env: GIT_ENV });
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

test("a PR moving a file out of the DvP suite's paths runs the DvP suite", () => {
  assert.deepStrictEqual(prSuites('enygma_dvp/relayer/main.go', 'newdir/main.go'), ['dvp']);
});

test("a PR moving a file into the payments suite's paths runs the payments suite", () => {
  assert.deepStrictEqual(prSuites('newdir/main.go', 'enygma_payments/main.go'), ['payments']);
});

test('a push to main runs every suite, whatever it changes', () => {
  assert.deepStrictEqual(suites([], { args: ['--all'] }), ALL);
});

// stackScripts returns every stack script, read from where they live rather
// than from --wiring: each scripts/stack_*.sh, and the script each
// *_STACK_CMD in the root Makefile runs, from the dir its stack_start call
// runs it in.
function stackScripts() {
  const scripts = fs.readdirSync(path.join(ROOT, 'scripts'))
    .filter((f) => /^stack_.*\.sh$/.test(f)).map((f) => `scripts/${f}`);
  const makefile = fs.readFileSync(path.join(ROOT, 'Makefile'), 'utf8');
  const defs = makefile.match(/^\w+_STACK_CMD\s*[:?+]?=.*$/gm) || [];
  assert.ok(defs.length > 0, 'the Makefile defines no *_STACK_CMD');
  for (const def of defs) {
    const cmd = /^(\w+_STACK_CMD) := bash (\S+)/.exec(def);
    assert.ok(cmd, `${def}: read as NAME := bash <script>, which it is not`);
    const [, name, script] = cmd;
    const call = new RegExp(`\\$\\(call stack_start,(?:[^,]*,){4}([^,]*),\\$\\(${name}\\)`).exec(makefile);
    assert.ok(call, `no $(call stack_start, …) runs $(${name})`);
    scripts.push(path.posix.normalize(path.posix.join(call[1], script)));
  }
  return scripts;
}

// The suites' wiring besides the stack scripts, with own_group.sh, the
// helper they start services through, which stackScripts' stack_*.sh misses.
const WIRING = ['Makefile', '.github/workflows/onchain.yml', 'scripts/suite_changes.sh', 'scripts/suite_result.sh', 'scripts/own_group.sh'];

for (const file of [...WIRING, ...stackScripts()]) {
  test(`a change to the suites' wiring ${file} runs every suite`, () => {
    assert.deepStrictEqual(suites([file]), ALL);
  });
}

// wiring returns the suites' shared wiring files, as --wiring prints them.
function wiring() {
  const r = spawnSync('bash', [SCRIPT, '--wiring'], { encoding: 'utf8' });
  assert.strictEqual(r.status, 0, r.stdout + r.stderr);
  return r.stdout.trim().split('\n');
}

test("every stack script is in the suites' wiring", () => {
  const listed = wiring();
  const missing = stackScripts().filter((f) => !listed.includes(f));
  assert.deepStrictEqual(missing, [], 'suite_changes.sh --wiring leaves out these stack scripts');
});

// workflowPaths returns the paths filter of onchain.yml's on.<event>, read
// from the workflow file, or null when that event has none (every change
// starts it). It reads the on: block as plain block YAML (event keys at two
// spaces, their paths: at four, its items at six) and fails on a paths
// filter in any other shape, so one it can't read never passes for none.
function workflowPaths(event) {
  const lines = fs.readFileSync(path.join(ROOT, '.github/workflows/onchain.yml'), 'utf8').split('\n');
  const on = lines.indexOf('on:');
  assert.notStrictEqual(on, -1, 'onchain.yml has no top-level on:');
  let inEvent = false, inPaths = false, found = false, paths = null;
  for (const line of lines.slice(on + 1)) {
    if (/^\S/.test(line)) break; // the next top-level key ends on:
    const key = /^  (\S[^:]*):/.exec(line);
    if (key) {
      inEvent = key[1] === event;
      found = found || inEvent;
      inPaths = false;
      continue;
    }
    if (!inEvent || /^\s*(#.*)?$/.test(line)) continue;
    if (/^    \S/.test(line)) {
      assert.ok(!/^    paths/.test(line) || /^    paths:\s*$/.test(line), `on.${event} in onchain.yml: can't read ${line.trim()}`);
      inPaths = /^    paths:/.test(line);
      if (inPaths) paths = [];
      continue;
    }
    if (!inPaths) continue;
    const item = /^      - (?:"([^"]+)"|'([^']+)'|([^\s"'#]+))\s*(#.*)?$/.exec(line);
    assert.ok(item, `on.${event}.paths in onchain.yml: can't read ${line.trim()}`);
    paths.push(item[1] || item[2] || item[3]);
  }
  assert.ok(found, `onchain.yml has no on.${event}`);
  return paths;
}

// globMatches says whether file matches a paths glob in the subset of
// GitHub's filter syntax onchain.yml uses: ** matches anything, / included,
// * anything within one path segment. It fails on a glob with any other of
// GitHub's special characters (? + [ ] !), which it would misread.
function globMatches(glob, file) {
  assert.ok(!/[?+[\]!]/.test(glob), `onchain.yml path ${glob}: only * and ** are read here`);
  const re = glob.replace(/\*\*|\*|[.^${}()|\\]/g, (t) => (t === '**' ? '.*' : t === '*' ? '[^/]*' : '\\' + t));
  return new RegExp(`^${re}$`).test(file);
}

test("the workflow's paths reader finds onchain.yml's pull_request paths", () => {
  // A reader that found none would make the next test pass vacuously.
  const paths = workflowPaths('pull_request');
  assert.notStrictEqual(paths, null, 'onchain.yml has no on.pull_request.paths');
  assert.ok(paths.includes('Makefile'), paths);
});

test("a change to any of the suites' wiring starts the onchain workflow", () => {
  const files = wiring();
  for (const event of ['pull_request', 'push']) {
    const globs = workflowPaths(event);
    if (globs === null) continue; // no paths filter: every change starts it
    const missed = files.filter((f) => !globs.some((g) => globMatches(g, f)));
    assert.deepStrictEqual(missed, [], `on.${event}.paths in onchain.yml matches none of these wiring files`);
  }
});

test("a change to scripts/keys_src.sh, the DvP and Burn keys' stamp, runs the payments and DvP suites and starts the workflow", () => {
  const f = 'scripts/keys_src.sh';
  assert.deepStrictEqual(suites([f]), ['payments', 'dvp']);
  const globs = workflowPaths('pull_request');
  assert.ok(globs.some((g) => globMatches(g, f)), `on.pull_request.paths in onchain.yml does not match ${f}`);
});
