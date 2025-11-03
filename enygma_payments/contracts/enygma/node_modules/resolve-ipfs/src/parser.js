module.exports = async function (importPath) {
  let [, url] = require('./index').match.exec(importPath);
  // replace ipfs:// with /ipfs/
  url = url.replace(/^ipfs:\/\/?/, 'ipfs/');
  url = 'https://gateway.ipfs.io/' + url;

  try {
    const response = await fetch(url, { method: 'GET' });
    const data = await response.text();
    if (!response.ok || response.status !== 200) throw Error(data);
    return data;
  } catch (error) {
    throw error;
  }
};