# resolve swarm

![Travis](https://img.shields.io/travis/alincode/resolve-swarm.svg)
[![codecov](https://codecov.io/gh/alincode/resolve-swarm/branch/master/graph/badge.svg)](https://codecov.io/gh/alincode/resolve-swarm)![npm downloads](https://img.shields.io/npm/dt/resolve-swarm.svg)
[![Dependency Status](https://img.shields.io/david/alincode/resolve-swarm.svg?style=flat)](https://david-dm.org/alincode/resolve-swarm)

### Install

```sh
npm install resolve-swarm
```

### Usage

* parser

```js
const resolve = require('resolve-swarm');
const path = 'http://raw.githubusercontent.com/OpenZeppelin/openzeppelin-solidity/master/contracts/math/SafeMath.sol';
let content = await resolve.parser(path);
```

## License
MIT © [alincode](https://github.com/alincode/resolve-swarm)