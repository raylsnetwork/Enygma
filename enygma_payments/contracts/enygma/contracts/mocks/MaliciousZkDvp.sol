// SPDX-License-Identifier: GPL3
pragma solidity ^0.8.24;

/// @notice Test-only stand-in for a ZkDvp bridge that attempts to reenter
///         Enygma mid-call (Fix L-12). depositThroughEnygma calls back into
///         the configured Enygma instance's withdraw() with whatever calldata
///         it was last armed with — if Enygma's nonReentrant guard is
///         working, that inner call reverts ReentrancyGuardReentrantCall(),
///         and the whole outer withdraw() reverts too (Solidity's atomic
///         revert semantics), which is exactly what
///         TestL12_WithdrawRejectsReentrantCall checks for.
///
///         Used by go_client/enygma_test/l02_l12_repro_test.go.
contract MaliciousZkDvp {
    address public enygma;
    bytes public reentryCalldata;

    function setTarget(address _enygma, bytes calldata _reentryCalldata) external {
        enygma = _enygma;
        reentryCalldata = _reentryCalldata;
    }

    function depositThroughEnygma(
        uint256[] memory
    ) external returns (bool, uint256) {
        // Fix L-12 repro: attempt to call back into Enygma.withdraw()
        // before the outer call has finished. A vulnerable (pre-fix)
        // Enygma would still be mid-way through its own withdraw() state
        // updates here; a fixed one has nonReentrant set and this call
        // reverts — re-thrown verbatim below so the test can assert on
        // the exact custom error (ReentrancyGuardReentrantCall), not a
        // generic wrapper message.
        (bool ok, bytes memory ret) = enygma.call(reentryCalldata);
        if (!ok) {
            assembly {
                revert(add(ret, 32), mload(ret))
            }
        }
        return (true, 0);
    }
}
