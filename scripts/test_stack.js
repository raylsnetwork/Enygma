// Fake stacks' services for the stack scripts' tests (stack_lib, stack_stop,
// stack_start). Each service leads its own process group (own_group.sh), so
// a test that kills only the stack script leaves its services running for
// good, orphaned: kill what the stack's service lists name instead. Every
// group killed is recorded, and stack_stop's and stack_start's tests end on
// testNoLeftovers, so a cleanup that leaks one fails.

const fs = require('fs');
const path = require('path');

const seen = new Set();

// listed returns the pids every *.pids file in logDir names (the stack's
// services.pids, and any copy a test kept), recording each; none when
// logDir is gone.
function listed(logDir) {
  let files = [];
  try { files = fs.readdirSync(logDir).filter((n) => n.endsWith('.pids')); } catch { return []; }
  const pids = [];
  for (const f of files) {
    for (const line of fs.readFileSync(path.join(logDir, f), 'utf8').split('\n')) {
      const pid = Number(line.split(' ')[1]);
      if (pid) { pids.push(pid); seen.add(pid); }
    }
  }
  return pids;
}

// killGroup SIGKILLs process group pid, if it still runs.
function killGroup(pid) {
  try { process.kill(-pid, 'SIGKILL'); } catch { /* already gone */ }
}

// killServices SIGKILLs every group logDir's lists name, recording each.
function killServices(logDir) {
  for (const pid of listed(logDir)) killGroup(pid);
}

// groupAlive says whether any process of group pid runs.
function groupAlive(pid) {
  try { process.kill(-pid, 0); return true; } catch { return false; }
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
    require('node:assert').deepStrictEqual(await leftovers(), [], 'these service groups outlived their tests');
  });
}

module.exports = { killGroup, killServices, testNoLeftovers };
