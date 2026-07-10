/** @type import('hardhat/config').HardhatUserConfig */
module.exports = {
  solidity: "0.8.27",
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
