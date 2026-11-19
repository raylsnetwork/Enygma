// make's page-check dependencies (PAGE_DEPS): on a node older than .nvmrc's
// major, they stop before npm ci, naming the node to switch to; on .nvmrc's
// node they install. Run on a scratch dir with the repo's Makefile, a fake
// node of the version under test and a fake npm that records its call (and
// makes node_modules, as npm ci does).
//
//   node --test scripts/page_deps.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const ROOT = path.join(__dirname, '..');

// deps runs make's PAGE_DEPS target in a scratch dir whose node is version
// (e.g. '18.20.8') and whose .nvmrc holds nvmrc; it returns make's exit code,
// its output and whether npm ran.
function deps(version, nvmrc = '22\n') {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'page-deps-test-'));
  try {
    const bin = path.join(dir, 'bin');
    fs.mkdirSync(bin);
    fs.mkdirSync(path.join(dir, 'scripts'));
    fs.writeFileSync(path.join(dir, 'scripts/package-lock.json'), '{}');
    fs.writeFileSync(path.join(dir, '.nvmrc'), nvmrc);
    fs.cpSync(path.join(ROOT, 'Makefile'), path.join(dir, 'Makefile'));
    const major = version.split('.')[0];
    fs.writeFileSync(path.join(bin, 'node'),
      `#!/bin/sh\ncase "$1" in -v) echo v${version} ;; -p) echo ${major} ;; *) exit 1 ;; esac\n`, { mode: 0o755 });
    fs.writeFileSync(path.join(bin, 'npm'), `#!/bin/sh\nmkdir -p node_modules && touch "${path.join(dir, 'npm-ran')}"\n`, { mode: 0o755 });
    const r = spawnSync('make', ['--no-print-directory', `scripts/node_modules/.installed-with-node-v${version}`], {
      cwd: dir, encoding: 'utf8', env: { ...process.env, PATH: `${bin}:${process.env.PATH}` },
    });
    return { code: r.status, out: r.stdout + r.stderr, npm: fs.existsSync(path.join(dir, 'npm-ran')) };
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

test("stops before npm ci on a node older than .nvmrc's, naming the one to switch to", () => {
  const r = deps('18.20.8');
  assert.notStrictEqual(r.code, 0, r.out);
  assert.ok(!r.npm, 'npm ran');
  assert.match(r.out, /node v18\.20\.8 is older than \.nvmrc's 22[^]*nvm use[^]*\.nvm\/versions\/node\/v22\.\*\/bin/);
});

test("installs on .nvmrc's node", () => {
  const r = deps('22.22.2');
  assert.strictEqual(r.code, 0, r.out);
  assert.ok(r.npm, 'npm did not run');
});

test('installs on a newer node', () => {
  const r = deps('24.1.0');
  assert.strictEqual(r.code, 0, r.out);
  assert.ok(r.npm, 'npm did not run');
});

test('fails, naming .nvmrc, on one that holds no node major', () => {
  const r = deps('22.22.2', 'lts/*\n');
  assert.notStrictEqual(r.code, 0, r.out);
  assert.ok(!r.npm, 'npm ran');
  assert.match(r.out, /\.nvmrc holds no node major/);
});
