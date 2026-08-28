// SPDX-License-Identifier: GPL3
pragma solidity ^0.8.24;

import "../../interfaces/IZkDvp.sol";

/// @notice Minimal test-only stand-in for the ZkDvp bridge contract.
/// @dev Implements depositThroughEnygma (called by Enygma.withdraw() ->
///      _executeZkDvpDeposits) and withdrawThroughEnygma (called by
///      Enygma.deposit()). Tracks call counts / totals so a test can
///      assert on them directly, instead of inferring success from
///      Enygma-side state that H-14 is precisely about not trusting.
///
///      Used by go_client/enygma_test/h14_repro_test.go and
///      m14_m15_c09_repro_test.go.
contract MockZkDvp {
    uint256 public totalMinted;
    uint256 public callCount;
    uint256 public withdrawCallCount;

    function depositThroughEnygma(
        uint256[] memory depositParams
    ) external returns (bool, uint256) {
        totalMinted += depositParams[0]; // amount
        callCount += 1;
        return (true, uint256(keccak256(abi.encode(depositParams, callCount))));
    }

    function withdrawThroughEnygma(
        IZkDvp.JoinSplitTransaction memory
    ) external returns (bool) {
        withdrawCallCount += 1;
        return true;
    }
}
