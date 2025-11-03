# solcjs core

![Travis](https://img.shields.io/travis/alincode/solcjs-core.svg)
[![codecov](https://codecov.io/gh/alincode/solcjs-core/branch/master/graph/badge.svg)](https://codecov.io/gh/alincode/solcjs-core)![npm downloads](https://img.shields.io/npm/dt/solcjs-core.svg)
[![Dependency Status](https://img.shields.io/david/alincode/solcjs-core.svg?style=flat)](https://david-dm.org/alincode/solcjs-core)

### Install

```sh
npm install solcjs-core
```

### Usage

```js
const solcjsCore = require('solcjs-core');
```

* getVersion

```js
let version = await solcjsCore.getVersion();
```

```js
let version = await solcjsCore.getVersion('v0.5.1-stable-2018.12.03');
```

* solc

```js
let compiler = await solcjsCore.solc();
const sourceCode = `
pragma solidity >0.4.99 <0.6.0;

library OldLibrary {
  function someFunction(uint8 a) public returns(bool);
}

contract NewContract {
  function f(uint8 a) public returns (bool) {
      return OldLibrary.someFunction(a);
  }
}
`;
let output = await compiler(sourceCode);
```

## License
MIT © [alincode](https://github.com/alincode/solcjs-core)