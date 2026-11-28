// The ceremony's release (scripts/ceremony_release.sh, and make ceremony-dist)
// takes the tag, the coordinator's address, the repository and the commit as
// data: none that is not plain gets used (audit L14, M8), the notes give
// two-step commands that quote them, and the release is published only while
// its tag names the commit built (audit M10).
//
//   node --test scripts/ceremony_release.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');
const { git, makeEnv } = require('./test_git');

const ROOT = path.join(__dirname, '..');
const SCRIPT = path.join(__dirname, 'ceremony_release.sh');
const SHA = 'f'.repeat(40);
const GOOD = {
  TAG: 'ceremony-v1.2',
  CEREMONY_URL: 'https://ceremony.example.org:8443',
  GITHUB_REPOSITORY: 'owner/repo',
  GITHUB_SHA: SHA,
};
const FILES = ['ceremony-darwin-arm64', 'ceremony-darwin-amd64', 'ceremony-linux-amd64', 'ceremony-linux-arm64',
  'ceremony-windows-amd64.exe', 'ceremony-windows-arm64.exe'];

function tmp() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'ceremony-release-'));
}

function run(args, env, extraPath) {
  const PATH = extraPath ? `${extraPath}:${process.env.PATH}` : process.env.PATH;
  return spawnSync('bash', [SCRIPT, ...args], { env: { PATH, HOME: process.env.HOME, ...env }, encoding: 'utf8' });
}

// dist writes a dist directory with each binary and SHA256SUMS.
function dist() {
  const dir = tmp();
  const sums = FILES.map((f, i) => {
    fs.writeFileSync(path.join(dir, f), f);
    return `${String(i).repeat(64)}  ${f}`;
  });
  fs.writeFileSync(path.join(dir, 'SHA256SUMS'), sums.join('\n') + '\n');
  fs.writeFileSync(path.join(dir, 'BUILD'), 'build\n');
  return dir;
}

test('check takes plain values', () => {
  const r = run(['check'], GOOD);
  assert.strictEqual(r.status, 0, r.stderr);
});

test('check refuses a tag, an address, a repository or a commit that is not plain', () => {
  const marker = path.join(tmp(), 'ran');
  const bad = {
    TAG: [`ceremony-v1$(touch ${marker})`, 'ceremony-v1;id', 'ceremony-v1\nx', 'v1', 'ceremony-v1.', 'ceremony-v`id`'],
    CEREMONY_URL: ['', 'http://ceremony.example.org', 'https://a.example/path', 'https://a.example\nid',
      `https://a.example$(touch ${marker})`, "https://a.example'", 'https://a.example "-X main.x=y', 'https://a.example:123456'],
    GITHUB_REPOSITORY: ['owner', 'owner/repo;id', "o/r'x"],
    GITHUB_SHA: ['abc', 'F'.repeat(40), SHA + '\n'],
  };
  for (const [name, values] of Object.entries(bad)) {
    for (const value of values) {
      const r = run(['check'], { ...GOOD, [name]: value });
      assert.notStrictEqual(r.status, 0, `${name}=${JSON.stringify(value)} was taken`);
    }
  }
  assert.ok(!fs.existsSync(marker), 'a value ran as code');
});

test('notes give two steps for each system, quoted, and the pinned origin check', () => {
  const dir = dist();
  const r = run(['notes', dir], GOOD);
  assert.strictEqual(r.status, 0, r.stderr);
  const commands = fs.readFileSync(path.join(dir, 'COMMANDS.txt'), 'utf8');
  const notes = fs.readFileSync(path.join(dir, 'notes.md'), 'utf8');
  for (const f of FILES) {
    assert.ok(commands.includes(`'https://github.com/owner/repo/releases/download/ceremony-v1.2/${f}'`), `no quoted download of ${f}`);
  }
  // The download and its check, then, apart, the run, which checks again: a
  // pasted block never runs a program that fails the check.
  assert.match(commands, /1\. Download and check:\ncurl -fsSLO '[^']+\/ceremony-darwin-arm64' && echo '[0-9a-f]{64}  ceremony-darwin-arm64' \| shasum -a 256 -c\n2\. Compare [0-9a-f]{64} with the announcement, then run:\necho '[0-9a-f]{64}  ceremony-darwin-arm64' \| shasum -a 256 -c && chmod \+x 'ceremony-darwin-arm64' && '\.\/ceremony-darwin-arm64'\n/);
  assert.match(commands, /if \(\(Get-FileHash 'ceremony-windows-amd64\.exe' -Algorithm SHA256\)\.Hash -eq '[0-9a-f]{64}'\) \{ & '\.\\ceremony-windows-amd64\.exe' \} else \{ throw/);
  assert.match(commands, /Get-FileHash 'ceremony-windows-amd64\.exe' -Algorithm SHA256\)\.Hash -eq '[0-9a-f]{64}'\) \{ 'ceremony-windows-amd64\.exe matches this release' \} else \{ throw/);
  // From source, the build and the run only at the commit announced.
  assert.match(commands, /git checkout 'f{40}'\n\[ "\$\(git rev-parse HEAD\)" = 'f{40}' \] && cd enygma_ceremony\/cmd\/ceremony && go build && \.\/ceremony join -url 'https:\/\/ceremony\.example\.org:8443'/);
  assert.match(commands, /git checkout 'f{40}'\nif \(\(git rev-parse HEAD\) -eq 'f{40}'\) \{ cd enygma_ceremony\/cmd\/ceremony; go build; if \(\$\?\) \{ \.\\ceremony\.exe join -url 'https:\/\/ceremony\.example\.org:8443' \} \} else \{ throw/);
  // Every system's commands on the page, none of them a comment, which
  // interactive zsh would run as a command.
  for (const f of FILES) {
    assert.ok(notes.includes(`'https://github.com/owner/repo/releases/download/ceremony-v1.2/${f}'`), `no commands for ${f} in the notes`);
  }
  const blocks = [...notes.matchAll(/```(?:sh|powershell)\n([\s\S]*?)```/g)];
  assert.strictEqual(blocks.length, 2 * FILES.length + 2, 'a block per step, and from source two');
  for (const [, block] of blocks) {
    assert.doesNotMatch(block, /^#/m, 'a code block holds a comment');
  }
  assert.ok(notes.startsWith("**Before you run anything, compare with the ceremony's announcement.**"), 'the notes do not open with the announcement');
  assert.ok(notes.includes("--signer-workflow 'owner/repo/.github/workflows/ceremony-release.yml' --source-ref 'refs/tags/ceremony-v1.2' --source-digest '" + SHA + "' --deny-self-hosted-runners"),
    'the origin check is not pinned');
  for (const [i, f] of FILES.entries()) {
    assert.ok(notes.includes(`| \`${f}\` | \`${String(i).repeat(64)}\` |`), `no sha256 for ${f} in the notes`);
  }
});

test('each command says what it takes', () => {
  for (const args of [['notes'], ['publish'], ['notes', 'a', 'b'], ['what']]) {
    const r = run(args, GOOD);
    assert.notStrictEqual(r.status, 0, `${args} ran`);
    assert.match(r.stderr, /notes DIST|publish DIST|usage:/);
  }
});

test('notes refuse a dist without each sha256, or plain values', () => {
  const dir = dist();
  fs.writeFileSync(path.join(dir, 'SHA256SUMS'), `${'0'.repeat(64)}  ceremony-darwin-arm64\n`);
  assert.notStrictEqual(run(['notes', dir], GOOD).status, 0);
  assert.notStrictEqual(run(['notes', dist()], { ...GOOD, TAG: 'ceremony-v1;id' }).status, 0);
});

// fakeGh puts a gh on PATH that answers the tag's commit from moves, one
// answer each time it is asked (the last one repeating), and logs every call.
// An empty answer is an API that did not answer. Options: annotated, the
// tag's ref names a tag object, which names the commit; nested, through a
// second tag object; selfTag, a tag object naming itself; verifyFails,
// release verify fails that many times first; existing, the ids of the
// releases for the tag already there; deleteFails, release delete fails.
function fakeGh(moves, { annotated, nested, selfTag, verifyFails = 0, existing = '', deleteFails } = {}) {
  const dir = tmp();
  const log = path.join(dir, 'log');
  const answers = path.join(dir, 'answers');
  const E = 'e'.repeat(40);
  const D = 'd'.repeat(40);
  fs.writeFileSync(answers, moves.join('\n') + '\n');
  fs.writeFileSync(path.join(dir, 'fails'), `${verifyFails}`);
  fs.writeFileSync(path.join(dir, 'gh'), `#!/usr/bin/env bash
echo "$*" >> '${log}'
if [[ "$1 $2" == "release verify" ]]; then
  n=$(cat '${dir}/fails'); [[ $n -gt 0 ]] && { echo $((n - 1)) > '${dir}/fails'; exit 1; }
elif [[ "$1 $2" == "release delete" ]]; then
  [[ -n '${deleteFails ? 'yes' : ''}' ]] && exit 1
elif [[ "$1 $2" == "api repos/owner/repo/releases" ]]; then
  printf '%s' '${existing}'
elif [[ "$1 $2" == "api repos/owner/repo/git/ref/tags/ceremony-v1.2" ]]; then
  sha=$(head -1 '${answers}')
  [[ $(wc -l < '${answers}') -gt 1 ]] && sed -i.bak 1d '${answers}'
  if [[ -n '${annotated || selfTag ? 'yes' : ''}' ]]; then echo "${E} tag"; echo "$sha" > '${answers}.tagged'; else echo "$sha commit"; fi
elif [[ "$1 $2" == "api repos/owner/repo/git/tags/${E}" ]]; then
  if [[ -n '${selfTag ? 'yes' : ''}' ]]; then echo "${E} tag"
  elif [[ -n '${nested ? 'yes' : ''}' ]]; then echo "${D} tag"
  else echo "$(cat '${answers}.tagged') commit"; fi
elif [[ "$1 $2" == "api repos/owner/repo/git/tags/${D}" ]]; then
  echo "$(cat '${answers}.tagged') commit"
fi
exit 0
`);
  fs.chmodSync(path.join(dir, 'gh'), 0o755);
  return { dir, calls: () => (fs.existsSync(log) ? fs.readFileSync(log, 'utf8').trim().split('\n') : []) };
}

test('publish publishes a draft, then the release, while the tag names the commit built', () => {
  const gh = fakeGh([SHA]);
  const r = run(['publish', dist()], GOOD, gh.dir);
  assert.strictEqual(r.status, 0, r.stderr);
  const calls = gh.calls().filter((c) => c.startsWith('release'));
  assert.match(calls[0], /^release create ceremony-v1\.2 --repo owner\/repo --draft --verify-tag/);
  assert.match(calls[0], /COMMANDS\.txt/);
  assert.match(calls[1], /^release edit ceremony-v1\.2 --repo owner\/repo --draft=false/);
  assert.match(calls[2], /^release verify ceremony-v1\.2/);
});

test('publish follows an annotated tag to the commit it names', () => {
  const ok = fakeGh([SHA], { annotated: true });
  assert.strictEqual(run(['publish', dist()], GOOD, ok.dir).status, 0);
  const nested = fakeGh([SHA], { annotated: true, nested: true }); // a tag of a tag
  assert.strictEqual(run(['publish', dist()], GOOD, nested.dir).status, 0);
  const self = fakeGh([SHA], { selfTag: true }); // never a commit
  const r = run(['publish', dist()], GOOD, self.dir);
  assert.notStrictEqual(r.status, 0);
  assert.match(r.stderr, /after following 5 tags/);
  const moved = fakeGh(['a'.repeat(40)], { annotated: true });
  assert.notStrictEqual(run(['publish', dist()], GOOD, moved.dir).status, 0);
});

test('publish says an API that did not answer is not a tag that moved, and where the release stands', () => {
  const before = fakeGh(['']);
  let r = run(['publish', dist()], GOOD, before.dir);
  assert.notStrictEqual(r.status, 0);
  assert.match(r.stderr, /did not say which commit .*nothing is published/);
  assert.deepStrictEqual(before.calls().filter((c) => c.startsWith('release')), []);
  const draft = fakeGh([SHA, '']);
  r = run(['publish', dist()], GOOD, draft.dir);
  assert.notStrictEqual(r.status, 0);
  assert.match(r.stderr, /did not say which commit .*a draft release .* is left/);
  assert.deepStrictEqual(draft.calls().filter((c) => c.startsWith('release')).map((c) => c.split(' ')[1]), ['create']);
  const published = fakeGh([SHA, SHA, '']);
  r = run(['publish', dist()], GOOD, published.dir);
  assert.notStrictEqual(r.status, 0);
  assert.match(r.stderr, /did not say which commit .*is published, but not checked/);
  assert.doesNotMatch(r.stderr, /moved/);
});

test('publish waits for the release to verify, and says so when it never does', () => {
  const late = fakeGh([SHA], { verifyFails: 2 });
  assert.strictEqual(run(['publish', dist()], { ...GOOD, VERIFY_WAIT: '0' }, late.dir).status, 0);
  const never = fakeGh([SHA], { verifyFails: 100 });
  const r = run(['publish', dist()], { ...GOOD, VERIFY_WAIT: '0' }, never.dir);
  assert.notStrictEqual(r.status, 0);
  assert.match(r.stderr, /is published, but gh release verify/);
});

test('publish publishes nothing while a release of the tag, or a draft, exists', () => {
  const gh = fakeGh([SHA], { existing: '12345' });
  const r = run(['publish', dist()], GOOD, gh.dir);
  assert.notStrictEqual(r.status, 0);
  assert.match(r.stderr, /exists already/);
  assert.deepStrictEqual(gh.calls().filter((c) => c.startsWith('release')), []);
});

test('publish says so when the draft a moved tag left cannot be deleted', () => {
  const gh = fakeGh([SHA, 'a'.repeat(40)], { deleteFails: true });
  const r = run(['publish', dist()], GOOD, gh.dir);
  assert.notStrictEqual(r.status, 0);
  assert.match(r.stderr, /could not be deleted/);
});

test('publish publishes nothing when the tag names another commit', () => {
  const gh = fakeGh(['a'.repeat(40)]);
  const r = run(['publish', dist()], GOOD, gh.dir);
  assert.notStrictEqual(r.status, 0);
  assert.deepStrictEqual(gh.calls().filter((c) => c.startsWith('release')), []);
});

test('publish deletes the draft when the tag moves before it is published', () => {
  const gh = fakeGh([SHA, 'a'.repeat(40)]);
  const r = run(['publish', dist()], GOOD, gh.dir);
  assert.notStrictEqual(r.status, 0);
  const calls = gh.calls().filter((c) => c.startsWith('release'));
  assert.match(calls[0], /^release create/);
  assert.match(calls[1], /^release delete ceremony-v1\.2 --repo owner\/repo --yes/);
  assert.strictEqual(calls.length, 2, `published anyway: ${calls}`);
});

test('publish fails, loudly, when the tag moves as the release is published', () => {
  const gh = fakeGh([SHA, SHA, 'a'.repeat(40)]);
  const r = run(['publish', dist()], GOOD, gh.dir);
  assert.notStrictEqual(r.status, 0);
  assert.match(r.stderr, /moved .* as the release was published/);
  assert.ok(!gh.calls().some((c) => c.startsWith('release verify')), 'verified a release at another commit');
});

test('make ceremony-dist takes CEREMONY_URL as data, from the command line or the environment', () => {
  const marker = path.join(tmp(), 'ran');
  for (const url of [`https://x$(shell touch ${marker})`, `https://x\`touch ${marker}\``, 'https://a.example "-X main.x=y']) {
    for (const how of ['line', 'env']) {
      const args = ['-C', ROOT, 'ceremony-dist'];
      if (how === 'line') args.push(`CEREMONY_URL=${url}`);
      const r = spawnSync('make', args, { env: { ...makeEnv, ...(how === 'env' ? { CEREMONY_URL: url } : {}) }, encoding: 'utf8' });
      assert.notStrictEqual(r.status, 0, `${how} ${url} was built`);
      assert.match(r.stderr, /CEREMONY_URL is a plain https:\/\/host\[:port\]/);
    }
  }
  assert.ok(!fs.existsSync(marker), 'CEREMONY_URL ran as code');
});

test('the ceremony checks out with LF line endings everywhere', () => {
  for (const f of ['enygma_ceremony/coordinator/page.html', 'enygma_ceremony/cmd/ceremony/main.go', 'enygma_ceremony/go.mod']) {
    assert.strictEqual(git(ROOT, 'check-attr', 'eol', '--', f), `${f}: eol: lf`);
  }
});

test('the release workflow builds with read access alone, checking first, and publishes through the script', () => {
  const yml = fs.readFileSync(path.join(ROOT, '.github/workflows/ceremony-release.yml'), 'utf8');
  const build = /\n  build:\n([\s\S]*?)\n  publish:\n/.exec(yml);
  assert.ok(build, 'no build job before a publish job');
  assert.match(build[1], /permissions:\n      contents: read\n    env:/);
  assert.doesNotMatch(build[1], /: write/);
  const steps = build[1];
  const checks = steps.indexOf('scripts/ceremony_release.sh check');
  const builds = steps.indexOf('make ceremony-dist');
  assert.ok(checks >= 0 && builds >= 0, 'the build job neither checks nor builds');
  assert.ok(checks < builds, 'built before checking');
  assert.match(yml.slice(build.index + build[0].length), /needs: build[\s\S]*run: scripts\/ceremony_release\.sh publish dist/);
});
