// primer_page.js prints the Protocol primer
// (enygma_retail_payments/shield_demo/protocol.html) for reading, its data
// block summarised: each step's value names with their lengths and its
// ledger's fields, in place of the run's full values (mostly long hex). Every
// other line prints as it is, numbered as in the file (as Read numbers it),
// so an edit can be placed from it.
//
//   node scripts/primer_page.js            # the page, data block summarised
//   node scripts/primer_page.js <step>     # one step's values and ledger, in full
//   node scripts/primer_page.js --section <step>   # one step's <section>, numbered
//   node scripts/primer_page.js --script   # the page's script (CHIPS, LIT, CIRCUIT), numbered
//   node scripts/primer_page.js --textconv <file>  # <file> for git diff (.gitattributes, make hooks)

const fs = require('fs');
const path = require('path');

const PRIMER = path.join(__dirname, '..', 'enygma_retail_payments', 'shield_demo', 'protocol.html');
const OPEN = '<script type="application/json" id="primer-data">';

// dataBlock returns the data block's parsed data and the indexes in lines of
// its opening and closing tags.
function dataBlock(lines) {
  const start = lines.indexOf(OPEN);
  const end = lines.indexOf('</script>', start);
  if (start < 0 || end < 0) throw new Error(`no data block (${OPEN})`);
  return { start, end, data: JSON.parse(lines.slice(start + 1, end).join('\n')) };
}

// ledgerSummary returns one step's ledger on a line: each item's tx and field.
function ledgerSummary(ledger) {
  return ledger.map(i => `${i.tx}: ${i.field}${i.msgSender ? ' (msg.sender)' : ''}`).join(' | ') || '(none)';
}

// summarise returns page's lines, numbered, with the data block's lines
// replaced by a summary of its steps.
function summarise(page) {
  const lines = page.split('\n');
  const { start, end, data } = dataBlock(lines);
  const num = (n, l) => `${String(n).padStart(6)}\t${l}`;
  const summary = [`  [data block, lines ${start + 2}–${end}: run ${JSON.stringify(data.run)}]`];
  for (const [step, e] of Object.entries(data.steps)) {
    const values = Object.entries(e.values).map(([k, v]) => `${k}(${v.length})`).join(' ');
    summary.push(`  [${step}] values: ${values}`, `  [${step}] ledger: ${ledgerSummary(e.ledger)}`);
  }
  return [
    ...lines.slice(0, start + 1).map((l, i) => num(i + 1, l)),
    ...summary,
    ...lines.slice(end).map((l, i) => num(end + 1 + i, l)),
  ].join('\n');
}

// stepData returns one step's entry in page's data block, as indented JSON.
function stepData(page, step) {
  const { data } = dataBlock(page.split('\n'));
  if (!(step in data.steps)) throw new Error(`no step ${step}; the steps are ${Object.keys(data.steps).join(', ')}`);
  return JSON.stringify(data.steps[step], null, 1);
}

// numbered returns lines[from..to] of the page, each with its line number in the file.
function numbered(lines, from, to) {
  return lines.slice(from, to + 1).map((l, i) => `${String(from + 1 + i).padStart(6)}\t${l}`).join('\n');
}

// sectionLines returns step's <section> in page, numbered as in the file.
function sectionLines(page, step) {
  const lines = page.split('\n');
  const start = lines.findIndex(l => l.includes('<section') && l.includes(`data-step="${step}"`));
  if (start < 0) throw new Error(`no section ${step}`);
  return numbered(lines, start, lines.findIndex((l, i) => i > start && l.includes('</section>')));
}

// scriptLines returns the page's main <script>, the one after the data block,
// numbered as in the file.
function scriptLines(page) {
  const lines = page.split('\n');
  const block = lines.indexOf(OPEN);
  const start = lines.findIndex((l, i) => i > block && l.trim() === '<script>');
  return numbered(lines, start, lines.indexOf('</script>', start));
}

// textconv returns page as git diff should show it: its data block reduced
// to each step's value names and ledger fields, so a new run's values diff to
// nothing and a new step or value still shows. Lines carry no numbers, which
// would shift. A page without a readable block comes back as it is.
function textconv(page) {
  const lines = page.split('\n');
  try {
    const { start, end, data } = dataBlock(lines);
    const summary = Object.entries(data.steps).flatMap(([step, e]) => [
      `  [${step}] values: ${Object.keys(e.values).join(' ')}`,
      `  [${step}] ledger: ${ledgerSummary(e.ledger)}`,
    ]);
    return [...lines.slice(0, start + 1), ...summary, ...lines.slice(end)].join('\n');
  } catch (_) {
    return page;
  }
}

if (require.main === module) {
  const [arg, step] = process.argv.slice(2);
  const page = fs.readFileSync(arg === '--textconv' ? step : PRIMER, 'utf8');
  console.log(arg === '--textconv' ? textconv(page) : arg === '--section' ? sectionLines(page, step) : arg === '--script' ? scriptLines(page) : arg ? stepData(page, arg) : summarise(page));
}

module.exports = { summarise, stepData, sectionLines, scriptLines, textconv, PRIMER, OPEN };
