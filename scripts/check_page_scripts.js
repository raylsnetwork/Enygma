// check_page_scripts.js compiles, without running, every inline <script> in
// the HTML pages it is given, and fails listing each one with a syntax error.
// The demo pages carry their whole UI as inline script, which no Go check sees.
//
//   node scripts/check_page_scripts.js <page.html>...   (make page-check)

const fs = require('fs');
const vm = require('vm');

let bad = 0;
for (const page of process.argv.slice(2)) {
  const html = fs.readFileSync(page, 'utf8');
  // Inline classic scripts only: no src, and no type other than JavaScript.
  const re = /<script(\s[^>]*)?>([\s\S]*?)<\/script>/gi;
  for (let m; (m = re.exec(html)); ) {
    const attrs = m[1] || '';
    if (/\bsrc\s*=/i.test(attrs)) continue;
    const type = (attrs.match(/\btype\s*=\s*["']?([^"'\s>]+)/i) || [])[1];
    if (type && !/^(text|application)\/javascript$/i.test(type)) continue;
    const line = html.slice(0, m.index).split('\n').length;
    try {
      new vm.Script(m[2], { filename: page, lineOffset: line - 1 });
    } catch (e) {
      bad++;
      console.error(`${page}: <script> at line ${line}: ${e.name}: ${e.message}`);
    }
  }
}
if (bad) process.exit(1);
console.log(`page scripts: ${process.argv.length - 2} pages clean`);
