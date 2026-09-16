// SPDX-License-Identifier: GPL3
pragma solidity ^0.8.24;

/// @notice Test-only stand-in for the real Groth16 USDr transfer (82-signal
///         — the fee amount and the Fix L-01 domain separator are both
///         public here, unlike the main transfer proof's 80) verifier.
///         Enygma.transfer() invokes the registered USDr verifier via
///         staticcall and only checks that the call did not revert — see
///         MockWithdrawVerifier.sol for the full rationale, identical here
///         just for the 82-signal USDr proof shape.
///
///         Used by go_client/enygma_test's *_repro_test.go files to test
///         contract-side logic unrelated to USDr (fingerprints, duplicate
///         participant ids, epoch rollover, etc.) now that transfer()
///         unconditionally requires a USDr leg too — those tests don't
///         need the USDr leg's proof to be cryptographically real, only
///         its public_signal to satisfy Enygma.sol's own on-chain checks
///         (_verifyUsdrMainBinding, _verifyPublicInputsUsdr,
///         _verifyBlockNumberUsdr).
contract MockUsdrVerifier {
    function verifyProof(
        uint256[8] calldata,
        uint256[82] calldata
    ) external pure returns (bool) {
        return true;
    }
}
