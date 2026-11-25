/**
 * regen_poseidon.js
 *
 * Poseidon.sol is a placeholder: every compile writes its stub bytecode over
 * the PoseidonT3/T5 artifacts, and hardhat.config.js's compile task calls
 * writePoseidonArtifacts() after each one to write circomlibjs's real ones
 * back. test/hardhat/PoseidonArtifacts.test.js checks them against
 * poseidonArtifacts(). To do it by hand, from the project root:
 *   node scripts/regen_poseidon.js
 */
const { poseidonContract: poseidon_gencontract } = require("circomlibjs");
const fs = require("fs");
const path = require("path");

const sourceName = "contracts/core/contracts/Poseidon.sol";
const outDir = path.join(__dirname, "..", "artifacts", sourceName);

// poseidonArtifact returns the Hardhat artifact of the Poseidon library over
// nInputs field elements, named contractName.
function poseidonArtifact(contractName, nInputs) {
  const bytecode = poseidon_gencontract.createCode(nInputs);
  return {
    _format: "hh-sol-artifact-1",
    contractName,
    sourceName,
    abi: poseidon_gencontract.generateABI(nInputs),
    bytecode,
    deployedBytecode: "0x" + bytecode.slice(2 + 24),
    linkReferences: {},
    deployedBytecodeImmutables: {},
  };
}

// poseidonArtifacts returns the real artifacts of Poseidon.sol's libraries,
// by contract name.
function poseidonArtifacts() {
  return {
    PoseidonT3: poseidonArtifact("PoseidonT3", 2),
    PoseidonT5: poseidonArtifact("PoseidonT5", 4),
  };
}

// writePoseidonArtifacts writes poseidonArtifacts() over the compiler's.
function writePoseidonArtifacts() {
  fs.mkdirSync(outDir, { recursive: true });
  for (const [name, artifact] of Object.entries(poseidonArtifacts())) {
    fs.writeFileSync(path.join(outDir, `${name}.json`), JSON.stringify(artifact, null, 2));
    console.log(`${name}.json: ${JSON.stringify(artifact).length} bytes`);
  }
}

module.exports = { poseidonArtifacts, writePoseidonArtifacts, POSEIDON_SOURCE: sourceName };

if (require.main === module) {
  writePoseidonArtifacts();
  console.log("done");
}
