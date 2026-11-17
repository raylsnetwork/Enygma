// scripts/stack_lib.sh, the helpers the three local stack scripts source, on
// small stacks of fake services: how a stack stops on a signal, on a service
// dying, and on a failed step.
//
//   node --test scripts/stack_lib.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const net = require('net');
const os = require('os');
const path = require('path');
const { spawn } = require('child_process');

const LIB = path.join(__dirname, 'stack_lib.sh');

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

// server is a service command that listens on port.
const server = (port) => `python3 -m http.server ${port} --bind 127.0.0.1`;

// stack runs body as a stack script that sources the lib, in a scratch dir
// that is also its LOG_DIR. onUp, if given, runs once the script prints
// "stack is up", with the child. It resolves with the exit code, the signal
// and the output, once the scratch dir is removed.
function stack(body, onUp) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'stack-lib-test-'));
  const script = path.join(dir, 'stack.sh');
  fs.writeFileSync(script, `set -euo pipefail\n. ${JSON.stringify(LIB)}\nuse_log_dir stack_lib_test\n${body}\n`);
  const child = spawn('bash', [script], { cwd: dir, env: { ...process.env, LOG_DIR: dir } });
  let out = '';
  let up = false;
  const collect = (d) => {
    out += d;
    if (!up && onUp && out.includes('stack is up')) { up = true; onUp(child); }
  };
  child.stdout.on('data', collect);
  child.stderr.on('data', collect);
  return new Promise((resolve) => {
    child.on('close', (code, signal) => {
      fs.rmSync(dir, { recursive: true, force: true });
      resolve({ code, signal, out });
    });
  });
}

for (const [sig, code] of [['SIGTERM', 143], ['SIGINT', 130]]) {
  test(`${sig} stops the services and exits ${code}, reporting none of them dead`, { timeout: 30000 }, async () => {
    const port = await freePort();
    const r = await stack(`start web . ${server(port)}\nwait_port web ${port}\nsay "stack is up"\nwatch_services`,
      (child) => child.kill(sig));
    assert.strictEqual(r.code, code, r.out);
    assert.doesNotMatch(r.out, /exited/);
    assert.match(r.out, /stopped\. Logs: /);
    assert.strictEqual(await listening(port), false, 'the service still listens');
  });
}

test('a service exiting stops the stack with its log tail, and stops the others', { timeout: 30000 }, async () => {
  const port = await freePort();
  const r = await stack(`start web . ${server(port)}\nwait_port web ${port}\n` +
    `start brief . sh -c 'echo its-last-words; sleep 1'\nsay "stack is up"\nwatch_services`);
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /brief exited — last log lines:\nits-last-words\n/);
  assert.strictEqual(await listening(port), false, 'the other service still listens');
});

test('wait_port fails at once with the log tail when its service exits before listening', { timeout: 30000 }, async () => {
  const port = await freePort();
  const t0 = Date.now();
  const r = await stack(`start web . sh -c 'echo cannot-bind; exit 1'\nwait_port web ${port}\nsay "stack is up"`);
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /web exited — last log lines:\ncannot-bind\n/);
  assert.ok(Date.now() - t0 < 10000, 'wait_port waited on a dead service');
  assert.doesNotMatch(r.out, /stack is up/);
});

test('a failed step stops the stack with its log tail', { timeout: 30000 }, async () => {
  const r = await stack(`step build . sh -c 'echo compile-error; exit 2'\nsay "stack is up"`);
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, /build failed — last log lines:\ncompile-error\n/);
  assert.doesNotMatch(r.out, /stack is up/);
});

test('a busy port fails with its own message, not a stop of services never started', { timeout: 30000 }, async () => {
  const busy = net.createServer();
  await new Promise((resolve) => busy.listen(0, '127.0.0.1', resolve));
  const { port } = busy.address();
  const r = await stack(`require_free_ports ${port}\nsay "stack is up"`);
  busy.close();
  assert.strictEqual(r.code, 1, r.out);
  assert.match(r.out, new RegExp(`port ${port} is already in use`));
  assert.doesNotMatch(r.out, /stopping services/);
});
