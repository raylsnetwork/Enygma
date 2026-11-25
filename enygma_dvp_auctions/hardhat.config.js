require("@nomicfoundation/hardhat-toolbox");
const { task } = require("hardhat/config");

// Poseidon.sol is a placeholder, and every compile (compile, test, node) writes
// its stub bytecode over the PoseidonT3/T5 artifacts: write circomlibjs's real
// ones back after each one.
task("compile").setAction(async (args, hre, runSuper) => {
  const result = await runSuper(args);
  // Required here, not at the top: regen_poseidon.js requires circomlibjs,
  // which only a compile needs.
  require("./scripts/regen_poseidon.js").writePoseidonArtifacts();
  return result;
});

/** @type import('hardhat/config').HardhatUserConfig */
module.exports = {
  solidity: {
    version: "0.8.24",
    settings: {
      optimizer: { enabled: true, runs: 200 },
      viaIR: true,
    },
  },
  paths: {
    sources: "./contracts",
    // Nested under the existing test/ dir (which holds the Go integration
    // tests) so Hardhat's mocha runner only picks up JS specs here.
    tests: "./test/hardhat",
    cache: "./cache",
    artifacts: "./artifacts",
  },
};
