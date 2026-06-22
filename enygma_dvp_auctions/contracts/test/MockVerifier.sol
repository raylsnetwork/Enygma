// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.20;

// Mirrors the G1Point/G2Point/SnarkProof tuple layout declared locally in
// EnygmaAuction.sol so the ABI encoding matches exactly — Solidity resolves
// external-call ABI encoding structurally (by tuple shape), not by the
// Solidity-level type name, so this is safe despite living in a separate file.
struct G1Point { uint256 x; uint256 y; }
struct G2Point { uint256[2] x; uint256[2] y; }
struct SnarkProof { G1Point a; G2Point b; G1Point c; }

/// @title MockVerifier
/// @notice Test double for IVerifier. Ignores the actual proof/inputs and
/// returns whatever `result` was last set via setResult() — lets tests drive
/// EnygmaAuction's state-machine logic deterministically, independent of
/// real ZK proof generation/verification (which is covered separately by the
/// gnark circuit test suite).
contract MockVerifier {
    bool public result = true;

    function setResult(bool r) external {
        result = r;
    }

    function verifyProof(
        uint256 /*vkId*/,
        SnarkProof calldata /*proof*/,
        uint256[] calldata /*inputs*/
    ) external view returns (bool) {
        return result;
    }
}
