const solcImport = require('solc-import');

module.exports = getCompileOutput;

function getCompileOutput(oldSolc, sourcecode, readCallback) {
  let output;
  if (solcImport.isExistImport(sourcecode)) {
    // this is wrapper.compile
    output = oldSolc.compile(sourcecode, 1, readCallback);
  } else {
    output = oldSolc.compile(sourcecode, 1);
  }
  return output;
}