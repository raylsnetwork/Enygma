const getImports = require('./getImports');

module.exports = isExistImport;

function isExistImport(sourcecode) {
  const allImportPath = getImports(sourcecode);
  return allImportPath.length != 0;
}