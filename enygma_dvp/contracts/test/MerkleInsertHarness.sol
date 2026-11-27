// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.0;

import {Merkle} from "../core/contracts/vaults/Merkle.sol";

/// @notice Test-only: a Merkle tree of any depth with its internal
///         insertLeaves exposed, one batch per call or two in one call, so
///         the rollover and the positions LeafInserted logs can be checked
///         without a vault.
contract MerkleInsertHarness is Merkle {
    constructor(uint256 depth, uint256 id, address poseidonWrapper) {
        initializeMerkle(depth, id, poseidonWrapper);
    }

    function insert(uint256[] memory leaves) external {
        insertLeaves(leaves);
    }

    function insertTwice(uint256[] memory first, uint256[] memory second) external {
        insertLeaves(first);
        insertLeaves(second);
    }
}
