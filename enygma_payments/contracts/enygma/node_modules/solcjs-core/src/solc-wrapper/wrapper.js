const linker = require('./linker.js');
const translate = require('./translate');
let soljson;
const assert = (bool, msg) => { if (!bool) throw new Error(msg); };

module.exports = wrapper;

function wrapCallback(callback) {
  assert(typeof callback === 'function', 'Invalid callback specified.');
  return function (path, contents, error) {
    var result = callback(soljson.Pointer_stringify(path));
    if (typeof result.contents === 'string') copyString(result.contents, contents);
    if (typeof result.error === 'string') copyString(result.error, error);
  };
}

function copyString(str, ptr) {
  var length = soljson.lengthBytesUTF8(str);
  var buffer = soljson._malloc(length + 1);
  soljson.stringToUTF8(str, buffer, length + 1);
  soljson.setValue(ptr, buffer, '*');
}

function runWithReadCallback(readCallback, compile, args) {
  if (readCallback === undefined) {
    readCallback = function (path) {
      return {
        error: 'File import callback not supported'
      };
    };
  }

  // This is to support multiple versions of Emscripten.
  var addFunction = soljson.addFunction || soljson.Runtime.addFunction;
  var removeFunction = soljson.removeFunction || soljson.Runtime.removeFunction;

  var cb = addFunction(wrapCallback(readCallback));
  var output;
  try {
    args.push(cb);
    // console.log('=== cb ====');
    // console.log(cb);
    output = compile.apply(undefined, args);
  } catch (e) {
    removeFunction(cb);
    throw e;
  }
  removeFunction(cb);
  return output;
}

function getCompileJSON() {
  if ('_compileJSON' in soljson) {
    return soljson.cwrap('compileJSON', 'string', ['string', 'number']);
  }
}

// function getCompileJSONMulti() {
//   if ('_compileJSONMulti' in soljson) {
//     return soljson.cwrap('compileJSONMulti', 'string', ['string', 'number']);
//   }
// }

// function getCompileJSONCallback() {
//   if ('_compileJSONCallback' in soljson) {
//     var compileInternal = soljson.cwrap('compileJSONCallback', 'string', ['string', 'number', 'number']);
//     var compileJSONCallback = function (input, optimize, readCallback) {
//       return runWithReadCallback(readCallback, compileInternal, [input, optimize]);
//     };
//     return compileJSONCallback;
//   }
// }

function getCompileStandard() {
  var compileStandard;
  if ('_compileStandard' in soljson) {
    var compileStandardInternal = soljson.cwrap('compileStandard', 'string', ['string', 'number']);
    compileStandard = function (input, readCallback) {
      return runWithReadCallback(readCallback, compileStandardInternal, [input]);
    };
  }
  if ('_solidity_compile' in soljson) {
    var solidityCompile = soljson.cwrap('solidity_compile', 'string', ['string', 'number']);
    compileStandard = function (input, readCallback) {
      return runWithReadCallback(readCallback, solidityCompile, [input]);
    };
  }
  return compileStandard;
}

function getVersion() {
  let version;
  if ('_solidity_version' in soljson) {
    version = soljson.cwrap('solidity_version', 'string', []);
  } else {
    version = soljson.cwrap('version', 'string', []);
  }
  return version;
}

function getLicense() {
  let license;
  if ('_solidity_license' in soljson) {
    license = soljson.cwrap('solidity_license', 'string', []);
  } else if ('_license' in soljson) {
    license = soljson.cwrap('license', 'string', []);
  } else {
    // pre 0.4.14
    license = function () {};
  }
  return license;
}

function getWrapperFormat(sourcecode) {
  let input = {
    language: 'Solidity',
    settings: {
      optimizer: {
        enabled: true
      },
      metadata: {
        useLiteralContent: true
      },
      outputSelection: { '*': { '*': ['*'], '': ['*'] } }
    },
    sources: {
      'MyContract': {
        content: sourcecode
      }
    }
  };
  return input;
}

function wrapper(_soljson) {
  soljson = _soljson;
  var compileJSON = getCompileJSON();
  // var compileJSONMulti = getCompileJSONMulti();
  // var compileJSONCallback = getCompileJSONCallback();
  var compileStandard = getCompileStandard();
  let version = getVersion();

  function compile(input, optimise, readCallback) {
    let result;
    if (compileStandard) {
      result = compileStandardWrapper(input, readCallback);
    } else {
      result = compileJSON(input, optimise);
    }
    return JSON.parse(result);
  }

  function compileStandardWrapper (input, readCallback) {
    let newInput = JSON.stringify(getWrapperFormat(input));
    return compileStandard(newInput, readCallback);
  }

  // function versionToSemver() { return translate.versionToSemver(version()); }
  let license = getLicense();

  return {
    version: version,
    // semver: versionToSemver,
    license: license,
    compile: compile,
    linkBytecode: linker.linkBytecode
  };
}