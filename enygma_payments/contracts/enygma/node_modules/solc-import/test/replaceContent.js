const solcImport = require('../src');

describe('replaceContent', () => {

  it('case 1', async () => {
    const content = `
    pragma solidity ^0.5.2;

    import "../token/ERC20/ERC20.sol";
    import "../token/ERC20/ERC20Detailed.sol";

    contract SimpleToken is ERC20, ERC20Detailed {
        uint8 public constant DECIMALS = 18;
        uint256 public constant INITIAL_SUPPLY = 10000 * (10 ** uint256(DECIMALS));

        constructor () public ERC20Detailed("SimpleToken", "SIM", DECIMALS) {
            _mint(msg.sender, INITIAL_SUPPLY);
        }
    }`;
    const from = 'https://github.com/OpenZeppelin/openzeppelin-solidity/contracts/examples/SimpleToken.sol';

    let resolver = function (content, from, subImportPath) {
      let newContent = content;
      let url = new window.URL(subImportPath, from);
      let fixedPath = url.href;
      newContent = newContent.replace(`import '${subImportPath}'`, `import '${fixedPath}'`);
      newContent = newContent.replace(`import "${subImportPath}"`, `import "${fixedPath}"`);
      return newContent;
    };

    let newContent = solcImport.replaceContent(content, from, resolver);
    newContent.should.be.a('string');
    newContent.length.should.be.above(50);
  });

});