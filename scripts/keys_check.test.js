// make retail-keys-check and make payments-keys-check (scripts/keys_check.sh),
// and make retail-keygen and make payments-keygen, on scratch repos laid out
// like each module's keygen, circuits and enygma_gnark_shared, with the repo's
// Makefile and scripts: a module's committed keys' stamp must name the working
// tree's circuit sources, and its keygen target writes it so that it does.
//
//   node --test scripts/keys_check.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');
const { GIT_ENV, git, scratchRepo } = require('./test_git');

const ROOT = path.join(__dirname, '..');
const goEnv = { ...GIT_ENV, GOFLAGS: '', GOWORK: 'off', GOTOOLCHAIN: 'local' };

// keygenSource is a scratch keygen in module mod, building circuit package
// pkg: it writes key, or, as the scratch repo's KEYGEN_DO says, fails or
// edits circuit (module-relative) as it runs.
const keygenSource = (mod, pkg, circuit, key) => `package main

import (
	"os"

	"${mod}/${pkg}"
)

func main() {
	${path.basename(pkg)}.Define()
	switch os.Getenv("KEYGEN_DO") {
	case "fail":
		os.Exit(1)
	case "edit":
		f, _ := os.OpenFile("${circuit}", os.O_APPEND|os.O_WRONLY, 0)
		f.WriteString("// edited during keygen\\n")
		f.Close()
	}
	os.WriteFile("${key}", []byte("pk"), 0o644)
}
`;

const circuitSource = (pkg) =>
  `package ${path.basename(pkg)}\n\nimport "enygma_gnark_shared/poseidon"\n\nfunc Define() { poseidon.Hash() }\n`;

const goMod = (mod) =>
  `module ${mod}\n\ngo 1.21\n\nrequire enygma_gnark_shared v0.0.0\n\nreplace enygma_gnark_shared => ../../enygma_gnark_shared\n`;

const SHARED = {
  'enygma_gnark_shared/go.mod': 'module enygma_gnark_shared\n\ngo 1.21\n',
  'enygma_gnark_shared/poseidon/poseidon.go': 'package poseidon\n\nfunc Hash() {}\n',
};
const MAIN = 'package main\n\nfunc main() {}\n';

// Each module whose keys are committed: its dir and keygen package, its
// committed stamp and any other stamp its keygen target writes, the scratch files (a circuit, the keygen, and files keygen does
// not build), and the steps after keygen its failure must name, in order.
const RETAIL = 'enygma_retail_payments/gnark_circuits';
const PAYMENTS = 'enygma_payments/gnark-server';
const MODULES = [
  {
    name: 'retail',
    dir: RETAIL,
    pkg: './cmd/keygen',
    stamp: `${RETAIL}/scripts/keys/keys.src`,
    stamps: [`${RETAIL}/scripts/keys/keys.src`],
    circuit: `${RETAIL}/templates/circuit.go`,
    keygen: `${RETAIL}/cmd/keygen/main.go`,
    files: {
      [`${RETAIL}/go.mod`]: goMod(RETAIL),
      [`${RETAIL}/go.sum`]: '',
      [`${RETAIL}/cmd/keygen/main.go`]: keygenSource(RETAIL, 'templates', 'templates/circuit.go', 'scripts/keys/PaymentPK.key'),
      [`${RETAIL}/cmd/server/main.go`]: MAIN,
      [`${RETAIL}/templates/circuit.go`]: circuitSource('templates'),
      [`${RETAIL}/templates/circuit_test.go`]: 'package templates\n',
      [`${RETAIL}/scripts/keys/PaymentPK.key`]: 'pk',
      'enygma_retail_payments/shield_demo/go.mod': 'module shield_demo\n\ngo 1.21\n',
      'enygma_retail_payments/shield_demo/main.go': MAIN,
    },
    unbuilt: [`${RETAIL}/templates/circuit_test.go`, `${RETAIL}/cmd/server/main.go`, 'enygma_retail_payments/shield_demo/main.go'],
    fix: /make retail-keygen[^]*export_vk[^]*export_verifier[^]*PrivateMintVerifier[^]*make dvp-keygen/,
  },
  {
    name: 'payments',
    dir: PAYMENTS,
    pkg: './keygen',
    stamp: `${PAYMENTS}/keys/keys.src`,
    stamps: [`${PAYMENTS}/keys/keys.src`, `${PAYMENTS}/keys/burn.src`],
    circuit: `${PAYMENTS}/pkg/circuits/enygma/circuit.go`,
    keygen: `${PAYMENTS}/keygen/generate_keys.go`,
    files: {
      [`${PAYMENTS}/go.mod`]: goMod(PAYMENTS),
      [`${PAYMENTS}/go.sum`]: '',
      [`${PAYMENTS}/keygen/generate_keys.go`]: keygenSource(PAYMENTS, 'pkg/circuits/enygma', 'pkg/circuits/enygma/circuit.go', 'keys/EnygmaPk.key'),
      [`${PAYMENTS}/cmd/server/main.go`]: MAIN,
      [`${PAYMENTS}/pkg/circuits/enygma/circuit.go`]: circuitSource('enygma'),
      [`${PAYMENTS}/pkg/circuits/enygma/circuit_test.go`]: 'package enygma\n',
      [`${PAYMENTS}/keys/EnygmaPk.key`]: 'pk',
      'enygma_payments/go_client/go.mod': 'module go_client\n\ngo 1.21\n',
      'enygma_payments/go_client/main.go': MAIN,
      'enygma_payments/relayer/go.mod': 'module relayer\n\ngo 1.21\n',
      'enygma_payments/relayer/main.go': MAIN,
    },
    unbuilt: [
      `${PAYMENTS}/pkg/circuits/enygma/circuit_test.go`,
      `${PAYMENTS}/cmd/server/main.go`,
      'enygma_payments/go_client/main.go',
      'enygma_payments/relayer/main.go',
    ],
    fix: /make payments-keygen[^]*keys\/EnygmaVerifier\.sol[^]*contracts\/enygma\/contracts\/EnygmaVerifier\.sol[^]*EnygmaFeeVerifier\.sol[^]*UsdrVerifier\.sol[^]*DepositVerifier\.sol[^]*WithdrawVerifier6\.sol/,
  },
];

const at = (dir, f) => path.join(dir, f);

// keysSrc runs keys_src.sh in dir on m's keygen, as its keygen target does,
// and returns the fingerprint.
function keysSrc(dir, m) {
  const src = spawnSync('bash', [at(dir, 'scripts/keys_src.sh'), m.dir, m.pkg], { encoding: 'utf8', env: goEnv });
  assert.strictEqual(src.status, 0, src.stderr);
  return src.stdout;
}

// repo writes m's files, the Makefile and the scripts the check runs to a
// scratch git repo, writes each of m's stamps with keys_src.sh as m's keygen
// target would, and commits it all; it returns the repo's root.
function repo(m) {
  const dir = scratchRepo(`${m.name}-keys-check-test`);
  Object.entries({ ...m.files, ...SHARED }).forEach(([f, c]) => {
    fs.mkdirSync(path.dirname(at(dir, f)), { recursive: true });
    fs.writeFileSync(at(dir, f), c);
  });
  fs.cpSync(at(ROOT, 'Makefile'), at(dir, 'Makefile'));
  // Copied, not rewritten, so they keep their mode.
  for (const f of ['keys_check.sh', 'stack_lib.sh', 'keys_src.sh']) fs.cpSync(at(__dirname, f), at(dir, `scripts/${f}`));
  const src = keysSrc(dir, m);
  m.stamps.forEach((f) => fs.writeFileSync(at(dir, f), src));
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

// inRepo runs fn on a fresh scratch repo for m, then removes it.
function inRepo(m, fn) {
  const dir = repo(m);
  try {
    return fn(dir);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

const append = (d, f) => fs.appendFileSync(at(d, f), '// x\n');

for (const m of MODULES) {
  const target = `${m.name}-keys-check`;
  const keygen = `${m.name}-keygen`;

  // check runs make <target> in a fresh scratch repo after edit (on its root).
  const check = (edit = () => {}) => inRepo(m, (d) => {
    edit(d);
    return make(d, [target]);
  });

  // failsNamingTheFix asserts r failed, naming each step that rebuilds the
  // keys and what is built from them, in order.
  const failsNamingTheFix = (r) => {
    assert.notStrictEqual(r.code, 0, r.out);
    assert.match(r.out, m.fix);
  };

  test(`${target}: passes on keys stamped with the working tree's sources`, () => {
    const r = check();
    assert.strictEqual(r.code, 0, r.out);
    assert.match(r.out, new RegExp(`${m.name} keys: built from the working tree's circuit sources`));
  });

  test(`${target}: fails, naming make ${keygen} and the steps after it, on an uncommitted edit to a circuit`, () => {
    const r = check((d) => append(d, m.circuit));
    failsNamingTheFix(r);
    assert.match(r.out, /built from other circuit sources/);
  });

  test(`${target}: fails on a committed edit to a circuit`, () => {
    failsNamingTheFix(check((d) => {
      append(d, m.circuit);
      git(d, 'commit', '-qam', 'edit');
    }));
  });

  test(`${target}: fails on an edit to enygma_gnark_shared`, () => {
    failsNamingTheFix(check((d) => append(d, 'enygma_gnark_shared/poseidon/poseidon.go')));
  });

  test(`${target}: fails on an edit to the keygen`, () => {
    failsNamingTheFix(check((d) => append(d, m.keygen)));
  });

  test(`${target}: fails, naming make ${keygen}, without a stamp`, () => {
    const r = check((d) => fs.rmSync(at(d, m.stamp)));
    failsNamingTheFix(r);
    assert.match(r.out, /no record of the circuit sources/);
  });

  test(`${target}: passes on new file times (a rebase, a fresh checkout)`, () => {
    const r = check((d) => {
      const t = new Date(Date.now() + 3600e3);
      Object.keys(m.files).forEach((f) => fs.utimesSync(at(d, f), t, t));
    });
    assert.strictEqual(r.code, 0, r.out);
  });

  for (const f of m.unbuilt) {
    test(`${target}: passes on an edit to ${f}, which keygen does not build`, () => {
      const r = check((d) => append(d, f));
      assert.strictEqual(r.code, 0, r.out);
    });
  }

  // runKeygen edits a circuit (after edit), runs make <keygen> with
  // KEYGEN_DO=how, then make <target>; it returns both results and the
  // stamps left (of m's stamps).
  const runKeygen = (how = '', edit = () => {}) => inRepo(m, (d) => {
    edit(d);
    const gen = make(d, [keygen], { KEYGEN_DO: how });
    return { gen, check: make(d, [target]), stamped: m.stamps.filter((f) => fs.existsSync(at(d, f))) };
  });
  const editCircuit = (d) => append(d, m.circuit);

  test(`make ${keygen} after a circuit edit makes ${target} pass`, () => {
    const r = runKeygen('', editCircuit);
    assert.strictEqual(r.gen.code, 0, r.gen.out);
    assert.strictEqual(r.check.code, 0, r.check.out);
  });

  test(`make ${keygen}, failing, leaves no stamp (not the old ones)`, () => {
    const r = runKeygen('fail', editCircuit);
    assert.notStrictEqual(r.gen.code, 0, r.gen.out);
    assert.deepStrictEqual(r.stamped, [], 'a failed keygen left these stamps');
    failsNamingTheFix(r.check);
  });

  test(`make ${keygen} stamps the sources as they were before it ran`, () => {
    const r = runKeygen('edit');
    assert.strictEqual(r.gen.code, 0, r.gen.out);
    failsNamingTheFix(r.check);
  });

  test(`make check runs ${target}`, () => {
    const mk = fs.readFileSync(at(ROOT, 'Makefile'), 'utf8');
    const deps = /^check:([^#\n]*)/m.exec(mk);
    assert.ok(deps, 'the Makefile has no check target');
    assert.ok(deps[1].split(/\s+/).includes(target), `check: ${deps[1]}`);
  });

  // keys.src is committed beside the LFS keys; it must be stored as itself.
  test(`${m.stamp} is not an LFS file`, () => {
    assert.match(git(ROOT, 'check-attr', 'filter', '--', m.stamp), /: filter: unspecified$/);
  });

  // CI's fast jobs run the check, without the LFS keys it never opens.
  test(`CI runs make ${target}`, () => {
    const yml = fs.readFileSync(at(ROOT, '.github/workflows/ci.yml'), 'utf8');
    assert.match(yml, new RegExp(`^ +- run: make [^\\n]*\\b${target}\\b`, 'm'));
  });
}

// make payments-keygen regenerates the gitignored Burn keys too (-circuit
// all), so it stamps them as make payments-burn-keygen does: after either,
// the payments stack's Burn preflight (enygma_payments/run_scripts/stack.sh)
// passes on a circuit edited before it ran.
for (const keygen of ['payments-keygen', 'payments-burn-keygen']) {
  test(`make ${keygen} stamps the Burn keys for the payments stack's preflight`, () => {
    const m = MODULES.find((x) => x.name === 'payments');
    const burn = `${PAYMENTS}/keys/burn.src`;
    inRepo(m, (d) => {
      append(d, m.circuit);
      const gen = make(d, [keygen]);
      assert.strictEqual(gen.code, 0, gen.out);
      const pre = spawnSync('bash', ['-c', `. scripts/stack_lib.sh; check_keys_stamp 'Burn keys' ${PAYMENTS} ${m.pkg} ${burn} payments-burn-keygen`],
        { cwd: d, encoding: 'utf8', env: goEnv });
      assert.strictEqual(pre.status, 0, pre.stdout + pre.stderr);
    });
  });
}

test('keys_check.sh fails, printing its usage, on a module it does not know', () => {
  const r = spawnSync('bash', [at(__dirname, 'keys_check.sh'), 'dvp'], { encoding: 'utf8', env: goEnv });
  assert.strictEqual(r.status, 2, r.stdout + r.stderr);
  assert.match(r.stderr, /usage: scripts\/keys_check\.sh retail\|payments/);
});
