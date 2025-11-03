const processList = require('./processList');
const getlist = require('./getlist');
const groupByVersion = require('./groupByVersion');

module.exports = versions;

function versions(list) {
  return new Promise(async (resolve, reject) => {
    try {
      let data = list ? list : await getlist();
      let select = groupByVersion(processList(data), false);
      resolve(select);
    } catch (error) {
      reject(error);
    }
  });
}