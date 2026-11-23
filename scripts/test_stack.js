// Fake stacks for the stack scripts' tests (stack_lib, stack_stop,
// stack_start): the helpers they share, the one reader of the pids in a
// stack's `<name> <pid>` service lists, and the teardown that stops a fake
// stack.
// Each service leads its own process group (own_group.sh), so a test that
// kills only the stack script leaves its services running for good,
// orphaned: kill what the stack's service lists name instead. Every group
// killed is recorded, and stack_stop's and stack_start's tests end on
// testNoLeftovers, so a cleanup that leaks one fails.

const fs = require('fs');
const net = require('net');
const path = require('path');
const { spawnSync } = require('child_process');
const assert = require('node:assert');

const seen = new Set();

// freePort asks the OS for a port nothing listens on.
function freePort() {
  return new Promise((resolve) => {
    const s = net.createServer().listen(0, '127.0.0.1', () => {
      const { port } = s.address();
      s.close(() => resolve(port));
    });
  });
}

// listening says whether something accepts connections on port.
function listening(port) {
  return new Promise((resolve) => {
    const c = net.connect(port, '127.0.0.1');
    c.on('connect', () => { c.destroy(); resolve(true); });
    c.on('error', () => resolve(false));
  });
}

// running says whether some process that ps lists matches (its row's pid
// and pgid) and has not exited. A zombie (stat Z: exited, not yet reaped)
// has: Linux's kill -0 still finds one, as a pid and as a group, while
// macOS's finds it only as a pid, so kill -0 alone would count a stopped
// service its parent never reaped as running, on CI alone. A ps that fails
// counts as running, as kill -0 said.
function running(matches) {
  const r = spawnSync('ps', ['-axo', 'pid=,pgid=,stat='], { encoding: 'utf8' });
  if (r.status !== 0) return true;
  return r.stdout.split('\n').some((line) => {
    const [pid, pgid, stat] = line.trim().split(/\s+/);
    return stat && !stat.startsWith('Z') && matches(Number(pid), Number(pgid));
  });
}

// alive says whether process pid still runs (a zombie does not).
function alive(pid) {
  try { process.kill(pid, 0); } catch { return false; }
  return running((p) => p === pid);
}

// run resolves with child's exit status and output.
function run(child) {
  let out = '';
  child.stdout.on('data', (d) => { out += d; });
  child.stderr.on('data', (d) => { out += d; });
  return new Promise((resolve) => child.on('close', (status) => resolve({ status, out })));
}

// signallable says whether pid may be signalled as a group: an integer
// above 1. Signalling group 0 reaches the test's own group, and -1 every
// process the user owns.
const signallable = (pid) => Number.isInteger(pid) && pid > 1;

// serviceList reads the `<name> <pid>` lines of the service list at file,
// as [name, pid] pairs, dropping any line whose pid is not signallable.
function serviceList(file) {
  const entries = [];
  for (const line of fs.readFileSync(file, 'utf8').split('\n')) {
    const [name, pid] = line.split(' ');
    if (/^\d+$/.test(pid || '') && signallable(Number(pid))) entries.push([name, Number(pid)]);
  }
  return entries;
}

// servicePid reads NAME's pid from the stack's service list in logDir.
function servicePid(logDir, name) {
  const entry = serviceList(path.join(logDir, 'services.pids')).find(([n]) => n === name);
  return entry && entry[1];
}

// listed returns the pids every *.pids file in logDir names (the stack's
// services.pids, and any copy a test kept), recording each; none when
// logDir is gone.
function listed(logDir) {
  let files = [];
  try { files = fs.readdirSync(logDir).filter((n) => n.endsWith('.pids')); } catch { return []; }
  const pids = [];
  for (const f of files) {
    for (const [, pid] of serviceList(path.join(logDir, f))) { pids.push(pid); seen.add(pid); }
  }
  return pids;
}

// killGroup SIGKILLs process group pid, if it still runs; a pid that is
// not signallable (a pid file read as 0 or NaN) is left alone.
function killGroup(pid) {
  if (!signallable(pid)) return;
  try { process.kill(-pid, 'SIGKILL'); } catch { /* already gone */ }
}

// killServices SIGKILLs every group logDir's lists name, recording each.
function killServices(logDir) {
  for (const pid of listed(logDir)) killGroup(pid);
}

// killStack stops a fake stack whose script leads its own group: the
// groups logDir's lists name, then each of groupPids (its script's group).
function killStack(logDir, ...groupPids) {
  killServices(logDir);
  groupPids.forEach(killGroup);
}

// groupAlive says whether any process of group pid runs (a zombie does
// not).
function groupAlive(pid) {
  try { process.kill(-pid, 0); } catch { return false; }
  return running((_, pgid) => pgid === pid);
}

// survivors runs kill (the stack's death), then resolves with those of
// the groups pids that still run after about 15 s (the reaper's poll and
// its 10 s stop), killing them so a failed test leaks nothing. Every group
// must run before kill: one already gone, or not yet started, would pass
// as stopped with nothing tested.
async function survivors(pids, kill = () => {}) {
  assert.ok(pids.length && pids.every((pid) => Number.isInteger(pid) && pid > 1), `not a list of service pids: ${pids}`);
  const idle = pids.filter((pid) => !groupAlive(pid));
  assert.deepStrictEqual(idle, [], 'these service groups were not running before the stack died, so the check would test nothing');
  kill();
  for (let i = 0; i < 150 && pids.some(groupAlive); i++) await new Promise((r) => setTimeout(r, 100));
  const left = pids.filter(groupAlive);
  left.forEach(killGroup);
  return left;
}

// leftovers resolves with the recorded groups still running after about
// 5 s, killing them so one test file's leak can't reach the next.
async function leftovers() {
  for (let i = 0; i < 50 && [...seen].some(groupAlive); i++) await new Promise((r) => setTimeout(r, 100));
  const left = [...seen].filter(groupAlive);
  left.forEach(killGroup);
  return left;
}

// testNoLeftovers adds, with node:test's test, a file's last test: every
// service group its stacks listed has stopped, so no test left one running.
function testNoLeftovers(test) {
  test('no service a stack in this file started is left running', async () => {
    assert.deepStrictEqual(await leftovers(), [], 'these service groups outlived their tests');
  });
}

module.exports = { freePort, listening, alive, groupAlive, run, servicePid, killGroup, killServices, killStack, survivors, testNoLeftovers };
