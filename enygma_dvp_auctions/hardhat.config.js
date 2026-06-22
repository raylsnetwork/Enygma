require("@nomicfoundation/hardhat-toolbox");

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
