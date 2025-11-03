const kvidb = require('kv-idb')
const cache = kvidb('store-solcjs')

module.exports = ajaxcache
ajaxcache.clear = () => cache.clear()

let waitingQueue = {}

function ajaxcache(opts, done) {
  let url
  if (opts) url = typeof opts === 'string' ? opts : opts.url
  if (!url) done(new Error('`url` or `{ url }` must be a string'))
  // console.log(url);
  let { transform, caching } = opts
  let lastModified
  if (window && window.localStorage[url] && caching) {
    fetch(url, { method: 'HEAD' })
      .then(response => {
        if (!response.ok) done(response)
        lastModified = getLastModified(response)
        if (getCacheTime(url) > lastModified) caching = true
        cacheFetch({ cache, url, caching, transform, lastModified }, done)
      })
      .catch(e => {
        console.error('[error]', e)
        cacheFetch(
          { cache, url, caching: true, transform, lastModified: undefined },
          done
        )
      })
  } else if (waitingQueue[url]) {
    waitingQueue[url].push(done)
  } else {
    cacheFetch({ cache, url, caching, transform, lastModified: null }, done)
  }
}

function getLastModified(response) {
  let lastModified = response.headers.get('last-modified')
  lastModified = Date.parse(lastModified) / 1000
  return lastModified
}

function cacheFetch({ cache, url, caching, transform, lastModified }, done) {
  const fromCache = isLatest(caching, url, lastModified)
  // console.log(`caching:${caching}, fromCache: ${fromCache}`);
  if (fromCache) return cache.get(url, done)
  waitingQueue[url] = [done]
  fetch(url)
    .then(response => response.text())
    .then(json => {
      const data = transform ? transform(json) : json
      setCache(url, data, caching)
    })
    .catch(e => {
      done(e)
    })
}

function isLatest(caching, url, lastModified) {
  const condition1 = getCacheTime(url)
  const condition2 = lastModified
  // console.log(`cache time: ${condition1}, lastModified: ${condition2}`);
  return caching && condition1 > condition2
}

function setCache(url, data, caching) {
  cache.put(url, data, error => {
    const listener = waitingQueue[url]
    waitingQueue[url] = undefined
    if (error) return listener.forEach(fn => fn(error))
    setCacheTime(caching, url)
    listener.forEach(fn => fn(null, data))
  })
}

function getCacheTime(url) {
  return window.localStorage[url]
}

function setCacheTime(caching, url) {
  if (caching) {
    const dateTime = Date.now()
    let timestamp = Math.floor(dateTime / 1000)
    window.localStorage[url] = timestamp
  }
}
