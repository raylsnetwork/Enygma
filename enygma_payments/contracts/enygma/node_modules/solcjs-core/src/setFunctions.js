module.exports = setFunctions;

function setFunctions(oldSolc, newSolc) {
  Object.keys(oldSolc).forEach(key => {
    if (key !== 'compile' && typeof oldSolc[key] === 'function') {
      newSolc[key] = function (...args) {
        console.error(`compiler.${key}(...args)`, args);
        return oldSolc[key].apply(oldSolc, args);
      };
    }
  });
}

// oldSolc: 0.4.25
// key: version
// key: semver
// key: license
// key: compile
// key: compileStandard
// key: compileStandardWrapper
// key: linkBytecode
// key: supportsMulti
// key: supportsImportCallback
// key: supportsStandard

// newSolc:
//  { compile: [Function: compile],
//   version: [Function],
//   semver: [Function],
//   license: [Function],
//   linkBytecode: [Function],
//   opts: [Getter/Setter] }