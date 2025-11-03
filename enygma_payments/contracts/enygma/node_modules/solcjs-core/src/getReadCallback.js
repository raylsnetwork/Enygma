const solcImport = require('solc-import');

module.exports = getReadCallback;

async function getReadCallback(sourcecode, getImportContent) {
  if (!solcImport.isExistImport(sourcecode)) return;
  return await solcImport.getReadCallback(sourcecode, getImportContent);
}