const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { git, makeEnv, scratchRepo } = require('./test_git');
const withChrome = require('./headless_chrome');

const ROOT = path.join(__dirname, '..');
const STATE = path.join(__dirname, 'stack_state.sh');

function scratch(t, prefix) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), prefix));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  return dir;
}

function make(target, args, env = {}, cwd = ROOT) {
  const r = spawnSync('make', ['--no-print-directory', target, ...args], {
    cwd, encoding: 'utf8', timeout: 20000, env: { ...makeEnv, ...env },
  });
  assert.ifError(r.error);
  return { status: r.status, out: r.stdout + r.stderr };
}

test('make hooks uses the installed primer converter after the working copy changes', t => {
  const dir = scratchRepo('primer-pin');
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  fs.mkdirSync(path.join(dir, 'scripts'));
  fs.copyFileSync(path.join(ROOT, 'Makefile'), path.join(dir, 'Makefile'));
  fs.copyFileSync(path.join(__dirname, 'primer_page.js'), path.join(dir, 'scripts/primer_page.js'));
  fs.writeFileSync(path.join(dir, 'pre-commit'), '#!/bin/sh\n[ "$1" != --version ] || echo "pre-commit 4.0.0"\n', { mode: 0o755 });
  fs.writeFileSync(path.join(dir, '.gitattributes'), 'page.html diff=primer\n');
  fs.writeFileSync(path.join(dir, 'page.html'), 'original page\n');
  git(dir, 'add', '.gitattributes', 'page.html');
  git(dir, 'commit', '-m', 'add page');

  const installed = make('hooks', [], { PATH: `${dir}:${process.env.PATH}` }, dir);
  assert.equal(installed.status, 0, installed.out);
  fs.writeFileSync(path.join(dir, 'scripts/primer_page.js'), 'console.log("working tree converter");\n');
  fs.writeFileSync(path.join(dir, 'package.json'), '{"type":"module"}\n');
  fs.writeFileSync(path.join(dir, 'page.html'), 'updated page\n');
  assert.match(git(dir, 'diff', '--textconv', '--', 'page.html'), /^\+updated page$/m);
});

test('default stack state and logs live in a private per-user directory', t => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'stack-state-'));
  const script = path.join(dir, 'stack.sh');
  fs.writeFileSync(script, 'trap "exit 0" TERM\nprintf "stack is up\\n"\nwhile :; do sleep 0.1; done\n');
  const cmd = `bash ${script}`;
  const state = path.join(dir, `enygma-${process.getuid()}`);
  const pid = path.join(state, 'enygma_dvp_stack.pid');
  const legacy = path.join(dir, 'enygma_dvp_stack.pid');
  t.after(() => {
    try {
      for (const file of [pid, legacy]) {
        if (!fs.existsSync(file)) continue;
        const pidToStop = Number(fs.readFileSync(file, 'utf8'));
        const stopped = spawnSync('bash', [path.join(__dirname, 'stack_stop.sh'), file, cmd, 'stack.log', 'dvp-stack'],
          { encoding: 'utf8', timeout: 5000, env: { ...makeEnv, TMPDIR: dir } });
        if (stopped.status !== 0 && Number.isInteger(pidToStop) && pidToStop > 1) {
          // The scratch PID files only name this fixture.
          try { process.kill(-pidToStop, 'SIGTERM'); } catch (err) {
            if (err.code !== 'ESRCH') throw err;
          }
        }
      }
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });
  const r = make('dvp-stack', [`DVP_STACK_CMD=${cmd}`], { TMPDIR: dir });
  assert.equal(r.status, 0, r.out);
  assert.equal(fs.statSync(state).mode & 0o777, 0o700);
  const logs = fs.readFileSync(`${pid}.logs`, 'utf8').trim();
  assert.equal(fs.realpathSync(path.dirname(logs)), fs.realpathSync(state));
  const stopped = spawnSync('bash', [path.join(__dirname, 'stack_stop.sh'), pid, cmd, 'stack.log', 'dvp-stack'],
    { encoding: 'utf8', timeout: 5000, env: { ...makeEnv, TMPDIR: dir } });
  assert.equal(stopped.status, 0, stopped.stdout + stopped.stderr);
});

test('stack start refuses a per-user directory that other users can write', t => {
  const dir = scratch(t, 'stack-state-mode-');
  const state = path.join(dir, `enygma-${process.getuid()}`);
  fs.mkdirSync(state);
  fs.chmodSync(state, 0o777);
  const r = make('dvp-stack', ['DVP_STACK_CMD=false'], { TMPDIR: dir });
  assert.notEqual(r.status, 0, r.out);
  assert.match(r.out, /refusing stack state directory .*expected an owned directory with mode 700/);
});

test('stack state rejects a file owned by another user', t => {
  const candidate = ['/etc/hosts', '/etc/passwd'].find(f => fs.statSync(f).uid !== process.getuid() && !fs.lstatSync(f).isSymbolicLink());
  if (!candidate) return t.skip('the system fixtures are owned by this test user');
  const r = spawnSync('bash', [STATE, 'check', candidate], { encoding: 'utf8' });
  assert.equal(r.status, 1, r.stdout + r.stderr);
  assert.match(r.stderr, /expected an owned regular file/);
});

test('dvp-test checks the log pointer and environment file before loading them', t => {
  const dir = scratch(t, 'stack-state-read-');
  const pid = path.join(dir, 'stack.pid');
  const logs = path.join(dir, 'logs');
  fs.mkdirSync(logs);
  fs.writeFileSync(path.join(dir, 'go'), '#!/bin/sh\n[ "$STACK_TEST_VALUE" = owned ] || exit 1\nprintf -- "--- PASS: TestStackEnvironment (0.00s)\\n"\n', { mode: 0o755 });
  const env = { PATH: `${dir}:${process.env.PATH}` };
  const args = [`DVP_STACK_PID=${pid}`];
  fs.writeFileSync(path.join(dir, 'actual.logs'), `${logs}\n`);
  fs.symlinkSync(path.join(dir, 'actual.logs'), `${pid}.logs`);
  const badPointer = make('dvp-test', args, env);
  assert.match(badPointer.out, /refusing stack state file .*stack\.pid\.logs/);
  assert.notEqual(badPointer.status, 0);

  fs.unlinkSync(`${pid}.logs`);
  fs.writeFileSync(`${pid}.logs`, `${logs}\n`);
  fs.writeFileSync(path.join(logs, 'actual.env'), 'STACK_TEST_VALUE=owned\n');
  fs.symlinkSync(path.join(logs, 'actual.env'), path.join(logs, 'test.env'));
  const badEnv = make('dvp-test', args, env);
  assert.match(badEnv.out, /refusing stack state file .*test\.env/);
  assert.notEqual(badEnv.status, 0);

  fs.unlinkSync(path.join(logs, 'test.env'));
  fs.renameSync(path.join(logs, 'actual.env'), path.join(logs, 'test.env'));
  const ownedEnv = make('dvp-test', args, env);
  assert.equal(ownedEnv.status, 0, ownedEnv.out);
  assert.match(ownedEnv.out, /suite PASS: 1/);
});

test('the default stack reader still recognizes an owned legacy PID and log pointer', t => {
  const dir = scratch(t, 'stack-legacy-state-');
  const legacy = path.join(dir, 'enygma_dvp_stack.pid');
  fs.writeFileSync(legacy, `${process.pid}\n`);
  fs.writeFileSync(`${legacy}.logs`, `${dir}/existing-logs\n`);
  const pid = path.join(dir, `enygma-${process.getuid()}`, path.basename(legacy));
  const r = spawnSync('bash', [STATE, 'logs', pid], { encoding: 'utf8', env: { ...makeEnv, TMPDIR: dir } });
  assert.equal(r.status, 0, r.stderr);
  assert.equal(r.stdout.trim(), `${dir}/existing-logs`);
});

function fakeChrome(t, body) {
  const dir = scratch(t, 'chrome-pipe-');
  const bin = path.join(dir, 'chrome');
  fs.writeFileSync(bin, `#!/usr/bin/env node\n${body}`, { mode: 0o755 });
  const previous = process.env.CHROME;
  process.env.CHROME = bin;
  t.after(() => {
    if (previous === undefined) delete process.env.CHROME;
    else process.env.CHROME = previous;
  });
}

test('headless Chrome uses a pipe and reads split replies in the page session', async t => {
  fakeChrome(t, String.raw`
const fs = require('node:fs');
if (!process.argv.includes('--remote-debugging-pipe') ||
    process.argv.some(arg => arg.startsWith('--remote-debugging-port'))) process.exit(3);
const input = fs.createReadStream(null, { fd: 3, encoding: 'utf8' });
const output = fs.createWriteStream(null, { fd: 4 });
const frame = value => JSON.stringify(value) + '\0';
let remaining = '';
input.on('data', chunk => {
  const frames = (remaining + chunk).split('\0');
  remaining = frames.pop();
  for (const text of frames) {
    const { id, method, sessionId } = JSON.parse(text);
    if (method === 'Target.createTarget') output.write(frame({ id, result: { targetId: 'page' } }));
    if (method === 'Target.attachToTarget') output.write(frame({ id, result: { sessionId: 'attached' } }));
    if (method === 'Runtime.evaluate' && sessionId === 'attached') {
      const result = frame({ sessionId, method: 'Runtime.consoleAPICalled' }) +
        frame({ id, sessionId, result: { result: { value: 'pipe ✓' } } });
      output.write(result.slice(0, 5));
      setTimeout(() => output.write(result.slice(5)), 10);
    }
  }
});
`);
  await withChrome(async ({ evaluate, events }) => {
    assert.equal(await evaluate('1'), 'pipe ✓');
    assert.equal(events[0].method, 'Runtime.consoleAPICalled');
  });
});

test('headless Chrome rejects a command when its pipe closes', async t => {
  fakeChrome(t, 'process.exit(4);');
  await assert.rejects(withChrome(() => assert.fail('Chrome did not open a page')), /Chrome (closed|exited)|EPIPE/);
});
