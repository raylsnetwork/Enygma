// SPDX-License-Identifier: GPL3
pragma solidity ^0.8.24;

/// @notice Test-only stand-in for the real Groth16 fee-transfer (55-signal, the
///         55th being the Fix L-01 domain separator)
///         verifier. Enygma.transferWithFee() invokes the registered
///         verifier via staticcall and only checks that the call did not
///         revert — see MockTransferVerifier.sol for the full rationale,
///         identical here just for the 55-signal fee proof shape.
///
///         Used by go_client/enygma_test/m13_repro_test.go to test the fee
///         accounting logic (M-13) in isolation from the enygma_fee
///         circuit itself, whose real proof this test doesn't need:
///         transferWithFee's fee-burn logic only inspects
///         public_signal[FEE_OFFSET], not proof validity.
contract MockFeeVerifier {
    function verifyProof(
        uint256[8] calldata,
        uint256[55] calldata
    ) external pure returns (bool) {
        return true;
    }
}
