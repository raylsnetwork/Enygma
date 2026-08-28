// SPDX-License-Identifier: GPL3
pragma solidity ^0.8.24;

/// @notice Test-only stand-in for a real Groth16 burn verifier (Fix H-13).
/// @dev Enygma.burn() invokes the registered verifier via
///      `delegatecall(abi.encodeWithSignature("verifyProof(uint256[8],uint256[9])", proof))`
///      and only checks that the call did not revert — same convention as
///      MockWithdrawVerifier/MockTransferVerifier. Lets tests exercise
///      Enygma's own burn() logic (public-signal binding, nullifier
///      consumption, supply accounting, invariant enforcement) with a
///      fabricated but well-formed public_signal array, independent of the
///      real burn circuit's proving pipeline (which needs a live gnark
///      server and the batched trusted-setup ceremony — H-12 — neither of
///      which this branch's other circuit-touching fixes have run either).
contract MockBurnVerifier {
    function verifyProof(
        uint256[8] calldata,
        uint256[9] calldata
    ) external pure returns (bool) {
        return true;
    }
}
