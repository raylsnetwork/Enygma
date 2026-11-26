"use strict";
// The browser's client: runs the participant's program, built for the browser
// (cmd/ceremony-web), off the page's thread, so the page stays responsive
// while the contribution's arithmetic runs. The page posts {url, name}
// once; the worker posts back {type: "event", event} for each step, then
// {type: "result", result} or {type: "error", message}.
importScripts("/web/wasm_exec.js");

// enygmaPut uploads the contribution with the browser's own upload API, so
// the page can show the upload's progress; the program calls it in place of
// its own request, which sends a body whole.
self.enygmaPut = (url, token, bytes, onProgress) =>
  new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", url);
    xhr.setRequestHeader("Authorization", "Bearer " + token);
    xhr.upload.onprogress = (e) => onProgress(e.loaded);
    xhr.onload = () => resolve({ status: xhr.status, body: xhr.responseText });
    xhr.onerror = () => reject(new Error("the upload did not reach the ceremony's server"));
    xhr.onabort = () => reject(new Error("the upload was stopped"));
    xhr.send(bytes);
  });

let started = false;

onmessage = async (ev) => {
  if (started) return;
  started = true;
  try {
    const go = new Go();
    const { instance } = await WebAssembly.instantiateStreaming(fetch("/web/ceremony.wasm"), go.importObject);
    go.run(instance); // registers enygmaContribute, then waits for calls
    const result = await self.enygmaContribute(ev.data, (event) => {
      postMessage({ type: "event", event: JSON.parse(event) });
    });
    postMessage({ type: "result", result: JSON.parse(result) });
  } catch (err) {
    postMessage({ type: "error", message: String((err && err.message) || err) });
  }
};
