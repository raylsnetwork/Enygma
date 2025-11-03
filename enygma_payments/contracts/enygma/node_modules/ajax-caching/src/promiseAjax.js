const cacheAjax = require('./ajaxCache');

module.exports = promiseAjax;

function promiseAjax(opts) {
  return new Promise(function (resolve, reject) {
    try {
      cacheAjax(opts, (error, data) => {
        if (error) return reject(error);
        resolve(data);
      });
    } catch (error) {
      reject(error);
    }
  });
}