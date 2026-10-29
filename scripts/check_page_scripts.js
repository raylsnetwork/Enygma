// check_page_scripts.js checks the inline <script> and <style> of the HTML
// pages it is given, and fails listing each problem:
//   - a <script> with a syntax error (compiled, without running);
//   - a selector a <style> defines twice at the same level: the same selector
//     list in the same @media / at-rule (or nesting) context, whether in one
//     block or two blocks with the same at-rule.
// The demo pages carry their whole UI inline, which no Go check sees.
//
//   node scripts/check_page_scripts.js <page.html>...   (make page-check)

const fs = require('fs');
const vm = require('vm');

// lineAt is the 1-based line of offset i in text.
const lineAt = (text, i) => text.slice(0, i).split('\n').length;

// blank turns each match of re in text to spaces (newlines kept), so offsets,
// and so lines, stay put.
const blank = (text, re) => text.replace(re, m => m.replace(/[^\n]/g, ' '));

// norm spells a selector list or at-rule prelude one way: single spaces and
// ", " between selectors; in an at-rule, also "a: b" and no padding inside
// parentheses ("(max-width:600px)" is "(max-width: 600px)").
function norm(prelude) {
  const t = prelude.trim().replace(/\s+/g, ' ').replace(/\s*,\s*/g, ', ');
  return t.startsWith('@') ? t.replace(/\s*:\s*/g, ': ').replace(/\(\s*/g, '(').replace(/\s*\)/g, ')') : t;
}

// duplicateSelectors returns each selector the stylesheet css defines twice
// in the same context (its enclosing at-rules and, with nesting, selectors),
// with both lines; offset is where css starts in html. Two blocks with the
// same at-rule are one context.
function duplicateSelectors(html, css, offset) {
  const shown = blank(css, /\/\*[\s\S]*?\*\//g);
  // Strings and unquoted url()s blanked too, so their braces don't count.
  const src = blank(shown, /"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|url\(\s*[^\s'")][^)]*\)/gi);
  const stack = [], seen = new Map(), dups = [];
  let start = 0;
  for (let i = 0; i < src.length; i++) {
    const ch = src[i];
    if (ch === '{') {
      const prelude = norm(shown.slice(start, i));
      if (!prelude.startsWith('@')) {
        const raw = src.slice(start, i);
        const line = lineAt(html, offset + start + raw.length - raw.trimStart().length);
        // NUL joins the context and the selector: no selector contains it.
        const key = [...stack, prelude].join('\u0000');
        if (seen.has(key)) dups.push({ selector: prelude, context: stack.join(' '), lines: [seen.get(key), line] });
        else seen.set(key, line);
      }
      stack.push(prelude);
      start = i + 1;
    } else if (ch === '}') {
      stack.pop();
      start = i + 1;
    } else if (ch === ';') {
      start = i + 1;
    }
  }
  return dups;
}

let bad = 0;
for (const page of process.argv.slice(2)) {
  const html = fs.readFileSync(page, 'utf8');
  // Each <style> is its own stylesheet; one inside a script or an HTML
  // comment is not one.
  const markup = blank(html, /<!--[\s\S]*?-->|<script\b[\s\S]*?<\/script>/gi);
  for (let m, re = /<style(\s[^>]*)?>([\s\S]*?)<\/style>/gi; (m = re.exec(markup)); ) {
    for (const d of duplicateSelectors(html, m[2], m.index + m[0].indexOf('>') + 1)) {
      bad++;
      console.error(`${page}: selector "${d.selector}"${d.context ? ` in ${d.context}` : ''} defined at lines ${d.lines[0]} and ${d.lines[1]}`);
    }
  }
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
console.log(`page scripts and styles: ${process.argv.length - 2} pages clean`);
