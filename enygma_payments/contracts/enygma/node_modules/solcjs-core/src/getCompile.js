const solcImport = require('solc-import');
const getReadCallback = require('./getReadCallback');
const wrapperCompile = require('./wrapperCompile');

module.exports = getCompile;

function getCompile(oldSolc) {
  let compile;
  Object.keys(oldSolc).forEach(key => {
    if (key != 'compile') return;

    compile = async function (sourcecode = '', getImportContent) {
      if (solcImport.isExistImport(sourcecode)) {
        if (getImportContent == undefined) throw Error('you should pass getImportContent function in the second pararmeter.');
      }

      let readCallback = await getReadCallback(
        sourcecode,
        getImportContent
      );
      return wrapperCompile(oldSolc, sourcecode, readCallback);
    };
  });
  return compile;
}
