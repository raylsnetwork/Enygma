# resolve ipfs

![Travis](https://img.shields.io/travis/alincode/resolve-ipfs.svg)
[![codecov](https://codecov.io/gh/alincode/resolve-ipfs/branch/master/graph/badge.svg)](https://codecov.io/gh/alincode/resolve-ipfs)![npm downloads](https://img.shields.io/npm/dt/resolve-ipfs.svg)
[![Dependency Status](https://img.shields.io/david/alincode/resolve-ipfs.svg?style=flat)](https://david-dm.org/alincode/resolve-ipfs)

### Install

```sh
npm install resolve-ipfs
```

### Usage

* parser

```js
const path = 'ipfs://QmTeW79w7QQ6Npa3b1d5tANreCDxF2iDaAPsDvW6KtLmfB/styles/jquery/jquery.min.js';
    let content = await resolve.parser(path);
let content = await resolve.parser(path);
```

## License
MIT © [alincode](https://github.com/alincode/resolve-ipfs)