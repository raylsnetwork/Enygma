#!/usr/bin/env node
/**
 * Deploy Enygma + EnygmaVerifier using ethers.js (from contracts/enygma/node_modules).
 * Writes run_scripts/build/enygma/web3/deploy_receipts.json.
 *
 * Usage (from enygma_payments/ root):
 *   OWNER_KEY=34d091c661... node run_scripts/deploy_node.js
 */
const { ethers } = require("../contracts/enygma/node_modules/ethers");
const { execFileSync } = require("node:child_process");
const fs   = require("fs");
const path = require("path");

// Hardhat account[0] — well-known default dev key, public, never a security
// boundary (see .gitleaks.toml, which allowlists it by exact value).
const DEFAULT_OWNER_KEY = "34d091c661db4c814d65c8ae9277b7055c0dde5a752ce5a3fdfd4ea11a8f7154";
const LOCAL_CHAIN_ID = 1337;
const CHAIN_ID       = parseInt(process.env.CHAIN_ID || String(LOCAL_CHAIN_ID));
const RPC_URL        = process.env.RPC_URL    || "http://127.0.0.1:8545";
const EPOCH_INTERVAL = parseInt(process.env.EPOCH_INTERVAL || "1");
// Seconds a replacement of an already-set verifier waits before
// executeVerifierChange can apply it (Enygma.verifierChangeDelay, at most 30
// days). 0 applies replacements at once: local development only.
const VERIFIER_CHANGE_DELAY = parseInt(process.env.VERIFIER_CHANGE_DELAY || "0");

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

// Refuse redirects for every RPC request made with the demo key.
function localRpcRequest() {
  const request = new ethers.FetchRequest(RPC_URL);
  const getUrl = request.getUrlFunc;
  request.getUrlFunc = async (req, signal) => {
    const response = await getUrl(req, signal);
    if (response.statusCode >= 300 && response.statusCode < 400) {
      throw new Error("set OWNER_KEY when the local RPC redirects");
    }
    return response;
  };
  return request;
}

async function main() {
  const verifierArtifactPath = path.join(CONTRACTS_DIR, "EnygmaVerifier.sol", "Verifier.json");
  const localChain = CHAIN_ID === LOCAL_CHAIN_ID && ["http://127.0.0.1:8545", "http://localhost:8545"].includes(RPC_URL);
  const ownerKey = process.env.OWNER_KEY;
  if (!ownerKey && !localChain) {
    throw new Error("set OWNER_KEY for deployments outside local Hardhat");
  }
  const provider = new ethers.JsonRpcProvider(ownerKey ? RPC_URL : localRpcRequest(), CHAIN_ID);
  if (!ownerKey) {
    const rpcChainId = await provider.send("eth_chainId", []).catch(() => null);
    if (rpcChainId !== `0x${LOCAL_CHAIN_ID.toString(16)}`) {
      throw new Error(`set OWNER_KEY when the local RPC cannot confirm chain ID ${LOCAL_CHAIN_ID}`);
    }
  }
  const reviewedPath = process.env.REVIEWED_VERIFIER_ARTIFACT;
  if (!localChain && !reviewedPath) {
    throw new Error("set REVIEWED_VERIFIER_ARTIFACT to the reviewed transfer verifier build for remote deployment");
  }
  const reviewedArtifactPath = reviewedPath ? path.resolve(reviewedPath) : verifierArtifactPath;
  const reviewedArtifact = JSON.parse(fs.readFileSync(reviewedArtifactPath));
  const verifierArtifact = JSON.parse(fs.readFileSync(verifierArtifactPath));
  if (reviewedArtifact.deployedBytecode !== verifierArtifact.deployedBytecode) {
    throw new Error("local verifier build differs from the reviewed runtime");
  }
  const wallet = new ethers.NonceManager(new ethers.Wallet(ownerKey || DEFAULT_OWNER_KEY, provider));
  console.log("Deployer:", await wallet.getAddress());
  console.log("Block:", await provider.getBlockNumber());

  console.log(`\nDeploying Enygma (epochInterval=${EPOCH_INTERVAL}, verifierChangeDelay=${VERIFIER_CHANGE_DELAY})...`);
  const enygmaArtifact = JSON.parse(fs.readFileSync(path.join(CONTRACTS_DIR, "Enygma.sol", "Enygma.json")));
  const tokenAddr = await deploy(wallet, enygmaArtifact, [EPOCH_INTERVAL, VERIFIER_CHANGE_DELAY]);

  console.log("\nDeploying EnygmaVerifier...");
  const verifierAddr = await deploy(wallet, verifierArtifact, []);

  execFileSync("go", [
    "run", "./cmd/verify_deployed_verifier",
    "-vk", "keys/EnygmaVk.key",
    "-artifact", reviewedArtifactPath,
    "-rpc", RPC_URL,
    "-address", verifierAddr,
  ], { cwd: path.join(ROOT, "gnark-server"), stdio: "inherit" });

  const receipts = {
    TOKEN:    { contractAddress: tokenAddr },
    VERIFIER: { contractAddress: verifierAddr, runtimeHash: ethers.keccak256(reviewedArtifact.deployedBytecode) },
  };

  fs.mkdirSync(path.dirname(RECEIPTS_PATH), { recursive: true });
  fs.writeFileSync(RECEIPTS_PATH, JSON.stringify(receipts, null, 2));
  console.log("\nWrote", RECEIPTS_PATH);
  console.log(JSON.stringify(receipts, null, 2));
}

main().catch(e => { console.error(e); process.exit(1); });
