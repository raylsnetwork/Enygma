// scripts/keys_src.sh on a scratch repo laid out like enygma_dvp's
// gnark_circuits, enygma_payments' gnark-server and enygma_gnark_shared:
// which changes move the fingerprint make dvp-keygen and make
// payments-burn-keygen stamp the keys with, and which do not.
//
//   node --test scripts/keys_src.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const SCRIPT = path.join(__dirname, 'keys_src.sh');

// The arguments make dvp-keygen and make payments-burn-keygen pass.
const DVP = ['enygma_dvp/gnark_circuits', './cmd/keygen'];
const PAYMENTS = ['enygma_payments/gnark-server', './keygen'];

const FILES = {
  'enygma_dvp/gnark_circuits/go.mod':
    'module enygma_dvp/gnark_circuits\n\ngo 1.21\n\nrequire enygma_gnark_shared v0.0.0\n\nreplace enygma_gnark_shared => ../../enygma_gnark_shared\n',
  'enygma_dvp/gnark_circuits/go.sum': '',
  'enygma_dvp/gnark_circuits/cmd/keygen/main.go':
    'package main\n\nimport "enygma_dvp/gnark_circuits/templates"\n\nfunc main() { templates.Define() }\n',
  'enygma_dvp/gnark_circuits/cmd/server/main.go': 'package main\n\nfunc main() {}\n',
  'enygma_dvp/gnark_circuits/templates/circuit.go':
    'package templates\n\nimport "enygma_gnark_shared/poseidon"\n\nfunc Define() { poseidon.Hash() }\n',
  'enygma_dvp/gnark_circuits/templates/circuit_test.go': 'package templates\n',
  'enygma_dvp/gnark_circuits/relayer/relayer.go': 'package relayer\n',
  'enygma_gnark_shared/go.mod': 'module enygma_gnark_shared\n\ngo 1.21\n',
  'enygma_gnark_shared/poseidon/poseidon.go': 'package poseidon\n\nfunc Hash() {}\n',
  'enygma_payments/gnark-server/go.mod':
    'module gnark-server\n\ngo 1.21\n\nrequire enygma_gnark_shared v0.0.0\n\nreplace enygma_gnark_shared => ../../enygma_gnark_shared\n',
  'enygma_payments/gnark-server/go.sum': '',
  'enygma_payments/gnark-server/keygen/generate_keys.go':
    'package main\n\nimport "gnark-server/pkg/circuits/burn"\n\nfunc main() { burn.Define() }\n',
  'enygma_payments/gnark-server/pkg/circuits/burn/burn.go':
    'package burn\n\nimport "enygma_gnark_shared/poseidon"\n\nfunc Define() { poseidon.Hash() }\n',
  'enygma_payments/gnark-server/pkg/circuits/burn/burn_test.go': 'package burn\n',
  'enygma_payments/gnark-server/cmd/server/main.go': 'package main\n\nfunc main() {}\n',
  'enygma_payments/go_client/go.mod': 'module go_client\n\ngo 1.21\n',
  'enygma_payments/go_client/client.go': 'package client\n',
};

// repo writes FILES (plus the script) to a scratch git repo with one commit;
// it returns the repo's root.
function repo() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'keys-src-test-'));
  const files = { ...FILES, 'scripts/keys_src.sh': fs.readFileSync(SCRIPT, 'utf8') };
  Object.entries(files).forEach(([f, c]) => {
    fs.mkdirSync(path.dirname(path.join(dir, f)), { recursive: true });
    fs.writeFileSync(path.join(dir, f), c);
  });
  const git = (...a) => spawnSync('git', ['-C', dir, ...a], { encoding: 'utf8' });
  git('init', '-q');
  git('add', '-A');
  git('-c', 'user.name=t', '-c', 'user.email=t@t', 'commit', '-qm', 'init');
  return dir;
}

// run runs the scratch repo's copy of the script with args (DvP's unless given).
function run(dir, args = DVP) {
  return spawnSync('bash', [path.join(dir, 'scripts/keys_src.sh'), ...args], {
    encoding: 'utf8', env: { ...process.env, GOFLAGS: '', GOWORK: 'off', GOTOOLCHAIN: 'local' } });
}

// fingerprint runs the script; it fails the test on a non-zero exit or
// output that is not one sha256.
function fingerprint(dir, args = DVP) {
  const r = run(dir, args);
  assert.strictEqual(r.status, 0, r.stderr);
  assert.match(r.stdout, /^[0-9a-f]{64}\n$/);
  return r.stdout;
}

// changed says whether edit (run on the repo's root, after setup) moves the
// fingerprint the script prints for args (DvP's unless given).
function changed(edit, { setup = () => {}, args = DVP } = {}) {
  const dir = repo();
  try {
    setup(dir);
    const before = fingerprint(dir, args);
    edit(dir);
    return fingerprint(dir, args) !== before;
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

const at = (dir, f) => path.join(dir, f);

test('an uncommitted edit to a circuit template moves the fingerprint', () => {
  assert.ok(changed((d) => fs.appendFileSync(at(d, 'enygma_dvp/gnark_circuits/templates/circuit.go'), '// x\n')));
});

test('new file times on the same content (a rebase, a fresh checkout) leave it', () => {
  assert.ok(!changed((d) => {
    const t = new Date(Date.now() + 3600e3);
    Object.keys(FILES).forEach((f) => fs.utimesSync(at(d, f), t, t));
  }));
});

test('a new, untracked file in a circuit package moves it', () => {
  assert.ok(changed((d) => fs.writeFileSync(at(d, 'enygma_dvp/gnark_circuits/templates/extra.go'), 'package templates\n')));
});

test('an edit to enygma_gnark_shared, a module keygen replaces locally, moves it', () => {
  assert.ok(changed((d) => fs.appendFileSync(at(d, 'enygma_gnark_shared/poseidon/poseidon.go'), '// x\n')));
});

test('an edit to go.mod moves it', () => {
  assert.ok(changed((d) => fs.appendFileSync(at(d, 'enygma_dvp/gnark_circuits/go.mod'), '// x\n')));
});

// gadget is a package in the repo that templates does not import, until
// importGadget makes it.
const GADGET = 'enygma_dvp/gnark_circuits/gadget/gadget.go';
const addGadget = (d) => {
  fs.mkdirSync(path.dirname(at(d, GADGET)), { recursive: true });
  fs.writeFileSync(at(d, GADGET), 'package gadget\n\nfunc G() {}\n');
};
const editGadget = (d) => fs.appendFileSync(at(d, GADGET), '// x\n');

test('an edit to a package keygen does not import leaves it', () => {
  assert.ok(!changed(editGadget, { setup: addGadget }));
});

test('a package counts once a circuit imports it', () => {
  assert.ok(changed(editGadget, { setup: (d) => {
    addGadget(d);
    fs.writeFileSync(at(d, 'enygma_dvp/gnark_circuits/templates/uses_gadget.go'),
      'package templates\n\nimport "enygma_dvp/gnark_circuits/gadget"\n\nvar _ = gadget.G\n');
  } }));
});

test('fails, rather than fingerprint nothing, on a local replace outside the repo', () => {
  const dir = repo();
  const outside = fs.mkdtempSync(path.join(os.tmpdir(), 'dvp-keys-src-outside-'));
  try {
    fs.cpSync(at(dir, 'enygma_gnark_shared'), outside, { recursive: true });
    const gomod = at(dir, 'enygma_dvp/gnark_circuits/go.mod');
    fs.writeFileSync(gomod, fs.readFileSync(gomod, 'utf8').replace('../../enygma_gnark_shared', outside));
    const r = run(dir);
    assert.notStrictEqual(r.status, 0, r.stdout);
    assert.match(r.stderr, /outside the repo/);
    assert.strictEqual(r.stdout, '');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
    fs.rmSync(outside, { recursive: true, force: true });
  }
});

test("fails on a package none of whose sources git sees (all gitignored)", () => {
  const dir = repo();
  try {
    fs.writeFileSync(at(dir, '.gitignore'), 'enygma_gnark_shared/poseidon/poseidon.go\n');
    spawnSync('git', ['-C', dir, 'rm', '-q', '--cached', 'enygma_gnark_shared/poseidon/poseidon.go']);
    const r = run(dir);
    assert.notStrictEqual(r.status, 0, r.stdout);
    assert.match(r.stderr, /no non-test \.go files found for package enygma_gnark_shared\/poseidon/);
    assert.strictEqual(r.stdout, '');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('an edit to go.sum moves it', () => {
  assert.ok(changed((d) => fs.appendFileSync(at(d, 'enygma_dvp/gnark_circuits/go.sum'), 'example.com/x v1.0.0 h1:x=\n')));
});

for (const f of ['templates/circuit_test.go', 'cmd/server/main.go', 'relayer/relayer.go']) {
  test(`an edit to ${f}, which keygen does not build, leaves it`, () => {
    assert.ok(!changed((d) => fs.appendFileSync(at(d, `enygma_dvp/gnark_circuits/${f}`), '// x\n')));
  });
}

// A second module: enygma_payments' gnark-server, whose keygen is ./keygen.
const BURN = 'enygma_payments/gnark-server/pkg/circuits/burn/burn.go';

test("an edit to the payments burn circuit moves payments' fingerprint, not DvP's", () => {
  const edit = (d) => fs.appendFileSync(at(d, BURN), '// x\n');
  assert.ok(changed(edit, { args: PAYMENTS }));
  assert.ok(!changed(edit));
});

test("an edit to a DvP circuit template leaves payments' fingerprint", () => {
  assert.ok(!changed((d) => fs.appendFileSync(at(d, 'enygma_dvp/gnark_circuits/templates/circuit.go'), '// x\n'),
    { args: PAYMENTS }));
});

test("an edit to enygma_gnark_shared, which payments' keygen also replaces locally, moves payments' fingerprint", () => {
  assert.ok(changed((d) => fs.appendFileSync(at(d, 'enygma_gnark_shared/poseidon/poseidon.go'), '// x\n'),
    { args: PAYMENTS }));
});

test("an edit to gnark-server's go.sum moves payments' fingerprint", () => {
  assert.ok(changed((d) => fs.appendFileSync(at(d, 'enygma_payments/gnark-server/go.sum'), 'example.com/x v1.0.0 h1:x=\n'),
    { args: PAYMENTS }));
});

for (const f of ['enygma_payments/gnark-server/pkg/circuits/burn/burn_test.go',
  'enygma_payments/gnark-server/cmd/server/main.go', 'enygma_payments/go_client/client.go']) {
  test(`an edit to ${f}, which payments' keygen does not build, leaves its fingerprint`, () => {
    assert.ok(!changed((d) => fs.appendFileSync(at(d, f), '// x\n'), { args: PAYMENTS }));
  });
}

test('the two modules fingerprint differently', () => {
  const dir = repo();
  try {
    assert.notStrictEqual(fingerprint(dir, PAYMENTS), fingerprint(dir, DVP));
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('fails, naming its usage, without a module dir and a keygen package', () => {
  const dir = repo();
  try {
    for (const args of [[], [DVP[0]]]) {
      const r = run(dir, args);
      assert.notStrictEqual(r.status, 0, r.stdout);
      assert.match(r.stderr, /usage: scripts\/keys_src\.sh <module dir> <keygen package>/);
      assert.strictEqual(r.stdout, '');
    }
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

// onchain.yml's dvp and payments jobs cache the DvP and Burn keys. Keyed on
// anything but the fingerprint of the sources they were built from, a cache
// can restore keys whose stamp the stack's preflight rejects (or keys from
// other sources under a stale key).
const CACHES = [
  { cache: 'dvp-keys', src: 'dvp-keys-src', prefix: 'dvp-keys-', args: DVP },
  { cache: 'burn-keys', src: 'burn-keys-src', prefix: 'payments-burn-keys-', args: PAYMENTS },
];
const re = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
for (const { cache, src, prefix, args } of CACHES) {
  test(`CI's ${cache} cache is keyed on this fingerprint of ${args[0]}`, () => {
    const yml = fs.readFileSync(path.join(__dirname, '..', '.github/workflows/onchain.yml'), 'utf8');
    const step = new RegExp(`- id: ${re(src)}\\n((?: {8}.*\\n)+)`).exec(yml);
    assert.ok(step, `onchain.yml has no ${src} step`);
    assert.match(step[1], new RegExp(`src=\\$\\(scripts/keys_src\\.sh ${re(args.join(' '))}\\)`),
      `the ${src} step does not run scripts/keys_src.sh ${args.join(' ')}`);
    const c = new RegExp(`- id: ${re(cache)}\\n((?: {8}.*\\n)+)`).exec(yml);
    assert.ok(c, `onchain.yml has no ${cache} cache step`);
    assert.match(c[1], new RegExp(`^ +key: ${re(prefix)}\\$\\{\\{ steps\\.${re(src)}\\.outputs\\.src \\}\\}`, 'm'),
      `the ${cache} cache key does not start with the ${src} step's fingerprint`);
  });
}
