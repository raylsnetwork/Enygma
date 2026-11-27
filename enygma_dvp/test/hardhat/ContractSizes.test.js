// EIP-170 caps a contract's runtime code at 24,576 bytes: a chain that
// enforces it refuses to deploy a larger contract. This spec reads the
// runtime code of EnygmaDvp, Erc20CoinVault and Verifier from their artifacts
// (npx hardhat test compiles first), prints the room each has left, and fails
// on any over the cap or with no runtime code. The optimizer runs in
// hardhat.config.js trade code size for gas.
const { expect } = require("chai");
const { artifacts } = require("hardhat");

const EIP170_MAX_BYTES = 24576;

// Fully qualified names: a contract renamed or moved fails readArtifact
// rather than dropping out of the check.
const CONTRACTS = [
  "contracts/core/contracts/EnygmaDvp.sol:EnygmaDvp",
  "contracts/core/contracts/vaults/Erc20CoinVault.sol:Erc20CoinVault",
  "contracts/core/contracts/Verifier.sol:Verifier",
];

describe("EIP-170 contract size limit", function () {
  for (const name of CONTRACTS) {
    const contract = name.split(":")[1];
    it(`${contract}'s runtime code fits in ${EIP170_MAX_BYTES} bytes`, async function () {
      const { deployedBytecode } = await artifacts.readArtifact(name);
      const size = (deployedBytecode.length - 2) / 2;
      // An abstract contract or an interface has no runtime code to measure.
      expect(size, `${name} has no runtime code`).to.be.above(0);
      const room = EIP170_MAX_BYTES - size;
      console.log(`      ${contract}: ${size} bytes, ${room >= 0 ? `${room} under` : `${-room} over`} the limit`);
      expect(size, `${contract}'s runtime code is ${-room} bytes over EIP-170's limit`).to.be.at.most(EIP170_MAX_BYTES);
    });
  }
});
