# kv-idb
minimal zero-dependencies key-value store backed by indexedDB

# usage
`npm install kv-idb`

```js
var kvidb = require('kv-idb')

var defaultName = 'store'

var db = kvidb(/* defaultName */)

var log = (e, x) => e ? console.error(e) : console.log(x)

db.clear(log),                  // true
db.put('foo', 'bar', log),      // true
db.put('null', {}, log),        // true
db.length(log),                 // 2
db.keys(log),                   // ['foo', 'null']
db.get('foo', log),             // 'bar'
db.clear(log),                  // true
db.get('foo', log),             // ERROR
db.put('foo', 'bar1', log),     // true
db.put('foo2', 'bar2', log),    // true
db.put('foo3', 'bar3', log),    // true
db.put('foo4', null, log),      // true
db.put('foo5', undefined, log), // ERROR
db.get('foo', log),             // 'bar1'
db.get('foo3', log),            // 'bar3'
db.del('foo3', log),            // true
db.get('foo3', log),            // ERROR
db.length(log),                 // 3
db.keys(log),                   // ['foo', 'foo2', 'foo4']
```
