const combineSource = require('./combineSource');

module.exports = getReadCallback;

async function getReadCallback(sourceCode, getImportContent) {
  let sources = await combineSource(sourceCode, getImportContent);

  // import: it must be sync function
  function readCallback(path) {
    for (let source of sources) {
      if (source.path == path) {
        return { contents: source.content }; 
      } 
    }
  }
  return readCallback;
}