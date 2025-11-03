const ajaxCaching = require('ajax-caching');
const promiseAjax = ajaxCaching.promiseAjax;

module.exports = getCompilersource;

async function getCompilersource(compilerURL) {
  try {
    const opts = {
      url: compilerURL,
      caching: true,
      transform: function (data) {
        if (data.substring(0, 10) != 'var Module') {
          throw Error('get compiler source fail');
        }
        return data;
      }
    };
    return await promiseAjax(opts);
  } catch (error) {
    throw error;
  }
}