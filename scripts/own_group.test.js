// scripts/own_group.sh, which starts a command in its own process group for
// the stacks, make's demo restarts and mutant.sh: the command leads its
// group, runs without a shell, and fails as a shell would when it can't run.
//
//   node --test scripts/own_group.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const OWN_GROUP = path.join(__dirname, 'own_group.sh');

// run runs own_group.sh with args and returns its exit code and output.
function run(args) {
  const r = spawnSync('bash', [OWN_GROUP, ...args], { encoding: 'utf8', timeout: 15000 });
  return { code: r.status, out: r.stdout, err: r.stderr, pid: r.pid };
}

test('the command leads its own process group, under the pid its caller started', () => {
  // The command prints its pid, its process group and its parent's group.
  const r = run(['bash', '-c', 'echo $$ $(ps -o pgid= -p $$) $(ps -o pgid= -p $PPID)']);
  assert.strictEqual(r.code, 0, r.err);
  const [pid, pgid, parentPgid] = r.out.trim().split(/\s+/).map(Number);
  assert.strictEqual(pid, r.pid, 'own_group.sh execs the command, so its caller holds the command\'s pid');
  assert.strictEqual(pgid, pid, 'the command leads its group');
  assert.notStrictEqual(pgid, parentPgid, "a group of its own, not its caller's");
});

test('a command passed as one word is not split by a shell', () => {
  // Through /bin/sh -c this would print "split" and exit 0; run as a
  // shell runs it, it names no command.
  const r = run(['echo split; true']);
  assert.doesNotMatch(r.out, /split/, 'no shell ran the word');
  assert.strictEqual(r.code, 127, r.err);
});

test('a command that is not found exits 127, naming it', () => {
  const r = run(['no-such-command-for-own-group-test', 'arg']);
  assert.strictEqual(r.code, 127, r.err);
  assert.match(r.err, /^own_group\.sh: no-such-command-for-own-group-test: /m);
});

test("a command that can't run exits 126, naming it", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'own-group-test-'));
  const file = path.join(dir, 'not-executable');
  fs.writeFileSync(file, 'echo ran\n', { mode: 0o644 });
  const r = run([file]);
  fs.rmSync(dir, { recursive: true });
  assert.strictEqual(r.code, 126, r.err);
  assert.doesNotMatch(r.out, /ran/);
  assert.ok(r.err.includes(`own_group.sh: ${file}: `), r.err);
});

test('no command is a usage error, exit 2', () => {
  const r = run([]);
  assert.strictEqual(r.code, 2, r.err);
  assert.match(r.err, /^usage: own_group\.sh /m);
});
