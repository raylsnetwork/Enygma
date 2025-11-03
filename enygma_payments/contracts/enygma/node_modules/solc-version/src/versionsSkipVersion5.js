const processList = require('./processList');
const getlist = require('./getlist');
const groupByVersion = require('./groupByVersion');

module.exports = versionsSkipVersion5;

function versionsSkipVersion5() {
  return new Promise(async (resolve, reject) => {
    try {
      let data = await getlist();
      let select = groupByVersion(processList(data), true);
      resolve(select);
    } catch (error) {
      reject(error);
    }
  });
}