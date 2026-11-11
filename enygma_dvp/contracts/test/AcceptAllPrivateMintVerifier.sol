// SPDX-License-Identifier: BUSL-1.1
pragma solidity ^0.8.0;

/// @notice Test-only: accepts every private mint proof, so EnygmaDvp's own
///         privateMint checks can be exercised without real proofs.
contract AcceptAllPrivateMintVerifier {
    function verifyProof(uint256[8] calldata, uint256[4] calldata) external pure {}
}
