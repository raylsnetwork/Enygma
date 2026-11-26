"use strict";
// Runs the browser's client (cmd/ceremony-web, built as ceremony.wasm) as the
// coordinator's page runs it, under Node, for TestBrowserClientContributes:
//
//   node browser.js <wasm_exec.js> <ceremony.wasm> <coordinator url> <name>
//
// It prints {"result": …, "stages": [...]} or {"error": …, "stages": [...]}.
const fs = require("fs");
const [wasmExec, wasmPath, url, name] = process.argv.slice(2);

// Go's net/http talks to a fake network when it finds itself under Node
// (globalThis.process.argv0 "node"), and over fetch in a browser: be a
// browser, which has no process; wasm_exec.js then stands one in, as there.
const node = process;
globalThis.process = undefined;
require(wasmExec);

(async () => {
  const go = new Go();
  const { instance } = await WebAssembly.instantiate(fs.readFileSync(wasmPath), go.importObject);
  go.run(instance);
  const stages = [];
  try {
    const result = await globalThis.enygmaContribute({ url, name }, (e) => {
      const stage = JSON.parse(e).stage; // a stage reports its progress many times: keep each once, in order
      if (stages[stages.length - 1] !== stage) stages.push(stage);
    });
    console.log(JSON.stringify({ result: JSON.parse(result), stages }));
    node.exit(0);
  } catch (err) {
    console.log(JSON.stringify({ error: String(err), stages }));
    node.exit(1);
  }
})();
