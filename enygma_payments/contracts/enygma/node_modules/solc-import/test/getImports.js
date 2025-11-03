const solcImport = require('../src/');

describe('getImports', () => {

  it('one github', async () => {
    const sourceCode = `
    import 'https://github.com/OpenZeppelin/openzeppelin-solidity/contracts/math/SafeMath.sol';
    `;

    let imports = solcImport.getImports(sourceCode);
    imports.should.be.a('array');
    imports.length.should.be.eq(1);
    imports[0].should.be.eq('https://github.com/OpenZeppelin/openzeppelin-solidity/contracts/math/SafeMath.sol');
  });

  it('two github', async () => {
    const sourceCode = `
    import 'openzeppelin-solidity/contracts/token/ERC721/ERC721Full.sol';
    import 'openzeppelin-solidity/contracts/token/ERC721/ERC721Mintable.sol';
    `;

    let imports = solcImport.getImports(sourceCode);
    imports.should.be.a('array');
    imports.length.should.be.eq(2);
    imports[0].should.be.eq('openzeppelin-solidity/contracts/token/ERC721/ERC721Full.sol');
  });


  it('local file', async () => {
    const sourceCode = `
    import 'SafeMath.sol';
    `;

    let result = solcImport.getImports(sourceCode);
    result.should.be.a('array');
    result[0].should.be.eq('SafeMath.sol');
  });

});