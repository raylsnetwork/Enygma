/* eslint-disable import/no-extraneous-dependencies */
/* global task */
const ethers = require("@nomiclabs/hardhat-ethers");
require("@nomiclabs/hardhat-etherscan");
require("hardhat-gas-reporter");
require("hardhat-artifactor");
require("hardhat-tracer");

let networks;

try {
  // eslint-disable-next-line
  networks = require("./networks.config");
} catch (e) {
  if (e.code !== "MODULE_NOT_FOUND") {
    // Re-throw not "Module not found" errors
    throw e;
  }
  networks = {
    hardhat: {
      timeout: 10000000,
      chainId: 1337,
      accounts: {
        // WARNING: This mnemonic is the well-known Hardhat default and is for demo/local development only.
        // Never use it on any public or production network. Do not send real funds to these addresses.
        mnemonic:
          "federal unhappy avoid mistake life barrel beauty raccoon recycle unknown review link",
        path: "m/44'/60'/0'/0",
        initialIndex: 0,
        count: 10,
        passphrase: "",
      },
      blockGasLimit: 400000000,
    },
    localhost: {
      timeout: 10000000,
      chainId: 1337,
      accounts: {
        mnemonic:
          "federal unhappy avoid mistake life barrel beauty raccoon recycle unknown review link",
        path: "m/44'/60'/0'/0",
        initialIndex: 0,
        count: 10,
        passphrase: "",
      },
      url: "http://localhost:8545",
    },
  };
  // console.log(networks);
}

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

task("accounts", "Prints the list of accounts", async () => {
  const accounts = await ethers.getSigners();

  accounts.forEach((account) => {
    // eslint-disable-next-line no-console
    console.log(account.address);
  });
});

module.exports = {
  defaultNetwork: "hardhat",
  networks,
  solidity: {
    version: "0.8.24",
    settings: {
      viaIR: true,
      // The EVM target, pinned so that a Hardhat upgrade cannot change it,
      // and with it every contract's bytecode and size. paris is the last
      // target without PUSH0.
      evmVersion: "paris",
      optimizer: {
        enabled: true,
        // Set for size: EnygmaDvp's runtime code has to fit EIP-170's 24,576
        // bytes (test/hardhat/ContractSizes.test.js checks it). Higher runs
        // make calls a little cheaper and the code larger.
        runs: 800,
      },
      outputSelection: {
        "*": {
          "*": ["storageLayout"],
        },
      },
    },
  },
  mocha: {
    timeout: 10 * 60 * 1000, // 10 minutes
  },
  etherscan: {
    apiKey: process.env.ETHERSCAN_API_KEY,
  },
  gasReporter: {
    currency: "USD",
    gasPrice: 47,
    gasPriceApi:
      "https://api.etherscan.io/api?module=proxy&action=eth_gasPrice",
  },
};
