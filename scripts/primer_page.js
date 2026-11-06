// primer_page.js prints the Protocol primer
// (enygma_retail_payments/shield_demo/protocol.html) for reading, its data
// block summarised: each step's value names with their lengths and its
// ledger's fields, in place of the run's full values (mostly long hex). Every
// other line prints as it is, numbered as in the file (as Read numbers it),
// so an edit can be placed from it.
//
//   node scripts/primer_page.js            # the page, data block summarised
//   node scripts/primer_page.js <step>     # one step's values and ledger, in full

const fs = require('fs');
const path = require('path');

const PRIMER = path.join(__dirname, '..', 'enygma_retail_payments', 'shield_demo', 'protocol.html');
const OPEN = '<script type="application/json" id="primer-data">';

// summarise returns page's lines, numbered, with the data block's lines
// replaced by a summary of its steps.
function summarise(page) {
  const lines = page.split('\n');
  const start = lines.indexOf(OPEN);
  const end = lines.indexOf('</script>', start);
  if (start < 0 || end < 0) throw new Error(`no data block (${OPEN})`);
  const data = JSON.parse(lines.slice(start + 1, end).join('\n'));
  const num = (n, l) => `${String(n).padStart(6)}\t${l}`;
  const summary = [`  [data block, lines ${start + 2}–${end}: run ${JSON.stringify(data.run)}]`];
  for (const [step, e] of Object.entries(data.steps)) {
    const values = Object.entries(e.values).map(([k, v]) => `${k}(${v.length})`).join(' ');
    const ledger = e.ledger.map(i => `${i.tx}: ${i.field}${i.msgSender ? ' (msg.sender)' : ''}`).join(' | ');
    summary.push(`  [${step}] values: ${values}`, `  [${step}] ledger: ${ledger || '(none)'}`);
  }
  return [
    ...lines.slice(0, start + 1).map((l, i) => num(i + 1, l)),
    ...summary,
    ...lines.slice(end).map((l, i) => num(end + 1 + i, l)),
  ].join('\n');
}

// stepData returns one step's entry in page's data block, as indented JSON.
function stepData(page, step) {
  const lines = page.split('\n');
  const start = lines.indexOf(OPEN);
  const data = JSON.parse(lines.slice(start + 1, lines.indexOf('</script>', start)).join('\n'));
  if (!(step in data.steps)) throw new Error(`no step ${step}; the steps are ${Object.keys(data.steps).join(', ')}`);
  return JSON.stringify(data.steps[step], null, 1);
}

if (require.main === module) {
  const page = fs.readFileSync(PRIMER, 'utf8');
  console.log(process.argv[2] ? stepData(page, process.argv[2]) : summarise(page));
}

module.exports = { summarise, stepData, PRIMER };
