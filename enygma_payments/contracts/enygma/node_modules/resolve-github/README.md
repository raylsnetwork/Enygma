# resolve github

![Travis](https://img.shields.io/travis/alincode/resolve-github.svg)
[![codecov](https://codecov.io/gh/alincode/resolve-github/branch/master/graph/badge.svg)](https://codecov.io/gh/alincode/resolve-github)![npm downloads](https://img.shields.io/npm/dt/resolve-github.svg)
[![Dependency Status](https://img.shields.io/david/alincode/resolve-github.svg?style=flat)](https://david-dm.org/alincode/resolve-github)

### Install

```sh
npm install resolve-github
```

### Usage

* parser

```js
const path = 'https://github.com/OpenZeppelin/openzeppelin-solidity/contracts/math/SafeMath.sol';
let content = await github.parser(path);
```

## release change log

* 0.2.0
  * support symbolic link

## License
MIT © [alincode](https://github.com/alincode/resolve-github)