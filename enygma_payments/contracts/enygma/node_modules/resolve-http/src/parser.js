const replaceContent = require('solc-import').replaceContent;
const resolver = require('./resolver');

module.exports = async function (importPath) {
  const [, url,] = require('./index').match.exec(importPath);
  try {
    const response = await fetch(url, { method: 'GET' });
    let data = await response.text();
    if (!response.ok || response.status !== 200) throw Error('Content ' + data);
    data = replaceContent(data, importPath, resolver);
    return data;
  } catch (error) {
    throw error;
  }
};