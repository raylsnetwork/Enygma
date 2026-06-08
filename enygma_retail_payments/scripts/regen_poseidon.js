#!/usr/bin/env node
// Regenerates PoseidonT3 and PoseidonT5 Hardhat artifacts from circomlibjs.
//
// Run from the enygma_retail_payments/ root:
//   node scripts/regen_poseidon.js
//
// Must be re-run after every `npx hardhat compile` because the placeholder
// Poseidon.sol overwrites the artifacts with stubs that return 0.

// circomlibjs lives in enygma_dvp's node_modules (that's where Hardhat is installed).
const circomlibsPath = require.resolve('circomlibjs', {
  paths: [require('path').join(__dirname, '..', '..', 'enygma_dvp')],
});
const { poseidonContract: poseidon_gencontract } = require(circomlibsPath);
const fs = require('fs');
const path = require('path');

const outDir = path.join(__dirname, '..', 'artifacts', 'contracts', 'core', 'contracts', 'Poseidon.sol');
fs.mkdirSync(outDir, { recursive: true });

function makeArtifact(arity) {
  const bc = poseidon_gencontract.createCode(arity);
  const runtime = '0x' + bc.slice(2 + 24);
  const name = arity === 2 ? 'PoseidonT3' : 'PoseidonT5';
  return {
    _format: 'hh-sol-artifact-1',
    contractName: name,
    sourceName: 'contracts/core/contracts/Poseidon.sol',
    abi: poseidon_gencontract.generateABI(arity),
    bytecode: bc,
    deployedBytecode: runtime,
    linkReferences: {},
    deployedBytecodeImmutables: {},
  };
}

const t3 = makeArtifact(2);
const t5 = makeArtifact(4);

fs.writeFileSync(path.join(outDir, 'PoseidonT3.json'), JSON.stringify(t3, null, 2));
fs.writeFileSync(path.join(outDir, 'PoseidonT5.json'), JSON.stringify(t5, null, 2));

console.log('PoseidonT3:', path.join(outDir, 'PoseidonT3.json'));
console.log('PoseidonT5:', path.join(outDir, 'PoseidonT5.json'));
console.log('done');
