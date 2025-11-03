const baseURL = 'https://solc-bin.ethereum.org/bin';

const ajaxCaching = require('ajax-caching');
const promiseAjax = ajaxCaching.promiseAjax;

module.exports = getlist;

async function getlist() {
  try {
    const opts = {
      url: `${baseURL}/list.json`,
      caching: true,
      transform: function (data) {
        if (data.releases) throw Error('get list fail');
        return data;
      }
    };
    return await promiseAjax(opts);
  } catch (error) {
    throw error;
  }
}