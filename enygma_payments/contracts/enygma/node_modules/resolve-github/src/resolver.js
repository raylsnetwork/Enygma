module.exports = function (content, from, subImportPath) {
  let newContent = content;
  let url = new window.URL(subImportPath, from);
  let fixedPath = url.href;
  newContent = newContent.replace(`import '${subImportPath}'`, `import '${fixedPath}'`);
  newContent = newContent.replace(`import "${subImportPath}"`, `import "${fixedPath}"`);
  return newContent;
};