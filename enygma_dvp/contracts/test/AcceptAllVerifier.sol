// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.0;

import {IEnygmaDvp} from "../core/interfaces/IEnygmaDvp.sol";

/// @notice Test-only: accepts every proof, so a vault's own checks (roots,
///         nullifiers, locks, openings) can be exercised without real proofs.
contract AcceptAllVerifier {
    function verifyProof(
        uint256,
        IEnygmaDvp.SnarkProof memory,
        uint256[] memory
    ) external pure returns (bool) {
        return true;
    }
}
