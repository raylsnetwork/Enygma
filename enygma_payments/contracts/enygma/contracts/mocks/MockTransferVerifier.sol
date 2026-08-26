// SPDX-License-Identifier: GPL3
pragma solidity ^0.8.24;

/// @notice Test-only stand-in for the real Groth16 transfer (FingerPrint,
///         81-signal, the 81st being the Fix L-01 domain separator)
///         verifier. Enygma.transfer() invokes the registered
///         verifier via delegatecall and only checks that the call did not
///         revert — see MockWithdrawVerifier.sol for the full rationale,
///         identical here just for the 81-signal transfer proof shape.
///
///         Used by go_client/enygma_test/c04_repro_test.go to test the new
///         fingerprint-registry logic (C-04) in isolation from the
///         transfer circuit itself, whose real proof this test doesn't
///         need: _verifyFingerprints only inspects public_signal content,
///         not proof validity.
contract MockTransferVerifier {
    function verifyProof(
        uint256[8] calldata,
        uint256[81] calldata
    ) external pure returns (bool) {
        return true;
    }
}
