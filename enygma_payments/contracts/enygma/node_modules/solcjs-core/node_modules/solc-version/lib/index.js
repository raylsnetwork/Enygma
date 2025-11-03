(function(){function r(e,n,t){function o(i,f){if(!n[i]){if(!e[i]){var c="function"==typeof require&&require;if(!f&&c)return c(i,!0);if(u)return u(i,!0);var a=new Error("Cannot find module '"+i+"'");throw a.code="MODULE_NOT_FOUND",a}var p=n[i]={exports:{}};e[i][0].call(p.exports,function(r){var n=e[i][1][r];return o(n||r)},p,p.exports,r,e,n,t)}return n[i].exports}for(var u="function"==typeof require&&require,i=0;i<t.length;i++)o(t[i]);return o}return r})()({1:[function(require,module,exports){
const kvidb = require('kv-idb');
const cache = kvidb('store-solcjs');

module.exports = ajaxcache;
ajaxcache.clear = () => cache.clear();

let waitingQueue = {};

function ajaxcache(opts, done) {
  let url;
  if (opts) url = (typeof opts === 'string') ? opts : opts.url;
  if (!url) done(new Error('`url` or `{ url }` must be a string'));
  // console.log(url);
  let { transform, caching } = opts;
  let lastModified;
  if (window.localStorage[url] && caching) {
    fetch(url, { method: 'HEAD' }).then(response => {
      if (!response.ok) done(response);
      lastModified = getLastModified(response);
      if (getCacheTime(url) > lastModified) caching = true;
      cacheFetch({ cache, url, caching, transform, lastModified }, done);
    }).catch(e => {
      console.error('[error]', e);
      cacheFetch({ cache, url, caching: true, transform, lastModified: undefined }, done);
    });
  } else if (waitingQueue[url]) {
    waitingQueue[url].push(done);
  } else {
    cacheFetch({ cache, url, caching, transform, lastModified: null }, done);
  }
}

function getLastModified(response) {
  let lastModified = response.headers.get('last-modified');
  lastModified = Date.parse(lastModified) / 1000;
  return lastModified;
}

function cacheFetch({ cache, url, caching, transform, lastModified }, done) {
  const fromCache = isLatest(caching, url, lastModified);
  // console.log(`caching:${caching}, fromCache: ${fromCache}`);
  if (fromCache) return cache.get(url, done);
  waitingQueue[url] = [done];
  fetch(url)
    .then(response => response.text())
    .then(json => {
      const data = transform ? transform(json) : json;
      setCache(url, data, caching);
    }).catch(e => {
      done(e);
    });
}

function isLatest(caching, url, lastModified) {
  const condition1 = getCacheTime(url); 
  const condition2 = lastModified;
  // console.log(`cache time: ${condition1}, lastModified: ${condition2}`);
  return caching && condition1 > condition2;
}

function setCache(url, data, caching) {
  cache.put(url, data, error => {
    const listener = waitingQueue[url];
    waitingQueue[url] = undefined;
    if (error) return listener.forEach(fn => fn(error));
    setCacheTime(caching, url);
    listener.forEach(fn => fn(null, data));
  });
}

function getCacheTime(url) {
  return window.localStorage[url];
}

function setCacheTime(caching, url) {
  if (caching) {
    const dateTime = Date.now();
    let timestamp = Math.floor(dateTime / 1000);
    window.localStorage[url] = timestamp;
  }
}
},{"kv-idb":4}],2:[function(require,module,exports){
module.exports = {
  promiseAjax: require('./promiseAjax')
};
},{"./promiseAjax":3}],3:[function(require,module,exports){
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
},{"./ajaxCache":1}],4:[function(require,module,exports){
const indexedDB = window.indexedDB
const console = window.console

module.exports = kvidb

const dbname = 'kvidb'
// const dbopts = { keyPath: 'key' }
const version = 1

function kvidb (opts) {
  const name = opts ? opts.name || ('' + opts) : 'store'
  const scope = `${dbname}-${name}`
  var IDB
  const makeDB = done => {
    var idb = indexedDB.open(dbname, version)
    idb.onerror = e => console.error(`[${dbname}]`, idb.error)
    idb.onupgradeneeded = () => idb.result.createObjectStore(scope/*, dbopts*/)
    idb.onsuccess = () => done(IDB = idb.result)
  }
  const use = (mode, done) => {
    const next = (IDB, tx) => (tx = IDB.transaction([scope], mode),
      done(tx.objectStore(scope), tx))
    IDB ? next(IDB) : makeDB(next)
  }
  const api = {
    get: (key, done) => use('readonly', (store, tx) => {
      const req = store.get('' + key)
      tx.oncomplete = e => next(req.error, req.result)
      const next = (e, x) => {
        e ? done(e) : x === undefined ? done(`key "${key}" is undefined`)
        : done(null, x)
      }
    }),
    put: (key, val, done) => val === undefined ? done('`value` is undefined')
      : use('readwrite', (store, tx) => {
        const req = store.put(val, '' + key)
        tx.oncomplete = e => done(req.error, !req.error)
    }),
    del: (key, done) => api.get('' + key, (e, x) => {
      e ? done(e) : use('readwrite', (store, tx) => {
        const req = store.delete('' + key)
        tx.oncomplete = e => done(req.error, !req.error)
      })
    }),
    clear: done => use('readwrite',  (store, tx) => {
      const req = store.clear()
      tx.oncomplete = e => done(req.error, !req.error)
    }),
    length: done => use('readwrite',  (store, tx) => {
      const req = store.count()
      tx.oncomplete = e => done(req.error, req.result)
    }),
    close: done => (IDB ? IDB.close() : makeDB(IDB => IDB.close()), done(null, true)),
    batch: (ops, done) => done('@TODO: implement `.batch(...)`'),
    keys: done => use('readonly', (store, tx, keys = []) => {
      const openCursor = (store.openKeyCursor || store.openCursor)
      const req = openCursor.call(store)
      tx.oncomplete = e => done(req.error, req.error ? undefined : keys)
      req.onsuccess = () => {
        const x = req.result
        if (x) (keys.push(x.key), x.continue())
      }
    })
    // key: (n, done) => (n < 0) ? done(null) : use('readonly', store => {
    //   var advanced = false
    //   var req = store.openCursor()
    //   req.onsuccess = () => {
    //     var cursor = req.result
    //     if (!cursor) return
    //     if (n === 0 || advanced) return // Either 1) maybe return first key, or 2) we've got the nth key
    //     advanced = true // Otherwise, ask the cursor to skip ahead n records
    //     cursor.advance(n)
    //   }
    //   req.onerror = () => (console.error('Error in asyncStorage.key(): '), req.error.name)
    //   req.onsuccess = () => done((req.result || {}).key || null)
    // }),
    // This would be store.getAllKeys(), but it isn't supported by Edge or Safari.
    // And openKeyCursor isn't supported by Safari.
    // tx.oncomplete = () => done(null, keys)
  }
  return api
}

},{}],5:[function(require,module,exports){
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
},{"ajax-caching":2}],6:[function(require,module,exports){
module.exports = groupByVersion;

function removeAllZeroPointFiveVersion(select) {
  select.nightly = select.nightly.filter(x => !~x.indexOf('v0.5.'));
  select.all = select.all.filter(x => !~x.indexOf('v0.5.'));
  select.releases = select.releases.filter(x => !~x.indexOf('v0.5.'));
}

function groupByVersion(data, skip5 = true) {
  const { releases, nightly, all } = data;
  let select = {};
  select.nightly = Object.keys(nightly).reverse();
  select.all = Object.keys(all).reverse();
  select.releases = Object.keys(releases).reverse();
  if (skip5) removeAllZeroPointFiveVersion(select);
  return select;
}
},{}],7:[function(require,module,exports){
module.exports = {
  version2url: require('./version2url'),
  versions: require('./versions'),
  versionsSkipVersion5: require('./versionsSkipVersion5')
};
},{"./version2url":9,"./versions":10,"./versionsSkipVersion5":11}],8:[function(require,module,exports){
module.exports = processList;

function processList(json) {
  const data = JSON.parse(json);
  const lists = Object.values(data.builds).reduce(({ agg, d }, x, i, arr) => {
    const { path, prerelease, version } = x;
    if (prerelease) {
      d = prerelease.split('nightly.')[1];
      var [year0, month0, day0] = d.split('.').map(Number);
      if ((month0 + '').length < 2) month0 = '0' + month0;
      if ((day0 + '').length < 2) day0 = '0' + day0;
      d = [year0, month0, day0].join('.');
      const entry = [`v${version}-nightly-${d}`, path];
      agg.nightly.push(entry);
      agg.all.push(entry);
    } else {
      for (var j = i + 1, ahead; j < arr.length && !(ahead = arr[j].prerelease); j++) { }
      if (ahead) ahead = ahead.split('nightly.')[1];
      else ahead = d;
      if (!d) d = ahead;
      if (ahead !== d) {
        var [year1, month1, day1] = d.split('.').map(Number);
        var [year2, month2, day2] = ahead.split('.').map(Number);
        var d1 = new Date(year1, month1 - 1, day1);
        var d2 = new Date(year2, month2 - 1, day2);
        var diffDays = parseInt((d2 - d1) / (1000 * 60 * 60 * 24));
        var d3 = new Date(d1);
        d3.setDate(d3.getDate() + diffDays / 2);
        var month = d3.getUTCMonth() + 1;
        var day = d3.getDate();
        var year = d3.getUTCFullYear();
        var current = [year, month, day].join('.');
      } else {
        var current = ahead;
      }
      var [year0, month0, day0] = current.split('.').map(Number);
      if ((month0 + '').length < 2) month0 = '0' + month0;
      if ((day0 + '').length < 2) day0 = '0' + day0;
      current = [year0, month0, day0].join('.');
      const entry = [`v${version}-stable-${current}`, path];
      agg.releases.push(entry);
      agg.all.push(entry);
    }
    return { agg, d };
  }, { agg: { releases: [], nightly: [], all: [] }, d: null }).agg;
  const { releases, nightly, all } = lists;
  lists.releases = releases.reduce((o, x) => ((o[x[0]] = x[1]), o), {});
  lists.nightly = nightly.reduce((o, x) => ((o[x[0]] = x[1]), o), {});
  lists.all = all.reduce((o, x) => ((o[x[0]] = x[1]), o), {});
  return lists;
}
},{}],9:[function(require,module,exports){
const baseURL = 'https://solc-bin.ethereum.org/bin';

const processList = require('./processList');
const getlist = require('./getlist');

module.exports = version2url;

function version2url(version) {
  return new Promise(async (resolve, reject) => {
    try {
      let data = await getlist();
      let select = processList(data);
      const { all, releases } = select;
      if (version === 'latest') version = Object.keys(releases)[0];
      if (version === 'nightly') version = Object.keys(all)[0];
      var path = all[version];
      if (!path) return reject(new Error(`unknown version: ${version}`));
      resolve(`${baseURL}/${path}`);
    } catch (error) {
      reject(error);
    }
  });
}
},{"./getlist":5,"./processList":8}],10:[function(require,module,exports){
const processList = require('./processList');
const getlist = require('./getlist');
const groupByVersion = require('./groupByVersion');

module.exports = versions;

function versions() {
  return new Promise(async (resolve, reject) => {
    try {
      let data = await getlist();
      let select = groupByVersion(processList(data), false);
      resolve(select);
    } catch (error) {
      reject(error);
    }
  });
}
},{"./getlist":5,"./groupByVersion":6,"./processList":8}],11:[function(require,module,exports){
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
},{"./getlist":5,"./groupByVersion":6,"./processList":8}]},{},[7]);
