// Regenerates real PoseidonT3/T5 artifacts from circomlibjs.
// Must be run after `npx hardhat compile` which overwrites them with placeholders.
import { poseidonContract } from 'circomlibjs';
import { writeFileSync } from 'fs';

const bc3 = poseidonContract.createCode(2);
const bc5 = poseidonContract.createCode(4);
const runtime3 = '0x' + bc3.slice(2 + 24);
const runtime5 = '0x' + bc5.slice(2 + 24);

const a3 = {
  _format: 'hh-sol-artifact-1',
  contractName: 'PoseidonT3',
  sourceName: 'contracts/core/contracts/Poseidon.sol',
  abi: poseidonContract.generateABI(2),
  bytecode: bc3,
  deployedBytecode: runtime3,
  linkReferences: {},
  deployedBytecodeImmutables: {},
};
const a5 = {
  _format: 'hh-sol-artifact-1',
  contractName: 'PoseidonT5',
  sourceName: 'contracts/core/contracts/Poseidon.sol',
  abi: poseidonContract.generateABI(4),
  bytecode: bc5,
  deployedBytecode: runtime5,
  linkReferences: {},
  deployedBytecodeImmutables: {},
};

writeFileSync('artifacts/contracts/core/contracts/Poseidon.sol/PoseidonT3.json', JSON.stringify(a3, null, 2));
writeFileSync('artifacts/contracts/core/contracts/Poseidon.sol/PoseidonT5.json', JSON.stringify(a5, null, 2));
console.log('Poseidon artifacts regenerated.');
