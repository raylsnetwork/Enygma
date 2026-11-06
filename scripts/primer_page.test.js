// primer_page.js on the Protocol primer as it is: the summary keeps every
// line outside the data block, numbered as in the file, and names every step,
// value and ledger field in the block.
//
//   node --test scripts/primer_page.test.js   (make page-check)

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const { summarise, stepData, PRIMER } = require('./primer_page');

const PAGE = fs.readFileSync(PRIMER, 'utf8');
const LINES = PAGE.split('\n');
const BLOCK = /<script type="application\/json" id="primer-data">\n([\s\S]*?)\n<\/script>/;
const DATA = JSON.parse(PAGE.match(BLOCK)[1]);
const OUT = summarise(PAGE).split('\n');

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
