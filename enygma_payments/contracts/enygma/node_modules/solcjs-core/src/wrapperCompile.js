const translateJsonCompilerOutput = require('./solc-wrapper/translate/standardTranslateJsonCompilerOutput');
const getCompileOutput = require('./getCompileOutput');
const getStandardError = require('./getStandardError');

module.exports = wrapperCompile;

function wrapperCompile(oldSolc, sourcecode, readCallback) {
  return new Promise(function (resolve, reject) {
    let output = getCompileOutput(oldSolc, sourcecode, readCallback);
    if (isCompilerFail(output)) {
      const standardError = getStandardError(output.errors);
      return reject(standardError);
    } else {
      const translateOutput = translateJsonCompilerOutput(oldSolc.opts, output);
      resolve(translateOutput);
    }
  });

  function isCompilerFail(output) {
    return !output.contracts || Object.keys(output.contracts).length == 0;
  }
}