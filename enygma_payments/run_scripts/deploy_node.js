#!/usr/bin/env node
/**
 * Deploy Enygma + EnygmaVerifier using ethers.js (from contracts/enygma/node_modules).
 * Writes run_scripts/build/enygma/web3/deploy_receipts.json.
 *
 * Usage (from enygma_payments/ root):
 *   OWNER_KEY=34d091c661... node run_scripts/deploy_node.js
 */
const { ethers } = require("../contracts/enygma/node_modules/ethers");
const fs   = require("fs");
const path = require("path");

const OWNER_KEY      = process.env.OWNER_KEY || "34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154";
const CHAIN_ID       = parseInt(process.env.CHAIN_ID || "1337");
const RPC_URL        = process.env.RPC_URL    || "http://127.0.0.1:8545";
const EPOCH_INTERVAL = parseInt(process.env.EPOCH_INTERVAL || "1");

const ROOT          = path.join(__dirname, "..");
const CONTRACTS_DIR = path.join(ROOT, "contracts", "enygma", "artifacts", "contracts");
const RECEIPTS_PATH = path.join(ROOT, "run_scripts", "build", "enygma", "web3", "deploy_receipts.json");

async function deploy(wallet, artifact, constructorArgs = []) {
  const factory = new ethers.ContractFactory(artifact.abi, artifact.bytecode, wallet);
  const contract = await factory.deploy(...constructorArgs);
  await contract.deploymentTransaction().wait();
  const addr = await contract.getAddress();
  console.log("  deployed at:", addr);
  return addr;
}

async function main() {
  const provider = new ethers.JsonRpcProvider(RPC_URL, CHAIN_ID);
  const wallet   = new ethers.Wallet(OWNER_KEY, provider);
  console.log("Deployer:", wallet.address);
  console.log("Block:", await provider.getBlockNumber());

  console.log(`\nDeploying Enygma (epochInterval=${EPOCH_INTERVAL})...`);
  const enygmaArtifact = JSON.parse(fs.readFileSync(path.join(CONTRACTS_DIR, "Enygma.sol", "Enygma.json")));
  const tokenAddr = await deploy(wallet, enygmaArtifact, [EPOCH_INTERVAL]);

  console.log("\nDeploying EnygmaVerifier...");
  const verifierArtifact = JSON.parse(fs.readFileSync(path.join(CONTRACTS_DIR, "EnygmaVerifier.sol", "Verifier.json")));
  const verifierAddr = await deploy(wallet, verifierArtifact, []);

  const receipts = {
    TOKEN:    { contractAddress: tokenAddr },
    VERIFIER: { contractAddress: verifierAddr },
  };

  fs.mkdirSync(path.dirname(RECEIPTS_PATH), { recursive: true });
  fs.writeFileSync(RECEIPTS_PATH, JSON.stringify(receipts, null, 2));
  console.log("\nWrote", RECEIPTS_PATH);
  console.log(JSON.stringify(receipts, null, 2));
}

main().catch(e => { console.error(e); process.exit(1); });
