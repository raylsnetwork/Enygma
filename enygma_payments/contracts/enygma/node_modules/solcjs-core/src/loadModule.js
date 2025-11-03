module.exports = loadModule;

// HELPER
function loadModule(sourcecode) {
  let script = window.document.createElement('script');
  let exists = true;
  if (!('Module' in window)) {
    exists = false;
    window.Module = {};
  }
  script.text = `window.Module=((Module)=>{${sourcecode};return Module})()`;
  window.document.head.appendChild(script);
  window.document.head.removeChild(script);
  const compiler = window.Module;
  if (!exists) delete window.Module;
  return compiler;
}

// function loadModule(sourcecode) {
//   let script = window.document.createElement('script');
//   let oldModule, exists;
//   if ('Module' in window) {
//     oldModule = window.Module;
//     exists = true;
//   } else {
//     window.Module = {};
//   }
//   script.text = `window.Module=((Module)=>{${sourcecode};return Module})()`;
//   window.document.head.appendChild(script);
//   window.document.head.removeChild(script);
//   const compiler = window.Module;
//   if (exists) {
//     window.Module = oldModule;
//   } else {
//     delete window.Module;
//   }
//   return compiler;
// }