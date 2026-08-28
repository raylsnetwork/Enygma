// SPDX-License-Identifier: GPL3
pragma solidity ^0.8.24;

/// @notice Test-only stand-in for a real Groth16 deposit verifier.
/// @dev Enygma.deposit() invokes the registered verifier via
///      `staticcall(abi.encodeWithSignature("verifyProof(uint256[8],uint256[52])", proof))`
///      and only checks that the call did not revert — see
///      MockWithdrawVerifier.sol for the full rationale, identical here
///      just for the deposit circuit's own 52-signal shape (Fix M-14/L-01: the
///      real circuit produces 52 signals — HashedSharedSecrets through
///      Nullifier at 0-49, Hash at 50, DomainId at 51 (Fix L-01) — the
///      original bug was Enygma.sol declaring [50], not the circuit's own
///      arity).
///
///      Used by go_client/enygma_test/m14_m15_c09_repro_test.go to test
///      deposit()'s own state-handling and supply-accounting logic in
///      isolation from the deposit circuit, whose prover route is broken
///      independently (M-03) and cannot produce a real proof today.
contract MockDepositVerifier {
    function verifyProof(
        uint256[8] calldata,
        uint256[52] calldata
    ) external pure returns (bool) {
        return true;
    }
}
