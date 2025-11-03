# Solc resolver

![Travis](https://img.shields.io/travis/alincode/solc-resolver.svg)
[![codecov](https://codecov.io/gh/alincode/solc-resolver/branch/master/graph/badge.svg)](https://codecov.io/gh/alincode/solc-resolver)![npm downloads](https://img.shields.io/npm/dt/solc-resolver.svg)
[![Dependency Status](https://img.shields.io/david/alincode/solc-resolver.svg?style=flat)](https://david-dm.org/alincode/solc-resolver)

### Install

```sh
npm install solc-resolver
```

### Usage

* require

```js
const ResolverEngine = require('solc-resolver').resolverEngine;
let resolverEngine = new ResolverEngine();

let resolveGithub = require('resolve-github');
resolverEngine.addResolver(resolveGithub);

let resolveHttp = require('resolve-http');
resolverEngine.addResolver(resolveHttp);

let resolveIPFS = require('resolve-ipfs');
resolverEngine.addResolver(resolveIPFS);

let resolveSwarm = require('resolve-swarm');
resolverEngine.addResolver(resolveSwarm);

const uri = 'https://github.com/OpenZeppelin/openzeppelin-solidity/contracts/math/SafeMath.sol';
const content = await resolverEngine.require(uri);

const uri2 = 'http://raw.githubusercontent.com/OpenZeppelin/openzeppelin-solidity/master/contracts/math/SafeMath.sol';
const content = await resolverEngine.require(uri2);

const uri3 = 'https://raw.githubusercontent.com/OpenZeppelin/openzeppelin-solidity/master/contracts/math/SafeMath.sol';
const content = await resolverEngine.require(uri3);

const uri4 = 'ipfs://QmTeW79w7QQ6Npa3b1d5tANreCDxF2iDaAPsDvW6KtLmfB/styles/jquery/jquery.min.js';
const content = await resolverEngine.require(uri4);

const uri5 = 'bzz:/photoalbum.eth/';
const content = await resolverEngine.require(uri5);
```

## License
MIT © [alincode](https://github.com/alincode/solc-resolver)