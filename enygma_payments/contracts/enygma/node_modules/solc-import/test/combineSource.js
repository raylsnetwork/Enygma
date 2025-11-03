require('solcjs-mock')();

const solcImport = require('../src/');

const sourceCode = `
import 'lib.sol';

contract Casino {
    using SafeMath for uint256;
    function example(uint256 _value) {
        uint number = msg.value.add(_value);
    }
}`;

const sourceCodeIncludeImport = `
import 'lib.sol';

contract Casino {
    using SafeMath for uint256;
    function example(uint256 _value) {
        uint number = msg.value.add(_value);
    }
}`;

const libContent = `
    import 'lib2.sol';
    library L { function f() internal returns (uint) { return 7; } }
    `;

let myDB;
const getImportContent = async function (path) {
  return myDB.get(path);
};

describe('combineSource', () => {

  it('one import', async () => {
    myDB = new Map();
    myDB.set('lib.sol', 'library L { function f() internal returns (uint) { return 7; } }');
    let sources = await solcImport.combineSource(sourceCodeIncludeImport, getImportContent);
    sources.should.be.a('array');
    sources[0].should.have.all.keys('path', 'content');
  });

  it('for loop import', async () => {
    myDB = new Map();
    myDB.set('lib.sol', libContent);
    myDB.set('lib2.sol', 'library L2 { function f() internal returns (uint) { return 7; } }');
    let sources = await solcImport.combineSource(sourceCodeIncludeImport, getImportContent);
    sources.should.be.a('array');
    sources[0].should.have.all.keys('path', 'content');
  });

  it('deep for loop import and same importPath only import once', async () => {
    myDB = new Map();
    const libContent2 = `
    import 'lib3.sol';
    import 'lib.sol';
    library L2 { function f() internal returns (uint) { return 7; } }
    `;
    myDB.set('lib.sol', libContent);
    myDB.set('lib2.sol', libContent2);
    let sources = await solcImport.combineSource(sourceCodeIncludeImport, getImportContent);
    sources.should.be.a('array');
    sources[0].should.have.all.keys('path', 'content');
  });

  it('case 1', async () => {
    let sources = await solcImport.combineSource(sourceCode, getImportContent);
    sources.should.be.a('array');
    sources[0].should.have.all.keys('path', 'content');
  });

});