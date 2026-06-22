/**
 * regen_poseidon.js
 *
 * Regenerates the circomlibjs Poseidon artifacts after `npx hardhat compile`
 * overwrites them with the stub implementations in Poseidon.sol.
 *
 * Run from the project root:
 *   node scripts/regen_poseidon.js
 */
const { poseidonContract: poseidon_gencontract } = require("circomlibjs");
const fs = require("fs");
const path = require("path");

const outDir = path.join(__dirname, "..", "artifacts", "contracts", "core", "contracts", "Poseidon.sol");

const bc3 = poseidon_gencontract.createCode(2);
const bc5 = poseidon_gencontract.createCode(4);
const runtime3 = "0x" + bc3.slice(2 + 24);
const runtime5 = "0x" + bc5.slice(2 + 24);

const sourceName = "contracts/core/contracts/Poseidon.sol";

const a3 = {
  _format: "hh-sol-artifact-1",
  contractName: "PoseidonT3",
  sourceName,
  abi: poseidon_gencontract.generateABI(2),
  bytecode: bc3,
  deployedBytecode: runtime3,
  linkReferences: {},
  deployedBytecodeImmutables: {},
};
const a5 = {
  _format: "hh-sol-artifact-1",
  contractName: "PoseidonT5",
  sourceName,
  abi: poseidon_gencontract.generateABI(4),
  bytecode: bc5,
  deployedBytecode: runtime5,
  linkReferences: {},
  deployedBytecodeImmutables: {},
};

fs.mkdirSync(outDir, { recursive: true });
fs.writeFileSync(path.join(outDir, "PoseidonT3.json"), JSON.stringify(a3, null, 2));
fs.writeFileSync(path.join(outDir, "PoseidonT5.json"), JSON.stringify(a5, null, 2));

console.log(`PoseidonT3.json: ${JSON.stringify(a3).length} bytes`);
console.log(`PoseidonT5.json: ${JSON.stringify(a5).length} bytes`);
console.log("done");
