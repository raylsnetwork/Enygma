/* eslint-disable import/no-extraneous-dependencies */
/* global task */
const ethers = require("@nomiclabs/hardhat-ethers");
require("@nomiclabs/hardhat-etherscan");
require("@nomiclabs/hardhat-waffle");
require("hardhat-gas-reporter");
require("hardhat-artifactor");
require("hardhat-tracer");
require("hardhat-docgen");

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
    rayls: {
      timeout: 10000000,
      chainId: 149401,
      accounts: {
        mnemonic:
          "federal unhappy avoid mistake life barrel beauty raccoon recycle unknown review link",
        path: "m/44'/60'/0'/0",
        initialIndex: 0,
        count: 10,
        passphrase: "",
      },
      url: "http://commitchain-dev.parfin.corp:8545",
    },
  };
  // console.log(networks);
}

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
  allowUnlimitedContractSize: true,
  solidity: {
    version: "0.8.24",
    settings: {
      optimizer: {
        enabled: true,
        runs: 1600,
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
  docgen: {
    path: "./docs",
    clear: true,
    runOnCompile: false,
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
