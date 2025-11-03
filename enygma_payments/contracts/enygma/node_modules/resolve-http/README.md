# resolve http

![Travis](https://img.shields.io/travis/alincode/resolve-http.svg)
[![codecov](https://codecov.io/gh/alincode/resolve-http/branch/master/graph/badge.svg)](https://codecov.io/gh/alincode/resolve-http)![npm downloads](https://img.shields.io/npm/dt/resolve-http.svg)
[![Dependency Status](https://img.shields.io/david/alincode/resolve-http.svg?style=flat)](https://david-dm.org/alincode/resolve-http)

### Install

```sh
npm install resolve-http
```

### Usage

* parser

```js
const resolveHttp = require('resolve-http');
const path = 'http://raw.githubusercontent.com/OpenZeppelin/openzeppelin-solidity/master/contracts/math/SafeMath.sol';
let content = await resolveHttp.parser(path);
```

## License
MIT © [alincode](https://github.com/alincode/resolve-http)