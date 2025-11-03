(function(){function r(e,n,t){function o(i,f){if(!n[i]){if(!e[i]){var c="function"==typeof require&&require;if(!f&&c)return c(i,!0);if(u)return u(i,!0);var a=new Error("Cannot find module '"+i+"'");throw a.code="MODULE_NOT_FOUND",a}var p=n[i]={exports:{}};e[i][0].call(p.exports,function(r){var n=e[i][1][r];return o(n||r)},p,p.exports,r,e,n,t)}return n[i].exports}for(var u="function"==typeof require&&require,i=0;i<t.length;i++)o(t[i]);return o}return r})()({1:[function(require,module,exports){
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

},{}],2:[function(require,module,exports){
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
},{"kv-idb":1}],3:[function(require,module,exports){
module.exports = {
  promiseAjax: require('./promiseAjax')
};
},{"./promiseAjax":4}],4:[function(require,module,exports){
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
},{"./ajaxCache":2}]},{},[3]);
