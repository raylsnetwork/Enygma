// primer_page.js on the Protocol primer as it is: the summary keeps every
// line outside the data block, numbered as in the file, and names every step,
// value and ledger field in the block.
//
//   node --test scripts/primer_page.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const { summarise, stepData, sectionLines, scriptLines, PRIMER } = require('./primer_page');

const PAGE = fs.readFileSync(PRIMER, 'utf8');
const LINES = PAGE.split('\n');
const BLOCK = /<script type="application\/json" id="primer-data">\n([\s\S]*?)\n<\/script>/;
const DATA = JSON.parse(PAGE.match(BLOCK)[1]);
const OUT = summarise(PAGE).split('\n');
// parsed splits a numbered print into [, number, line] rows, each checked against the file.
function parsed(out, what) {
  const rows = out.split('\n').map(l => l.match(/^\s*(\d+)\t(.*)$/s));
  for (const [, n, line] of rows) assert.strictEqual(line, LINES[n - 1], `${what}: line ${n}`);
  return rows;
}

test('every line outside the data block prints as it is, with its line number in the file', () => {
  const numbered = OUT.filter(l => /^\s*\d+\t/.test(l)).map(l => l.match(/^\s*(\d+)\t(.*)$/s));
  const block = PAGE.match(BLOCK)[1].split('\n').length;
  assert.strictEqual(numbered.length, LINES.length - block, 'all lines but the block\'s');
  for (const [, n, line] of numbered) assert.strictEqual(line, LINES[n - 1], `line ${n}`);
});

test('the summary names each step, each value with its length, and each ledger field', () => {
  const summary = OUT.filter(l => l.startsWith('  [')).join('\n');
  for (const [step, e] of Object.entries(DATA.steps)) {
    for (const [k, v] of Object.entries(e.values)) assert.ok(summary.includes(`${k}(${v.length})`), `${step}: ${k}`);
    for (const i of e.ledger) assert.ok(summary.includes(`${i.tx}: ${i.field}`), `${step}: ledger ${i.field}`);
  }
  assert.ok(summary.length < PAGE.match(BLOCK)[1].length / 4, `the summary (${summary.length} chars) is a fraction of the block it replaces`);
});

test('one step prints in full', () => {
  const step = Object.keys(DATA.steps)[0];
  assert.deepStrictEqual(JSON.parse(stepData(PAGE, step)), DATA.steps[step]);
  assert.throws(() => stepData(PAGE, 'nope'), /no step nope/);
});

test('one step\'s section prints alone, each line numbered as in the file', () => {
  for (const step of Object.keys(DATA.steps)) {
    const out = parsed(sectionLines(PAGE, step), step);
    assert.match(out[0][2], new RegExp(`data-step="${step}"`), `${step} starts at its <section>`);
    assert.match(out[out.length - 1][2], /<\/section>/, `${step} ends at its </section>`);
    assert.strictEqual(out.filter(([, , l]) => /<section\b/.test(l)).length, 1, `${step} is one section`);
  }
  assert.throws(() => sectionLines(PAGE, 'nope'), /no section nope/);
});

test('the page\'s script prints alone, without the data block', () => {
  const out = parsed(scriptLines(PAGE), 'script');
  const text = out.map(([, , l]) => l).join('\n');
  for (const name of ['const CHIPS', 'const LIT', 'function showStep', 'const CIRCUIT']) assert.ok(text.includes(name), name);
  assert.ok(!text.includes('"steps"'), 'not the data block');
  assert.ok(text.length < PAGE.length / 4, 'a fraction of the page');
});
