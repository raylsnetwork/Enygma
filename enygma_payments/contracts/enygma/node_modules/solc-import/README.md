# Solc import

![Travis](https://img.shields.io/travis/alincode/solc-import.svg)
[![codecov](https://codecov.io/gh/alincode/solc-import/branch/master/graph/badge.svg)](https://codecov.io/gh/alincode/solc-import)![npm downloads](https://img.shields.io/npm/dt/solc-import.svg)
[![Dependency Status](https://img.shields.io/david/alincode/solc-import.svg?style=flat)](https://david-dm.org/alincode/solc-import)

### Install

```sh
npm install solc-import
```

### usage

* combineSource

```js
let myDB = new Map();
myDB.set('lib.sol', 'library L { function f() internal returns (uint) { return 7; } }');

const getImportContent = async function (path) {
  return myDB.get(path);
};

const sourceCodeIncludeImport = `
import 'lib.sol';

contract Casino {
    using SafeMath for uint256;
    function example(uint256 _value) {
        uint number = msg.value.add(_value);
    }
}`;

let sources = await solcImport.combineSource(sourceCodeIncludeImport, getImportContent);
// [{ path: 'lib.sol', content: '....'}]

```

* getImports

```js
const sourceCode = `
import 'https://github.com/OpenZeppelin/openzeppelin-solidity/contracts/math/SafeMath.sol';

contract Casino {
    using SafeMath for uint256;
    function example(uint256 _value) {
        uint number = msg.value.add(_value);
    }
}`;

let imports = solcImport.getImports(sourceCode);
// ['https://github.com/OpenZeppelin/openzeppelin-solidity/contracts/math/SafeMath.sol']
```

* getReadCallback

```js
const sourceCode = `
pragma solidity >0.4.99 <0.6.0;

import "lib.sol";

library OldLibrary {
  function someFunction(uint8 a) public returns(bool);
}

contract NewContract {
  function f(uint8 a) public returns (bool) {
      return OldLibrary.someFunction(a);
  }
}`;

let libContent = 'library L { function f() internal returns (uint) { return 7; } }';

let myDB = new Map();
myDB.set('lib.sol', libContent);

const getImportContent = async function (path) {
  return myDB.get(path);
};

let readCallback = await solcImport.getReadCallback(sourceCode, getImportContent);  // function
```

* isExistImport

```js
const sourceCode = `
pragma solidity >0.4.99 <0.6.0;

import "lib.sol";

library OldLibrary {
  function someFunction(uint8 a) public returns(bool);
}

contract NewContract {
  function f(uint8 a) public returns (bool) {
      return OldLibrary.someFunction(a);
  }
}`;
let isExist = solcImport.isExistImport(sourceCode); // true
```


## License
MIT © [alincode](https://github.com/alincode/solc-import)

