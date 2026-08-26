// SPDX-License-Identifier: GPL3
pragma solidity ^0.8.24;

/// @notice Test-only stand-in for a real Groth16 withdraw verifier.
/// @dev Enygma.withdraw() invokes the registered verifier via
///      `staticcall(abi.encodeWithSignature("verifyProof(uint256[8],uint256[52])", proof))`
///      (Fix M-01: delegatecall -> staticcall) and only checks that the call
///      did not revert — the return value of verifyProof itself is never
///      inspected. So a mock only needs to expose a same-signature function
///      that doesn't revert.
///
///      Fix M-14/C-09/L-01: arity is [52], not [50] — the withdraw circuit
///      produces 52 signals once slot 50 (TotalDepositValue, Fix C-09) and
///      slot 51 (DomainId, Fix L-01) were added; see Enygma.sol's
///      WITHDRAW_TOTAL_DEPOSIT_VALUE_OFFSET doc comment.
///
///      Used by go_client/enygma_test/h14_repro_test.go, h15_repro_test.go,
///      and m15_c09_repro_test.go to test Enygma's own state-handling logic
///      in isolation from the withdraw circuit, whose prover route is
///      broken independently (M-03) and cannot produce a real proof today.
contract MockWithdrawVerifier {
    function verifyProof(
        uint256[8] calldata,
        uint256[52] calldata
    ) external pure returns (bool) {
        return true;
    }
}
