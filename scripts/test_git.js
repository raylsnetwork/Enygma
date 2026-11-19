// Git for the scripts' tests: in scratch repos, reading none of the
// machine's git config (a global core.hooksPath, commit.gpgsign or
// diff.renames would otherwise change what a test sees, on one machine and
// not in CI), committing as one throwaway author. GIT_ENV is that
// environment, for a script under test that runs git itself; makeEnv is it
// for a make a test runs. test_git.test.js fails on a test under scripts/
// that runs git without these, or make without makeEnv.

const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const GIT_ENV = {
  ...process.env,
  GIT_CONFIG_GLOBAL: '/dev/null',
  GIT_CONFIG_NOSYSTEM: '1',
  GIT_AUTHOR_NAME: 't',
  GIT_AUTHOR_EMAIL: 't@t',
  GIT_COMMITTER_NAME: 't',
  GIT_COMMITTER_EMAIL: 't@t',
};

// makeEnv is GIT_ENV without what a parent make hands down: its flags, and
// each variable set on its command line, which it both exports and lists in
// MAKEFLAGS as a NAME=value word (a space in a value escaped as \ ; make 4
// puts the words after a --, make 3.81 with no flags gives them alone).
// Under make check MODULE=<dir>, MODULE would otherwise reach every make a
// test runs, and a scratch repo's Makefile then fails on a module it lacks.
const makeEnv = { ...GIT_ENV };
for (const w of (GIT_ENV.MAKEFLAGS || '').split(/(?<!\\)\s+/)) {
  const v = /^([A-Za-z_]\w*)[:+?!]?=/.exec(w);
  if (v) delete makeEnv[v[1]];
}
for (const v of ['MAKEFLAGS', 'MFLAGS', 'MAKELEVEL']) delete makeEnv[v];

// git runs git with args in dir and returns its output, trimmed; it throws
// on a non-zero exit.
function git(dir, ...args) {
  const r = spawnSync('git', args, { cwd: dir, env: GIT_ENV, encoding: 'utf8' });
  if (r.status !== 0) throw new Error(`git ${args.join(' ')}: ${r.stderr}`);
  return r.stdout.trim();
}

// scratchRepo makes an empty git repo in a new temp dir named for prefix and
// returns the dir; the caller removes it.
function scratchRepo(prefix) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), `${prefix}-`));
  git(dir, 'init', '-q');
  return dir;
}

module.exports = { GIT_ENV, git, makeEnv, scratchRepo };
