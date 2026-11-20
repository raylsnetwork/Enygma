const compiler = {
  version: "0.8.27",
  settings: { optimizer: { enabled: true, runs: 1 } },
};

/** @type import('hardhat/config').HardhatUserConfig */
module.exports = {
  solidity: {
    compilers: [compiler],
    // Keep the ledger below the deployment size limit without rebuilding
    // the standalone proof verifiers with a different compiler pipeline.
    overrides: {
      "contracts/Enygma.sol": {
        ...compiler,
        settings: { ...compiler.settings, viaIR: true },
      },
    },
  },
  networks: {
    hardhat: {
      chainId: 1337,
      blockGasLimit: 300_000_000,
      accounts: [
        // owner key used by go_client/enygma_test
        {
          privateKey: "0x34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154",
          balance: "100000000000000000000000",
        },
      ],
    },
  },
};
