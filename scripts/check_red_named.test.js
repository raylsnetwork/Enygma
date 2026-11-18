// scripts/check_red_named.sh in a scratch repo: as the commit-msg hook,
// which staged diffs need a named red and which messages name one; with
// --pr, the same for a PR's range, body and commits; and the
// red-named workflow that runs --pr in CI, read as text.
//
//   node --test scripts/check_red_named.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const HOOK = path.join(__dirname, 'check_red_named.sh');

function git(dir, ...args) {
  const r = spawnSync('git', args, { cwd: dir, encoding: 'utf8' });
  if (r.status !== 0) throw new Error(`git ${args.join(' ')}: ${r.stderr}`);
  return r.stdout.trim();
}

// commitAll commits everything in dir as a throwaway author.
function commitAll(dir, msg) {
  git(dir, 'add', '-A');
  git(dir, '-c', 'user.name=t', '-c', 'user.email=t@t', 'commit', '-q', '--allow-empty', '-m', msg);
}
const revParse = (dir, ref) => git(dir, 'rev-parse', ref);

// commit stages files (path → content), and the removal of each path in
// remove, on top of base in a scratch repo and runs the hook on message; it
// returns the hook's exit code and output.
function commit(files, message, { base = {}, remove = [] } = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'red-named-test-'));
  git(dir, 'init', '-q');
  commitAll(dir, 'root');
  const write = (fs_) => Object.entries(fs_).forEach(([p, c]) => {
    fs.mkdirSync(path.dirname(path.join(dir, p)), { recursive: true });
    fs.writeFileSync(path.join(dir, p), c);
  });
  if (Object.keys(base).length) {
    write(base);
    commitAll(dir, 'base');
  }
  remove.forEach((p) => fs.rmSync(path.join(dir, p)));
  write(files);
  git(dir, 'add', '-A');
  const msgFile = path.join(dir, '.git', 'COMMIT_EDITMSG');
  fs.writeFileSync(msgFile, message);
  const r = spawnSync('bash', [HOOK, msgFile], { cwd: dir, encoding: 'utf8' });
  fs.rmSync(dir, { recursive: true });
  return { code: r.status, out: r.stdout + r.stderr };
}

const GO_TEST = 'package x\n\nfunc TestRefusesShortStatement(t *testing.T) {\n}\n';

test('a new Go test with no red in the message is refused, and named', () => {
  const r = commit({ 'x_test.go': GO_TEST }, 'Refuse a short statement\n');
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /func TestRefusesShortStatement\(t \*testing\.T\)/);
});

test('a message naming the red passes', () => {
  const r = commit({ 'x_test.go': GO_TEST },
    'Refuse a short statement\n\nShown red: "!= LEN" -> "< 0": RED.\n');
  assert.strictEqual(r.code, 0, r.out);
});

test('a "No red:" line passes', () => {
  const r = commit({ 'x_test.go': GO_TEST }, 'Refuse a short statement\n\nNo red: live-chain only, stated in the PR.\n');
  assert.strictEqual(r.code, 0, r.out);
});

test('a new subtest in an existing test needs a red', () => {
  const base = { 'x_test.go': 'package x\n\nfunc TestShapes(t *testing.T) {\n}\n' };
  const r = commit({ 'x_test.go': 'package x\n\nfunc TestShapes(t *testing.T) {\n\tt.Run("three inputs", func(t *testing.T) {})\n}\n' },
    'Pin three inputs\n', { base });
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /t\.Run\("three inputs"/);
});

test('a new JS test needs a red', () => {
  const r = commit({ 'a.test.js': "test('refuses a bad page', () => {});\n" }, 'Check pages\n');
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /test\('refuses a bad page'/);
});

test('a commit adding no check passes', () => {
  const r = commit({ 'x.go': 'package x\n\nfunc Read() {}\n' }, 'Read by name\n');
  assert.strictEqual(r.code, 0, r.out);
});

test('removing a test needs no red', () => {
  const base = { 'x_test.go': GO_TEST };
  const r = commit({ 'x_test.go': 'package x\n' }, 'Drop a test\n', { base });
  assert.strictEqual(r.code, 0, r.out);
});

test('editing an existing test, even its func line, needs no red', () => {
  const base = { 'x_test.go': 'package x\n\nfunc TestShapes(t *testing.T) {}\n' };
  const r = commit({ 'x_test.go': 'package x\n\nfunc TestShapes(t *testing.T) {\n\tt.Log("x")\n}\n' },
    'Log in a test\n', { base });
  assert.strictEqual(r.code, 0, r.out);
});

test('moving an existing subtest to another file needs no red', () => {
  const base = { 'a_test.go': 'package x\n\nfunc TestA(t *testing.T) {\n\tt.Run("three inputs", nil)\n}\n' };
  const r = commit({
    'a_test.go': 'package x\n\nfunc TestA(t *testing.T) {\n}\n',
    'b_test.go': 'package x\n\nfunc TestB(t *testing.T) {\n\t\tt.Run("three inputs", nil)\n}\n',
  }, 'Move a subtest\n', { base });
  // TestB is new; the subtest is not.
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /func TestB/);
  assert.doesNotMatch(r.out, /three inputs/);
});

test('a renamed subtest counts as a new check', () => {
  const base = { 'x_test.go': 'package x\n\nfunc TestA(t *testing.T) {\n\tt.Run("three inputs", nil)\n}\n' };
  const r = commit({ 'x_test.go': 'package x\n\nfunc TestA(t *testing.T) {\n\tt.Run("four inputs", nil)\n}\n' },
    'Pin four inputs\n', { base });
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /four inputs/);
});

// git diff --cached detects a rename when the new file is mostly the old
// one (R), and records a delete and an add when it is mostly new (D, A).
const JS_TESTS = "test('one', () => {});\ntest('two', () => {});\ntest('three', () => {});\n";

test('a new test in a renamed file (a small edit: git records a rename) needs a red', () => {
  const r = commit({ 'b.test.js': JS_TESTS + "test('brand new', () => {});\n" }, 'Rename and add\n',
    { base: { 'a.test.js': JS_TESTS }, remove: ['a.test.js'] });
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /test\('brand new'/);
  assert.doesNotMatch(r.out, /test\('one'/);
});

test('tests moved by a rename with a big edit (git records a delete and an add) need no red', () => {
  const rewrite = Array.from({ length: 30 }, (_, i) => `// rewritten line ${i}\n`).join('');
  const r = commit({ 'b.test.js': rewrite + JS_TESTS }, 'Rename and rewrite\n',
    { base: { 'a.test.js': JS_TESTS + '// old\n'.repeat(30) }, remove: ['a.test.js'] });
  assert.strictEqual(r.code, 0, r.out);
});

test('"RED" inside a word is not a red', () => {
  const r = commit({ 'x_test.go': GO_TEST }, 'Refuse a short statement\n\nCovers the REDUCED case.\n');
  assert.strictEqual(r.code, 1, r.out);
});

test('a fixup! commit passes', () => {
  const r = commit({ 'x_test.go': GO_TEST }, 'fixup! Refuse a short statement\n');
  assert.strictEqual(r.code, 0, r.out);
});

// pr makes a scratch repo with base committed, then commits ([files,
// message] each) on top as a PR's branch, and runs the check in --pr mode
// on that range with body as the PR body; it returns the exit code and
// output.
function pr(commits, body, { base = { 'README.md': 'x\n' } } = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'red-named-pr-test-'));
  const write = (fs_) => Object.entries(fs_).forEach(([p, content]) => {
    fs.mkdirSync(path.dirname(path.join(dir, p)), { recursive: true });
    fs.writeFileSync(path.join(dir, p), content);
  });
  try {
    git(dir, 'init', '-q');
    write(base);
    commitAll(dir, 'base');
    const baseSha = revParse(dir, 'HEAD');
    for (const [files, msg] of commits) {
      write(files);
      commitAll(dir, msg);
    }
    const bodyFile = path.join(dir, '.git', 'PR_BODY');
    fs.writeFileSync(bodyFile, body);
    const r = spawnSync('bash', [HOOK, '--pr', baseSha, 'HEAD', bodyFile], { cwd: dir, encoding: 'utf8' });
    return { code: r.status, out: r.stdout + r.stderr };
  } finally {
    fs.rmSync(dir, { recursive: true });
  }
}

test('a PR adding a test with no red in its body or commits is refused, and the test named', () => {
  const r = pr([[{ 'x_test.go': GO_TEST }, 'Refuse a short statement']], '## Summary\n\nRefuses it.\n');
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /This PR adds checks/);
  assert.match(r.out, /func TestRefusesShortStatement/);
});

test('a PR whose body names the red passes', () => {
  const r = pr([[{ 'x_test.go': GO_TEST }, 'Refuse a short statement']],
    '## Summary\n\nShown red: "!= LEN" -> "< 0": RED.\n');
  assert.strictEqual(r.code, 0, r.out);
});

test('a PR whose red is in one of its commits, not the body, passes', () => {
  const r = pr([
    [{ 'x.go': 'package x\n' }, 'Add x'],
    [{ 'x_test.go': GO_TEST }, 'Refuse a short statement\n\nNo red: live-chain only.'],
  ], 'Refuses it.\n');
  assert.strictEqual(r.code, 0, r.out);
});

test("a \"No red:\" line in a body GitHub saved with CRLF line ends passes", () => {
  const r = pr([[{ 'x_test.go': GO_TEST }, 'Refuse a short statement']], 'Refuses it.\r\n\r\nNo red: live-chain only.\r\n');
  assert.strictEqual(r.code, 0, r.out);
});

test('a PR adding no check passes', () => {
  const r = pr([[{ 'x.go': 'package x\n\nfunc Read() {}\n' }, 'Read by name']], 'Reads it.\n');
  assert.strictEqual(r.code, 0, r.out);
});

// scratch runs fn in a scratch repo whose HEAD holds a new test, with a
// PR body file, and removes it after.
function scratch(fn) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'red-named-pr-test-'));
  try {
    git(dir, 'init', '-q');
    fs.writeFileSync(path.join(dir, 'x_test.go'), GO_TEST);
    commitAll(dir, 'add a test');
    fs.writeFileSync(path.join(dir, 'body'), 'Refuses it.\n');
    return fn(dir, (...a) => spawnSync('bash', [HOOK, '--pr', ...a, path.join(dir, 'body')], { cwd: dir, encoding: 'utf8' }));
  } finally {
    fs.rmSync(dir, { recursive: true });
  }
}

test('a PR whose base git does not have fails, rather than read as adding nothing', () => {
  scratch((dir, check) => {
    const r = check('0123456789abcdef0123456789abcdef01234567', 'HEAD');
    assert.strictEqual(r.status, 2, r.stdout + r.stderr);
    assert.match(r.stderr, /git diff 0123456789abcdef0123456789abcdef01234567\.\.\.HEAD failed/);
  });
});

test('a PR with no merge base (a shallow or unrelated history) fails, rather than read as adding nothing', () => {
  scratch((dir, check) => {
    const head = revParse(dir, 'HEAD');
    git(dir, 'checkout', '-q', '--orphan', 'other');
    git(dir, 'rm', '-q', '-rf', '.');
    commitAll(dir, 'unrelated');
    const r = check('other', head);
    assert.strictEqual(r.status, 2, r.stdout + r.stderr);
    assert.match(r.stderr, /git diff other\.\.\.[0-9a-f]+ failed/);
  });
});

test('a PR whose body file is missing fails, rather than read as an empty body', () => {
  scratch((dir, check) => {
    fs.rmSync(path.join(dir, 'body'));
    const r = check('HEAD', 'HEAD');
    assert.strictEqual(r.status, 2, r.stdout + r.stderr);
  });
});

test('--pr without a base, a head and a body file fails, naming its usage', () => {
  scratch((dir, check) => {
    const r = check('HEAD');
    assert.strictEqual(r.status, 2, r.stdout + r.stderr);
    assert.match(r.stderr, /usage: check_red_named\.sh --pr <base> <head> <PR body file>/);
  });
});

// .github/workflows/red-named.yml runs --pr on each pull request. Read as
// text: a shape this cannot find fails the test, rather than pass for absent.
test("CI's red-named job runs --pr on the PR's range and body, and re-runs on a body edit", () => {
  const yml = fs.readFileSync(path.join(__dirname, '..', '.github/workflows/red-named.yml'), 'utf8');
  const types = /^ {2}pull_request:\n {4}types: \[([^\]]*)\]$/m.exec(yml);
  assert.ok(types, 'red-named.yml has no on.pull_request.types list');
  assert.deepStrictEqual(types[1].split(',').map((t) => t.trim()).sort(),
    ['edited', 'opened', 'reopened', 'synchronize'], 'red-named.yml runs on the wrong pull_request events');
  // The whole history, for the merge base of base...head.
  assert.match(yml, /^ {10}fetch-depth: 0$/m, 'red-named.yml checks out a shallow history');
  // The body through env, never inlined into the script.
  assert.match(yml, /^ {10}BODY: \$\{\{ github\.event\.pull_request\.body \}\}$/m, 'the PR body does not reach the step as BODY');
  // The base branch as fetched now: the event's base.sha goes stale once main moves.
  assert.match(yml, /^ {10}BASE_REF: \$\{\{ github\.event\.pull_request\.base\.ref \}\}$/m);
  assert.match(yml, /^ {10}HEAD: \$\{\{ github\.event\.pull_request\.head\.sha \}\}$/m);
  assert.match(yml, /printf '%s\\n' "\$BODY" > "\$RUNNER_TEMP\/pr_body"/, 'the step does not write BODY to the body file');
  assert.match(yml, /bash scripts\/check_red_named\.sh --pr "origin\/\$BASE_REF" "\$HEAD" "\$RUNNER_TEMP\/pr_body"/,
    'the step does not run check_red_named.sh --pr on the range and body file');
  // Every expression sits on an env line or the concurrency group, so none
  // is pasted into a script.
  const inlined = yml.split('\n').filter((l) => l.includes('${{') && !/^ {10}(BODY|BASE_REF|HEAD): /.test(l) && !/^ {2}group: /.test(l));
  assert.deepStrictEqual(inlined, [], 'red-named.yml has an expression outside env');
});
