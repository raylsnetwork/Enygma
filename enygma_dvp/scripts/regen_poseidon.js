// Poseidon.sol is a placeholder: every compile writes its stub bytecode over
// the PoseidonT3/T5 artifacts, and hardhat.config.js's compile task calls
// writePoseidonArtifacts() after each one to write circomlibjs's real ones
// back. Run as a script (or as ../regen_poseidon.mjs) it does the same by hand.
// test/hardhat/PoseidonArtifacts.test.js checks the artifacts against
// poseidonArtifacts().
const { poseidonContract: poseidon_gencontract } = require('circomlibjs');
const fs = require('fs');
const path = require('path');

const src = 'contracts/core/contracts/Poseidon.sol';

// poseidonArtifact returns the Hardhat artifact of the Poseidon library over
// nInputs field elements, named contractName.
function poseidonArtifact(contractName, nInputs) {
  const bytecode = poseidon_gencontract.createCode(nInputs);
  return {
    _format: 'hh-sol-artifact-1',
    contractName,
    sourceName: src,
    abi: poseidon_gencontract.generateABI(nInputs),
    bytecode,
    deployedBytecode: '0x' + bytecode.slice(26),
    linkReferences: {},
    deployedBytecodeImmutables: {},
  };
}

// poseidonArtifacts returns the real artifacts of Poseidon.sol's libraries,
// by contract name.
function poseidonArtifacts() {
  return {
    PoseidonT3: poseidonArtifact('PoseidonT3', 2),
    PoseidonT5: poseidonArtifact('PoseidonT5', 4),
  };
}

// writePoseidonArtifacts writes poseidonArtifacts() over the compiler's.
function writePoseidonArtifacts() {
  const dir = path.join(__dirname, '..', 'artifacts', src);
  for (const [name, artifact] of Object.entries(poseidonArtifacts())) {
    fs.writeFileSync(path.join(dir, `${name}.json`), JSON.stringify(artifact, null, 2));
  }
  console.log('Poseidon artifacts regenerated');
}

module.exports = { poseidonArtifacts, writePoseidonArtifacts, POSEIDON_SOURCE: src };

if (require.main === module) {
  writePoseidonArtifacts();
}
