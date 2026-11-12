// scripts/check_red_named.sh, run as the commit-msg hook in a scratch repo:
// which staged diffs need a named red, and which messages name one.
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
}

// commit stages files (path → content) on top of base in a scratch repo and
// runs the hook on message; it returns the hook's exit code and output.
function commit(files, message, { base = {} } = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'red-named-test-'));
  git(dir, 'init', '-q');
  git(dir, '-c', 'user.name=t', '-c', 'user.email=t@t', 'commit', '-q', '--allow-empty', '-m', 'root');
  const write = (fs_) => Object.entries(fs_).forEach(([p, c]) => {
    fs.mkdirSync(path.dirname(path.join(dir, p)), { recursive: true });
    fs.writeFileSync(path.join(dir, p), c);
  });
  if (Object.keys(base).length) {
    write(base);
    git(dir, 'add', '-A');
    git(dir, '-c', 'user.name=t', '-c', 'user.email=t@t', 'commit', '-q', '-m', 'base');
  }
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

test('"RED" inside a word is not a red', () => {
  const r = commit({ 'x_test.go': GO_TEST }, 'Refuse a short statement\n\nCovers the REDUCED case.\n');
  assert.strictEqual(r.code, 1, r.out);
});

test('a fixup! commit passes', () => {
  const r = commit({ 'x_test.go': GO_TEST }, 'fixup! Refuse a short statement\n');
  assert.strictEqual(r.code, 0, r.out);
});
