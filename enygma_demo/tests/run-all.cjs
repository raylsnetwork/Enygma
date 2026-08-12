#!/usr/bin/env node
/* Runs every suite in sequence and prints a summary.
 * Each suite exits non-zero if any of its checks failed; so does this runner. */
const { spawnSync } = require("child_process");
const path = require("path");

const SUITES = [
  ["env-test.cjs",   "payment envelope + Retrieve (trial decryption)"],
  ["pk-test.cjs",    "settlement vs client-payment legs, padded payloads"],
  ["flow-test.cjs",  "tabs, guided walkthrough, manual payment derivation"],
  ["inv-test.cjs",   "identity balances and supply invariance"],
  ["aud-test.cjs",   "audit by decapsulation, mismatch handling"],
  ["dvp-test.cjs",   "bridge, notes, atomic swap, revert path"],
  ["nf-test.cjs",    "nullifier set — no per-leaf spend state is shown"],
  ["frz-test.cjs",   "operator freeze"],
  ["frz-test2.cjs",  "freeze during the guided walkthrough"],
  ["faq-test.cjs",   "protocol explainer + technical FAQ"],
];

let failed = [], total = 0;
for(const [file, what] of SUITES){
  process.stdout.write(`\n\x1b[1m▸ ${file}\x1b[0m — ${what}\n`);
  const r = spawnSync(process.execPath, [path.join(__dirname, file)], { encoding: "utf8" });
  const out = (r.stdout || "") + (r.stderr || "");
  const passes = (out.match(/ {2}PASS {2}/g) || []).length;
  const fails  = (out.match(/ {2}FAIL {2}/g) || []).length;
  total += passes;
  if(r.status !== 0 || fails){
    failed.push(file);
    process.stdout.write(out);
  } else {
    process.stdout.write(`  ${passes} checks passed\n`);
  }
}

console.log("\n" + "─".repeat(60));
if(failed.length){
  console.log(`\x1b[31m${failed.length} suite(s) failed:\x1b[0m ${failed.join(", ")}`);
  process.exit(1);
}
console.log(`\x1b[32mAll ${SUITES.length} suites passed — ${total} checks.\x1b[0m`);
