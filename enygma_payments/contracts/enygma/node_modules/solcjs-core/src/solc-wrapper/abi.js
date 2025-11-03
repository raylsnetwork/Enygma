// from: sindresorhus/semver-regex
var semverRegex = /\bv?(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-[\da-z-]+(?:\.[\da-z-]+)*)?(?:\+[\da-z-]+(?:\.[\da-z-]+)*)?\b/ig;

// from: substack/semver-compare
function cmp (a, b) {
  var pa = a.split('.'), pb = b.split('.');
  for (var i = 0; i < 3; i++) {
    var na = Number(pa[i]), nb = Number(pb[i]);
    if (na > nb) return 1;
    if (nb > na) return -1;
    if (!isNaN(na) && isNaN(nb)) return 1;
    if (isNaN(na) && !isNaN(nb)) return -1;
  }
  return 0;
}

// semver.lt('1.2.3', '9.8.7') // true
var semver = {
  lt(a, b) { 
    var A = a.match(semverRegex), B = b.match(semverRegex);
    if (A && A.length === 1 && B && B.length === 1) {
      return cmp(A[0], B[0]) === -1;
    }
  }
};

function update (compilerVersion, abi) {
  var hasConstructor = false;
  var hasFallback = false;

  for (var i = 0; i < abi.length; i++) {
    var item = abi[i];

    if (item.type === 'constructor') {
      hasConstructor = true;

      // <0.4.5 assumed every constructor to be payable
      if (semver.lt(compilerVersion, '0.4.5')) {
        item.payable = true;
      }
    } else if (item.type === 'fallback') {
      hasFallback = true;
    }

    if (item.type !== 'event') {
      // add 'payable' to everything
      if (semver.lt(compilerVersion, '0.4.0')) {
        item.payable = true;
      }

      // add stateMutability field
      if (semver.lt(compilerVersion, '0.4.16')) {
        if (item.payable) {
          item.stateMutability = 'payable';
        } else if (item.constant) {
          item.stateMutability = 'view';
        } else {
          item.stateMutability = 'nonpayable';
        }
      }
    }
  }

  // 0.1.2 from Aug 2015 had it. The code has it since May 2015 (e7931ade)
  if (!hasConstructor && semver.lt(compilerVersion, '0.1.2')) {
    abi.push({
      type: 'constructor',
      payable: true,
      stateMutability: 'payable',
      inputs: []
    });
  }

  if (!hasFallback && semver.lt(compilerVersion, '0.4.0')) {
    abi.push({
      type: 'fallback',
      payable: true,
      stateMutability: 'payable'
    });
  }

  return abi;
}

module.exports = {
  update: update
};
