// make retail-keys-check (scripts/retail_keys_check.sh) and make
// retail-keygen, on a scratch repo laid out like enygma_retail_payments'
// gnark_circuits and enygma_gnark_shared, with the repo's Makefile and
// scripts: the committed retail keys' stamp must name the working tree's
// circuit sources, and make retail-keygen writes it so that it does.
//
//   node --test scripts/retail_keys_check.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');
const { GIT_ENV, git, scratchRepo } = require('./test_git');

const ROOT = path.join(__dirname, '..');
const MODULE = 'enygma_retail_payments/gnark_circuits';
const STAMP = `${MODULE}/scripts/keys/keys.src`;
const goEnv = { ...GIT_ENV, GOFLAGS: '', GOWORK: 'off', GOTOOLCHAIN: 'local' };

// KEYGEN is the scratch keygen: it writes a key, or, as the scratch repo's
// KEYGEN_DO says, fails or edits a circuit source as it runs.
const KEYGEN = `package main

import (
	"os"

	"enygma_retail_payments/gnark_circuits/templates"
)

func main() {
	templates.Define()
	switch os.Getenv("KEYGEN_DO") {
	case "fail":
		os.Exit(1)
	case "edit":
		f, _ := os.OpenFile("templates/circuit.go", os.O_APPEND|os.O_WRONLY, 0)
		f.WriteString("// edited during keygen\\n")
		f.Close()
	}
	os.WriteFile("scripts/keys/PaymentPK.key", []byte("pk"), 0o644)
}
`;

const FILES = {
  [`${MODULE}/go.mod`]:
    'module enygma_retail_payments/gnark_circuits\n\ngo 1.21\n\nrequire enygma_gnark_shared v0.0.0\n\nreplace enygma_gnark_shared => ../../enygma_gnark_shared\n',
  [`${MODULE}/go.sum`]: '',
  [`${MODULE}/cmd/keygen/main.go`]: KEYGEN,
  [`${MODULE}/cmd/server/main.go`]: 'package main\n\nfunc main() {}\n',
  [`${MODULE}/templates/circuit.go`]:
    'package templates\n\nimport "enygma_gnark_shared/poseidon"\n\nfunc Define() { poseidon.Hash() }\n',
  [`${MODULE}/templates/circuit_test.go`]: 'package templates\n',
  [`${MODULE}/scripts/keys/PaymentPK.key`]: 'pk',
  'enygma_gnark_shared/go.mod': 'module enygma_gnark_shared\n\ngo 1.21\n',
  'enygma_gnark_shared/poseidon/poseidon.go': 'package poseidon\n\nfunc Hash() {}\n',
  'enygma_retail_payments/shield_demo/go.mod': 'module shield_demo\n\ngo 1.21\n',
  'enygma_retail_payments/shield_demo/main.go': 'package main\n\nfunc main() {}\n',
};

const at = (dir, f) => path.join(dir, f);

// repo writes FILES, the Makefile and the scripts the check runs to a
// scratch git repo, stamps the keys with keys_src.sh as make retail-keygen
// would, and commits it all; it returns the repo's root.
function repo() {
  const dir = scratchRepo('retail-keys-check-test');
  Object.entries(FILES).forEach(([f, c]) => {
    fs.mkdirSync(path.dirname(at(dir, f)), { recursive: true });
    fs.writeFileSync(at(dir, f), c);
  });
  fs.cpSync(at(ROOT, 'Makefile'), at(dir, 'Makefile'));
  // Copied, not rewritten, so they keep their mode.
  for (const f of ['retail_keys_check.sh', 'stack_lib.sh', 'keys_src.sh']) fs.cpSync(at(__dirname, f), at(dir, `scripts/${f}`));
  const src = spawnSync('bash', [at(dir, 'scripts/keys_src.sh'), MODULE, './cmd/keygen'], { encoding: 'utf8', env: goEnv });
  assert.strictEqual(src.status, 0, src.stderr);
  fs.writeFileSync(at(dir, STAMP), src.stdout);
  git(dir, 'add', '-A');
  git(dir, 'commit', '-qm', 'init');
  return dir;
}

// make runs make with args in dir, with env added; it returns the exit code
// and the output.
function make(dir, args, env = {}) {
  const r = spawnSync('make', ['--no-print-directory', ...args], { cwd: dir, encoding: 'utf8', env: { ...goEnv, ...env } });
  return { code: r.status, out: r.stdout + r.stderr };
}

// check runs make retail-keys-check in a fresh scratch repo after edit (on
// its root).
function check(edit = () => {}) {
  const dir = repo();
  try {
    edit(dir);
    return make(dir, ['retail-keys-check']);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

// failsNamingTheFix asserts r failed, naming each step that rebuilds the
// keys and what is built from them, in order.
function failsNamingTheFix(r) {
  assert.notStrictEqual(r.code, 0, r.out);
  assert.match(r.out, /make retail-keygen[^]*export_vk[^]*export_verifier[^]*PrivateMintVerifier[^]*make dvp-keygen/);
}

test("passes on keys stamped with the working tree's sources", () => {
  const r = check();
  assert.strictEqual(r.code, 0, r.out);
  assert.match(r.out, /retail keys: built from the working tree's circuit sources/);
});

test('fails, naming make retail-keygen and the steps after it, on an uncommitted edit to a circuit', () => {
  const r = check((d) => fs.appendFileSync(at(d, `${MODULE}/templates/circuit.go`), '// x\n'));
  failsNamingTheFix(r);
  assert.match(r.out, /built from other circuit sources/);
});

test('fails on a committed edit to a circuit', () => {
  failsNamingTheFix(check((d) => {
    fs.appendFileSync(at(d, `${MODULE}/templates/circuit.go`), '// x\n');
    git(d, 'commit', '-qam', 'edit');
  }));
});

test('fails on an edit to enygma_gnark_shared', () => {
  failsNamingTheFix(check((d) => fs.appendFileSync(at(d, 'enygma_gnark_shared/poseidon/poseidon.go'), '// x\n')));
});

test('fails, naming make retail-keygen, without a stamp', () => {
  const r = check((d) => fs.rmSync(at(d, STAMP)));
  failsNamingTheFix(r);
  assert.match(r.out, /no record of the circuit sources/);
});

test('passes on new file times (a rebase, a fresh checkout)', () => {
  const r = check((d) => {
    const t = new Date(Date.now() + 3600e3);
    Object.keys(FILES).forEach((f) => fs.utimesSync(at(d, f), t, t));
  });
  assert.strictEqual(r.code, 0, r.out);
});

for (const f of [`${MODULE}/templates/circuit_test.go`, `${MODULE}/cmd/server/main.go`, 'enygma_retail_payments/shield_demo/main.go']) {
  test(`passes on an edit to ${f}, which keygen does not build`, () => {
    const r = check((d) => fs.appendFileSync(at(d, f), '// x\n'));
    assert.strictEqual(r.code, 0, r.out);
  });
}

// keygen edits a circuit (after edit), runs make retail-keygen with
// KEYGEN_DO=how, then make retail-keys-check; it returns both results and
// whether the stamp is there.
function keygen(how = '', edit = () => {}) {
  const dir = repo();
  try {
    edit(dir);
    const gen = make(dir, ['retail-keygen'], { KEYGEN_DO: how });
    return { gen, check: make(dir, ['retail-keys-check']), stamped: fs.existsSync(at(dir, STAMP)) };
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

const editCircuit = (d) => fs.appendFileSync(at(d, `${MODULE}/templates/circuit.go`), '// x\n');

test('make retail-keygen after a circuit edit makes the check pass', () => {
  const r = keygen('', editCircuit);
  assert.strictEqual(r.gen.code, 0, r.gen.out);
  assert.strictEqual(r.check.code, 0, r.check.out);
});

test('make retail-keygen, failing, leaves no stamp (not the old one)', () => {
  const r = keygen('fail', editCircuit);
  assert.notStrictEqual(r.gen.code, 0, r.gen.out);
  assert.ok(!r.stamped, 'a failed keygen left a stamp');
  failsNamingTheFix(r.check);
});

test('make retail-keygen stamps the sources as they were before it ran', () => {
  const r = keygen('edit');
  assert.strictEqual(r.gen.code, 0, r.gen.out);
  failsNamingTheFix(r.check);
});

test('make check runs retail-keys-check', () => {
  const mk = fs.readFileSync(at(ROOT, 'Makefile'), 'utf8');
  const deps = /^check:([^#\n]*)/m.exec(mk);
  assert.ok(deps, 'the Makefile has no check target');
  assert.ok(deps[1].split(/\s+/).includes('retail-keys-check'), `check: ${deps[1]}`);
});

// keys.src is committed beside the LFS keys; it must be stored as itself.
test("keys.src is not an LFS file", () => {
  assert.match(git(ROOT, 'check-attr', 'filter', '--', STAMP), /: filter: unspecified$/);
});

// CI's fast jobs run make retail-keys-check, without the LFS keys it never opens.
test('CI runs make retail-keys-check', () => {
  const yml = fs.readFileSync(at(ROOT, '.github/workflows/ci.yml'), 'utf8');
  assert.match(yml, /^ +- run: make [^\n]*\bretail-keys-check\b/m);
});
