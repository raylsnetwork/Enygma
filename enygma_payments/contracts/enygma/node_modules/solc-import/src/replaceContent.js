const getImports = require('./getImports');
const isExistImport = require('./isExistImport');

module.exports = replaceContent;

function replaceContent(content, from, resolver) {
  let newContent = content;
  if (isExistImport(content)) {
    const allSubImportPath = getImports(content);
    for (let subImportPath of allSubImportPath) {
      if (isExplicitlyRelative(subImportPath)) {
        newContent = resolver(newContent, from, subImportPath);
      }
    }
  }
  return newContent;
}

function isExplicitlyRelative(importPath) {
  return importPath.indexOf('.') === 0;
}