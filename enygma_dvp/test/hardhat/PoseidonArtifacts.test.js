// Poseidon.sol is a placeholder: the real PoseidonT3/T5 are circomlibjs's.
// hardhat.config.js writes them over the compiler's after every compile, and
// npx hardhat test compiles before this spec runs, so the artifacts on disk
// here are what a compile leaves behind.
const { expect } = require("chai");
const { artifacts } = require("hardhat");
const { poseidonArtifacts, POSEIDON_SOURCE } = require("../../scripts/regen_poseidon.js");

describe("Poseidon.sol's artifacts", function () {
  for (const [contract, real] of Object.entries(poseidonArtifacts())) {
    it(`${contract} is circomlibjs's after a compile, not the placeholder's`, async function () {
      const onDisk = await artifacts.readArtifact(`${POSEIDON_SOURCE}:${contract}`);
      // === rather than equal: a failure would print both bytecodes, tens of KB.
      expect(
        onDisk.deployedBytecode === real.deployedBytecode,
        `artifacts/${POSEIDON_SOURCE}/${contract}.json holds the placeholder's bytecode: the compile hook in hardhat.config.js did not run`
      ).to.equal(true);
      expect(onDisk.bytecode === real.bytecode, `${contract}'s creation bytecode is not circomlibjs's`).to.equal(true);
      expect(onDisk.abi).to.deep.equal(real.abi);
    });
  }
});
