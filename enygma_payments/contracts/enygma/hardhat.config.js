/** @type import('hardhat/config').HardhatUserConfig */
module.exports = {
  // Both the burn circuit's on-chain proof verification (Fix H-13) and the
  // USDr fee-proof feature independently pushed Enygma.sol past EIP-170's
  // 24576-byte deployment limit with the optimizer off (the prior default).
  // With both features now combined, bytecode is larger than either side
  // saw alone, so use runs=1 (max bytecode-size priority) for safety
  // margin, per solc's own suggestion in the oversized-contract warning.
  solidity: {
    version: "0.8.27",
    settings: {
      optimizer: {
        enabled: true,
        runs: 1,
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
