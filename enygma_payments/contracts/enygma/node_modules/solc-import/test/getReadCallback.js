require('solcjs-mock')();
const solcImport = require('../src/');

describe('getReadCallback', () => {

  it('case 1', async () => {

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

    let readCallback = await solcImport.getReadCallback(sourceCode, getImportContent);
    readCallback.should.be.a('function');

    let contentObject = readCallback('lib.sol');
    contentObject.should.be.a('object');
    contentObject.should.have.all.keys('contents');
    contentObject.contents.should.be.eq(libContent);
  });

});