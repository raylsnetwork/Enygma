// SPDX-License-Identifier: GPL3
pragma solidity ^0.8.24;

/// @notice Test-only stand-in for the real Groth16 fee-transfer
///         (TRANSFER_FEE_LEN = 85 signals) verifier. Enygma.transferWithFee() invokes the registered
///         verifier via staticcall and only checks that the call did not
///         revert — see MockTransferVerifier.sol for the full rationale,
///         identical here just for the fee proof shape.
///
///         Used by go_client/enygma_test/m13_repro_test.go and
///         fee_coverage_test.go to test transferWithFee's own checks (fee
///         accounting, participant count, fingerprints) in isolation from
///         the enygma_fee circuit itself, whose real proof they don't need.
contract MockFeeVerifier {
    function verifyProof(
        uint256[8] calldata,
        uint256[85] calldata
    ) external pure returns (bool) {
        return true;
    }
}
