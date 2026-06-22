// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.20;

// ============================================================
// WARNING — INTENTIONAL STUB IMPLEMENTATIONS
// ============================================================
//
// PoseidonT3 and PoseidonT5 below have EMPTY function bodies.
// They exist only so that Solidity can resolve the library
// symbols during compilation. The real EVM bytecode is injected
// at deploy time from the circomlibjs-generated artifacts:
//
//   artifacts/contracts/core/contracts/Poseidon.sol/PoseidonT3.json
//   artifacts/contracts/core/contracts/Poseidon.sol/PoseidonT5.json
//
// CRITICAL: Running `npx hardhat compile` will OVERWRITE those
// artifact files with the stub bytecode below, causing every
// on-chain Poseidon call to return 0. This silently breaks all
// Merkle root checks and ZK proof verification.
//
// After any `npx hardhat compile`, regenerate the real artifacts
// by running this command from the project root:
//
//   node scripts/regen_poseidon.js
//
// ============================================================

library PoseidonT3 {
    // solhint-disable-next-line no-empty-blocks
    function poseidon(uint256[2] memory input) public pure returns (uint256) {}
}

library PoseidonT5 {
    // solhint-disable-next-line no-empty-blocks
    function poseidon(uint256[4] memory input) public pure returns (uint256) {}
}
