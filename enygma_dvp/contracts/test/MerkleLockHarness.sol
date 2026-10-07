// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.0;

import {Merkle} from "../core/contracts/vaults/Merkle.sol";

/// @notice Test-only: exposes Merkle's internal nullifier lock/spend so the
///         lock rules can be checked without a full vault and DvP setup.
contract MerkleLockHarness is Merkle {
    function spend(uint256 treeNumber_, uint256 nullifier) external {
        setNullifier(treeNumber_, nullifier);
    }

    function lockNote(uint256 treeNumber_, uint256 nullifier) external {
        lock(treeNumber_, nullifier);
    }
}
