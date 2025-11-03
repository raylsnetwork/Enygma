const getImports = require('./getImports');
const isExistImport = require('./isExistImport');

module.exports = combineSource;

async function combineSource(source, getImportContent) {
  try {
    const allImportPath = getImports(source);
    let allSubImportPath = [];
    let sourceMap = new Map();

    for (let importPath of allImportPath) {
      let content = await getImportContent(importPath);
      allSubImportPath = allSubImportPath.concat(getImports(content));
      sourceMap.set(importPath, content);
    }

    sourceMap = await getMergeSubImportMap(allSubImportPath, sourceMap, getImportContent);

    let sources = [];
    for (let [key, value] of sourceMap) {
      sources.push({ path: key, content: value });
    }
    return sources;
  } catch (error) {
    throw(error);
  }
}

async function getMergeSubImportMap(allSubImportPath, sourceMap, getImportContent) {
  if (allSubImportPath.length != 0) {
    let search = true;
    let nextAllSubImportPath = [];
    while (search) {
      for (let subImportPath of allSubImportPath) {
        if (sourceMap.has(subImportPath)) break;
        let content = await getImportContent(subImportPath);
        sourceMap.set(subImportPath, content);
        if (isExistImport(content)) {
          let sub2ImportPath = getImports(content);
          nextAllSubImportPath = nextAllSubImportPath.concat(sub2ImportPath);
        }
      }
      search = nextAllSubImportPath.length != 0;
      allSubImportPath = nextAllSubImportPath;
      nextAllSubImportPath = [];
    }
  }
  return sourceMap;
}