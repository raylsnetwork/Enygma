import { createRequire } from 'module';
import { readFileSync, writeFileSync } from 'fs';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __dirname = dirname(fileURLToPath(import.meta.url));
const poseidon_gencontract = await import('./node_modules/circomlibjs/src/poseidon_gencontract.js').then(m => m.default ?? m);

const bc3 = poseidon_gencontract.createCode(2);
const bc5 = poseidon_gencontract.createCode(4);
const runtime3 = '0x' + bc3.slice(2 + 24);
const runtime5 = '0x' + bc5.slice(2 + 24);

const a3 = {
  _format: 'hh-sol-artifact-1',
  contractName: 'PoseidonT3',
  sourceName: 'contracts/core/contracts/Poseidon.sol',
  abi: poseidon_gencontract.generateABI(2),
  bytecode: bc3,
  deployedBytecode: runtime3,
  linkReferences: {},
  deployedBytecodeImmutables: {}
};

const a5 = {
  _format: 'hh-sol-artifact-1',
  contractName: 'PoseidonT5',
  sourceName: 'contracts/core/contracts/Poseidon.sol',
  abi: poseidon_gencontract.generateABI(4),
  bytecode: bc5,
  deployedBytecode: runtime5,
  linkReferences: {},
  deployedBytecodeImmutables: {}
};

writeFileSync(
  join(__dirname, 'artifacts/contracts/core/contracts/Poseidon.sol/PoseidonT3.json'),
  JSON.stringify(a3, null, 2)
);
writeFileSync(
  join(__dirname, 'artifacts/contracts/core/contracts/Poseidon.sol/PoseidonT5.json'),
  JSON.stringify(a5, null, 2)
);

console.log('done');
