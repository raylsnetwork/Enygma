module.exports = async function (importPath) {
  const [, url,] = require('./index').match.exec(importPath);
  try {
    let content = await swarmgw.get(url);
    return content;
  } catch (error) {
    throw error;
  }
};

const swarmgw = swarmgwMaker();


async function getFile(gateway, url) {
  const httpsURL = gateway + '/' + url;
  try {
    const response = await fetch(httpsURL, { method: 'GET' });
    const data = await response.text();
    if (!response.ok || response.status !== 200) throw Error(data);
    return data;
  } catch (error) {
    throw error;
  }
}

function swarmgwMaker(opts) {
  opts = opts || {};
  var gateway;
  if (opts.gateway) {
    gateway = opts.gateway;
  } else if (opts.mode === 'http') {
    gateway = 'http://swarm-gateways.net';
  } else {
    gateway = 'https://swarm-gateways.net';
  }
  return {
    get: async function (url) {
      return await getFile(gateway, url);
    }
  };
}
