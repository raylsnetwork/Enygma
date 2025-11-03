const solcVersion = require('solc-version');

module.exports = getVersion;

async function getVersion(_version) {
  if (typeof _version == 'string' && _version.length < 30) return _version;
  let select;
  if (_version == undefined) {
    select = await solcVersion.versions();
  } else if (typeof _version == 'string') {
    select = await solcVersion.versions(_version);
  } else {
    throw Error('unknow getVersion error');
  }
  return select.releases[0];
}