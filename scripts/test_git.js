// Git for the scripts' tests: in scratch repos, reading none of the
// machine's git config (a global core.hooksPath, commit.gpgsign or
// diff.renames would otherwise change what a test sees, on one machine and
// not in CI), committing as one throwaway author. GIT_ENV is that
// environment, for a script under test that runs git itself.
// test_git.test.js fails on a test under scripts/ that runs git without it.

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

module.exports = { GIT_ENV, git, scratchRepo };
