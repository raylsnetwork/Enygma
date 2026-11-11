// SPDX-License-Identifier: GPL3
pragma solidity ^0.8.24;

/// @notice Test-only stand-in for a real Groth16 deposit verifier.
/// @dev Enygma.deposit() invokes the registered verifier via
///      `staticcall(abi.encodeWithSignature("verifyProof(uint256[8],uint256[82])", proof))`
///      and only checks that the call did not revert — see
///      MockWithdrawVerifier.sol for the full rationale, identical here
///      just for the deposit circuit's own 82-signal shape: the 6×6
///      shared-secret fingerprint matrix through Nullifier at 0-79, Hash at
///      80, DomainId at 81 (see StatementLayout.DEPOSIT_*).
///
///      Used by go_client/enygma_test/m14_m15_c09_repro_test.go to test
///      deposit()'s own state-handling and supply-accounting logic in
///      isolation from the deposit circuit, whose prover route is broken
///      independently (M-03) and cannot produce a real proof today.
contract MockDepositVerifier {
    function verifyProof(
        uint256[8] calldata,
        uint256[82] calldata
    ) external pure returns (bool) {
        return true;
    }
}
