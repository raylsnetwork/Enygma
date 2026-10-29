// The page check, run as make page-check runs it, on pages written to a
// temporary directory.
//
//   node --test scripts/check_page_scripts.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const { spawnSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

// check writes each page to a temporary file and runs the check on them all.
function check(...pages) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'page-check-'));
  test.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  const files = pages.map((html, i) => {
    const f = path.join(dir, `page${i}.html`);
    fs.writeFileSync(f, html);
    return f;
  });
  const r = spawnSync(process.execPath, [path.join(__dirname, 'check_page_scripts.js'), ...files], { encoding: 'utf8' });
  return { status: r.status, stderr: r.stderr, files };
}

// page is a page whose one <style> holds css, from line 4.
const page = css => `<!doctype html>\n<html><head>\n<style>\n${css}\n</style>\n</head><body></body></html>\n`;

test('a selector defined twice fails, naming the page, the selector and both lines', () => {
  const r = check(page('.legend { color: red; }\n.chip { margin: 0; }\n.legend { gap: 4px; }'));
  assert.strictEqual(r.status, 1);
  assert.match(r.stderr, new RegExp(`${r.files[0]}: selector "\\.legend" defined at lines 4 and 6`));
});

test('the same selector list, spaced differently, is a duplicate', () => {
  const r = check(page('a,b { color: red; }\n\na , b{ gap: 1px }'));
  assert.strictEqual(r.status, 1);
  assert.match(r.stderr, /selector "a, b" defined at lines 4 and 6/);
});

test('the same selector in different @media blocks or themes passes', () => {
  const r = check(page(`
:root { --bg: #fff; }
:root[data-theme="dark"] { --bg: #000; }
@media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) { --bg: #000; } .chip { gap: 0; } }
@media (max-width: 600px) { .chip { gap: 2px; } }
.chip { gap: 1px; }`));
  assert.strictEqual(r.status, 0, r.stderr);
});

test('a selector defined twice inside the same @media block fails', () => {
  const r = check(page('@media (max-width: 600px) {\n  .chip { gap: 2px; }\n  .chip { gap: 3px; }\n}'));
  assert.strictEqual(r.status, 1);
  assert.match(r.stderr, /selector "\.chip" in @media \(max-width: 600px\) defined at lines 5 and 6/);
});

test('braces in comments and strings do not count', () => {
  const r = check(page('/* .chip { } */\n.chip::before { content: "}"; }\n.chip { gap: 1px; }'));
  assert.strictEqual(r.status, 0, r.stderr);
});

test('a brace inside an unquoted url() does not count', () => {
  const r = check(page('@media (max-width: 1px) { .a { background: url(x}y.png); } .c { gap: 0; } }\n.c { gap: 1px; }'));
  assert.strictEqual(r.status, 0, r.stderr);
});

test('@media blocks that differ only in spacing are the same context', () => {
  const r = check(page('@media (max-width:600px) { .c { gap: 1px; } }\n@media (max-width: 600px) { .c { gap: 2px; } }'));
  assert.strictEqual(r.status, 1);
  assert.match(r.stderr, /selector "\.c" in @media \(max-width: 600px\) defined at lines 4 and 5/);
});

test('<style> text inside a script or an HTML comment is not a stylesheet', () => {
  const r = check('<html>\n<!-- <style>.d{} .d{}</style> -->\n<script>const s = "<style>.d{} .d{}</style>";</script>\n</html>');
  assert.strictEqual(r.status, 0, r.stderr);
});

test('a script syntax error still fails', () => {
  const r = check('<html><script>let = ;</script></html>');
  assert.strictEqual(r.status, 1);
  assert.match(r.stderr, /<script> at line 1: SyntaxError/);
});
