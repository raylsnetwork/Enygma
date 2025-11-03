const solcImport = require('../src/');

describe('isExistImport', () => {

  it('exist', async () => {
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
    let isExist = solcImport.isExistImport(sourceCode);
    isExist.should.be.a('boolean');
    isExist.should.be.eq(true);
  });

  it('no exist', async () => {
    const sourceCode = `
    pragma solidity >0.4.99 <0.6.0;

    library OldLibrary {
      function someFunction(uint8 a) public returns(bool);
    }

    contract NewContract {
      function f(uint8 a) public returns (bool) {
          return OldLibrary.someFunction(a);
      }
    }`;
    let isExist = solcImport.isExistImport(sourceCode);
    isExist.should.be.a('boolean');
    isExist.should.be.eq(false);
  });

});