/** @type import('hardhat/config').HardhatUserConfig */
module.exports = {
  // Fix H-13: adding the burn circuit's on-chain proof verification pushed
  // Enygma.sol past the EIP-170 24576-byte deployment size limit with the
  // optimizer off (the prior default — this project compiled unoptimized
  // until now). Enabled specifically to bring Enygma.sol back under that
  // limit; runs lowered from an initial 200 to 1 once the USDr fee-proof
  // feature pushed the unoptimized size further, to 26515 bytes — a low
  // "runs" value prioritizes bytecode size over runtime gas efficiency, per
  // the compiler's own suggestion in the oversized-contract warning.
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
