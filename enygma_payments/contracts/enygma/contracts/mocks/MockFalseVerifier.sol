// SPDX-License-Identifier: GPL3
pragma solidity ^0.8.24;

/// @notice Test-only verifier that reports failure by RETURNING false instead
///         of reverting — the behaviour of enygma_dvp's GenericGroth16Verifier
///         and the opposite of the generated gnark verifiers this contract
///         ships with. Enygma must reject a proof from a verifier like this;
///         a bare "did the call revert" check would accept it.
///
///         Both proof shapes transfer() uses are provided (main: 81 signals,
///         USDr: 83) so the same instance can be registered for either.
contract MockFalseVerifier {
    function verifyProof(
        uint256[8] calldata,
        uint256[81] calldata
    ) external pure returns (bool) {
        return false;
    }

    function verifyProof(
        uint256[8] calldata,
        uint256[83] calldata
    ) external pure returns (bool) {
        return false;
    }
}
